/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"k8s.io/apimachinery/pkg/types"
)

// MySQLStore implements Store using MySQL/MariaDB
type MySQLStore struct {
	db       *sql.DB
	host     string
	port     int32
	database string
	user     string
	password string
}

// NewMySQLStore creates a new MySQL store
func NewMySQLStore(host string, port int32, database, user, password string) *MySQLStore {
	return &MySQLStore{
		host:     host,
		port:     port,
		database: database,
		user:     user,
		password: password,
	}
}

// Init initializes the MySQL database
func (s *MySQLStore) Init() error {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		s.user, s.password, s.host, s.port, s.database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to mysql: %w", err)
	}
	s.db = db

	// Create schema
	_, err = s.db.Exec(
		`
		CREATE TABLE IF NOT EXISTS executions (
			id              BIGINT AUTO_INCREMENT PRIMARY KEY,
			cronjob_ns      VARCHAR(253) NOT NULL,
			cronjob_name    VARCHAR(253) NOT NULL,
			job_name        VARCHAR(253) NOT NULL,
			scheduled_time  DATETIME,
			start_time      DATETIME NOT NULL,
			completion_time DATETIME,
			duration_secs   DOUBLE,
			succeeded       TINYINT(1) NOT NULL,
			exit_code       INT,
			reason          VARCHAR(255),
			is_retry        TINYINT(1) DEFAULT 0,
			retry_of        VARCHAR(253),
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,

			INDEX idx_cronjob_time (cronjob_ns, cronjob_name, start_time DESC),
			INDEX idx_start_time (start_time),
			INDEX idx_job_name (job_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`,
	)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *MySQLStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// RecordExecution stores a new execution record
func (s *MySQLStore) RecordExecution(ctx context.Context, exec Execution) error {
	_, err := s.db.ExecContext(
		ctx, `
		INSERT INTO executions (
			cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			completion_time, duration_secs, succeeded, exit_code, reason,
			is_retry, retry_of
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		exec.CronJobNamespace,
		exec.CronJobName,
		exec.JobName,
		exec.ScheduledTime,
		exec.StartTime,
		exec.CompletionTime,
		exec.Duration.Seconds(),
		exec.Succeeded,
		exec.ExitCode,
		exec.Reason,
		exec.IsRetry,
		exec.RetryOf,
	)
	return err
}

// GetExecutions returns executions for a CronJob since a given time
func (s *MySQLStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	rows, err := s.db.QueryContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
		ORDER BY start_time DESC
	`, cronJob.Namespace, cronJob.Name, since,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	return s.scanExecutions(rows)
}

// GetLastExecution returns the most recent execution
func (s *MySQLStore) GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ?
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name,
	)

	return s.scanExecution(row)
}

// GetLastSuccessfulExecution returns the most recent successful execution
func (s *MySQLStore) GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND succeeded = 1
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name,
	)

	return s.scanExecution(row)
}

// GetMetrics calculates SLA metrics for a CronJob
// MySQL doesn't have native percentile functions, so we compute in Go
func (s *MySQLStore) GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*Metrics, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	// Get counts
	var total, succeeded, failed int32
	row := s.db.QueryRowContext(
		ctx, `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN succeeded = 1 THEN 1 ELSE 0 END), 0) as succeeded,
			COALESCE(SUM(CASE WHEN succeeded = 0 THEN 1 ELSE 0 END), 0) as failed
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
	`, cronJob.Namespace, cronJob.Name, since,
	)

	if err := row.Scan(&total, &succeeded, &failed); err != nil {
		return nil, err
	}

	// Get durations for percentile calculation
	rows, err := s.db.QueryContext(
		ctx, `
		SELECT duration_secs
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL
		ORDER BY duration_secs
	`, cronJob.Namespace, cronJob.Name, since,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var durations []float64
	var sum float64
	for rows.Next() {
		var d float64
		if err := rows.Scan(&d); err != nil {
			continue
		}
		durations = append(durations, d)
		sum += d
	}

	metrics := &Metrics{
		WindowDays:     int32(windowDays),
		TotalRuns:      total,
		SuccessfulRuns: succeeded,
		FailedRuns:     failed,
	}

	if total > 0 {
		metrics.SuccessRate = float64(succeeded) / float64(total) * 100
	}

	if len(durations) > 0 {
		metrics.AvgDurationSeconds = sum / float64(len(durations))
		metrics.P50DurationSeconds = percentileMySQL(durations, 50)
		metrics.P95DurationSeconds = percentileMySQL(durations, 95)
		metrics.P99DurationSeconds = percentileMySQL(durations, 99)
	}

	return metrics, nil
}

