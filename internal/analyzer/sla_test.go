package analyzer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// mockStore implements store.Store for testing
// Note: This is kept local to avoid import cycles with testutil package
type mockStore struct {
	SuccessRate             float64
	GetSuccessRateError     error
	LastExecution           *store.Execution
	GetLastExecutionError   error
	LastSuccessExec         *store.Execution
	GetLastSuccessfulError  error
	Metrics                 *store.Metrics
	GetMetricsError         error
	DurationPercentile      time.Duration
	DurationPercentileError error
	DurationPercentileMap   map[int]time.Duration
}

func (m *mockStore) Init() error                                                { return nil }
func (m *mockStore) Close() error                                               { return nil }
func (m *mockStore) Health(_ context.Context) error                             { return nil }
func (m *mockStore) RecordExecution(_ context.Context, _ store.Execution) error { return nil }
func (m *mockStore) GetExecutions(_ context.Context, _ types.NamespacedName, _ time.Time) ([]store.Execution, error) {
	return nil, nil
}
func (m *mockStore) GetExecutionsPaginated(_ context.Context, _ types.NamespacedName, _ time.Time, _, _ int) ([]store.Execution, int64, error) {
	return nil, 0, nil
}
func (m *mockStore) GetExecutionsFiltered(_ context.Context, _ types.NamespacedName, _ time.Time, _ string, _, _ int) ([]store.Execution, int64, error) {
	return nil, 0, nil
}
func (m *mockStore) GetLastExecution(_ context.Context, _ types.NamespacedName) (*store.Execution, error) {
	return m.LastExecution, m.GetLastExecutionError
}
func (m *mockStore) GetLastSuccessfulExecution(_ context.Context, _ types.NamespacedName) (*store.Execution, error) {
	return m.LastSuccessExec, m.GetLastSuccessfulError
}
func (m *mockStore) GetExecutionByJobName(_ context.Context, _, _ string) (*store.Execution, error) {
	return nil, nil
}
func (m *mockStore) GetMetrics(_ context.Context, _ types.NamespacedName, _ int) (*store.Metrics, error) {
	return m.Metrics, m.GetMetricsError
}
func (m *mockStore) GetDurationPercentile(_ context.Context, _ types.NamespacedName, percentile, _ int) (time.Duration, error) {
	if m.DurationPercentileMap != nil {
		if d, ok := m.DurationPercentileMap[percentile]; ok {
			return d, m.DurationPercentileError
		}
	}
	return m.DurationPercentile, m.DurationPercentileError
}
func (m *mockStore) GetSuccessRate(_ context.Context, _ types.NamespacedName, _ int) (float64, error) {
	return m.SuccessRate, m.GetSuccessRateError
}
func (m *mockStore) Prune(_ context.Context, _ time.Time) (int64, error)     { return 0, nil }
func (m *mockStore) PruneLogs(_ context.Context, _ time.Time) (int64, error) { return 0, nil }
func (m *mockStore) DeleteExecutionsByCronJob(_ context.Context, _ types.NamespacedName) (int64, error) {
	return 0, nil
}
func (m *mockStore) DeleteExecutionsByUID(_ context.Context, _ types.NamespacedName, _ string) (int64, error) {
	return 0, nil
}
func (m *mockStore) GetCronJobUIDs(_ context.Context, _ types.NamespacedName) ([]string, error) {
	return nil, nil
}
func (m *mockStore) GetExecutionCount(_ context.Context) (int64, error) { return 0, nil }
func (m *mockStore) GetExecutionCountSince(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockStore) StoreAlert(_ context.Context, _ store.AlertHistory) error { return nil }
func (m *mockStore) ListAlertHistory(_ context.Context, _ store.AlertHistoryQuery) ([]store.AlertHistory, int64, error) {
	return nil, 0, nil
}
func (m *mockStore) ResolveAlert(_ context.Context, _, _, _ string) error { return nil }
func (m *mockStore) GetChannelAlertStats(_ context.Context) (map[string]store.ChannelAlertStats, error) {
	return nil, nil
}
func (m *mockStore) SaveChannelStats(_ context.Context, _ store.ChannelStatsRecord) error {
	return nil
}
func (m *mockStore) GetChannelStats(_ context.Context, _ string) (*store.ChannelStatsRecord, error) {
	return nil, nil
}
func (m *mockStore) GetAllChannelStats(_ context.Context) (map[string]*store.ChannelStatsRecord, error) {
	return nil, nil
}

