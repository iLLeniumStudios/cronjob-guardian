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
	"strings"
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
	_, err = s.db.Exec(
		`
		CREATE TABLE IF NOT EXISTS executions (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			cronjob_ns      TEXT NOT NULL,
			cronjob_name    TEXT NOT NULL,
			cronjob_uid     TEXT,
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
			logs            TEXT,
			events          TEXT,
			created_at      TEXT DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_cronjob_time
			ON executions(cronjob_ns, cronjob_name, start_time DESC);
		CREATE INDEX IF NOT EXISTS idx_start_time ON executions(start_time);
		CREATE INDEX IF NOT EXISTS idx_job_name ON executions(job_name);
		CREATE INDEX IF NOT EXISTS idx_cronjob_uid
			ON executions(cronjob_ns, cronjob_name, cronjob_uid);

		CREATE TABLE IF NOT EXISTS alert_history (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_type       TEXT NOT NULL,
			severity         TEXT NOT NULL,
			title            TEXT NOT NULL,
			message          TEXT,
			cronjob_ns       TEXT,
			cronjob_name     TEXT,
			monitor_ns       TEXT,
			monitor_name     TEXT,
			channels_notified TEXT,
			occurred_at      TEXT NOT NULL,
			resolved_at      TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_alert_occurred
			ON alert_history(occurred_at DESC);
		CREATE INDEX IF NOT EXISTS idx_alert_cronjob
			ON alert_history(cronjob_ns, cronjob_name);
		CREATE INDEX IF NOT EXISTS idx_alert_severity
			ON alert_history(severity);
	`,
	)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations for existing databases
	if err := s.migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// migrate adds new columns to existing databases
func (s *SQLiteStore) migrate() error {
	// Check if cronjob_uid column exists
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('executions') WHERE name='cronjob_uid'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add new columns
		migrations := []string{
			`ALTER TABLE executions ADD COLUMN cronjob_uid TEXT`,
			`ALTER TABLE executions ADD COLUMN logs TEXT`,
			`ALTER TABLE executions ADD COLUMN events TEXT`,
			`CREATE INDEX IF NOT EXISTS idx_cronjob_uid ON executions(cronjob_ns, cronjob_name, cronjob_uid)`,
		}
		for _, m := range migrations {
			if _, err := s.db.Exec(m); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
		}
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
	_, err := s.db.ExecContext(
		ctx, `
		INSERT INTO executions (
			cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			completion_time, duration_secs, succeeded, exit_code, reason,
			is_retry, retry_of, logs, events
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		exec.CronJobNamespace,
		exec.CronJobName,
		nullString(exec.CronJobUID),
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
		nullString(exec.Logs),
		nullString(exec.Events),
	)
	return err
}

// GetExecutions returns executions for a CronJob since a given time
func (s *SQLiteStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	rows, err := s.db.QueryContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
		ORDER BY start_time DESC
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339),
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
func (s *SQLiteStore) GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ?
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name,
	)

	return s.scanExecution(row)
}

// GetLastSuccessfulExecution returns the most recent successful execution
func (s *SQLiteStore) GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND succeeded = 1
		ORDER BY start_time DESC
		LIMIT 1
	`, cronJob.Namespace, cronJob.Name,
	)

	return s.scanExecution(row)
}

// GetMetrics calculates SLA metrics for a CronJob
func (s *SQLiteStore) GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*Metrics, error) {
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
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339),
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
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339),
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
		metrics.P50DurationSeconds = percentile(durations, 50)
		metrics.P95DurationSeconds = percentile(durations, 95)
		metrics.P99DurationSeconds = percentile(durations, 99)
	}

	return metrics, nil
}

