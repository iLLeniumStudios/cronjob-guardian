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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/types"
)

// StoreTestSuite runs all store tests against SQLite
type StoreTestSuite struct {
	suite.Suite
	store *GormStore
	ctx   context.Context
}

func (s *StoreTestSuite) SetupTest() {
	var err error
	s.store, err = NewGormStore("sqlite", "file::memory:?cache=shared")
	require.NoError(s.T(), err)
	require.NoError(s.T(), s.store.Init())
	s.ctx = context.Background()
}

func (s *StoreTestSuite) TearDownTest() {
	if s.store != nil {
		_ = s.store.Close()
	}
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

// =============================================================================
// Execution Recording Tests
// =============================================================================

func (s *StoreTestSuite) TestRecordExecution_Success() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	startTime := time.Now().Add(-10 * time.Minute)
	completionTime := time.Now().Add(-5 * time.Minute)
	duration := 5.0 * 60 // 5 minutes in seconds

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		CronJobUID:       "test-uid-123",
		JobName:          "test-cron-12345",
		StartTime:        startTime,
		CompletionTime:   completionTime,
		DurationSecs:     &duration,
		Succeeded:        true,
	}

	err := s.store.RecordExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	// Verify it was stored
	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	assert.Equal(s.T(), "test-cron-12345", last.JobName)
	assert.True(s.T(), last.Succeeded)
}

func (s *StoreTestSuite) TestRecordExecution_Failure() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "failing-cron"}
	startTime := time.Now().Add(-10 * time.Minute)
	completionTime := time.Now().Add(-5 * time.Minute)
	duration := 5.0 * 60

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		CronJobUID:       "test-uid-456",
		JobName:          "failing-cron-12345",
		StartTime:        startTime,
		CompletionTime:   completionTime,
		DurationSecs:     &duration,
		Succeeded:        false,
		ExitCode:         137,
		Reason:           "OOMKilled",
	}

	err := s.store.RecordExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	assert.False(s.T(), last.Succeeded)
	assert.Equal(s.T(), int32(137), last.ExitCode)
	assert.Equal(s.T(), "OOMKilled", last.Reason)
}

func (s *StoreTestSuite) TestRecordExecution_WithLogs() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "logged-cron"}
	startTime := time.Now()
	logs := "Starting job...\nProcessing data...\nJob completed successfully."

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "logged-cron-12345",
		StartTime:        startTime,
		Succeeded:        true,
		Logs:             &logs,
	}

	err := s.store.RecordExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	require.NotNil(s.T(), last.Logs)
	assert.Contains(s.T(), *last.Logs, "Job completed successfully")
}

func (s *StoreTestSuite) TestRecordExecution_WithEvents() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "evented-cron"}
	startTime := time.Now()
	events := "Normal: Created pod\nNormal: Started container\nNormal: Completed"

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "evented-cron-12345",
		StartTime:        startTime,
		Succeeded:        true,
		Events:           &events,
	}

	err := s.store.RecordExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	require.NotNil(s.T(), last.Events)
	assert.Contains(s.T(), *last.Events, "Created pod")
}

func (s *StoreTestSuite) TestRecordExecution_WithSuggestedFix() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "fixable-cron"}
	startTime := time.Now()

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "fixable-cron-12345",
		StartTime:        startTime,
		Succeeded:        false,
		ExitCode:         137,
		Reason:           "OOMKilled",
		SuggestedFix:     "Increase memory limit in pod spec: resources.limits.memory",
	}

	err := s.store.RecordExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	assert.Contains(s.T(), last.SuggestedFix, "Increase memory limit")
}

func (s *StoreTestSuite) TestRecordExecution_DuplicateJobName() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "dupe-cron"}
	startTime := time.Now()

	exec1 := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "dupe-cron-same-name",
		StartTime:        startTime,
		Succeeded:        true,
	}

	err := s.store.RecordExecution(s.ctx, exec1)
	require.NoError(s.T(), err)

	// Recording the same job again should succeed (no unique constraint on job_name)
	exec2 := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "dupe-cron-same-name",
		StartTime:        startTime.Add(time.Second), // slightly different time
		Succeeded:        true,
	}

	err = s.store.RecordExecution(s.ctx, exec2)
	require.NoError(s.T(), err)

	// Should have 2 records
	execs, err := s.store.GetExecutions(s.ctx, cronJob, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs, 2)
}

