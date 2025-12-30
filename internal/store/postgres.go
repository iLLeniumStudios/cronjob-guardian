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
	"strings"
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
			cronjob_uid     VARCHAR(36),
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
			logs            TEXT,
			events          TEXT,
			created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_cronjob_time
			ON executions(cronjob_ns, cronjob_name, start_time DESC);
		CREATE INDEX IF NOT EXISTS idx_start_time ON executions(start_time);
		CREATE INDEX IF NOT EXISTS idx_job_name ON executions(job_name);
		CREATE INDEX IF NOT EXISTS idx_cronjob_uid
			ON executions(cronjob_ns, cronjob_name, cronjob_uid);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations for existing databases
	if err := s.migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create alert_history table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS alert_history (
			id               BIGSERIAL PRIMARY KEY,
			alert_type       VARCHAR(100) NOT NULL,
			severity         VARCHAR(20) NOT NULL,
			title            VARCHAR(500) NOT NULL,
			message          TEXT,
			cronjob_ns       VARCHAR(253),
			cronjob_name     VARCHAR(253),
			monitor_ns       VARCHAR(253),
			monitor_name     VARCHAR(253),
			channels_notified TEXT,
			occurred_at      TIMESTAMP NOT NULL,
			resolved_at      TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_alert_history_occurred
			ON alert_history(occurred_at DESC);
		CREATE INDEX IF NOT EXISTS idx_alert_history_cronjob
			ON alert_history(cronjob_ns, cronjob_name);
		CREATE INDEX IF NOT EXISTS idx_alert_history_severity
			ON alert_history(severity);
	`)
	if err != nil {
		return fmt.Errorf("failed to create alert_history table: %w", err)
	}

	return nil
}

// migrate adds new columns to existing databases
func (s *PostgresStore) migrate() error {
	// Check if cronjob_uid column exists
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'executions' AND column_name = 'cronjob_uid'
		)
	`).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		// Add new columns
		migrations := []string{
			`ALTER TABLE executions ADD COLUMN IF NOT EXISTS cronjob_uid VARCHAR(36)`,
			`ALTER TABLE executions ADD COLUMN IF NOT EXISTS logs TEXT`,
			`ALTER TABLE executions ADD COLUMN IF NOT EXISTS events TEXT`,
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
			cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			completion_time, duration_secs, succeeded, exit_code, reason,
			is_retry, retry_of, logs, events
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		exec.CronJobNamespace,
		exec.CronJobName,
		nullStringPtr(exec.CronJobUID),
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
		nullStringPtr(exec.Logs),
		nullStringPtr(exec.Events),
	)
	return err
}

func nullStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetExecutions returns executions for a CronJob since a given time
func (s *PostgresStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
		FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND start_time >= $3
		ORDER BY start_time DESC
	`, cronJob.Namespace, cronJob.Name, since)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	return s.scanExecutions(rows)
}

// GetLastExecution returns the most recent execution
func (s *PostgresStore) GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
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
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
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
	var cronJobUID sql.NullString
	var scheduledTime, completionTime sql.NullTime
	var durationSecs sql.NullFloat64
	var exitCode sql.NullInt32
	var reason, retryOf, logs, events sql.NullString

	err := row.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&cronJobUID,
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
		&logs,
		&events,
		&exec.CreatedAt,
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
	if logs.Valid {
		exec.Logs = logs.String
	}
	if events.Valid {
		exec.Events = events.String
	}

	return &exec, nil
}

// scanExecutionRow scans a single row from a Rows result
func (s *PostgresStore) scanExecutionRow(rows *sql.Rows) (*Execution, error) {
	var exec Execution
	var cronJobUID sql.NullString
	var scheduledTime, completionTime sql.NullTime
	var durationSecs sql.NullFloat64
	var exitCode sql.NullInt32
	var reason, retryOf, logs, events sql.NullString

	err := rows.Scan(
		&exec.ID,
		&exec.CronJobNamespace,
		&exec.CronJobName,
		&cronJobUID,
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
		&logs,
		&events,
		&exec.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if cronJobUID.Valid {
		exec.CronJobUID = cronJobUID.String
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
	if logs.Valid {
		exec.Logs = logs.String
	}
	if events.Valid {
		exec.Events = events.String
	}

	return &exec, nil
}

// PruneLogs removes logs from executions older than the given time
func (s *PostgresStore) PruneLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE executions SET logs = NULL, events = NULL
		WHERE start_time < $1 AND (logs IS NOT NULL OR events IS NOT NULL)
	`, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteExecutionsByCronJob deletes all executions for a specific CronJob
func (s *PostgresStore) DeleteExecutionsByCronJob(ctx context.Context, cronJob types.NamespacedName) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM executions WHERE cronjob_ns = $1 AND cronjob_name = $2
	`, cronJob.Namespace, cronJob.Name)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteExecutionsByUID deletes executions for a specific CronJob UID
func (s *PostgresStore) DeleteExecutionsByUID(ctx context.Context, cronJob types.NamespacedName, uid string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND cronjob_uid = $3
	`, cronJob.Namespace, cronJob.Name, uid)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetCronJobUIDs returns distinct UIDs for a CronJob
