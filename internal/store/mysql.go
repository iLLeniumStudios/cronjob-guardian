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
			cronjob_uid     VARCHAR(36),
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
			logs            LONGTEXT,
			events          LONGTEXT,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,

			INDEX idx_cronjob_time (cronjob_ns, cronjob_name, start_time DESC),
			INDEX idx_start_time (start_time),
			INDEX idx_job_name (job_name),
			INDEX idx_cronjob_uid (cronjob_ns, cronjob_name, cronjob_uid)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`,
	)
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
			id               BIGINT AUTO_INCREMENT PRIMARY KEY,
			alert_type       VARCHAR(100) NOT NULL,
			severity         VARCHAR(20) NOT NULL,
			title            VARCHAR(500) NOT NULL,
			message          TEXT,
			cronjob_ns       VARCHAR(253),
			cronjob_name     VARCHAR(253),
			monitor_ns       VARCHAR(253),
			monitor_name     VARCHAR(253),
			channels_notified TEXT,
			occurred_at      DATETIME NOT NULL,
			resolved_at      DATETIME,

			INDEX idx_alert_history_occurred (occurred_at DESC),
			INDEX idx_alert_history_cronjob (cronjob_ns, cronjob_name),
			INDEX idx_alert_history_severity (severity)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return fmt.Errorf("failed to create alert_history table: %w", err)
	}

	return nil
}

// migrate adds new columns to existing databases
func (s *MySQLStore) migrate() error {
	// Check if cronjob_uid column exists
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = 'executions' AND column_name = 'cronjob_uid'
	`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add new columns
		migrations := []string{
			`ALTER TABLE executions ADD COLUMN cronjob_uid VARCHAR(36)`,
			`ALTER TABLE executions ADD COLUMN logs LONGTEXT`,
			`ALTER TABLE executions ADD COLUMN events LONGTEXT`,
			`ALTER TABLE executions ADD INDEX idx_cronjob_uid (cronjob_ns, cronjob_name, cronjob_uid)`,
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
			cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			completion_time, duration_secs, succeeded, exit_code, reason,
			is_retry, retry_of, logs, events
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		exec.CronJobNamespace,
		exec.CronJobName,
		nullStringMySQL(exec.CronJobUID),
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
		nullStringMySQL(exec.Logs),
		nullStringMySQL(exec.Events),
	)
	return err
}

func nullStringMySQL(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetExecutions returns executions for a CronJob since a given time
func (s *MySQLStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	rows, err := s.db.QueryContext(
		ctx, `
		SELECT id, cronjob_ns, cronjob_name, cronjob_uid, job_name, scheduled_time, start_time,
			   completion_time, duration_secs, succeeded, exit_code, reason,
			   is_retry, retry_of, logs, events, created_at
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
func (s *MySQLStore) GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
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
func (s *MySQLStore) scanExecutionRow(rows *sql.Rows) (*Execution, error) {
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

// percentileMySQL calculates percentile in Go since MySQL doesn't have native support
func percentileMySQL(data []float64, p int) float64 {
	if len(data) == 0 {
		return 0
	}
	sort.Float64s(data)
	idx := int(float64(len(data)-1) * float64(p) / 100)
	return data[idx]
}

// PruneLogs removes logs from executions older than the given time
func (s *MySQLStore) PruneLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(
		ctx, `
		UPDATE executions SET logs = NULL, events = NULL
		WHERE start_time < ? AND (logs IS NOT NULL OR events IS NOT NULL)
	`, olderThan,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteExecutionsByCronJob deletes all executions for a specific CronJob
func (s *MySQLStore) DeleteExecutionsByCronJob(ctx context.Context, cronJob types.NamespacedName) (int64, error) {
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
func (s *MySQLStore) DeleteExecutionsByUID(ctx context.Context, cronJob types.NamespacedName, uid string) (int64, error) {
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
func (s *MySQLStore) GetCronJobUIDs(ctx context.Context, cronJob types.NamespacedName) ([]string, error) {
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
func (s *MySQLStore) GetExecutionCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM executions`).Scan(&count)
	return count, err
}

// GetExecutionCountSince returns the count of executions since a given time
func (s *MySQLStore) GetExecutionCountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM executions WHERE start_time >= ?
	`, since).Scan(&count)
	return count, err
}

// StoreAlert stores an alert in history
func (s *MySQLStore) StoreAlert(ctx context.Context, alert AlertHistory) error {
	channelsStr := ""
	if len(alert.ChannelsNotified) > 0 {
		channelsStr = strings.Join(alert.ChannelsNotified, ",")
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
		nullStringMySQL(alert.CronJobNamespace),
		nullStringMySQL(alert.CronJobName),
		nullStringMySQL(alert.MonitorNamespace),
		nullStringMySQL(alert.MonitorName),
		nullStringMySQL(channelsStr),
		alert.OccurredAt,
		alert.ResolvedAt,
	)
	return err
}

// ListAlertHistory returns alert history with pagination
func (s *MySQLStore) ListAlertHistory(ctx context.Context, query AlertHistoryQuery) ([]AlertHistory, int64, error) {
	// Build query
	baseQuery := alertHistoryBaseQuery
	args := []any{}

	if query.Since != nil {
		baseQuery += " AND occurred_at >= ?"
		args = append(args, *query.Since)
	}

	if query.Severity != "" {
		baseQuery += " AND severity = ?"
		args = append(args, query.Severity)
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
		%s ORDER BY occurred_at DESC LIMIT ? OFFSET ?
	`, baseQuery)
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
func (s *MySQLStore) ResolveAlert(ctx context.Context, alertType, cronJobNs, cronJobName string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE alert_history
		SET resolved_at = ?
		WHERE alert_type = ? AND cronjob_ns = ? AND cronjob_name = ?
			AND resolved_at IS NULL
	`, time.Now(), alertType, cronJobNs, cronJobName)
	return err
}

// GetChannelAlertStats returns alert statistics for all channels
func (s *MySQLStore) GetChannelAlertStats(ctx context.Context) (map[string]ChannelAlertStats, error) {
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
	defer func() { _ = rows.Close() }()

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