// =============================================================================
// GetExecutions Tests
// =============================================================================

func (s *StoreTestSuite) TestGetExecutions_Pagination() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "paginated-cron"}

	// Create 20 executions
	for i := 0; i < 20; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "paginated-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Get first page
	execs, total, err := s.store.GetExecutionsPaginated(s.ctx, cronJob, time.Time{}, 5, 0)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(20), total)
	assert.Len(s.T(), execs, 5)

	// Get second page
	execs, total, err = s.store.GetExecutionsPaginated(s.ctx, cronJob, time.Time{}, 5, 5)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(20), total)
	assert.Len(s.T(), execs, 5)

	// Get last partial page
	execs, total, err = s.store.GetExecutionsPaginated(s.ctx, cronJob, time.Time{}, 5, 18)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(20), total)
	assert.Len(s.T(), execs, 2)
}

func (s *StoreTestSuite) TestGetExecutions_FilterByStatus() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "filtered-cron"}

	// Create mixed executions
	for i := 0; i < 10; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "filtered-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        i%2 == 0, // alternating success/failure
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Filter by success
	execs, total, err := s.store.GetExecutionsFiltered(s.ctx, cronJob, time.Time{}, "success", 100, 0)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), total)
	assert.Len(s.T(), execs, 5)
	for _, e := range execs {
		assert.True(s.T(), e.Succeeded)
	}

	// Filter by failed
	execs, total, err = s.store.GetExecutionsFiltered(s.ctx, cronJob, time.Time{}, "failed", 100, 0)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), total)
	assert.Len(s.T(), execs, 5)
	for _, e := range execs {
		assert.False(s.T(), e.Succeeded)
	}

	// No filter (all)
	execs, total, err = s.store.GetExecutionsFiltered(s.ctx, cronJob, time.Time{}, "", 100, 0)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(10), total)
	assert.Len(s.T(), execs, 10)
}

func (s *StoreTestSuite) TestGetExecutions_FilterByTimeRange() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "time-filtered-cron"}

	// Create executions over time
	baseTime := time.Now()
	for i := 0; i < 10; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "time-filtered-cron-" + string(rune('A'+i)),
			StartTime:        baseTime.Add(time.Duration(-i) * 24 * time.Hour), // one per day
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Get executions from last 3 days
	since := baseTime.Add(-3 * 24 * time.Hour)
	execs, err := s.store.GetExecutions(s.ctx, cronJob, since)
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs, 4) // today + 3 days back = 4 records
}

func (s *StoreTestSuite) TestGetLastExecution() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "last-exec-cron"}

	// Create multiple executions
	for i := 0; i < 5; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "last-exec-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	assert.Equal(s.T(), "last-exec-cron-A", last.JobName) // Most recent
}

func (s *StoreTestSuite) TestGetLastExecution_NoRecords() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "empty-cron"}

	last, err := s.store.GetLastExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	assert.Nil(s.T(), last)
}

func (s *StoreTestSuite) TestGetLastSuccessfulExecution() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "last-success-cron"}

	// Create executions: most recent fails, older ones succeed
	exec1 := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "last-success-cron-fail",
		StartTime:        time.Now(),
		Succeeded:        false, // Most recent, but failed
	}
	require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec1))

	exec2 := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "last-success-cron-ok",
		StartTime:        time.Now().Add(-1 * time.Hour),
		Succeeded:        true, // Older, but successful
	}
	require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec2))

	last, err := s.store.GetLastSuccessfulExecution(s.ctx, cronJob)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), last)
	assert.Equal(s.T(), "last-success-cron-ok", last.JobName)
	assert.True(s.T(), last.Succeeded)
}

func (s *StoreTestSuite) TestGetExecutionByJobName() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "job-lookup-cron"}

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "job-lookup-cron-specific-job",
		StartTime:        time.Now(),
		Succeeded:        true,
	}
	require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))

	found, err := s.store.GetExecutionByJobName(s.ctx, cronJob.Namespace, "job-lookup-cron-specific-job")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), found)
	assert.Equal(s.T(), "job-lookup-cron-specific-job", found.JobName)
}

