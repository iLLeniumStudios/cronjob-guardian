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
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// Store defines the storage interface for execution history
// All struct types (Execution, AlertHistory, Metrics, etc.) are defined in models.go
type Store interface {
	// Init initializes the store (creates tables, connections, etc.)
	Init() error

	// Close closes the store and releases resources
	Close() error

	// RecordExecution stores a new execution record
	RecordExecution(ctx context.Context, exec Execution) error

	// GetExecutions returns executions for a CronJob since a given time
	GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error)

	// GetExecutionsPaginated returns executions with database-level pagination
	GetExecutionsPaginated(ctx context.Context, cronJob types.NamespacedName, since time.Time, limit, offset int) ([]Execution, int64, error)

	// GetLastExecution returns the most recent execution
	GetLastExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error)

	// GetLastSuccessfulExecution returns the most recent successful execution
	GetLastSuccessfulExecution(ctx context.Context, cronJob types.NamespacedName) (*Execution, error)

	// GetMetrics calculates SLA metrics for a CronJob
	GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*Metrics, error)

	// GetDurationPercentile calculates a duration percentile
	GetDurationPercentile(ctx context.Context, cronJob types.NamespacedName, percentile int, windowDays int) (time.Duration, error)

	// GetSuccessRate calculates success rate
	GetSuccessRate(ctx context.Context, cronJob types.NamespacedName, windowDays int) (float64, error)

	// Prune removes old execution records
	Prune(ctx context.Context, olderThan time.Time) (int64, error)

	// PruneLogs removes logs from executions older than the given time
	// This allows separate retention for logs vs execution metadata
	PruneLogs(ctx context.Context, olderThan time.Time) (int64, error)

	// DeleteExecutionsByCronJob deletes all executions for a specific CronJob
	DeleteExecutionsByCronJob(ctx context.Context, cronJob types.NamespacedName) (int64, error)

	// DeleteExecutionsByUID deletes executions for a specific CronJob UID
	// Used for cleaning up after CronJob recreation when onRecreation=reset
	DeleteExecutionsByUID(ctx context.Context, cronJob types.NamespacedName, uid string) (int64, error)

	// GetCronJobUIDs returns distinct UIDs for a CronJob (for recreation detection)
	GetCronJobUIDs(ctx context.Context, cronJob types.NamespacedName) ([]string, error)

	// GetExecutionCount returns the total number of executions in the store
	GetExecutionCount(ctx context.Context) (int64, error)

	// GetExecutionCountSince returns the count of executions since a given time
	GetExecutionCountSince(ctx context.Context, since time.Time) (int64, error)

	// StoreAlert stores an alert in history
	StoreAlert(ctx context.Context, alert AlertHistory) error

	// ListAlertHistory returns alert history with pagination
	ListAlertHistory(ctx context.Context, query AlertHistoryQuery) ([]AlertHistory, int64, error)

	// ResolveAlert marks an alert as resolved
	ResolveAlert(ctx context.Context, alertType, cronJobNs, cronJobName string) error

	// GetChannelAlertStats returns alert statistics for all channels
	GetChannelAlertStats(ctx context.Context) (map[string]ChannelAlertStats, error)

	// SaveChannelStats persists channel statistics (upsert)
	SaveChannelStats(ctx context.Context, stats ChannelStatsRecord) error

	// GetChannelStats retrieves channel statistics by name
	GetChannelStats(ctx context.Context, channelName string) (*ChannelStatsRecord, error)

	// GetAllChannelStats retrieves all channel statistics
	GetAllChannelStats(ctx context.Context) (map[string]*ChannelStatsRecord, error)

	// Health checks if the store is healthy
	Health(ctx context.Context) error
}