// =============================================================================
// GetMetrics Tests
// =============================================================================

func TestGetMetrics_AllFields(t *testing.T) {
	mockMetrics := &store.Metrics{
		SuccessRate:        95.5,
		TotalRuns:          100,
		SuccessfulRuns:     95,
		FailedRuns:         5,
		AvgDurationSeconds: 60.5,
		P50DurationSeconds: 55.0,
		P95DurationSeconds: 90.0,
		P99DurationSeconds: 120.0,
	}

	ms := &mockStore{Metrics: mockMetrics}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	metrics, err := analyzer.GetMetrics(context.Background(), cronJob, 7)

	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Equal(t, 95.5, metrics.SuccessRate)
	assert.Equal(t, int32(100), metrics.TotalRuns)
	assert.Equal(t, int32(95), metrics.SuccessfulRuns)
	assert.Equal(t, int32(5), metrics.FailedRuns)
	assert.Equal(t, 60.5, metrics.AvgDurationSeconds)
	assert.Equal(t, 55.0, metrics.P50DurationSeconds)
	assert.Equal(t, 90.0, metrics.P95DurationSeconds)
	assert.Equal(t, 120.0, metrics.P99DurationSeconds)
}

func TestGetMetrics_StoreError(t *testing.T) {
	ms := &mockStore{GetMetricsError: errors.New("database error")}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	metrics, err := analyzer.GetMetrics(context.Background(), cronJob, 7)

	assert.Error(t, err)
	assert.Nil(t, metrics)
	assert.Contains(t, err.Error(), "database error")
}

// =============================================================================
// CheckSLA Tests
// =============================================================================

func TestCheckSLA_Passed(t *testing.T) {
	ms := &mockStore{SuccessRate: 98.0}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	minRate := 95.0
	config := &v1alpha1.SLAConfig{
		MinSuccessRate: &minRate,
	}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
	assert.Empty(t, result.Violations)
	assert.Equal(t, 98.0, result.SuccessRate)
	assert.Equal(t, 95.0, result.MinRequired)
}

func TestCheckSLA_Failed_SuccessRate(t *testing.T) {
	ms := &mockStore{SuccessRate: 80.0}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	minRate := 95.0
	config := &v1alpha1.SLAConfig{
		MinSuccessRate: &minRate,
	}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Len(t, result.Violations, 1)
	assert.Equal(t, "SuccessRate", result.Violations[0].Type)
	assert.Equal(t, 80.0, result.Violations[0].Current)
	assert.Equal(t, 95.0, result.Violations[0].Threshold)
	assert.Contains(t, result.Violations[0].Message, "80.0%")
}

func TestCheckSLA_Failed_MaxDuration(t *testing.T) {
	duration := 120.0 // 2 minutes
	lastExec := &store.Execution{
		DurationSecs:   &duration,
		CompletionTime: time.Now(),
	}
	ms := &mockStore{
		SuccessRate:   100.0,
		LastExecution: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	maxDuration := metav1.Duration{Duration: 60 * time.Second} // 1 minute max
	config := &v1alpha1.SLAConfig{
		MaxDuration: &maxDuration,
	}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed)
	assert.Len(t, result.Violations, 1)
	assert.Equal(t, "MaxDuration", result.Violations[0].Type)
	assert.Contains(t, result.Violations[0].Message, "exceeded max")
}

func TestCheckSLA_DefaultThresholds(t *testing.T) {
	// Test that defaults are 95% success rate and 7 day window
	ms := &mockStore{SuccessRate: 94.0}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{} // Empty config, use defaults

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Passed) // 94% < 95% default
	assert.Equal(t, 95.0, result.MinRequired)
}