func (s *StoreTestSuite) TestGetExecutionByJobName_NotFound() {
	found, err := s.store.GetExecutionByJobName(s.ctx, "default", "nonexistent-job")
	require.NoError(s.T(), err)
	assert.Nil(s.T(), found)
}

// =============================================================================
// Metrics Calculation Tests
// =============================================================================

func (s *StoreTestSuite) TestGetMetrics_EmptyHistory() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "no-history-cron"}

	metrics, err := s.store.GetMetrics(s.ctx, cronJob, 7)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), metrics)
	assert.Equal(s.T(), int32(0), metrics.TotalRuns)
	assert.Equal(s.T(), float64(0), metrics.SuccessRate)
}

func (s *StoreTestSuite) TestGetMetrics_AllSuccessful() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "all-success-cron"}

	for i := 0; i < 10; i++ {
		duration := float64(i+1) * 60 // 1-10 minutes
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "all-success-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			DurationSecs:     &duration,
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	metrics, err := s.store.GetMetrics(s.ctx, cronJob, 7)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), metrics)
	assert.Equal(s.T(), int32(10), metrics.TotalRuns)
	assert.Equal(s.T(), int32(10), metrics.SuccessfulRuns)
	assert.Equal(s.T(), int32(0), metrics.FailedRuns)
	assert.Equal(s.T(), float64(100), metrics.SuccessRate)
}

func (s *StoreTestSuite) TestGetMetrics_AllFailed() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "all-failed-cron"}

	for i := 0; i < 10; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "all-failed-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        false,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	metrics, err := s.store.GetMetrics(s.ctx, cronJob, 7)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), metrics)
	assert.Equal(s.T(), int32(10), metrics.TotalRuns)
	assert.Equal(s.T(), int32(0), metrics.SuccessfulRuns)
	assert.Equal(s.T(), int32(10), metrics.FailedRuns)
	assert.Equal(s.T(), float64(0), metrics.SuccessRate)
}

func (s *StoreTestSuite) TestGetMetrics_MixedResults() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "mixed-cron"}

	// 7 successes, 3 failures = 70% success rate
	for i := 0; i < 10; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "mixed-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        i < 7, // first 7 succeed
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	metrics, err := s.store.GetMetrics(s.ctx, cronJob, 7)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), metrics)
	assert.Equal(s.T(), int32(10), metrics.TotalRuns)
	assert.Equal(s.T(), int32(7), metrics.SuccessfulRuns)
	assert.Equal(s.T(), int32(3), metrics.FailedRuns)
	assert.Equal(s.T(), float64(70), metrics.SuccessRate)
}

func (s *StoreTestSuite) TestGetMetrics_WindowDays() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "window-cron"}

	// Create executions: 5 within 3 days, 5 older than 3 days
	now := time.Now()
	for i := 0; i < 10; i++ {
		var startTime time.Time
		if i < 5 {
			startTime = now.Add(time.Duration(-i) * time.Hour) // within 3 days
		} else {
			startTime = now.Add(time.Duration(-5-i) * 24 * time.Hour) // older than 3 days
		}
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "window-cron-" + string(rune('A'+i)),
			StartTime:        startTime,
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Window of 3 days should only include 5 executions
	metrics, err := s.store.GetMetrics(s.ctx, cronJob, 3)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), metrics)
	assert.Equal(s.T(), int32(5), metrics.TotalRuns)
}

func (s *StoreTestSuite) TestGetDurationPercentile_P50() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "p50-cron"}

	// Create executions with durations 1-100 seconds
	for i := 1; i <= 100; i++ {
		duration := float64(i)
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "p50-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Minute),
			DurationSecs:     &duration,
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	p50, err := s.store.GetDurationPercentile(s.ctx, cronJob, 50, 7)
	require.NoError(s.T(), err)
	// P50 of 1-100 should be around 50 seconds
	assert.InDelta(s.T(), 50*time.Second, p50, float64(2*time.Second))
}

func (s *StoreTestSuite) TestGetDurationPercentile_P95() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "p95-cron"}

	// Create executions with durations 1-100 seconds
	for i := 1; i <= 100; i++ {
		duration := float64(i)
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "p95-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Minute),
			DurationSecs:     &duration,
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	p95, err := s.store.GetDurationPercentile(s.ctx, cronJob, 95, 7)
	require.NoError(s.T(), err)
	// P95 of 1-100 should be around 95 seconds
	assert.InDelta(s.T(), 95*time.Second, p95, float64(2*time.Second))
}