// GetDurationPercentile calculates a duration percentile
func (s *MySQLStore) GetDurationPercentile(ctx context.Context, cronJob types.NamespacedName, p int, windowDays int) (time.Duration, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	rows, err := s.db.QueryContext(
		ctx, `
		SELECT duration_secs
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL
		ORDER BY duration_secs
	`, cronJob.Namespace, cronJob.Name, since,
	)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var durations []float64
	for rows.Next() {
		var d float64
		if err := rows.Scan(&d); err != nil {
			continue
		}
		durations = append(durations, d)
	}

	if len(durations) == 0 {
		return 0, nil
	}

	return time.Duration(percentileMySQL(durations, p) * float64(time.Second)), nil
}

// GetSuccessRate calculates success rate
func (s *MySQLStore) GetSuccessRate(ctx context.Context, cronJob types.NamespacedName, windowDays int) (float64, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	var total, succeeded int
	row := s.db.QueryRowContext(
		ctx, `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN succeeded = 1 THEN 1 ELSE 0 END), 0) as succeeded
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
	`, cronJob.Namespace, cronJob.Name, since,
	)

	if err := row.Scan(&total, &succeeded); err != nil {
		return 0, err
	}

	if total == 0 {
		return 100, nil // No data = assume healthy
	}

	return float64(succeeded) / float64(total) * 100, nil
}

// Prune removes old execution records
func (s *MySQLStore) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(
		ctx, `
		DELETE FROM executions WHERE start_time < ?
	`, olderThan,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Health checks if the store is healthy
func (s *MySQLStore) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// scanExecutions scans multiple execution rows
func (s *MySQLStore) scanExecutions(rows *sql.Rows) ([]Execution, error) {
	var executions []Execution
	for rows.Next() {
		exec, err := s.scanExecutionRow(rows)
		if err != nil {
			return nil, err
		}
		executions = append(executions, *exec)
	}
	return executions, rows.Err()
}

// scanExecution scans a single execution row
func (s *MySQLStore) scanExecution(row *sql.Row) (*Execution, error) {
	var exec Execution
	var scheduledTime, completionTime sql.NullTime
	var durationSecs sql.NullFloat64
	var exitCode sql.NullInt32
	var reason, retryOf sql.NullString

	err := row.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&exec.JobName,
		&scheduledTime,
		&exec.StartTime,
		&completionTime,
		&durationSecs,
		&exec.Succeeded,
		&exitCode,
		&reason,
		&exec.IsRetry,
		&retryOf,
		&exec.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if scheduledTime.Valid {
		exec.ScheduledTime = &scheduledTime.Time
	}
	if completionTime.Valid {
		exec.CompletionTime = completionTime.Time
	}
	if durationSecs.Valid {
		exec.Duration = time.Duration(durationSecs.Float64 * float64(time.Second))
	}
	if exitCode.Valid {
		exec.ExitCode = exitCode.Int32
	}
	if reason.Valid {
		exec.Reason = reason.String
	}
	if retryOf.Valid {
		exec.RetryOf = retryOf.String
	}

	return &exec, nil
}

// scanExecutionRow scans a single row from a Rows result
func (s *MySQLStore) scanExecutionRow(rows *sql.Rows) (*Execution, error) {
	var exec Execution
	var scheduledTime, completionTime sql.NullTime
	var durationSecs sql.NullFloat64
	var exitCode sql.NullInt32
	var reason, retryOf sql.NullString

	err := rows.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&exec.JobName,
		&scheduledTime,
		&exec.StartTime,
		&completionTime,
		&durationSecs,
		&exec.Succeeded,
		&exitCode,
		&reason,
		&exec.IsRetry,
		&retryOf,
		&exec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if scheduledTime.Valid {
		exec.ScheduledTime = &scheduledTime.Time
	}
	if completionTime.Valid {
		exec.CompletionTime = completionTime.Time
	}
	if durationSecs.Valid {
		exec.Duration = time.Duration(durationSecs.Float64 * float64(time.Second))
	}
	if exitCode.Valid {
		exec.ExitCode = exitCode.Int32
	}
	if reason.Valid {
		exec.Reason = reason.String
	}
	if retryOf.Valid {
		exec.RetryOf = retryOf.String
	}

	return &exec, nil
}

// percentileMySQL calculates percentile in Go since MySQL doesn't have native support
func percentileMySQL(data []float64, p int) float64 {
	if len(data) == 0 {
		return 0
	}
	sort.Float64s(data)
	idx := int(float64(len(data)-1) * float64(p) / 100)
	return data[idx]
}
