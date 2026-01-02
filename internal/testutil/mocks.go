// Package testutil provides shared test utilities and mock implementations
// for use across the cronjob-guardian test suites.
package testutil

import (
	"context"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// ============================================================================
// Mock Store Implementation
// ============================================================================

// MockStore is a configurable mock implementation of store.Store for testing.
// All fields are optional - set only what your test needs.
// Thread-safe for concurrent access in scheduler tests.
type MockStore struct {
	mu sync.Mutex

	// Health
	HealthError error

	// Executions
	Executions          []store.Execution
	ExecutionsFiltered  []store.Execution
	ExecutionsTotal     int64
	LastExecution       *store.Execution
	LastSuccessExec     *store.Execution
	ExecutionByJobName  *store.Execution
	ExecutionCount      int64
	ExecutionCountSince int64

	// Metrics
	Metrics            *store.Metrics
	DurationPercentile time.Duration
	SuccessRate        float64

	// Prune
	PrunedCount     int64
	PrunedLogsCount int64
	DeletedCount    int64

	// UIDs - map key: "namespace/name", value: list of UIDs
	CronJobUIDsMap map[string][]string
	CronJobUIDs    []string // Simple list for basic tests

	// Alerts
	AlertHistory      []store.AlertHistory
	AlertHistoryTotal int64

	// Channel Stats
	ChannelAlertStats map[string]store.ChannelAlertStats
	AllChannelStats   map[string]*store.ChannelStatsRecord
	SingleChannelStat *store.ChannelStatsRecord

	// Error injection - set these to simulate errors
	InitError                       error
	RecordExecutionError            error
	GetExecutionsError              error
	GetMetricsError                 error
	GetSuccessRateError             error
	GetLastExecutionError           error
	GetLastSuccessfulExecutionError error
	GetDurationPercentileError      error
	PruneError                      error
	PruneLogsError                  error
	StoreAlertError                 error
	ListAlertHistoryError           error
	GetChannelAlertStatsError       error
	SaveChannelStatsError           error
	GetChannelStatsError            error
	GetAllChannelStatsError         error
	DeleteExecutionsByCronJobError  error
	DeleteExecutionsByUIDError      error

	// For duration percentile tests that need different values per window
	DurationPercentileMap map[int]time.Duration // percentile -> duration

	// Call tracking - these record what was called for verification
	RecordedExecutions    []store.Execution
	DeletedUIDs           []string
	DeleteByCronJobCalled int
	DeleteByUIDCalled     int
	PruneCalled           int
	PruneCutoff           time.Time
	PruneLogsCalled       int
	LogPruneCutoff        time.Time
	ResolveAlertCalls     int
}

// Init implements store.Store
func (m *MockStore) Init() error {
	return m.InitError
}

// Close implements store.Store
func (m *MockStore) Close() error {
	return nil
}

// Health implements store.Store
func (m *MockStore) Health(_ context.Context) error {
	return m.HealthError
}

// RecordExecution implements store.Store
func (m *MockStore) RecordExecution(_ context.Context, exec store.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.RecordExecutionError != nil {
		return m.RecordExecutionError
	}
	m.RecordedExecutions = append(m.RecordedExecutions, exec)
	m.Executions = append(m.Executions, exec)
	return nil
}

// GetExecutions implements store.Store
func (m *MockStore) GetExecutions(_ context.Context, _ types.NamespacedName, _ time.Time) ([]store.Execution, error) {
	if m.GetExecutionsError != nil {
		return nil, m.GetExecutionsError
	}
	return m.Executions, nil
}

// GetExecutionsPaginated implements store.Store
func (m *MockStore) GetExecutionsPaginated(_ context.Context, _ types.NamespacedName, _ time.Time, _, _ int) ([]store.Execution, int64, error) {
	return m.Executions, m.ExecutionsTotal, nil
}

// GetExecutionsFiltered implements store.Store
func (m *MockStore) GetExecutionsFiltered(_ context.Context, _ types.NamespacedName, _ time.Time, _ string, _, _ int) ([]store.Execution, int64, error) {
	return m.ExecutionsFiltered, m.ExecutionsTotal, nil
}

// GetLastExecution implements store.Store
func (m *MockStore) GetLastExecution(_ context.Context, _ types.NamespacedName) (*store.Execution, error) {
	if m.GetLastExecutionError != nil {
		return nil, m.GetLastExecutionError
	}
	if m.LastExecution != nil {
		return m.LastExecution, nil
	}
	if len(m.Executions) > 0 {
		return &m.Executions[0], nil
	}
	return nil, nil
}

// GetLastSuccessfulExecution implements store.Store
func (m *MockStore) GetLastSuccessfulExecution(_ context.Context, _ types.NamespacedName) (*store.Execution, error) {
	if m.GetLastSuccessfulExecutionError != nil {
		return nil, m.GetLastSuccessfulExecutionError
	}
	if m.LastSuccessExec != nil {
		return m.LastSuccessExec, nil
	}
	for i := range m.Executions {
		if m.Executions[i].Succeeded {
			return &m.Executions[i], nil
		}
	}
	return nil, nil
}

// GetExecutionByJobName implements store.Store
func (m *MockStore) GetExecutionByJobName(_ context.Context, _, _ string) (*store.Execution, error) {
	if m.ExecutionByJobName != nil {
		return m.ExecutionByJobName, nil
	}
	if len(m.Executions) > 0 {
		return &m.Executions[0], nil
	}
	return nil, nil
}

// GetMetrics implements store.Store
func (m *MockStore) GetMetrics(_ context.Context, _ types.NamespacedName, _ int) (*store.Metrics, error) {
	if m.GetMetricsError != nil {
		return nil, m.GetMetricsError
	}
	return m.Metrics, nil
}

// GetDurationPercentile implements store.Store
func (m *MockStore) GetDurationPercentile(_ context.Context, _ types.NamespacedName, percentile, _ int) (time.Duration, error) {
	if m.GetDurationPercentileError != nil {
		return 0, m.GetDurationPercentileError
	}
	if m.DurationPercentileMap != nil {
		if d, ok := m.DurationPercentileMap[percentile]; ok {
			return d, nil
		}
	}
	return m.DurationPercentile, nil
}

// GetSuccessRate implements store.Store
func (m *MockStore) GetSuccessRate(_ context.Context, _ types.NamespacedName, _ int) (float64, error) {
	if m.GetSuccessRateError != nil {
		return 0, m.GetSuccessRateError
	}
	return m.SuccessRate, nil
}

// Prune implements store.Store
func (m *MockStore) Prune(_ context.Context, cutoff time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PruneCalled++
	m.PruneCutoff = cutoff
	if m.PruneError != nil {
		return 0, m.PruneError
	}
	return m.PrunedCount, nil
}

// PruneLogs implements store.Store
func (m *MockStore) PruneLogs(_ context.Context, cutoff time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PruneLogsCalled++
	m.LogPruneCutoff = cutoff
	if m.PruneLogsError != nil {
		return 0, m.PruneLogsError
	}
	return m.PrunedLogsCount, nil
}

// DeleteExecutionsByCronJob implements store.Store
func (m *MockStore) DeleteExecutionsByCronJob(_ context.Context, _ types.NamespacedName) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteByCronJobCalled++
	if m.DeleteExecutionsByCronJobError != nil {
		return 0, m.DeleteExecutionsByCronJobError
	}
	return m.DeletedCount, nil
}

