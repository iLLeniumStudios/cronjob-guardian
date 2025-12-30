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

// Execution represents a single CronJob execution
type Execution struct {
	ID               int64
	CronJobNamespace string
	CronJobName      string
	JobName          string
	ScheduledTime    *time.Time
	StartTime        time.Time
	CompletionTime   time.Time
	Duration         time.Duration
	Succeeded        bool
	ExitCode         int32
	Reason           string
	IsRetry          bool
	RetryOf          string
	CreatedAt        time.Time
}

// Metrics contains aggregated SLA metrics
type Metrics struct {
	SuccessRate        float64
	WindowDays         int32
	TotalRuns          int32
	SuccessfulRuns     int32
	FailedRuns         int32
	MissedRuns         int32
	AvgDurationSeconds float64
	P50DurationSeconds float64
	P95DurationSeconds float64
	P99DurationSeconds float64
}

// Store defines the storage interface for execution history
type Store interface {
	// Init initializes the store (creates tables, connections, etc.)
	Init() error

	// Close closes the store and releases resources
	Close() error

	// RecordExecution stores a new execution record
	RecordExecution(ctx context.Context, exec Execution) error

	// GetExecutions returns executions for a CronJob since a given time
	GetExecutions(ctx context.Context, cronJob types.NamespacedName, since time.Time) ([]Execution, error)

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

	// Health checks if the store is healthy
	Health(ctx context.Context) error
}