func TestCheckSLA_CustomWindow(t *testing.T) {
	ms := &mockStore{SuccessRate: 100.0}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	windowDays := int32(30)
	config := &v1alpha1.SLAConfig{
		WindowDays: &windowDays,
	}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
}

func TestCheckSLA_NoExecutions(t *testing.T) {
	// GetSuccessRate returns 100% when no data (assume healthy)
	ms := &mockStore{SuccessRate: 100.0}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed)
}

func TestCheckSLA_NilConfig(t *testing.T) {
	ms := &mockStore{}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Passed) // Nil config means SLA tracking disabled
}

func TestCheckSLA_StoreError(t *testing.T) {
	ms := &mockStore{GetSuccessRateError: errors.New("store error")}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckSLA(context.Background(), cronJob, config)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// =============================================================================
// CheckDeadManSwitch Tests
// =============================================================================

func TestDeadManSwitch_NotTriggered(t *testing.T) {
	completionTime := time.Now().Add(-10 * time.Minute)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	maxTime := metav1.Duration{Duration: 2 * time.Hour}
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled:                 &enabled,
		MaxTimeSinceLastSuccess: &maxTime,
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Triggered)
}

func TestDeadManSwitch_Triggered(t *testing.T) {
	// Last execution was 3 hours ago
	completionTime := time.Now().Add(-3 * time.Hour)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *",
		},
	}

	enabled := true
	maxTime := metav1.Duration{Duration: 2 * time.Hour}
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled:                 &enabled,
		MaxTimeSinceLastSuccess: &maxTime,
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Triggered)
	assert.Contains(t, result.Message, "No jobs have run")
}

func TestDeadManSwitch_Disabled(t *testing.T) {
	ms := &mockStore{}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *",
		},
	}

	enabled := false
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Triggered)
}

func TestDeadManSwitch_NilConfig(t *testing.T) {
	ms := &mockStore{}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Triggered)
}

func TestDeadManSwitch_AutoFromSchedule(t *testing.T) {
	// Last execution was 3 hours ago
	completionTime := time.Now().Add(-3 * time.Hour)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	autoEnabled := true
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Triggered) // 3h > 1h + 1h buffer
}

func TestDeadManSwitch_WithBuffer(t *testing.T) {
	// Last execution was 90 minutes ago
	completionTime := time.Now().Add(-90 * time.Minute)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	autoEnabled := true
	buffer := metav1.Duration{Duration: 2 * time.Hour} // 2 hour buffer
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
			Buffer:  &buffer,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Triggered) // 90m < 1h + 2h buffer
}

func TestDeadManSwitch_MissedScheduleCount(t *testing.T) {
	// Last execution was 5 hours ago
	completionTime := time.Now().Add(-5 * time.Hour)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	maxTime := metav1.Duration{Duration: 2 * time.Hour}
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled:                 &enabled,
		MaxTimeSinceLastSuccess: &maxTime,
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Triggered)
	// 5 hours / 2 hours = 2 missed schedules
	assert.GreaterOrEqual(t, result.MissedScheduleCount, int32(2))
}

func TestDeadManSwitch_NoExecutions_NewCronJob(t *testing.T) {
	ms := &mockStore{
		LastExecution: nil, // No executions
	}
	analyzer := NewSLAAnalyzer(ms)

	// Created very recently
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	maxTime := metav1.Duration{Duration: 2 * time.Hour}
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled:                 &enabled,
		MaxTimeSinceLastSuccess: &maxTime,
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Triggered) // Too new, hasn't had time to run
}

func TestDeadManSwitch_NoExecutions_OldCronJob(t *testing.T) {
	ms := &mockStore{
		LastExecution: nil, // No executions
	}
	analyzer := NewSLAAnalyzer(ms)

	// Created 5 hours ago but no executions
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *",
		},
	}

	enabled := true
	maxTime := metav1.Duration{Duration: 2 * time.Hour}
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled:                 &enabled,
		MaxTimeSinceLastSuccess: &maxTime,
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Triggered)
	assert.Contains(t, result.Message, "No jobs have run since creation")
}

// =============================================================================
// Cron Schedule Parsing Tests
// =============================================================================