// DeleteExecutionsByUID implements store.Store
func (m *MockStore) DeleteExecutionsByUID(_ context.Context, _ types.NamespacedName, uid string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteByUIDCalled++
	m.DeletedUIDs = append(m.DeletedUIDs, uid)
	if m.DeleteExecutionsByUIDError != nil {
		return 0, m.DeleteExecutionsByUIDError
	}
	return m.DeletedCount, nil
}

// GetCronJobUIDs implements store.Store
func (m *MockStore) GetCronJobUIDs(_ context.Context, cronJob types.NamespacedName) ([]string, error) {
	// Check map first for specific namespace/name lookup
	if m.CronJobUIDsMap != nil {
		key := cronJob.Namespace + "/" + cronJob.Name
		if uids, ok := m.CronJobUIDsMap[key]; ok {
			return uids, nil
		}
	}
	// Fall back to simple list
	return m.CronJobUIDs, nil
}

// GetExecutionCount implements store.Store
func (m *MockStore) GetExecutionCount(_ context.Context) (int64, error) {
	return m.ExecutionCount, nil
}

// GetExecutionCountSince implements store.Store
func (m *MockStore) GetExecutionCountSince(_ context.Context, _ time.Time) (int64, error) {
	return m.ExecutionCountSince, nil
}

