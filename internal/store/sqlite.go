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

	_ "github.com/mattn/go-sqlite3"
	"k8s.io/apimachinery/pkg/types"
)

// SQLiteStore implements Store using SQLite
type SQLiteStore struct {
	db   *sql.DB
	path string
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(path string) *SQLiteStore {
	return &SQLiteStore{path: path}
}

// Init initializes the SQLite database
func (s *SQLiteStore) Init() error {
	db, err := sql.Open("sqlite3", s.path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.db = db

	// Create schema
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS executions (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			cronjob_ns      TEXT NOT NULL,
			cronjob_name    TEXT NOT NULL,
			job_name        TEXT NOT NULL,
			scheduled_time  TEXT,
			start_time      TEXT NOT NULL,
			completion_time TEXT,
			duration_secs   REAL,
			succeeded       INTEGER NOT NULL,
			exit_code       INTEGER,
			reason          TEXT,
			is_retry        INTEGER DEFAULT 0,
			retry_of        TEXT,
			created_at      TEXT DEFAULT CURRENT_TIMESTAMP
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
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// RecordExecution stores a new execution record
func (s *SQLiteStore) RecordExecution(ctx context.Context, exec Execution) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO executions (
			cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			completion_time, duration_secs, succeeded, exit_code, reason,
			is_retry, retry_of
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		exec.CronJobNamespace,
		exec.CronJobName,
		exec.JobName,
		formatTime(exec.ScheduledTime),
		exec.StartTime.Format(time.RFC3339),
		exec.CompletionTime.Format(time.RFC3339),
		exec.Duration.Seconds(),
		boolToInt(exec.Succeeded),
		exec.ExitCode,
		exec.Reason,
		boolToInt(exec.IsRetry),
		exec.RetryOf,
	)
	return err
}

// GetExecutions returns executions for a CronJob since a given time
func (s *SQLiteStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
		ORDER BY start_time DESC
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanExecutions(rows)
}

// GetLastExecution returns the most recent execution
func (s *SQLiteStore) GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ?
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name)

	return s.scanExecution(row)
}

// GetLastSuccessfulExecution returns the most recent successful execution
func (s *SQLiteStore) GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND succeeded = 1
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name)

	return s.scanExecution(row)
}

// GetMetrics calculates SLA metrics for a CronJob
func (s *SQLiteStore) GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*Metrics, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	// Get counts
	var total, succeeded, failed int32
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN succeeded = 1 THEN 1 ELSE 0 END), 0) as succeeded,
			COALESCE(SUM(CASE WHEN succeeded = 0 THEN 1 ELSE 0 END), 0) as failed
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339))

	if err := row.Scan(&total, &succeeded, &failed); err != nil {
		return nil, err
	}

	// Get durations for percentile calculation
	rows, err := s.db.QueryContext(ctx, `
		SELECT duration_secs
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL
		ORDER BY duration_secs
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
		metrics.P50DurationSeconds = percentile(durations, 50)
		metrics.P95DurationSeconds = percentile(durations, 95)
		metrics.P99DurationSeconds = percentile(durations, 99)
	}

	return metrics, nil
}

// GetDurationPercentile calculates a duration percentile
func (s *SQLiteStore) GetDurationPercentile(ctx context.Context, cronJob types.NamespacedName, p int, windowDays int) (time.Duration, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	rows, err := s.db.QueryContext(ctx, `
		SELECT duration_secs
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL
		ORDER BY duration_secs
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

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

	return time.Duration(percentile(durations, p) * float64(time.Second)), nil
}

// GetSuccessRate calculates success rate
func (s *SQLiteStore) GetSuccessRate(ctx context.Context, cronJob types.NamespacedName, windowDays int) (float64, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	var total, succeeded int
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN succeeded = 1 THEN 1 ELSE 0 END), 0) as succeeded
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339))

	if err := row.Scan(&total, &succeeded); err != nil {
		return 0, err
	}

	if total == 0 {
		return 100, nil // No data = assume healthy
	}

	return float64(succeeded) / float64(total) * 100, nil
}

// Prune removes old execution records
func (s *SQLiteStore) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM executions WHERE start_time < ?
	`, olderThan.Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Health checks if the store is healthy
func (s *SQLiteStore) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// scanExecutions scans multiple execution rows
func (s *SQLiteStore) scanExecutions(rows *sql.Rows) ([]Execution, error) {
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
func (s *SQLiteStore) scanExecution(row *sql.Row) (*Execution, error) {
	var exec Execution
	var scheduledTime, startTime, completionTime, createdAt sql.NullString
	var durationSecs sql.NullFloat64
	var succeeded, isRetry int
	var exitCode sql.NullInt32
	var reason, retryOf sql.NullString

	err := row.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&exec.JobName,
		&scheduledTime,
		&startTime,
		&completionTime,
		&durationSecs,
		&succeeded,
		&exitCode,
		&reason,
		&isRetry,
		&retryOf,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if scheduledTime.Valid {
		t, _ := time.Parse(time.RFC3339, scheduledTime.String)
		exec.ScheduledTime = &t
	}
	if startTime.Valid {
		exec.StartTime, _ = time.Parse(time.RFC3339, startTime.String)
	}
	if completionTime.Valid {
		exec.CompletionTime, _ = time.Parse(time.RFC3339, completionTime.String)
	}
	if durationSecs.Valid {
		exec.Duration = time.Duration(durationSecs.Float64 * float64(time.Second))
	}
	exec.Succeeded = succeeded == 1
	if exitCode.Valid {
		exec.ExitCode = exitCode.Int32
	}
	if reason.Valid {
		exec.Reason = reason.String
	}
	exec.IsRetry = isRetry == 1
	if retryOf.Valid {
		exec.RetryOf = retryOf.String
	}
	if createdAt.Valid {
		exec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}

	return &exec, nil
}

// scanExecutionRow scans a single row from a Rows result
func (s *SQLiteStore) scanExecutionRow(rows *sql.Rows) (*Execution, error) {
	var exec Execution
	var scheduledTime, startTime, completionTime, createdAt sql.NullString
	var durationSecs sql.NullFloat64
	var succeeded, isRetry int
	var exitCode sql.NullInt32
	var reason, retryOf sql.NullString

	err := rows.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&exec.JobName,
		&scheduledTime,
		&startTime,
		&completionTime,
		&durationSecs,
		&succeeded,
		&exitCode,
		&reason,
		&isRetry,
		&retryOf,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if scheduledTime.Valid {
		t, _ := time.Parse(time.RFC3339, scheduledTime.String)
		exec.ScheduledTime = &t
	}
	if startTime.Valid {
		exec.StartTime, _ = time.Parse(time.RFC3339, startTime.String)
	}
	if completionTime.Valid {
		exec.CompletionTime, _ = time.Parse(time.RFC3339, completionTime.String)
	}
	if durationSecs.Valid {
		exec.Duration = time.Duration(durationSecs.Float64 * float64(time.Second))
	}
	exec.Succeeded = succeeded == 1
	if exitCode.Valid {
		exec.ExitCode = exitCode.Int32
	}
	if reason.Valid {
		exec.Reason = reason.String
	}
	exec.IsRetry = isRetry == 1
	if retryOf.Valid {
		exec.RetryOf = retryOf.String
	}
	if createdAt.Valid {
		exec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}

	return &exec, nil
}

// Helper functions

func percentile(data []float64, p int) float64 {
	if len(data) == 0 {
		return 0
	}
	sort.Float64s(data)
	idx := int(float64(len(data)-1) * float64(p) / 100)
	return data[idx]
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