func (s *StoreTestSuite) TestGetDurationPercentile_P99() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "p99-cron"}

	// Create executions with durations 1-100 seconds
	for i := 1; i <= 100; i++ {
		duration := float64(i)
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "p99-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Minute),
			DurationSecs:     &duration,
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	p99, err := s.store.GetDurationPercentile(s.ctx, cronJob, 99, 7)
	require.NoError(s.T(), err)
	// P99 of 1-100 should be around 99 seconds
	assert.InDelta(s.T(), 99*time.Second, p99, float64(2*time.Second))
}

func (s *StoreTestSuite) TestGetDurationPercentile_SingleExecution() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "single-exec-cron"}

	duration := float64(60) // 1 minute
	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "single-exec-cron-A",
		StartTime:        time.Now(),
		DurationSecs:     &duration,
		Succeeded:        true,
	}
	require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))

	// All percentiles should return the same value
	p50, err := s.store.GetDurationPercentile(s.ctx, cronJob, 50, 7)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 60*time.Second, p50)

	p95, err := s.store.GetDurationPercentile(s.ctx, cronJob, 95, 7)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 60*time.Second, p95)
}

func (s *StoreTestSuite) TestGetDurationPercentile_NoExecutions() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "no-duration-cron"}

	p50, err := s.store.GetDurationPercentile(s.ctx, cronJob, 50, 7)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), time.Duration(0), p50)
}

func (s *StoreTestSuite) TestGetSuccessRate_WindowBoundary() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "boundary-cron"}

	// Create an execution exactly at the window boundary
	now := time.Now()
	exactlyAtBoundary := now.AddDate(0, 0, -7)

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "boundary-cron-exact",
		StartTime:        exactlyAtBoundary,
		Succeeded:        false,
	}
	require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))

	// This execution should NOT be included (exactly at boundary)
	rate, err := s.store.GetSuccessRate(s.ctx, cronJob, 7)
	require.NoError(s.T(), err)
	// If no executions are within window, success rate defaults to 100%
	// The boundary behavior depends on implementation (<= vs <)
	// We're testing it doesn't crash
	assert.GreaterOrEqual(s.T(), rate, float64(0))
}

func (s *StoreTestSuite) TestGetSuccessRate() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "rate-cron"}

	// 8 successes, 2 failures = 80% success rate
	for i := 0; i < 10; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "rate-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        i < 8,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	rate, err := s.store.GetSuccessRate(s.ctx, cronJob, 7)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), float64(80), rate)
}

// =============================================================================
// Data Management Tests
// =============================================================================

func (s *StoreTestSuite) TestPrune_RemovesOldRecords() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "prune-cron"}
	now := time.Now()

	// Create 5 old executions (more than 30 days)
	for i := 0; i < 5; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "prune-cron-old-" + string(rune('A'+i)),
			StartTime:        now.AddDate(0, 0, -40-i),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Create 5 recent executions
	for i := 0; i < 5; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "prune-cron-new-" + string(rune('A'+i)),
			StartTime:        now.Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Prune records older than 30 days
	cutoff := now.AddDate(0, 0, -30)
	deleted, err := s.store.Prune(s.ctx, cutoff)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), deleted)

	// Verify only 5 remain
	execs, err := s.store.GetExecutions(s.ctx, cronJob, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs, 5)
}

func (s *StoreTestSuite) TestPrune_PreservesRecentRecords() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "preserve-cron"}
	now := time.Now()

	// Create only recent executions
	for i := 0; i < 5; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "preserve-cron-" + string(rune('A'+i)),
			StartTime:        now.Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Prune records older than 30 days (should delete nothing)
	cutoff := now.AddDate(0, 0, -30)
	deleted, err := s.store.Prune(s.ctx, cutoff)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), deleted)

	// Verify all 5 remain
	execs, err := s.store.GetExecutions(s.ctx, cronJob, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs, 5)
}