// GetDurationPercentile calculates a duration percentile
func (s *SQLiteStore) GetDurationPercentile(ctx context.Context, cronJob types.NamespacedName, p int, windowDays int) (time.Duration, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	rows, err := s.db.QueryContext(
		ctx, `
		SELECT duration_secs
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL
		ORDER BY duration_secs
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339),
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

	return time.Duration(percentile(durations, p) * float64(time.Second)), nil
}

// GetSuccessRate calculates success rate
func (s *SQLiteStore) GetSuccessRate(ctx context.Context, cronJob types.NamespacedName, windowDays int) (float64, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	var total, succeeded int
	row := s.db.QueryRowContext(
		ctx, `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN succeeded = 1 THEN 1 ELSE 0 END), 0) as succeeded
		FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?
	`, cronJob.Namespace, cronJob.Name, since.Format(time.RFC3339),
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
func (s *SQLiteStore) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(
		ctx, `
		DELETE FROM executions WHERE start_time < ?
	`, olderThan.Format(time.RFC3339),
	)
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
	var cronJobUID, scheduledTime, startTime, completionTime, createdAt sql.NullString
	var durationSecs sql.NullFloat64
	var succeeded, isRetry int
	var exitCode sql.NullInt32
	var reason, retryOf, logs, events sql.NullString

	err := row.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&cronJobUID,
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
		&logs,
		&events,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if cronJobUID.Valid {
		exec.CronJobUID = cronJobUID.String
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
	if logs.Valid {
		exec.Logs = logs.String
	}
	if events.Valid {
		exec.Events = events.String
	}
	if createdAt.Valid {
		exec.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}

	return &exec, nil
}

// scanExecutionRow scans a single row from a Rows result
func (s *SQLiteStore) scanExecutionRow(rows *sql.Rows) (*Execution, error) {
	var exec Execution
	var cronJobUID, scheduledTime, startTime, completionTime, createdAt sql.NullString
	var durationSecs sql.NullFloat64
	var succeeded, isRetry int
	var exitCode sql.NullInt32
	var reason, retryOf, logs, events sql.NullString

	err := rows.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&cronJobUID,
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
		&logs,
		&events,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if cronJobUID.Valid {
		exec.CronJobUID = cronJobUID.String
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
	if logs.Valid {
		exec.Logs = logs.String
	}
	if events.Valid {
		exec.Events = events.String
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

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// PruneLogs removes logs from executions older than the given time
func (s *SQLiteStore) PruneLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(
		ctx, `
		UPDATE executions SET logs = NULL, events = NULL
		WHERE start_time < ? AND (logs IS NOT NULL OR events IS NOT NULL)
	`, olderThan.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteExecutionsByCronJob deletes all executions for a specific CronJob
func (s *SQLiteStore) DeleteExecutionsByCronJob(ctx context.Context, cronJob types.NamespacedName) (int64, error) {
	result, err := s.db.ExecContext(
		ctx, `
		DELETE FROM executions WHERE cronjob_ns = ? AND cronjob_name = ?
	`, cronJob.Namespace, cronJob.Name,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteExecutionsByUID deletes executions for a specific CronJob UID
func (s *SQLiteStore) DeleteExecutionsByUID(ctx context.Context, cronJob types.NamespacedName, uid string) (int64, error) {
	result, err := s.db.ExecContext(
		ctx, `
		DELETE FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND cronjob_uid = ?
	`, cronJob.Namespace, cronJob.Name, uid,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetCronJobUIDs returns distinct UIDs for a CronJob
func (s *SQLiteStore) GetCronJobUIDs(ctx context.Context, cronJob types.NamespacedName) ([]string, error) {
	rows, err := s.db.QueryContext(
		ctx, `
		SELECT DISTINCT cronjob_uid FROM executions
		WHERE cronjob_ns = ? AND cronjob_name = ? AND cronjob_uid IS NOT NULL
		ORDER BY cronjob_uid
	`, cronJob.Namespace, cronJob.Name,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var uids []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		uids = append(uids, uid)
	}
	return uids, rows.Err()
}

// GetExecutionCount returns the total number of executions
func (s *SQLiteStore) GetExecutionCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM executions`).Scan(&count)
	return count, err
}