func TestDeadManSwitch_CronParsing_Daily(t *testing.T) {
	completionTime := time.Now().Add(-2 * time.Hour)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-48 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * *", // daily at midnight
		},
	}

	enabled := true
	autoEnabled := true
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Expected interval is 24h + 1h buffer = 25h
	// 2h since last run < 25h, so not triggered
	assert.False(t, result.Triggered)
	assert.InDelta(t, 25*time.Hour, result.ExpectedInterval, float64(time.Hour))
}

func TestDeadManSwitch_CronParsing_Hourly(t *testing.T) {
	completionTime := time.Now().Add(-30 * time.Minute)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hourly-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	autoEnabled := true
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Expected interval is 1h + 1h buffer = 2h
	assert.InDelta(t, 2*time.Hour, result.ExpectedInterval, float64(time.Minute))
}

func TestDeadManSwitch_CronParsing_Weekly(t *testing.T) {
	completionTime := time.Now().Add(-24 * time.Hour)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "weekly-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-14 * 24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * 0", // Every Sunday at midnight
		},
	}

	enabled := true
	autoEnabled := true
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Expected interval is ~7 days + 1h buffer
	assert.InDelta(t, 7*24*time.Hour+time.Hour, result.ExpectedInterval, float64(time.Hour))
}

func TestDeadManSwitch_CronParsing_Complex(t *testing.T) {
	completionTime := time.Now().Add(-10 * time.Minute)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "complex-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/15 * * * *", // Every 15 minutes
		},
	}

	enabled := true
	autoEnabled := true
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Expected interval is 15m + 1h buffer = 75 minutes
	assert.InDelta(t, 75*time.Minute, result.ExpectedInterval, float64(time.Minute))
}

func TestDeadManSwitch_CachesSchedule(t *testing.T) {
	// Use a unique schedule string for this test to avoid conflicts with other tests
	testSchedule := "0 */6 * * *"

	// Clear the test schedule from cache if present
	cache := getScheduleCache()
	cache.Remove(testSchedule)

	ms := &mockStore{
		LastExecution: &store.Execution{
			CompletionTime: time.Now(),
		},
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cache-test-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: testSchedule, // Every 6 hours
		},
	}

	enabled := true
	autoEnabled := true
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled: autoEnabled,
		},
	}

	// First call should parse and cache
	_, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)
	require.NoError(t, err)

	// Verify cache has the schedule
	_, cached := cache.Get(testSchedule)
	assert.True(t, cached, "Schedule should be cached after first parse")

	// Second call should use cache
	_, err = analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)
	require.NoError(t, err)
}

func TestDeadManSwitch_MissedScheduleThreshold(t *testing.T) {
	// Last execution was 5 hours ago
	completionTime := time.Now().Add(-5 * time.Hour)
	lastExec := &store.Execution{
		CompletionTime: completionTime,
	}
	ms := &mockStore{
		LastExecution:   lastExec,
		LastSuccessExec: lastExec,
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "threshold-cron",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-24 * time.Hour)),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *", // hourly
		},
	}

	enabled := true
	autoEnabled := true
	threshold := int32(5) // Only alert after 5 missed schedules
	config := &v1alpha1.DeadManSwitchConfig{
		Enabled: &enabled,
		AutoFromSchedule: &v1alpha1.AutoScheduleConfig{
			Enabled:                 autoEnabled,
			MissedScheduleThreshold: &threshold,
		},
	}

	result, err := analyzer.CheckDeadManSwitch(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	// 5h since last run, 2h expected interval = 2-3 missed schedules
	// Threshold is 5, so should NOT trigger
	assert.False(t, result.Triggered)
}

// =============================================================================
// CheckDurationRegression Tests
// =============================================================================

func TestDurationRegression_NotDetected(t *testing.T) {
	ms := &mockStore{
		DurationPercentileMap: map[int]time.Duration{
			95: 60 * time.Second, // Same for baseline and current
		},
	}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Detected)
}