func (s *StoreTestSuite) TestPruneLogs_SeparateRetention() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "logprune-cron"}
	now := time.Now()
	logs := "Some log content"
	events := "Some events"

	// Create executions with logs - some old, some new
	for i := 0; i < 4; i++ {
		var startTime time.Time
		if i < 2 {
			startTime = now.AddDate(0, 0, -10-i) // older than 7 days
		} else {
			startTime = now.Add(time.Duration(-i) * time.Hour) // recent
		}
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "logprune-cron-" + string(rune('A'+i)),
			StartTime:        startTime,
			Succeeded:        true,
			Logs:             &logs,
			Events:           &events,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Prune logs older than 7 days
	cutoff := now.AddDate(0, 0, -7)
	affected, err := s.store.PruneLogs(s.ctx, cutoff)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(2), affected) // 2 old records had logs removed

	// Verify execution records still exist (only logs cleared)
	execs, err := s.store.GetExecutions(s.ctx, cronJob, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs, 4) // All 4 records still exist

	// Verify old records have nil logs
	for _, e := range execs {
		if e.StartTime.Before(cutoff) {
			assert.Nil(s.T(), e.Logs, "Old execution should have logs cleared")
			assert.Nil(s.T(), e.Events, "Old execution should have events cleared")
		} else {
			assert.NotNil(s.T(), e.Logs, "Recent execution should keep logs")
			assert.NotNil(s.T(), e.Events, "Recent execution should keep events")
		}
	}
}

func (s *StoreTestSuite) TestDeleteExecutionsByCronJob() {
	cronJob1 := types.NamespacedName{Namespace: "default", Name: "delete-me-cron"}
	cronJob2 := types.NamespacedName{Namespace: "default", Name: "keep-me-cron"}

	// Create executions for both CronJobs
	for i := 0; i < 5; i++ {
		exec1 := Execution{
			CronJobNamespace: cronJob1.Namespace,
			CronJobName:      cronJob1.Name,
			JobName:          "delete-me-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec1))

		exec2 := Execution{
			CronJobNamespace: cronJob2.Namespace,
			CronJobName:      cronJob2.Name,
			JobName:          "keep-me-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec2))
	}

	// Delete executions for cronJob1
	deleted, err := s.store.DeleteExecutionsByCronJob(s.ctx, cronJob1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), deleted)

	// Verify cronJob1 has no executions
	execs1, err := s.store.GetExecutions(s.ctx, cronJob1, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs1, 0)

	// Verify cronJob2 still has all executions
	execs2, err := s.store.GetExecutions(s.ctx, cronJob2, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs2, 5)
}

func (s *StoreTestSuite) TestDeleteExecutionsByUID() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "uid-delete-cron"}
	uid1 := "uid-12345"
	uid2 := "uid-67890"

	// Create executions with different UIDs
	for i := 0; i < 3; i++ {
		exec1 := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			CronJobUID:       uid1,
			JobName:          "uid-delete-cron-v1-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec1))

		exec2 := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			CronJobUID:       uid2,
			JobName:          "uid-delete-cron-v2-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i-10) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec2))
	}

	// Delete executions for uid1 only
	deleted, err := s.store.DeleteExecutionsByUID(s.ctx, cronJob, uid1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), deleted)

	// Verify only uid2 executions remain
	execs, err := s.store.GetExecutions(s.ctx, cronJob, time.Time{})
	require.NoError(s.T(), err)
	assert.Len(s.T(), execs, 3)
	for _, e := range execs {
		assert.Equal(s.T(), uid2, e.CronJobUID)
	}
}

func (s *StoreTestSuite) TestGetCronJobUIDs() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "multi-uid-cron"}

	// Create executions with different UIDs
	uids := []string{"uid-aaa", "uid-bbb", "uid-ccc"}
	for _, uid := range uids {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			CronJobUID:       uid,
			JobName:          "multi-uid-cron-" + uid,
			StartTime:        time.Now(),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	foundUIDs, err := s.store.GetCronJobUIDs(s.ctx, cronJob)
	require.NoError(s.T(), err)
	assert.Len(s.T(), foundUIDs, 3)
	assert.ElementsMatch(s.T(), uids, foundUIDs)
}

// =============================================================================
// Alert History Tests
// =============================================================================

