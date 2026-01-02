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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite" // Pure Go SQLite driver (no CGO required)
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"k8s.io/apimachinery/pkg/types"
)

// GormStore implements Store using GORM
type GormStore struct {
	db      *gorm.DB
	dialect string
}

// ConnectionPoolConfig holds connection pool settings
type ConnectionPoolConfig struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewGormStore creates a new GORM-based store
func NewGormStore(dialect string, dsn string) (*GormStore, error) {
	return NewGormStoreWithPool(dialect, dsn, ConnectionPoolConfig{})
}

// NewGormStoreWithPool creates a new GORM-based store with connection pool settings
func NewGormStoreWithPool(dialect string, dsn string, pool ConnectionPoolConfig) (*GormStore, error) {
	var dialector gorm.Dialector
	switch dialect {
	case "sqlite":
		dialector = sqlite.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for non-SQLite databases
	if dialect != "sqlite" && (pool.MaxIdleConns > 0 || pool.MaxOpenConns > 0 || pool.ConnMaxLifetime > 0 || pool.ConnMaxIdleTime > 0) {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB for pool config: %w", err)
		}

		if pool.MaxIdleConns > 0 {
			sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
		}
		if pool.MaxOpenConns > 0 {
			sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
		}
		if pool.ConnMaxLifetime > 0 {
			sqlDB.SetConnMaxLifetime(pool.ConnMaxLifetime)
		}
		if pool.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
		}
	}

	return &GormStore{db: db, dialect: dialect}, nil
}

// Init initializes the store (creates tables via auto-migration)
func (s *GormStore) Init() error {
	return s.db.AutoMigrate(&Execution{}, &AlertHistory{}, &ChannelStatsRecord{})
}

// Close closes the store and releases resources
func (s *GormStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// RecordExecution stores a new execution record
func (s *GormStore) RecordExecution(ctx context.Context, exec Execution) error {
	return s.db.WithContext(ctx).Create(&exec).Error
}

// GetExecutions returns executions for a CronJob since a given time
func (s *GormStore) GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error) {
	var execs []Execution
	err := s.db.WithContext(ctx).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?",
			cronJob.Namespace, cronJob.Name, since).
		Order("start_time DESC").
		Find(&execs).Error
	return execs, err
}

// GetExecutionsPaginated returns executions with database-level pagination
func (s *GormStore) GetExecutionsPaginated(ctx context.Context, cronJob types.NamespacedName, since time.Time, limit, offset int) ([]Execution, int64, error) {
	var execs []Execution
	var total int64

	query := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?",
			cronJob.Namespace, cronJob.Name, since)

	// Get total count first
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("start_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&execs).Error

	return execs, total, err
}

// GetExecutionsFiltered returns executions with database-level filtering and pagination
func (s *GormStore) GetExecutionsFiltered(ctx context.Context, cronJob types.NamespacedName, since time.Time, status string, limit, offset int) ([]Execution, int64, error) {
	var execs []Execution
	var total int64

	query := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?",
			cronJob.Namespace, cronJob.Name, since)

	// Apply status filter at database level
	switch status {
	case "success":
		query = query.Where("succeeded = ?", true)
	case "failed":
		query = query.Where("succeeded = ?", false)
	}

	// Get total count first
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	err := query.Order("start_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&execs).Error

	return execs, total, err
}

// GetLastExecution returns the most recent execution
func (s *GormStore) GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	var exec Execution
	err := s.db.WithContext(ctx).
		Where("cronjob_ns = ? AND cronjob_name = ?", cronJob.Namespace, cronJob.Name).
		Order("start_time DESC").
		First(&exec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &exec, nil
}