// StoreAlert implements store.Store
func (m *MockStore) StoreAlert(_ context.Context, _ store.AlertHistory) error {
	return m.StoreAlertError
}

// ListAlertHistory implements store.Store
func (m *MockStore) ListAlertHistory(_ context.Context, _ store.AlertHistoryQuery) ([]store.AlertHistory, int64, error) {
	if m.ListAlertHistoryError != nil {
		return nil, 0, m.ListAlertHistoryError
	}
	return m.AlertHistory, m.AlertHistoryTotal, nil
}

// ResolveAlert implements store.Store
func (m *MockStore) ResolveAlert(_ context.Context, _, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ResolveAlertCalls++
	return nil
}

// GetChannelAlertStats implements store.Store
func (m *MockStore) GetChannelAlertStats(_ context.Context) (map[string]store.ChannelAlertStats, error) {
	if m.GetChannelAlertStatsError != nil {
		return nil, m.GetChannelAlertStatsError
	}
	if m.ChannelAlertStats == nil {
		return make(map[string]store.ChannelAlertStats), nil
	}
	return m.ChannelAlertStats, nil
}

// SaveChannelStats implements store.Store
func (m *MockStore) SaveChannelStats(_ context.Context, _ store.ChannelStatsRecord) error {
	return m.SaveChannelStatsError
}

// GetChannelStats implements store.Store
func (m *MockStore) GetChannelStats(_ context.Context, _ string) (*store.ChannelStatsRecord, error) {
	if m.GetChannelStatsError != nil {
		return nil, m.GetChannelStatsError
	}
	return m.SingleChannelStat, nil
}

// GetAllChannelStats implements store.Store
func (m *MockStore) GetAllChannelStats(_ context.Context) (map[string]*store.ChannelStatsRecord, error) {
	if m.GetAllChannelStatsError != nil {
		return nil, m.GetAllChannelStatsError
	}
	if m.AllChannelStats == nil {
		return make(map[string]*store.ChannelStatsRecord), nil
	}
	return m.AllChannelStats, nil
}

// Lock acquires the mutex for external synchronization in tests
func (m *MockStore) Lock() {
	m.mu.Lock()
}

// Unlock releases the mutex for external synchronization in tests
func (m *MockStore) Unlock() {
	m.mu.Unlock()
}

// ============================================================================
// Mock Dispatcher Implementation
// ============================================================================

// MockDispatcher is a configurable mock implementation of alerting.Dispatcher for testing.
// Thread-safe for concurrent access in scheduler tests.
type MockDispatcher struct {
	mu sync.Mutex

	// Tracking
	DispatchedAlerts      []alerting.Alert
	SentToChannel         map[string][]alerting.Alert
	SentChannelNames      []string
	SentAlerts            []alerting.Alert // All alerts sent via SendToChannel
	ClearedAlerts         []string
	CancelledAlerts       []string
	RegisteredChannels    []*guardianv1alpha1.AlertChannel
	RegisteredChannelsMap map[string]*guardianv1alpha1.AlertChannel // Map by channel name
	RemovedChannels       []string

	// Configuration
	Suppressed            bool
	SuppressionReason     string
	PendingAlertCancelled bool
	AlertCount24h         int32
	ChannelStats          map[string]*alerting.ChannelStats

	// Error injection
	DispatchError        error
	SendToChannelError   error
	ClearAlertError      error
	RegisterChannelError error
}