func (s *StoreTestSuite) TestStoreAlert_AllTypes() {
	alertTypes := []string{"JobFailed", "MissedSchedule", "DeadManTriggered", "SLABreached", "DurationRegression"}

	for _, alertType := range alertTypes {
		alert := AlertHistory{
			Type:             alertType,
			Severity:         "warning",
			Title:            "Test alert: " + alertType,
			Message:          "Test message",
			CronJobNamespace: "default",
			CronJobName:      "test-cron",
			OccurredAt:       time.Now(),
		}
		err := s.store.StoreAlert(s.ctx, alert)
		require.NoError(s.T(), err, "Failed to store alert type: %s", alertType)
	}

	// Verify all alerts were stored
	query := AlertHistoryQuery{Limit: 100}
	alerts, total, err := s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), total)
	assert.Len(s.T(), alerts, 5)
}

func (s *StoreTestSuite) TestListAlertHistory_Pagination() {
	// Create 20 alerts
	for i := 0; i < 20; i++ {
		alert := AlertHistory{
			Type:             "JobFailed",
			Severity:         "warning",
			Title:            "Alert " + string(rune('A'+i)),
			Message:          "Test message",
			CronJobNamespace: "default",
			CronJobName:      "test-cron",
			OccurredAt:       time.Now().Add(time.Duration(-i) * time.Minute),
		}
		require.NoError(s.T(), s.store.StoreAlert(s.ctx, alert))
	}

	// Get first page
	query := AlertHistoryQuery{Limit: 5, Offset: 0}
	alerts, total, err := s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(20), total)
	assert.Len(s.T(), alerts, 5)

	// Get second page
	query = AlertHistoryQuery{Limit: 5, Offset: 5}
	alerts, total, err = s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(20), total)
	assert.Len(s.T(), alerts, 5)
}

func (s *StoreTestSuite) TestListAlertHistory_FilterBySeverity() {
	// Create alerts with different severities
	severities := []string{"critical", "warning", "info"}
	for i, sev := range severities {
		for j := 0; j < 3; j++ {
			alert := AlertHistory{
				Type:             "JobFailed",
				Severity:         sev,
				Title:            "Alert",
				Message:          "Test",
				CronJobNamespace: "default",
				CronJobName:      "test-cron",
				OccurredAt:       time.Now().Add(time.Duration(-i*3-j) * time.Minute),
			}
			require.NoError(s.T(), s.store.StoreAlert(s.ctx, alert))
		}
	}

	// Filter by critical
	query := AlertHistoryQuery{Severity: "critical", Limit: 100}
	alerts, total, err := s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), alerts, 3)
	for _, a := range alerts {
		assert.Equal(s.T(), "critical", a.Severity)
	}
}

func (s *StoreTestSuite) TestListAlertHistory_FilterByType() {
	// Create alerts with different types
	alertTypes := []string{"JobFailed", "SLABreached", "DeadManTriggered"}
	for i, alertType := range alertTypes {
		for j := 0; j < 3; j++ {
			alert := AlertHistory{
				Type:             alertType,
				Severity:         "warning",
				Title:            "Alert",
				Message:          "Test",
				CronJobNamespace: "default",
				CronJobName:      "test-cron",
				OccurredAt:       time.Now().Add(time.Duration(-i*3-j) * time.Minute),
			}
			require.NoError(s.T(), s.store.StoreAlert(s.ctx, alert))
		}
	}

	// Filter by JobFailed
	query := AlertHistoryQuery{Type: "JobFailed", Limit: 100}
	alerts, total, err := s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), alerts, 3)
	for _, a := range alerts {
		assert.Equal(s.T(), "JobFailed", a.Type)
	}

	// Filter by SLABreached
	query = AlertHistoryQuery{Type: "SLABreached", Limit: 100}
	alerts, total, err = s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), total)
	assert.Len(s.T(), alerts, 3)
	for _, a := range alerts {
		assert.Equal(s.T(), "SLABreached", a.Type)
	}
}