func (s *PostgresStore) GetCronJobUIDs(ctx context.Context, cronJob types.NamespacedName) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT cronjob_uid FROM executions
		WHERE cronjob_ns = $1 AND cronjob_name = $2 AND cronjob_uid IS NOT NULL
		ORDER BY cronjob_uid
	`, cronJob.Namespace, cronJob.Name)
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
func (s *PostgresStore) GetExecutionCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM executions`).Scan(&count)
	return count, err
}

// GetExecutionCountSince returns the count of executions since a given time
func (s *PostgresStore) GetExecutionCountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM executions WHERE start_time >= $1
	`, since).Scan(&count)
	return count, err
}

// StoreAlert stores an alert in history
func (s *PostgresStore) StoreAlert(ctx context.Context, alert AlertHistory) error {
	channelsStr := ""
	if len(alert.ChannelsNotified) > 0 {
		channelsStr = strings.Join(alert.ChannelsNotified, ",")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_history (
			alert_type, severity, title, message,
			cronjob_ns, cronjob_name, monitor_ns, monitor_name,
			channels_notified, occurred_at, resolved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		alert.Type,
		alert.Severity,
		alert.Title,
		alert.Message,
		nullStringPtr(alert.CronJobNamespace),
		nullStringPtr(alert.CronJobName),
		nullStringPtr(alert.MonitorNamespace),
		nullStringPtr(alert.MonitorName),
		nullStringPtr(channelsStr),
		alert.OccurredAt,
		alert.ResolvedAt,
	)
	return err
}

// ListAlertHistory returns alert history with pagination
func (s *PostgresStore) ListAlertHistory(ctx context.Context, query AlertHistoryQuery) ([]AlertHistory, int64, error) {
	// Build query
	baseQuery := `FROM alert_history WHERE 1=1`
	args := []any{}
	argIdx := 1

	if query.Since != nil {
		baseQuery += fmt.Sprintf(" AND occurred_at >= $%d", argIdx)
		args = append(args, *query.Since)
		argIdx++
	}

	if query.Severity != "" {
		baseQuery += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, query.Severity)
		argIdx++
	}

	// Get total count
	var total int64
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	selectQuery := fmt.Sprintf(`
		SELECT id, alert_type, severity, title, message,
			   cronjob_ns, cronjob_name, monitor_ns, monitor_name,
			   channels_notified, occurred_at, resolved_at
		%s ORDER BY occurred_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var alerts []AlertHistory
	for rows.Next() {
		var a AlertHistory
		var cronjobNs, cronjobName, monitorNs, monitorName, channels sql.NullString
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&a.ID, &a.Type, &a.Severity, &a.Title, &a.Message,
			&cronjobNs, &cronjobName, &monitorNs, &monitorName,
			&channels, &a.OccurredAt, &resolvedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if cronjobNs.Valid {
			a.CronJobNamespace = cronjobNs.String
		}
		if cronjobName.Valid {
			a.CronJobName = cronjobName.String
		}
		if monitorNs.Valid {
			a.MonitorNamespace = monitorNs.String
		}
		if monitorName.Valid {
			a.MonitorName = monitorName.String
		}
		if channels.Valid && channels.String != "" {
			a.ChannelsNotified = strings.Split(channels.String, ",")
		}
		if resolvedAt.Valid {
			a.ResolvedAt = &resolvedAt.Time
		}

		alerts = append(alerts, a)
	}

	return alerts, total, rows.Err()
}

// ResolveAlert marks an alert as resolved
func (s *PostgresStore) ResolveAlert(ctx context.Context, alertType, cronJobNs, cronJobName string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE alert_history
		SET resolved_at = $1
		WHERE alert_type = $2 AND cronjob_ns = $3 AND cronjob_name = $4
			AND resolved_at IS NULL
	`, time.Now(), alertType, cronJobNs, cronJobName)
	return err
}

// GetChannelAlertStats returns alert statistics for all channels
func (s *PostgresStore) GetChannelAlertStats(ctx context.Context) (map[string]ChannelAlertStats, error) {
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
		var occurredAt time.Time
		if err := rows.Scan(&channelsStr, &occurredAt); err != nil {
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