// NewMockDispatcher creates a new MockDispatcher with initialized maps
func NewMockDispatcher() *MockDispatcher {
	return &MockDispatcher{
		SentToChannel:         make(map[string][]alerting.Alert),
		RegisteredChannelsMap: make(map[string]*guardianv1alpha1.AlertChannel),
		ChannelStats:          make(map[string]*alerting.ChannelStats),
	}
}

// Dispatch implements alerting.Dispatcher
func (m *MockDispatcher) Dispatch(_ context.Context, alert alerting.Alert, _ *guardianv1alpha1.AlertingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.DispatchError != nil {
		return m.DispatchError
	}
	m.DispatchedAlerts = append(m.DispatchedAlerts, alert)
	return nil
}

// RegisterChannel implements alerting.Dispatcher
func (m *MockDispatcher) RegisterChannel(channel *guardianv1alpha1.AlertChannel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.RegisterChannelError != nil {
		return m.RegisterChannelError
	}
	m.RegisteredChannels = append(m.RegisteredChannels, channel)
	if m.RegisteredChannelsMap == nil {
		m.RegisteredChannelsMap = make(map[string]*guardianv1alpha1.AlertChannel)
	}
	m.RegisteredChannelsMap[channel.Name] = channel
	return nil
}

// RemoveChannel implements alerting.Dispatcher
func (m *MockDispatcher) RemoveChannel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemovedChannels = append(m.RemovedChannels, name)
	delete(m.RegisteredChannelsMap, name)
}

// SendToChannel implements alerting.Dispatcher
func (m *MockDispatcher) SendToChannel(_ context.Context, channelName string, alert alerting.Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendToChannelError != nil {
		return m.SendToChannelError
	}
	if m.SentToChannel == nil {
		m.SentToChannel = make(map[string][]alerting.Alert)
	}
	m.SentToChannel[channelName] = append(m.SentToChannel[channelName], alert)
	m.SentChannelNames = append(m.SentChannelNames, channelName)
	m.SentAlerts = append(m.SentAlerts, alert)
	return nil
}

// IsSuppressed implements alerting.Dispatcher
func (m *MockDispatcher) IsSuppressed(_ alerting.Alert, _ *guardianv1alpha1.AlertingConfig) (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Suppressed, m.SuppressionReason
}

// ClearAlert implements alerting.Dispatcher
func (m *MockDispatcher) ClearAlert(_ context.Context, alertKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ClearAlertError != nil {
		return m.ClearAlertError
	}
	m.ClearedAlerts = append(m.ClearedAlerts, alertKey)
	return nil
}

// ClearAlertsForMonitor implements alerting.Dispatcher
func (m *MockDispatcher) ClearAlertsForMonitor(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ClearedAlerts = append(m.ClearedAlerts, namespace+"/"+name)
}

// CancelPendingAlert implements alerting.Dispatcher
func (m *MockDispatcher) CancelPendingAlert(alertKey string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CancelledAlerts = append(m.CancelledAlerts, alertKey)
	return m.PendingAlertCancelled
}

// CancelPendingAlertsForCronJob implements alerting.Dispatcher
func (m *MockDispatcher) CancelPendingAlertsForCronJob(namespace, name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CancelledAlerts = append(m.CancelledAlerts, namespace+"/"+name)
	if m.PendingAlertCancelled {
		return 1
	}
	return 0
}

// SetGlobalRateLimits implements alerting.Dispatcher
func (m *MockDispatcher) SetGlobalRateLimits(_ config.RateLimitsConfig) {}