// GetLastSuccessfulExecution returns the most recent successful execution
func (s *GormStore) GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error) {
	var exec Execution
	err := s.db.WithContext(ctx).
		Where("cronjob_ns = ? AND cronjob_name = ? AND succeeded = ?",
			cronJob.Namespace, cronJob.Name, true).
		Order("start_time DESC").
		First(&exec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &exec, nil
}

// GetExecutionByJobName returns an execution by its job name
func (s *GormStore) GetExecutionByJobName(ctx context.Context, namespace, jobName string) (*Execution, error) {
	var exec Execution
	err := s.db.WithContext(ctx).
		Where("cronjob_ns = ? AND job_name = ?", namespace, jobName).
		First(&exec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &exec, nil
}

// GetMetrics calculates SLA metrics for a CronJob
func (s *GormStore) GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*Metrics, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	// Count query
	type countResult struct {
		Total     int64
		Succeeded int64
		Failed    int64
	}
	var result countResult

	err := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?",
			cronJob.Namespace, cronJob.Name, since).
		Select("COUNT(*) as total, "+
			"SUM(CASE WHEN succeeded = ? THEN 1 ELSE 0 END) as succeeded, "+
			"SUM(CASE WHEN succeeded = ? THEN 1 ELSE 0 END) as failed",
			true, false).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}

	metrics := &Metrics{
		WindowDays:     int32(windowDays),
		TotalRuns:      int32(result.Total),
		SuccessfulRuns: int32(result.Succeeded),
		FailedRuns:     int32(result.Failed),
	}

	if result.Total > 0 {
		metrics.SuccessRate = float64(result.Succeeded) / float64(result.Total) * 100
	}

	// Get durations for percentile calculation
	// Use native percentile functions for PostgreSQL, in-memory for SQLite
	if s.dialect == "postgres" {
		// Use native PostgreSQL percentile_cont for O(1) memory usage
		type percentileResult struct {
			Avg float64
			P50 float64
			P95 float64
			P99 float64
		}
		var pr percentileResult
		err = s.db.WithContext(ctx).Model(&Execution{}).
			Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL",
				cronJob.Namespace, cronJob.Name, since).
			Select(`
				AVG(duration_secs) as avg,
				PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY duration_secs) as p50,
				PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_secs) as p95,
				PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY duration_secs) as p99
			`).
			Scan(&pr).Error
		if err == nil {
			metrics.AvgDurationSeconds = pr.Avg
			metrics.P50DurationSeconds = pr.P50
			metrics.P95DurationSeconds = pr.P95
			metrics.P99DurationSeconds = pr.P99
		}
	} else {
		// SQLite: Use in-memory percentile calculation
		var durations []float64
		err = s.db.WithContext(ctx).Model(&Execution{}).
			Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL",
				cronJob.Namespace, cronJob.Name, since).
			Order("duration_secs").
			Pluck("duration_secs", &durations).Error
		if err != nil {
			return nil, err
		}

		if len(durations) > 0 {
			var sum float64
			for _, d := range durations {
				sum += d
			}
			metrics.AvgDurationSeconds = sum / float64(len(durations))
			metrics.P50DurationSeconds = percentile(durations, 50)
			metrics.P95DurationSeconds = percentile(durations, 95)
			metrics.P99DurationSeconds = percentile(durations, 99)
		}
	}

	return metrics, nil
}

// GetDurationPercentile calculates a duration percentile using database-level
// LIMIT/OFFSET for O(1) memory usage instead of fetching all durations
func (s *GormStore) GetDurationPercentile(ctx context.Context, cronJob types.NamespacedName, p int, windowDays int) (time.Duration, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	// First get count
	var count int64
	if err := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL",
			cronJob.Namespace, cronJob.Name, since).
		Count(&count).Error; err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	// Calculate offset for percentile position
	offset := int(float64(count-1) * float64(p) / 100)

	// Get single value at percentile position using LIMIT/OFFSET
	var duration float64
	err := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ? AND duration_secs IS NOT NULL",
			cronJob.Namespace, cronJob.Name, since).
		Order("duration_secs").
		Offset(offset).
		Limit(1).
		Pluck("duration_secs", &duration).Error
	if err != nil {
		return 0, err
	}

	return time.Duration(duration * float64(time.Second)), nil
}

// GetSuccessRate calculates success rate
func (s *GormStore) GetSuccessRate(ctx context.Context, cronJob types.NamespacedName, windowDays int) (float64, error) {
	since := time.Now().AddDate(0, 0, -windowDays)

	type countResult struct {
		Total     int64
		Succeeded int64
	}
	var result countResult

	err := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND start_time >= ?",
			cronJob.Namespace, cronJob.Name, since).
		Select("COUNT(*) as total, "+
			"SUM(CASE WHEN succeeded = ? THEN 1 ELSE 0 END) as succeeded", true).
		Scan(&result).Error
	if err != nil {
		return 0, err
	}

	if result.Total == 0 {
		return 100, nil // No data = assume healthy
	}

	return float64(result.Succeeded) / float64(result.Total) * 100, nil
}

// Prune removes old execution records
func (s *GormStore) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("start_time < ?", olderThan).
		Delete(&Execution{})
	return result.RowsAffected, result.Error
}

// PruneLogs removes logs from executions older than the given time
func (s *GormStore) PruneLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Model(&Execution{}).
		Where("start_time < ? AND (logs IS NOT NULL OR events IS NOT NULL)", olderThan).
		Updates(map[string]interface{}{"logs": nil, "events": nil})
	return result.RowsAffected, result.Error
}

// DeleteExecutionsByCronJob deletes all executions for a specific CronJob
func (s *GormStore) DeleteExecutionsByCronJob(ctx context.Context, cronJob types.NamespacedName) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("cronjob_ns = ? AND cronjob_name = ?", cronJob.Namespace, cronJob.Name).
		Delete(&Execution{})
	return result.RowsAffected, result.Error
}

// DeleteExecutionsByUID deletes executions for a specific CronJob UID
func (s *GormStore) DeleteExecutionsByUID(ctx context.Context, cronJob types.NamespacedName, uid string) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("cronjob_ns = ? AND cronjob_name = ? AND cronjob_uid = ?",
			cronJob.Namespace, cronJob.Name, uid).
		Delete(&Execution{})
	return result.RowsAffected, result.Error
}