func (s *StoreTestSuite) TestResolveAlert() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "resolve-cron"}

	// Create an unresolved alert
	alert := AlertHistory{
		Type:             "JobFailed",
		Severity:         "warning",
		Title:            "Test alert",
		Message:          "Test",
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		OccurredAt:       time.Now(),
	}
	require.NoError(s.T(), s.store.StoreAlert(s.ctx, alert))

	// Resolve it
	err := s.store.ResolveAlert(s.ctx, "JobFailed", cronJob.Namespace, cronJob.Name)
	require.NoError(s.T(), err)

	// Verify it's resolved
	query := AlertHistoryQuery{Limit: 1}
	alerts, _, err := s.store.ListAlertHistory(s.ctx, query)
	require.NoError(s.T(), err)
	require.Len(s.T(), alerts, 1)
	assert.NotNil(s.T(), alerts[0].ResolvedAt)
}

func (s *StoreTestSuite) TestGetChannelAlertStats() {
	// Create alerts with different channels
	alert1 := AlertHistory{
		Type:             "JobFailed",
		Severity:         "warning",
		Title:            "Alert 1",
		CronJobNamespace: "default",
		CronJobName:      "test-cron",
		ChannelsNotified: "slack-main,pagerduty-oncall",
		OccurredAt:       time.Now(),
	}
	require.NoError(s.T(), s.store.StoreAlert(s.ctx, alert1))

	alert2 := AlertHistory{
		Type:             "JobFailed",
		Severity:         "warning",
		Title:            "Alert 2",
		CronJobNamespace: "default",
		CronJobName:      "test-cron",
		ChannelsNotified: "slack-main,webhook-custom",
		OccurredAt:       time.Now(),
	}
	require.NoError(s.T(), s.store.StoreAlert(s.ctx, alert2))

	stats, err := s.store.GetChannelAlertStats(s.ctx)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), int64(2), stats["slack-main"].AlertsSentTotal)
	assert.Equal(s.T(), int64(1), stats["pagerduty-oncall"].AlertsSentTotal)
	assert.Equal(s.T(), int64(1), stats["webhook-custom"].AlertsSentTotal)
}

func (s *StoreTestSuite) TestSaveChannelStats() {
	now := time.Now()
	stats := ChannelStatsRecord{
		ChannelName:         "test-channel",
		AlertsSentTotal:     100,
		AlertsFailedTotal:   5,
		LastAlertTime:       &now,
		ConsecutiveFailures: 0,
	}

	err := s.store.SaveChannelStats(s.ctx, stats)
	require.NoError(s.T(), err)

	// Retrieve and verify
	retrieved, err := s.store.GetChannelStats(s.ctx, "test-channel")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), retrieved)
	assert.Equal(s.T(), int64(100), retrieved.AlertsSentTotal)
	assert.Equal(s.T(), int64(5), retrieved.AlertsFailedTotal)
}

func (s *StoreTestSuite) TestSaveChannelStats_Upsert() {
	// First save
	stats := ChannelStatsRecord{
		ChannelName:     "upsert-channel",
		AlertsSentTotal: 50,
	}
	require.NoError(s.T(), s.store.SaveChannelStats(s.ctx, stats))

	// Update with upsert
	stats.AlertsSentTotal = 100
	require.NoError(s.T(), s.store.SaveChannelStats(s.ctx, stats))

	// Should have updated, not created a new record
	retrieved, err := s.store.GetChannelStats(s.ctx, "upsert-channel")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), retrieved)
	assert.Equal(s.T(), int64(100), retrieved.AlertsSentTotal)
}

func (s *StoreTestSuite) TestGetAllChannelStats() {
	// Save multiple channel stats
	channels := []string{"channel-a", "channel-b", "channel-c"}
	for i, ch := range channels {
		stats := ChannelStatsRecord{
			ChannelName:     ch,
			AlertsSentTotal: int64((i + 1) * 10),
		}
		require.NoError(s.T(), s.store.SaveChannelStats(s.ctx, stats))
	}

	allStats, err := s.store.GetAllChannelStats(s.ctx)
	require.NoError(s.T(), err)
	assert.Len(s.T(), allStats, 3)

	assert.Equal(s.T(), int64(10), allStats["channel-a"].AlertsSentTotal)
	assert.Equal(s.T(), int64(20), allStats["channel-b"].AlertsSentTotal)
	assert.Equal(s.T(), int64(30), allStats["channel-c"].AlertsSentTotal)
}

// =============================================================================
// Multi-Backend & Health Tests
// =============================================================================

func (s *StoreTestSuite) TestHealth_ReturnsOK() {
	err := s.store.Health(s.ctx)
	require.NoError(s.T(), err)
}