func TestDurationRegression_Detected(t *testing.T) {
	// Create a store that returns different values based on the window days
	baseline := 60 * time.Second
	current := 120 * time.Second // 100% increase

	analyzer := NewSLAAnalyzer(&customMockStore{
		baselineP95: baseline,
		currentP95:  current,
	})

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Detected)
	assert.InDelta(t, 100.0, result.PercentageIncrease, 1.0)
	assert.Contains(t, result.Message, "increased")
}

func TestDurationRegression_DefaultThreshold(t *testing.T) {
	// Test 50% default threshold
	baseline := 60 * time.Second
	current := 85 * time.Second // ~42% increase (below 50%)

	analyzer := NewSLAAnalyzer(&customMockStore{
		baselineP95: baseline,
		currentP95:  current,
	})

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Detected)
	assert.Equal(t, 50.0, result.Threshold) // Default threshold
}

func TestDurationRegression_CustomThreshold(t *testing.T) {
	baseline := 60 * time.Second
	current := 75 * time.Second // 25% increase

	analyzer := NewSLAAnalyzer(&customMockStore{
		baselineP95: baseline,
		currentP95:  current,
	})

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	threshold := int32(20) // 20% threshold
	config := &v1alpha1.SLAConfig{
		DurationRegressionThreshold: &threshold,
	}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Detected) // 25% > 20%
	assert.Equal(t, 20.0, result.Threshold)
}

func TestDurationRegression_BaselineWindow(t *testing.T) {
	// Just verify the config is respected (no error)
	analyzer := NewSLAAnalyzer(&customMockStore{
		baselineP95: 60 * time.Second,
		currentP95:  60 * time.Second,
	})

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	baselineWindow := int32(30) // 30 day baseline
	config := &v1alpha1.SLAConfig{
		DurationBaselineWindowDays: &baselineWindow,
	}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDurationRegression_InsufficientData(t *testing.T) {
	// No baseline data (returns 0)
	analyzer := NewSLAAnalyzer(&customMockStore{
		baselineP95: 0,
		currentP95:  60 * time.Second,
	})

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Detected) // Can't calculate regression without baseline
}

func TestDurationRegression_Improvement(t *testing.T) {
	// Duration improved (got faster)
	baseline := 120 * time.Second
	current := 60 * time.Second

	analyzer := NewSLAAnalyzer(&customMockStore{
		baselineP95: baseline,
		currentP95:  current,
	})

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}
	config := &v1alpha1.SLAConfig{}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, config)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Detected)                // Improvement, not regression
	assert.Equal(t, 0.0, result.PercentageIncrease) // No increase
}

func TestDurationRegression_NilConfig(t *testing.T) {
	ms := &mockStore{}
	analyzer := NewSLAAnalyzer(ms)

	cronJob := types.NamespacedName{Namespace: "default", Name: "test-cron"}

	result, err := analyzer.CheckDurationRegression(context.Background(), cronJob, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Detected)
}

// =============================================================================
// Helper Tests
// =============================================================================

func TestIsEnabled(t *testing.T) {
	assert.True(t, isEnabled(nil))

	enabled := true
	assert.True(t, isEnabled(&enabled))

	disabled := false
	assert.False(t, isEnabled(&disabled))
}

func TestGetOrDefaultInt32(t *testing.T) {
	assert.Equal(t, int32(42), getOrDefaultInt32(nil, 42))

	val := int32(10)
	assert.Equal(t, int32(10), getOrDefaultInt32(&val, 42))
}

func TestGetOrDefaultFloat64(t *testing.T) {
	assert.Equal(t, 95.0, getOrDefaultFloat64(nil, 95.0))

	val := 80.0
	assert.Equal(t, 80.0, getOrDefaultFloat64(&val, 95.0))
}

// customMockStore for regression tests that need different baseline vs current values
type customMockStore struct {
	mockStore
	baselineP95 time.Duration
	currentP95  time.Duration
	callCount   int
}

func (m *customMockStore) GetDurationPercentile(_ context.Context, _ types.NamespacedName, _ int, windowDays int) (time.Duration, error) {
	m.callCount++
	// First call is for baseline (larger window), second for current (1 day window)
	if windowDays == 1 {
		return m.currentP95, nil
	}
	return m.baselineP95, nil
}