// GetCronJobUIDs returns distinct UIDs for a CronJob
func (s *GormStore) GetCronJobUIDs(ctx context.Context, cronJob types.NamespacedName) ([]string, error) {
	var uids []string
	err := s.db.WithContext(ctx).Model(&Execution{}).
		Where("cronjob_ns = ? AND cronjob_name = ? AND cronjob_uid IS NOT NULL AND cronjob_uid != ''",
			cronJob.Namespace, cronJob.Name).
		Distinct("cronjob_uid").
		Order("cronjob_uid").
		Pluck("cronjob_uid", &uids).Error
	return uids, err
}

// GetExecutionCount returns the total number of executions
func (s *GormStore) GetExecutionCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&Execution{}).Count(&count).Error
	return count, err
}

// GetExecutionCountSince returns the count of executions since a given time
func (s *GormStore) GetExecutionCountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&Execution{}).
		Where("start_time >= ?", since).
		Count(&count).Error
	return count, err
}

// StoreAlert stores an alert in history
func (s *GormStore) StoreAlert(ctx context.Context, alert AlertHistory) error {
	return s.db.WithContext(ctx).Create(&alert).Error
}

// ListAlertHistory returns alert history with pagination
func (s *GormStore) ListAlertHistory(ctx context.Context, query AlertHistoryQuery) ([]AlertHistory, int64, error) {
	var alerts []AlertHistory
	var total int64

	db := s.db.WithContext(ctx).Model(&AlertHistory{})

	if query.Since != nil {
		db = db.Where("occurred_at >= ?", *query.Since)
	}
	if query.Severity != "" {
		db = db.Where("severity = ?", query.Severity)
	}
	if query.Type != "" {
		db = db.Where("alert_type = ?", query.Type)
	}

	// Get count first (before pagination)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	}
	if query.Offset > 0 {
		db = db.Offset(query.Offset)
	}

	err := db.Order("occurred_at DESC").Find(&alerts).Error
	return alerts, total, err
}

// ResolveAlert marks an alert as resolved
func (s *GormStore) ResolveAlert(ctx context.Context, alertType, cronJobNs, cronJobName string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&AlertHistory{}).
		Where("alert_type = ? AND cronjob_ns = ? AND cronjob_name = ? AND resolved_at IS NULL",
			alertType, cronJobNs, cronJobName).
		Update("resolved_at", &now).Error
}

// GetChannelAlertStats returns alert statistics for all channels.
// Uses batched queries to limit memory usage when processing large datasets.
func (s *GormStore) GetChannelAlertStats(ctx context.Context) (map[string]ChannelAlertStats, error) {
	// Use batched processing to avoid loading all rows into memory at once.
	// The channels_notified field is comma-separated, requiring app-level processing.
	const batchSize = 1000
	stats := make(map[string]ChannelAlertStats)
	var offset int

	for {
		var rows []string
		err := s.db.WithContext(ctx).Model(&AlertHistory{}).
			Select("channels_notified").
			Where("channels_notified IS NOT NULL AND channels_notified != ''").
			Order("id"). // Consistent ordering for pagination
			Offset(offset).Limit(batchSize).
			Pluck("channels_notified", &rows).Error
		if err != nil {
			return nil, fmt.Errorf("query alert_history batch at offset %d: %w", offset, err)
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			channels := strings.Split(row, ",")
			for _, ch := range channels {
				ch = strings.TrimSpace(ch)
				if ch == "" {
					continue
				}
				st := stats[ch]
				st.ChannelName = ch
				st.AlertsSentTotal++
				stats[ch] = st
			}
		}

		offset += batchSize
	}

	return stats, nil
}

// Health checks if the store is healthy
func (s *GormStore) Health(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// SaveChannelStats persists channel statistics using upsert
func (s *GormStore) SaveChannelStats(ctx context.Context, stats ChannelStatsRecord) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "channel_name"}},
			UpdateAll: true,
		}).Create(&stats).Error
}

// GetChannelStats retrieves channel statistics by name
func (s *GormStore) GetChannelStats(ctx context.Context, channelName string) (*ChannelStatsRecord, error) {
	var stats ChannelStatsRecord
	err := s.db.WithContext(ctx).
		Where("channel_name = ?", channelName).
		First(&stats).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetAllChannelStats retrieves all channel statistics
func (s *GormStore) GetAllChannelStats(ctx context.Context) (map[string]*ChannelStatsRecord, error) {
	var records []ChannelStatsRecord
	if err := s.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*ChannelStatsRecord, len(records))
	for i := range records {
		result[records[i].ChannelName] = &records[i]
	}
	return result, nil
}

// percentile calculates the p-th percentile from pre-sorted data.
// IMPORTANT: The input data must already be sorted in ascending order.
// The database query should use ORDER BY to ensure this.
func percentile(sortedData []float64, p int) float64 {
	if len(sortedData) == 0 {
		return 0
	}
	// Data is already sorted by database ORDER BY clause - no sort needed
	idx := int(float64(len(sortedData)-1) * float64(p) / 100)
	return sortedData[idx]
}