// GetAlertCount24h implements alerting.Dispatcher
func (m *MockDispatcher) GetAlertCount24h() int32 {
	return m.AlertCount24h
}

// GetChannelStats implements alerting.Dispatcher
func (m *MockDispatcher) GetChannelStats(name string) *alerting.ChannelStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ChannelStats == nil {
		return nil
	}
	return m.ChannelStats[name]
}

// Stop implements alerting.Dispatcher
func (m *MockDispatcher) Stop() error {
	return nil
}

// Lock acquires the mutex for external synchronization in tests
func (m *MockDispatcher) Lock() {
	m.mu.Lock()
}

// Unlock releases the mutex for external synchronization in tests
func (m *MockDispatcher) Unlock() {
	m.mu.Unlock()
}

// ============================================================================
// Mock Analyzer Implementation
// ============================================================================

// MockAnalyzer is a configurable mock implementation of analyzer.SLAAnalyzer for testing.
// Thread-safe for concurrent access in scheduler tests.
type MockAnalyzer struct {
	mu sync.Mutex

	// SLA Check results
	SLAResult *analyzer.SLAResult

	// Dead Man Switch results
	DeadManResult *analyzer.DeadManResult

	// Regression results
	RegressionResult *analyzer.RegressionResult

	// Metrics
	Metrics *guardianv1alpha1.CronJobMetrics

	// Error injection
	SLAError        error
	DeadManError    error
	RegressionError error
	MetricsError    error

	// Call tracking
	GetMetricsCalled         int
	CheckSLACalled           int
	CheckDeadManSwitchCalled int
	CheckRegressionCalled    int
}

// GetMetrics implements analyzer.SLAAnalyzer
func (m *MockAnalyzer) GetMetrics(_ context.Context, _ types.NamespacedName, _ int) (*guardianv1alpha1.CronJobMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetMetricsCalled++
	if m.MetricsError != nil {
		return nil, m.MetricsError
	}
	if m.Metrics != nil {
		return m.Metrics, nil
	}
	return &guardianv1alpha1.CronJobMetrics{SuccessRate: 100}, nil
}

// CheckSLA implements analyzer.SLAAnalyzer
func (m *MockAnalyzer) CheckSLA(_ context.Context, _ types.NamespacedName, _ *guardianv1alpha1.SLAConfig) (*analyzer.SLAResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CheckSLACalled++
	if m.SLAError != nil {
		return nil, m.SLAError
	}
	if m.SLAResult != nil {
		return m.SLAResult, nil
	}
	return &analyzer.SLAResult{Passed: true}, nil
}

// CheckDeadManSwitch implements analyzer.SLAAnalyzer
func (m *MockAnalyzer) CheckDeadManSwitch(_ context.Context, _ *batchv1.CronJob, _ *guardianv1alpha1.DeadManSwitchConfig) (*analyzer.DeadManResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CheckDeadManSwitchCalled++
	if m.DeadManError != nil {
		return nil, m.DeadManError
	}
	if m.DeadManResult != nil {
		return m.DeadManResult, nil
	}
	return &analyzer.DeadManResult{Triggered: false}, nil
}

// CheckDurationRegression implements analyzer.SLAAnalyzer
func (m *MockAnalyzer) CheckDurationRegression(_ context.Context, _ types.NamespacedName, _ *guardianv1alpha1.SLAConfig) (*analyzer.RegressionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CheckRegressionCalled++
	if m.RegressionError != nil {
		return nil, m.RegressionError
	}
	if m.RegressionResult != nil {
		return m.RegressionResult, nil
	}
	return &analyzer.RegressionResult{Detected: false}, nil
}

// Lock acquires the mutex for external synchronization in tests
func (m *MockAnalyzer) Lock() {
	m.mu.Lock()
}

// Unlock releases the mutex for external synchronization in tests
func (m *MockAnalyzer) Unlock() {
	m.mu.Unlock()
}
