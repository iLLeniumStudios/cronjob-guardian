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
	"time"

	_ "github.com/lib/pq"
	"k8s.io/apimachinery/pkg/types"
)

// PostgresStore implements Store using PostgreSQL
type PostgresStore struct {
	db       *sql.DB
	host     string
	port     int32
	database string
	user     string
	password string
	sslMode  string
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(host string, port int32, database, user, password, sslMode string) *PostgresStore {
	if sslMode == "" {
		sslMode = "require"
	}
	return &PostgresStore{
		host:     host,
		port:     port,
		database: database,
		user:     user,
		password: password,
		sslMode:  sslMode,
	}
}

// Init initializes the PostgreSQL database
func (s *PostgresStore) Init() error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		s.host, s.port, s.user, s.password, s.database, s.sslMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	s.db = db

	// Create schema
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS executions (
			id              BIGSERIAL PRIMARY KEY,
			cronjob_ns      VARCHAR(253) NOT NULL,
			cronjob_name    VARCHAR(253) NOT NULL,
			job_name        VARCHAR(253) NOT NULL,
			scheduled_time  TIMESTAMP,
			start_time      TIMESTAMP NOT NULL,
			completion_time TIMESTAMP,
			duration_secs   DOUBLE PRECISION,
			succeeded       BOOLEAN NOT NULL,
			exit_code       INTEGER,
			reason          VARCHAR(255),
			is_retry        BOOLEAN DEFAULT FALSE,
			retry_of        VARCHAR(253),
			created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_cronjob_time
			ON executions(cronjob_ns, cronjob_name, start_time DESC);
		CREATE INDEX IF NOT EXISTS idx_start_time ON executions(start_time);
		CREATE INDEX IF NOT EXISTS idx_job_name ON executions(job_name);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// RecordExecution stores a new execution record
func (s *PostgresStore) RecordExecution(ctx context.Context, exec Execution) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO executions (
			cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			completion_time, duration_secs, succeeded, exit_code, reason,
			is_retry, retry_of
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
func (s *PostgresStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND start_time >= $3
		ORDER BY start_time DESC
	`, cronJob.Namespace, cronJob.Name, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExecutions(rows)
}

// GetLastExecution returns the most recent execution
func (s *PostgresStore) GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name)

	return s.scanExecution(row)
}

// GetLastSuccessfulExecution returns the most recent successful execution
func (s *PostgresStore) GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND succeeded = true
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name)

	return s.scanExecution(row)
}

// GetMetrics calculates SLA metrics for a CronJob using PostgreSQL's native percentile functions
func (s *PostgresStore) GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*Metrics, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE succeeded = true) as succeeded,
			COUNT(*) FILTER (WHERE succeeded = false) as failed,
			AVG(duration_secs) as avg_duration,
			PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_secs) as p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_secs) as p95,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_secs) as p99
		FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND start_time >= $3
	`, cronJob.Namespace, cronJob.Name, since)

	var total, succeeded, failed int32
	var avgDuration, p50, p95, p99 sql.NullFloat64

	if err := row.Scan(&total, &succeeded, &failed, &avgDuration, &p50, &p95, &p99); err != nil {
		return nil, err
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

	if avgDuration.Valid {
		metrics.AvgDurationSeconds = avgDuration.Float64
	}
	if p50.Valid {
		metrics.P50DurationSeconds = p50.Float64
	}
	if p95.Valid {
		metrics.P95DurationSeconds = p95.Float64
	}
	if p99.Valid {
		metrics.P99DurationSeconds = p99.Float64
	}

	return metrics, nil
}

// GetDurationPercentile calculates a duration percentile
func (s *PostgresStore) GetDurationPercentile(ctx context.Context, cronJob types.NamespacedName, p int, windowDays int) (time.Duration, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	var result sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT PERCENTILE_CONT($1) WITHIN GROUP (ORDER BY duration_secs)
		FROM executions
		WHERE cronjob_ns = $2 AND cronjob_name = $3 AND start_time >= $4 AND duration_secs IS NOT NULL
	`, float64(p)/100, cronJob.Namespace, cronJob.Name, since).Scan(&result)

	if err != nil {
		return 0, err
	}

	if !result.Valid {
		return 0, nil
	}

	return time.Duration(result.Float64 * float64(time.Second)), nil
}

// GetSuccessRate calculates success rate
func (s *PostgresStore) GetSuccessRate(ctx context.Context, cronJob types.NamespacedName, windowDays int) (float64, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	var total, succeeded int
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE succeeded = true) as succeeded
		FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND start_time >= $3
	`, cronJob.Namespace, cronJob.Name, since)

	if err := row.Scan(&total, &succeeded); err != nil {
		return 0, err
	}

	if total == 0 {
		return 100, nil // No data = assume healthy
	}

	return float64(succeeded) / float64(total) * 100, nil
}

// Prune removes old execution records
func (s *PostgresStore) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM executions WHERE start_time < $1
	`, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Health checks if the store is healthy
func (s *PostgresStore) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// scanExecutions scans multiple execution rows
func (s *PostgresStore) scanExecutions(rows *sql.Rows) ([]Execution, error) {
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
func (s *PostgresStore) scanExecution(row *sql.Row) (*Execution, error) {
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
func (s *PostgresStore) scanExecutionRow(rows *sql.Rows) (*Execution, error) {
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