// GetExecutionCountSince returns the count of executions since a given time
func (s *SQLiteStore) GetExecutionCountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM executions WHERE start_time >= ?
	`, since.Format(time.RFC3339)).Scan(&count)
	return count, err
}

// StoreAlert stores an alert in history
func (s *SQLiteStore) StoreAlert(ctx context.Context, alert AlertHistory) error {
	channels := ""
	if len(alert.ChannelsNotified) > 0 {
		for i, ch := range alert.ChannelsNotified {
			if i > 0 {
				channels += ","
			}
			channels += ch
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_history (
			alert_type, severity, title, message,
			cronjob_ns, cronjob_name, monitor_ns, monitor_name,
			channels_notified, occurred_at, resolved_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		alert.Type,
		alert.Severity,
		alert.Title,
		alert.Message,
		nullString(alert.CronJobNamespace),
		nullString(alert.CronJobName),
		nullString(alert.MonitorNamespace),
		nullString(alert.MonitorName),
		nullString(channels),
		alert.OccurredAt.Format(time.RFC3339),
		formatTimePtr(alert.ResolvedAt),
	)
	return err
}

// ListAlertHistory returns alert history with pagination
func (s *SQLiteStore) ListAlertHistory(ctx context.Context, query AlertHistoryQuery) ([]AlertHistory, int64, error) {
	// Build query
	baseQuery := "FROM alert_history WHERE 1=1"
	args := []interface{}{}

	if query.Since != nil {
		baseQuery += " AND occurred_at >= ?"
		args = append(args, query.Since.Format(time.RFC3339))
	}
	if query.Severity != "" {
		baseQuery += " AND severity = ?"
		args = append(args, query.Severity)
	}

	// Get total count
	var total int64
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get paginated results
	selectQuery := `
		SELECT id, alert_type, severity, title, message,
			   cronjob_ns, cronjob_name, monitor_ns, monitor_name,
			   channels_notified, occurred_at, resolved_at
	` + baseQuery + " ORDER BY occurred_at DESC"

	if query.Limit > 0 {
		selectQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
	}
	if query.Offset > 0 {
		selectQuery += fmt.Sprintf(" OFFSET %d", query.Offset)
	}

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var alerts []AlertHistory
	for rows.Next() {
		var alert AlertHistory
		var cronjobNs, cronjobName, monitorNs, monitorName, channels, occurredAt sql.NullString
		var resolvedAt sql.NullString

		err := rows.Scan(
			&alert.ID,
			&alert.Type,
			&alert.Severity,
			&alert.Title,
			&alert.Message,
			&cronjobNs,
			&cronjobName,
			&monitorNs,
			&monitorName,
			&channels,
			&occurredAt,
			&resolvedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if cronjobNs.Valid {
			alert.CronJobNamespace = cronjobNs.String
		}
		if cronjobName.Valid {
			alert.CronJobName = cronjobName.String
		}
		if monitorNs.Valid {
			alert.MonitorNamespace = monitorNs.String
		}
		if monitorName.Valid {
			alert.MonitorName = monitorName.String
		}
		if channels.Valid && channels.String != "" {
			alert.ChannelsNotified = strings.Split(channels.String, ",")
		}
		if occurredAt.Valid {
			alert.OccurredAt, _ = time.Parse(time.RFC3339, occurredAt.String)
		}
		if resolvedAt.Valid {
			t, _ := time.Parse(time.RFC3339, resolvedAt.String)
			alert.ResolvedAt = &t
		}

		alerts = append(alerts, alert)
	}

	return alerts, total, rows.Err()
}

// ResolveAlert marks an alert as resolved
func (s *SQLiteStore) ResolveAlert(ctx context.Context, alertType, cronJobNs, cronJobName string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE alert_history
		SET resolved_at = ?
		WHERE alert_type = ? AND cronjob_ns = ? AND cronjob_name = ? AND resolved_at IS NULL
	`, time.Now().Format(time.RFC3339), alertType, cronJobNs, cronJobName)
	return err
}

// GetChannelAlertStats returns alert statistics for all channels
func (s *SQLiteStore) GetChannelAlertStats(ctx context.Context) (map[string]ChannelAlertStats, error) {
	// Get all alerts and count by channel
	// Since channels_notified is comma-separated, we need to process in application
	rows, err := s.db.QueryContext(ctx, `
		SELECT channels_notified, occurred_at
		FROM alert_history
		WHERE channels_notified IS NOT NULL AND channels_notified != ''
	`)
	if err != nil {
		return nil, fmt.Errorf("query alert_history: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]ChannelAlertStats)
	cutoff24h := time.Now().Add(-24 * time.Hour)

	for rows.Next() {
		var channelsStr string
		var occurredAtStr string
		if err := rows.Scan(&channelsStr, &occurredAtStr); err != nil {
			continue
		}

		occurredAt, err := time.Parse(time.RFC3339, occurredAtStr)
		if err != nil {
			continue
		}

		channels := strings.Split(channelsStr, ",")
		for _, ch := range channels {
			ch = strings.TrimSpace(ch)
			if ch == "" {
				continue
			}
			st := stats[ch]
			st.ChannelName = ch
			st.AlertsSentTotal++
			if occurredAt.After(cutoff24h) {
				st.AlertsSent24h++
			}
			stats[ch] = st
		}
	}

	return stats, rows.Err()
}

// Helper function to format time pointer
func formatTimePtr(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.Format(time.RFC3339), Valid: true}
}