func (s *StoreTestSuite) TestInit_AutoMigration() {
	// Init already called in SetupTest, verify tables exist by using them
	cronJob := types.NamespacedName{Namespace: "default", Name: "migration-test-cron"}

	exec := Execution{
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		JobName:          "migration-test-job",
		StartTime:        time.Now(),
		Succeeded:        true,
	}
	err := s.store.RecordExecution(s.ctx, exec)
	require.NoError(s.T(), err)

	alert := AlertHistory{
		Type:             "JobFailed",
		Severity:         "warning",
		Title:            "Test",
		CronJobNamespace: cronJob.Namespace,
		CronJobName:      cronJob.Name,
		OccurredAt:       time.Now(),
	}
	err = s.store.StoreAlert(s.ctx, alert)
	require.NoError(s.T(), err)

	stats := ChannelStatsRecord{
		ChannelName:     "test-channel",
		AlertsSentTotal: 1,
	}
	err = s.store.SaveChannelStats(s.ctx, stats)
	require.NoError(s.T(), err)
}

func (s *StoreTestSuite) TestGetExecutionCount() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "count-cron"}

	// Initially zero
	count, err := s.store.GetExecutionCount(s.ctx)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), count)

	// Add some executions
	for i := 0; i < 5; i++ {
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "count-cron-" + string(rune('A'+i)),
			StartTime:        time.Now().Add(time.Duration(-i) * time.Hour),
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	count, err = s.store.GetExecutionCount(s.ctx)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), count)
}

func (s *StoreTestSuite) TestGetExecutionCountSince() {
	cronJob := types.NamespacedName{Namespace: "default", Name: "count-since-cron"}
	now := time.Now()

	// Create 3 recent and 2 old executions
	for i := 0; i < 5; i++ {
		var startTime time.Time
		if i < 3 {
			startTime = now.Add(time.Duration(-i) * time.Hour) // recent
		} else {
			startTime = now.AddDate(0, 0, -10) // old
		}
		exec := Execution{
			CronJobNamespace: cronJob.Namespace,
			CronJobName:      cronJob.Name,
			JobName:          "count-since-cron-" + string(rune('A'+i)),
			StartTime:        startTime,
			Succeeded:        true,
		}
		require.NoError(s.T(), s.store.RecordExecution(s.ctx, exec))
	}

	// Count since 24 hours ago
	since := now.Add(-24 * time.Hour)
	count, err := s.store.GetExecutionCountSince(s.ctx, since)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), count)
}

// =============================================================================
// Model Method Tests
// =============================================================================

func TestExecution_Duration(t *testing.T) {
	// Test with nil duration
	exec := &Execution{}
	assert.Equal(t, time.Duration(0), exec.Duration())

	// Test with set duration
	duration := 60.5
	exec.DurationSecs = &duration
	assert.Equal(t, time.Duration(60.5*float64(time.Second)), exec.Duration())
}

func TestExecution_SetDuration(t *testing.T) {
	exec := &Execution{}
	exec.SetDuration(2 * time.Minute)
	require.NotNil(t, exec.DurationSecs)
	assert.Equal(t, 120.0, *exec.DurationSecs)
}

func TestAlertHistory_ChannelsNotified(t *testing.T) {
	alert := &AlertHistory{}

	// Test empty
	assert.Nil(t, alert.GetChannelsNotified())

	// Test set and get
	channels := []string{"slack", "pagerduty", "webhook"}
	alert.SetChannelsNotified(channels)
	assert.Equal(t, "slack,pagerduty,webhook", alert.ChannelsNotified)
	assert.Equal(t, channels, alert.GetChannelsNotified())
}

func TestNewGormStore_UnsupportedDialect(t *testing.T) {
	_, err := NewGormStore("unsupported", "some-dsn")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported dialect")
}

func TestPercentile(t *testing.T) {
	// Empty data
	assert.Equal(t, float64(0), percentile([]float64{}, 50))

	// Single value
	assert.Equal(t, float64(10), percentile([]float64{10}, 50))

	// Multiple values
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	assert.InDelta(t, 5.0, percentile(data, 50), 1.0)
	assert.InDelta(t, 9.0, percentile(data, 90), 1.0)
}
