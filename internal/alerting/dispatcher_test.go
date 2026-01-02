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

package alerting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// mockChannel implements the Channel interface for testing
type mockChannel struct {
	name       string
	chanType   string
	sendErr    error
	testErr    error
	sentAlerts []Alert
	mu         sync.Mutex
}

func newMockChannel(name, chanType string) *mockChannel {
	return &mockChannel{
		name:       name,
		chanType:   chanType,
		sentAlerts: make([]Alert, 0),
	}
}

func (m *mockChannel) Name() string { return m.name }
func (m *mockChannel) Type() string { return m.chanType }

func (m *mockChannel) Send(_ context.Context, alert Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentAlerts = append(m.sentAlerts, alert)
	return nil
}

func (m *mockChannel) Test(_ context.Context) error {
	return m.testErr
}

func (m *mockChannel) GetSentAlerts() []Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Alert, len(m.sentAlerts))
	copy(result, m.sentAlerts)
	return result
}

func (m *mockChannel) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentAlerts = m.sentAlerts[:0]
}

// mockStore implements the store.Store interface for testing
type mockStore struct {
	alerts       []store.AlertHistory
	channelStats map[string]*store.ChannelStatsRecord
	mu           sync.Mutex
}

func newMockStore() *mockStore {
	return &mockStore{
		alerts:       make([]store.AlertHistory, 0),
		channelStats: make(map[string]*store.ChannelStatsRecord),
	}
}

func (m *mockStore) Init() error                                                { return nil }
func (m *mockStore) Health(_ context.Context) error                             { return nil }
func (m *mockStore) Close() error                                               { return nil }
func (m *mockStore) RecordExecution(_ context.Context, _ store.Execution) error { return nil }
func (m *mockStore) GetExecutions(_ context.Context, _ types.NamespacedName, _ time.Time) ([]store.Execution, error) {
	return nil, nil
}
func (m *mockStore) GetExecutionsPaginated(_ context.Context, _ types.NamespacedName, _ time.Time, _, _ int) ([]store.Execution, int64, error) {
	return nil, 0, nil
}
func (m *mockStore) GetExecutionsFiltered(_ context.Context, _ types.NamespacedName, _ time.Time, _ string, _, _ int) (
	[]store.Execution, int64, error,
) {
	return nil, 0, nil
}
func (m *mockStore) GetLastExecution(_ context.Context, _ types.NamespacedName) (*store.Execution, error) {
	return nil, nil
}
func (m *mockStore) GetLastSuccessfulExecution(_ context.Context, _ types.NamespacedName) (*store.Execution, error) {
	return nil, nil
}
func (m *mockStore) GetExecutionByJobName(_ context.Context, _, _ string) (*store.Execution, error) {
	return nil, nil
}
func (m *mockStore) GetSuccessRate(_ context.Context, _ types.NamespacedName, _ int) (float64, error) {
	return 0, nil
}
func (m *mockStore) GetDurationPercentile(_ context.Context, _ types.NamespacedName, _, _ int) (time.Duration, error) {
	return 0, nil
}
func (m *mockStore) GetMetrics(_ context.Context, _ types.NamespacedName, _ int) (*store.Metrics, error) {
	return nil, nil
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

func (m *mockStore) StoreAlert(_ context.Context, alert store.AlertHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, alert)
	return nil
}

func (m *mockStore) ListAlertHistory(_ context.Context, _ store.AlertHistoryQuery) ([]store.AlertHistory, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alerts, int64(len(m.alerts)), nil
}

func (m *mockStore) ResolveAlert(_ context.Context, _, _, _ string) error { return nil }

func (m *mockStore) GetChannelAlertStats(_ context.Context) (map[string]store.ChannelAlertStats, error) {
	return nil, nil
}

func (m *mockStore) SaveChannelStats(_ context.Context, record store.ChannelStatsRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channelStats[record.ChannelName] = &record
	return nil
}

func (m *mockStore) GetChannelStats(_ context.Context, name string) (*store.ChannelStatsRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if stats, ok := m.channelStats[name]; ok {
		return stats, nil
	}
	return nil, nil
}

func (m *mockStore) GetAllChannelStats(_ context.Context) (map[string]*store.ChannelStatsRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]*store.ChannelStatsRecord)
	for k, v := range m.channelStats {
		result[k] = v
	}
	return result, nil
}

// testDispatcher creates a dispatcher for testing with no grace period
func testDispatcher(s store.Store) *dispatcher {
	d := &dispatcher{
		channels:           make(map[string]Channel),
		channelStats:       make(map[string]*ChannelStats),
		sentAlerts:         make(map[string]time.Time),
		activeAlerts:       make(map[string]Alert),
		pendingAlerts:      make(map[string]*PendingAlert),
		globalLimiter:      rate.NewLimiter(rate.Inf, 100),
		cleanupDone:        make(chan struct{}),
		startupGracePeriod: 0,
		readyAt:            time.Now().Add(-time.Second),
		store:              s,
	}
	return d
}

// Helper to create test alerting config
func testAlertingConfig(channelNames ...string) *v1alpha1.AlertingConfig {
	enabled := true
	refs := make([]v1alpha1.ChannelRef, 0, len(channelNames))
	for _, name := range channelNames {
		refs = append(refs, v1alpha1.ChannelRef{Name: name})
	}
	return &v1alpha1.AlertingConfig{
		Enabled:     &enabled,
		ChannelRefs: refs,
	}
}

// Helper to create test alert
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func testAlert(namespace, name, alertType, severity string) Alert {
	return Alert{
		Key:      namespace + "/" + name + "/" + alertType,
		Type:     alertType,
		Severity: severity,
		Title:    "Test Alert",
		Message:  "Test message",
		CronJob:  types.NamespacedName{Namespace: namespace, Name: name},
		MonitorRef: types.NamespacedName{
			Namespace: namespace,
			Name:      name + "-monitor",
		},
		Timestamp: time.Now(),
	}
}

// ==================== Dispatcher Tests ====================

func TestDispatcher_Dispatch_SingleChannel(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	sentAlerts := ch.GetSentAlerts()
	assert.Len(t, sentAlerts, 1)
	assert.Equal(t, "JobFailed", sentAlerts[0].Type)
	assert.Equal(t, "critical", sentAlerts[0].Severity)
}

func TestDispatcher_Dispatch_MultipleChannels(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	slackCh := newMockChannel("slack-main", "slack")
	pdCh := newMockChannel("pagerduty-main", "pagerduty")
	d.channels["slack-main"] = slackCh
	d.channels["pagerduty-main"] = pdCh

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main", "pagerduty-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	assert.Len(t, slackCh.GetSentAlerts(), 1)
	assert.Len(t, pdCh.GetSentAlerts(), 1)
}

func TestDispatcher_Dispatch_ChannelNotFound(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("non-existent-channel")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)
}

func TestDispatcher_Dispatch_PartialFailure(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	successCh := newMockChannel("slack-main", "slack")
	failCh := newMockChannel("pagerduty-broken", "pagerduty")
	failCh.sendErr = errors.New("connection refused")

	d.channels["slack-main"] = successCh
	d.channels["pagerduty-broken"] = failCh

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main", "pagerduty-broken")

	err := d.Dispatch(ctx, alert, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send to 1 channels")

	assert.Len(t, successCh.GetSentAlerts(), 1)
}

func TestDispatcher_Dispatch_DisabledAlerting(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	disabled := false
	cfg := &v1alpha1.AlertingConfig{
		Enabled:     &disabled,
		ChannelRefs: []v1alpha1.ChannelRef{{Name: "slack-main"}},
	}

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	assert.Len(t, ch.GetSentAlerts(), 0)
}

func TestDispatcher_Dispatch_NilConfig(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	err := d.Dispatch(ctx, alert, nil)
	require.NoError(t, err)
}

func TestDispatcher_Dispatch_GeneratesKey(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := Alert{
		Key:      "",
		Type:     "JobFailed",
		Severity: "critical",
		CronJob:  types.NamespacedName{Namespace: "prod", Name: "daily-backup"},
	}
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	d.alertMu.RLock()
	_, exists := d.sentAlerts["prod/daily-backup/JobFailed"]
	d.alertMu.RUnlock()
	assert.True(t, exists)
}

// ==================== IsSuppressed Tests ====================

func TestDispatcher_IsSuppressed_DuplicateWithinWindow(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now()
	d.activeAlerts[alert.Key] = alert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, reason := d.IsSuppressed(alert, cfg)

	assert.True(t, suppressed)
	assert.Contains(t, reason, "duplicate")
}

func TestDispatcher_IsSuppressed_DifferentAlerts(t *testing.T) {
	d := testDispatcher(nil)

	alert1 := testAlert("default", "cron-a", "JobFailed", "critical")
	alert2 := testAlert("default", "cron-b", "JobFailed", "critical")

	d.alertMu.Lock()
	d.sentAlerts[alert1.Key] = time.Now()
	d.activeAlerts[alert1.Key] = alert1
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(alert2, cfg)

	assert.False(t, suppressed)
}

func TestDispatcher_IsSuppressed_AfterSuppressionWindow(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now().Add(-2 * time.Hour)
	d.activeAlerts[alert.Key] = alert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(alert, cfg)

	assert.False(t, suppressed)
}

func TestDispatcher_IsSuppressed_CustomWindow(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now().Add(-30 * time.Minute)
	d.activeAlerts[alert.Key] = alert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	cfg.SuppressDuplicatesFor = &metav1.Duration{Duration: 2 * time.Hour}

	suppressed, _ := d.IsSuppressed(alert, cfg)
	assert.True(t, suppressed)
}

func TestDispatcher_IsSuppressed_ErrorSignatureChanged(t *testing.T) {
	d := testDispatcher(nil)

	oldAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	oldAlert.Context.ExitCode = 137

	newAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	newAlert.Context.ExitCode = 1

	d.alertMu.Lock()
	d.sentAlerts[oldAlert.Key] = time.Now()
	d.activeAlerts[oldAlert.Key] = oldAlert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(newAlert, cfg)

	assert.False(t, suppressed)
}

func TestDispatcher_IsSuppressed_SameErrorSignature(t *testing.T) {
	d := testDispatcher(nil)

	oldAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	oldAlert.Context.ExitCode = 1

	newAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	newAlert.Context.ExitCode = 2 // Still in same "app-error" category

	d.alertMu.Lock()
	d.sentAlerts[oldAlert.Key] = time.Now()
	d.activeAlerts[oldAlert.Key] = oldAlert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(newAlert, cfg)

	assert.True(t, suppressed)
}

// ==================== ClearAlert Tests ====================

func TestDispatcher_ClearAlert_RemovesFromActive(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now()
	d.activeAlerts[alert.Key] = alert
	d.alertMu.Unlock()

	ctx := context.Background()
	err := d.ClearAlert(ctx, alert.Key)
	require.NoError(t, err)

	d.alertMu.RLock()
	_, existsActive := d.activeAlerts[alert.Key]
	_, existsSent := d.sentAlerts[alert.Key]
	d.alertMu.RUnlock()

	assert.False(t, existsActive)
	assert.False(t, existsSent)
}

func TestDispatcher_ClearAlertsForMonitor_Bulk(t *testing.T) {
	d := testDispatcher(nil)

	keys := []string{
		"default/test-cron/JobFailed",
		"default/test-cron/SLABreached",
		"default/test-cron/DeadManTriggered",
		"default/other-cron/JobFailed", // Different CronJob
	}

	d.alertMu.Lock()
	for _, key := range keys {
		d.sentAlerts[key] = time.Now()
		d.activeAlerts[key] = Alert{Key: key}
	}
	d.alertMu.Unlock()

	d.ClearAlertsForMonitor("default", "test-cron")

	d.alertMu.RLock()
	defer d.alertMu.RUnlock()

	assert.NotContains(t, d.activeAlerts, "default/test-cron/JobFailed")
	assert.NotContains(t, d.activeAlerts, "default/test-cron/SLABreached")
	assert.NotContains(t, d.activeAlerts, "default/test-cron/DeadManTriggered")

	assert.Contains(t, d.activeAlerts, "default/other-cron/JobFailed")
}

// ==================== Pending Alert Tests ====================

func TestDispatcher_PendingAlert_DelayedDispatch(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	cfg := testAlertingConfig("slack-main")
	cfg.AlertDelay = &metav1.Duration{Duration: 100 * time.Millisecond}

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	assert.Len(t, ch.GetSentAlerts(), 0)

	d.pendingMu.RLock()
	_, pending := d.pendingAlerts[alert.Key]
	d.pendingMu.RUnlock()
	assert.True(t, pending)

	time.Sleep(200 * time.Millisecond)

	assert.Len(t, ch.GetSentAlerts(), 1)
}

func TestDispatcher_PendingAlert_ImmediateNoDelay(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	assert.Len(t, ch.GetSentAlerts(), 1)
}

func TestDispatcher_CancelPendingAlert_BeforeSend(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	cfg := testAlertingConfig("slack-main")
	cfg.AlertDelay = &metav1.Duration{Duration: 500 * time.Millisecond}

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	cancelled := d.CancelPendingAlert(alert.Key)
	assert.True(t, cancelled)

	time.Sleep(600 * time.Millisecond)

	assert.Len(t, ch.GetSentAlerts(), 0)
}

func TestDispatcher_CancelPendingAlertsForCronJob(t *testing.T) {
	d := testDispatcher(nil)

	d.pendingMu.Lock()
	d.pendingAlerts["default/test-cron/JobFailed"] = &PendingAlert{Cancel: make(chan struct{})}
	d.pendingAlerts["default/test-cron/SLABreached"] = &PendingAlert{Cancel: make(chan struct{})}
	d.pendingAlerts["default/other-cron/JobFailed"] = &PendingAlert{Cancel: make(chan struct{})}
	d.pendingMu.Unlock()

	count := d.CancelPendingAlertsForCronJob("default", "test-cron")
	assert.Equal(t, 2, count)

	d.pendingMu.RLock()
	assert.NotContains(t, d.pendingAlerts, "default/test-cron/JobFailed")
	assert.NotContains(t, d.pendingAlerts, "default/test-cron/SLABreached")
	assert.Contains(t, d.pendingAlerts, "default/other-cron/JobFailed")
	d.pendingMu.RUnlock()
}

func TestDispatcher_CancelPendingAlert_NotFound(t *testing.T) {
	d := testDispatcher(nil)

	cancelled := d.CancelPendingAlert("non-existent-key")
	assert.False(t, cancelled)
}

// ==================== Rate Limiting Tests ====================

func TestRateLimiter_UnderLimit(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	d.SetGlobalRateLimits(
		config.RateLimitsConfig{
			MaxAlertsPerMinute: 100,
		},
	)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	for i := 0; i < 5; i++ {
		alert := testAlert("default", "cron-"+string(rune('a'+i)), "JobFailed", "critical")
		err := d.Dispatch(ctx, alert, cfg)
		require.NoError(t, err)
	}

	assert.Len(t, ch.GetSentAlerts(), 5)
}

func TestRateLimiter_ExceedsLimit(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	d.globalLimiter = rate.NewLimiter(rate.Limit(1.0/60.0), 1) // 1/min, burst 1

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	alert1 := testAlert("default", "cron-a", "JobFailed", "critical")
	err := d.Dispatch(ctx, alert1, cfg)
	require.NoError(t, err)

	alert2 := testAlert("default", "cron-b", "JobFailed", "critical")
	err = d.Dispatch(ctx, alert2, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestRateLimiter_BurstHandling(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	d.globalLimiter = rate.NewLimiter(rate.Limit(1.0/60.0), 5) // 1/min, burst 5

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	for i := 0; i < 5; i++ {
		alert := testAlert("default", "cron-"+string(rune('a'+i)), "JobFailed", "critical")
		err := d.Dispatch(ctx, alert, cfg)
		require.NoError(t, err)
	}

	alert6 := testAlert("default", "cron-f", "JobFailed", "critical")
	err := d.Dispatch(ctx, alert6, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")

	assert.Len(t, ch.GetSentAlerts(), 5)
}

func TestSetGlobalRateLimits(t *testing.T) {
	d := testDispatcher(nil)

	d.SetGlobalRateLimits(
		config.RateLimitsConfig{
			MaxAlertsPerMinute: 30,
		},
	)

	assert.NotNil(t, d.globalLimiter)
}

func TestSetGlobalRateLimits_DefaultValue(t *testing.T) {
	d := testDispatcher(nil)

	d.SetGlobalRateLimits(
		config.RateLimitsConfig{
			MaxAlertsPerMinute: 0,
		},
	)

	assert.NotNil(t, d.globalLimiter)
}

// ==================== Startup Grace Period Tests ====================

func TestStartupGrace_SuppressesDuringPeriod(t *testing.T) {
	d := &dispatcher{
		channels:           make(map[string]Channel),
		channelStats:       make(map[string]*ChannelStats),
		sentAlerts:         make(map[string]time.Time),
		activeAlerts:       make(map[string]Alert),
		pendingAlerts:      make(map[string]*PendingAlert),
		globalLimiter:      rate.NewLimiter(rate.Inf, 100),
		cleanupDone:        make(chan struct{}),
		startupGracePeriod: 1 * time.Hour,
		readyAt:            time.Now().Add(1 * time.Hour), // Still in grace period
	}

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	// Alert should not be sent during grace period
	assert.Len(t, ch.GetSentAlerts(), 0)

	d.alertMu.RLock()
	_, exists := d.sentAlerts[alert.Key]
	d.alertMu.RUnlock()
	assert.False(t, exists)
}

func TestStartupGrace_AllowsAfterPeriod(t *testing.T) {
	d := &dispatcher{
		channels:           make(map[string]Channel),
		channelStats:       make(map[string]*ChannelStats),
		sentAlerts:         make(map[string]time.Time),
		activeAlerts:       make(map[string]Alert),
		pendingAlerts:      make(map[string]*PendingAlert),
		globalLimiter:      rate.NewLimiter(rate.Inf, 100),
		cleanupDone:        make(chan struct{}),
		startupGracePeriod: 0,
		readyAt:            time.Now().Add(-1 * time.Second), // Grace period passed
	}

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	assert.Len(t, ch.GetSentAlerts(), 1)
}

// ==================== Channel Stats Tests ====================

func TestDispatcher_ChannelStats_RecordsSuccess(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	stats := d.GetChannelStats("slack-main")
	require.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.AlertsSentTotal)
	assert.Equal(t, int64(0), stats.AlertsFailedTotal)
	assert.Equal(t, int32(0), stats.ConsecutiveFailures)
}

func TestDispatcher_ChannelStats_RecordsFailure(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-broken", "slack")
	ch.sendErr = errors.New("webhook failed")
	d.channels["slack-broken"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-broken")

	_ = d.Dispatch(ctx, alert, cfg)

	time.Sleep(50 * time.Millisecond)

	stats := d.GetChannelStats("slack-broken")
	require.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.AlertsSentTotal)
	assert.Equal(t, int64(1), stats.AlertsFailedTotal)
	assert.Equal(t, int32(1), stats.ConsecutiveFailures)
	assert.Contains(t, stats.LastFailedError, "webhook failed")
}

func TestDispatcher_ChannelStats_ConsecutiveFailures(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-flaky", "slack")
	ch.sendErr = errors.New("timeout")
	d.channels["slack-flaky"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-flaky")

	for i := 0; i < 3; i++ {
		alert := testAlert("default", "cron-"+string(rune('a'+i)), "JobFailed", "critical")
		_ = d.Dispatch(ctx, alert, cfg)
	}

	time.Sleep(100 * time.Millisecond)

	stats := d.GetChannelStats("slack-flaky")
	require.NotNil(t, stats)
	assert.Equal(t, int32(3), stats.ConsecutiveFailures)
}

func TestDispatcher_ChannelStats_ResetsOnSuccess(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	d.statsMu.Lock()
	d.channelStats["slack-main"] = &ChannelStats{
		AlertsFailedTotal:   5,
		ConsecutiveFailures: 3,
	}
	d.statsMu.Unlock()

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	stats := d.GetChannelStats("slack-main")
	require.NotNil(t, stats)
	assert.Equal(t, int32(0), stats.ConsecutiveFailures)
}

func TestDispatcher_GetChannelStats_NotFound(t *testing.T) {
	d := testDispatcher(nil)

	stats := d.GetChannelStats("non-existent")
	assert.Nil(t, stats)
}

// ==================== Register/Remove Channel Tests ====================

func TestDispatcher_RemoveChannel(t *testing.T) {
	d := testDispatcher(nil)

	d.channels["slack-main"] = newMockChannel("slack-main", "slack")
	assert.Contains(t, d.channels, "slack-main")

	d.RemoveChannel("slack-main")
	assert.NotContains(t, d.channels, "slack-main")
}

func TestDispatcher_SendToChannel_NotFound(t *testing.T) {
	d := testDispatcher(nil)

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	err := d.SendToChannel(ctx, "non-existent", alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDispatcher_SendToChannel_Success(t *testing.T) {
	d := testDispatcher(nil)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	err := d.SendToChannel(ctx, "slack-main", alert)
	require.NoError(t, err)
	assert.Len(t, ch.GetSentAlerts(), 1)
}

// ==================== Alert Count Tests ====================

func TestDispatcher_GetAlertCount24h(t *testing.T) {
	d := testDispatcher(nil)

	d.alertMu.Lock()
	d.alertCount24h = 42
	d.alertMu.Unlock()

	count := d.GetAlertCount24h()
	assert.Equal(t, int32(42), count)
}

// ==================== Severity Routing Tests ====================

func TestDispatcher_SeverityRouting(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	criticalCh := newMockChannel("pagerduty", "pagerduty")
	warningCh := newMockChannel("slack", "slack")
	d.channels["pagerduty"] = criticalCh
	d.channels["slack"] = warningCh

	enabled := true
	cfg := &v1alpha1.AlertingConfig{
		Enabled: &enabled,
		ChannelRefs: []v1alpha1.ChannelRef{
			{Name: "pagerduty", Severities: []string{"critical"}},
			{Name: "slack", Severities: []string{"warning", "info"}},
		},
	}

	ctx := context.Background()

	critAlert := testAlert("default", "cron-a", "JobFailed", "critical")
	err := d.Dispatch(ctx, critAlert, cfg)
	require.NoError(t, err)

	assert.Len(t, criticalCh.GetSentAlerts(), 1)
	assert.Len(t, warningCh.GetSentAlerts(), 0)

	criticalCh.Reset()
	warningCh.Reset()

	warnAlert := testAlert("default", "cron-b", "SLABreached", "warning")
	err = d.Dispatch(ctx, warnAlert, cfg)
	require.NoError(t, err)

	assert.Len(t, criticalCh.GetSentAlerts(), 0)
	assert.Len(t, warningCh.GetSentAlerts(), 1)
}

func TestDispatcher_SeverityRouting_NoFilterMeansAll(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack", "slack")
	d.channels["slack"] = ch

	enabled := true
	cfg := &v1alpha1.AlertingConfig{
		Enabled: &enabled,
		ChannelRefs: []v1alpha1.ChannelRef{
			{Name: "slack"},
		},
	}

	ctx := context.Background()

	for i, sev := range []string{"critical", "warning", "info"} {
		alert := testAlert("default", "cron-"+string(rune('a'+i)), "JobFailed", sev)
		err := d.Dispatch(ctx, alert, cfg)
		require.NoError(t, err)
	}

	assert.Len(t, ch.GetSentAlerts(), 3)
}

// ==================== Helper Function Tests ====================

func TestExitCodeCategory(t *testing.T) {
	tests := []struct {
		code     int32
		expected string
	}{
		{0, "success"},
		{1, "app-error"},
		{127, "app-error"},
		{128, "signal"},
		{137, "oom"},
		{143, "sigterm"},
		{255, "signal"},
	}

	for _, tc := range tests {
		t.Run(
			fmt.Sprintf("%d", tc.code), func(t *testing.T) {
				result := exitCodeCategory(tc.code)
				assert.Equal(t, tc.expected, result)
			},
		)
	}
}

func TestErrorSignatureChanged(t *testing.T) {
	tests := []struct {
		name     string
		old      AlertContext
		new      AlertContext
		expected bool
	}{
		{
			name:     "same category",
			old:      AlertContext{ExitCode: 1},
			new:      AlertContext{ExitCode: 2},
			expected: false,
		},
		{
			name:     "different category (app to oom)",
			old:      AlertContext{ExitCode: 1},
			new:      AlertContext{ExitCode: 137},
			expected: true,
		},
		{
			name:     "different reason",
			old:      AlertContext{Reason: "OOMKilled"},
			new:      AlertContext{Reason: "ImagePullBackOff"},
			expected: true,
		},
		{
			name:     "same reason",
			old:      AlertContext{Reason: "OOMKilled"},
			new:      AlertContext{Reason: "OOMKilled"},
			expected: false,
		},
		{
			name:     "empty to filled reason",
			old:      AlertContext{Reason: ""},
			new:      AlertContext{Reason: "Error"},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(
			tc.name, func(t *testing.T) {
				result := errorSignatureChanged(tc.old, tc.new)
				assert.Equal(t, tc.expected, result)
			},
		)
	}
}

func TestIsEnabled(t *testing.T) {
	assert.True(t, isEnabled(nil))

	trueVal := true
	assert.True(t, isEnabled(&trueVal))

	falseVal := false
	assert.False(t, isEnabled(&falseVal))
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, contains(slice, "a"))
	assert.True(t, contains(slice, "b"))
	assert.True(t, contains(slice, "c"))
	assert.False(t, contains(slice, "d"))
	assert.False(t, contains(nil, "a"))
	assert.False(t, contains([]string{}, "a"))
}

// ==================== Template Function Tests ====================

func TestTemplateFuncs_FormatTime(t *testing.T) {
	fn := templateFuncs["formatTime"].(func(time.Time, string) string)
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	assert.Equal(t, "2025-01-15T10:30:00Z", fn(now, "RFC3339"))
	assert.Equal(t, "2025-01-15", fn(now, "2006-01-02"))
}

func TestTemplateFuncs_HumanizeDuration(t *testing.T) {
	fn := templateFuncs["humanizeDuration"].(func(time.Duration) string)

	assert.Equal(t, "30s", fn(30*time.Second))
	assert.Equal(t, "5m", fn(5*time.Minute))
	assert.Equal(t, "2h", fn(2*time.Hour))
	assert.Equal(t, "3d", fn(72*time.Hour))
}

func TestTemplateFuncs_Truncate(t *testing.T) {
	fn := templateFuncs["truncate"].(func(string, int) string)

	assert.Equal(t, "hello", fn("hello", 10))
	assert.Equal(t, "hel...", fn("hello world", 3))
}

func TestTemplateFuncs_Upper(t *testing.T) {
	fn := templateFuncs["upper"].(func(string) string)
	assert.Equal(t, "HELLO", fn("hello"))
}

func TestTemplateFuncs_Lower(t *testing.T) {
	fn := templateFuncs["lower"].(func(string) string)
	assert.Equal(t, "hello", fn("HELLO"))
}

func TestTemplateFuncs_JsonEscape(t *testing.T) {
	fn := templateFuncs["jsonEscape"].(func(string) string)
	assert.Equal(t, `"hello \"world\""`, fn(`hello "world"`))
	assert.Equal(t, `"line1\nline2"`, fn("line1\nline2"))
}

// ==================== Store Alert Tests ====================

func TestDispatcher_StoresAlertHistory(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	alert.Context.ExitCode = 137
	alert.Context.Reason = "OOMKilled"
	alert.Context.SuggestedFix = "Increase memory limit"
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	require.Len(t, mockStore.alerts, 1)
	stored := mockStore.alerts[0]
	assert.Equal(t, "JobFailed", stored.Type)
	assert.Equal(t, "critical", stored.Severity)
	assert.Equal(t, "default", stored.CronJobNamespace)
	assert.Equal(t, "test-cron", stored.CronJobName)
	assert.Equal(t, int32(137), stored.ExitCode)
	assert.Equal(t, "OOMKilled", stored.Reason)
	assert.Equal(t, "Increase memory limit", stored.SuggestedFix)
}

func TestDispatcher_DoesNotStoreOnAllFailures(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-broken", "slack")
	ch.sendErr = errors.New("webhook failed")
	d.channels["slack-broken"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")
	cfg := testAlertingConfig("slack-broken")

	_ = d.Dispatch(ctx, alert, cfg)

	mockStore.mu.Lock()
	defer mockStore.mu.Unlock()

	assert.Len(t, mockStore.alerts, 0)
}

// ==================== PendingAlert.Close() Tests ====================

func TestPendingAlert_Close_MultipleCalls(t *testing.T) {
	// Test that calling Close() multiple times does not panic
	pending := &PendingAlert{
		Cancel: make(chan struct{}),
	}

	// First close should work
	assert.NotPanics(
		t, func() {
			pending.Close()
		}, "first Close() should not panic",
	)

	// Second close should not panic (protected by sync.Once)
	assert.NotPanics(
		t, func() {
			pending.Close()
		}, "second Close() should not panic",
	)

	// Third close should also not panic
	assert.NotPanics(
		t, func() {
			pending.Close()
		}, "third Close() should not panic",
	)

	// Verify the channel is actually closed
	select {
	case <-pending.Cancel:
		// Channel is closed, this is expected
	default:
		t.Fatal("channel should be closed after Close()")
	}
}

func TestPendingAlert_Close_ConcurrentCalls(t *testing.T) {
	// Test that concurrent Close() calls are safe
	for i := 0; i < 100; i++ {
		pending := &PendingAlert{
			Cancel: make(chan struct{}),
		}

		var wg sync.WaitGroup
		// Launch 10 goroutines to close concurrently
		for j := 0; j < 10; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.NotPanics(
					t, func() {
						pending.Close()
					},
				)
			}()
		}
		wg.Wait()
	}
}

// ==================== Dispatcher Stop Tests ====================

func TestDispatcher_Stop(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	// Verify stop doesn't panic and returns no error
	err := d.Stop()
	assert.NoError(t, err)

	// Verify cleanupDone channel is closed
	select {
	case <-d.cleanupDone:
		// Channel closed, as expected
	default:
		t.Fatal("cleanupDone channel should be closed after Stop()")
	}
}

func TestDispatcher_Stop_MultipleCalls(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	// First stop should work
	err := d.Stop()
	assert.NoError(t, err)
}

// ==================== Concurrent Dispatch Race Tests ====================
// These tests are designed to be run with -race flag to detect race conditions

func TestDispatcher_Dispatch_ConcurrentSameKey(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	var wg sync.WaitGroup
	const goroutines = 20

	// Launch multiple goroutines dispatching the same alert key
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := testAlert("default", "test-cron", "JobFailed", "critical")
			// Dispatch same key - should be deduplicated
			_ = d.Dispatch(ctx, alert, cfg)
		}(i)
	}

	wg.Wait()

	// Only one alert should be sent (due to deduplication)
	sentAlerts := ch.GetSentAlerts()
	assert.Equal(t, 1, len(sentAlerts), "only one alert should be sent due to deduplication")
}

func TestDispatcher_Dispatch_ConcurrentDifferentKeys(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	var wg sync.WaitGroup
	const goroutines = 50

	// Launch multiple goroutines dispatching different alert keys
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := testAlert("default", fmt.Sprintf("cron-%d", idx), "JobFailed", "critical")
			err := d.Dispatch(ctx, alert, cfg)
			// All should succeed (no rate limiting in test setup)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// All alerts should be sent (different keys)
	sentAlerts := ch.GetSentAlerts()
	assert.Equal(t, goroutines, len(sentAlerts), "all alerts with different keys should be sent")
}

func TestDispatcher_ConcurrentClearAndDispatch(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	var wg sync.WaitGroup

	// Goroutines dispatching alerts
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				alert := testAlert("default", fmt.Sprintf("cron-%d", idx), "JobFailed", "critical")
				_ = d.Dispatch(ctx, alert, cfg)
			}
		}(i)
	}

	// Goroutines clearing alerts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				key := fmt.Sprintf("default/cron-%d/JobFailed", idx)
				_ = d.ClearAlert(ctx, key)
			}
		}(i)
	}

	wg.Wait()
	// No panic = success for race detection
}

func TestDispatcher_ConcurrentChannelOperations(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ctx := context.Background()
	cfg := testAlertingConfig("dynamic-channel")

	var wg sync.WaitGroup

	// Goroutines registering/removing channels
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ch := newMockChannel(fmt.Sprintf("channel-%d", idx), "slack")
			d.channelMu.Lock()
			d.channels[ch.Name()] = ch
			d.channelMu.Unlock()

			// Dispatch using the channel
			alert := testAlert("default", "test-cron", "JobFailed", "critical")
			cfg := testAlertingConfig(ch.Name())
			_ = d.Dispatch(ctx, alert, cfg)

			// Remove channel
			d.RemoveChannel(ch.Name())
		}(i)
	}

	// Goroutines sending to potentially non-existent channels
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			alert := testAlert("default", "test-cron", "JobFailed", "critical")
			_ = d.Dispatch(ctx, alert, cfg)
		}()
	}

	wg.Wait()
	// No panic = success for race detection
}

func TestDispatcher_ConcurrentPendingAlertOperations(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")
	cfg.AlertDelay = &metav1.Duration{Duration: 200 * time.Millisecond}

	var wg sync.WaitGroup

	// Goroutines queueing delayed alerts
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := testAlert("default", fmt.Sprintf("cron-%d", idx), "JobFailed", "critical")
			_ = d.Dispatch(ctx, alert, cfg)
		}(i)
	}

	// Small delay to let some alerts queue
	time.Sleep(50 * time.Millisecond)

	// Goroutines cancelling pending alerts
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("default/cron-%d/JobFailed", idx)
			d.CancelPendingAlert(key)
		}(i)
	}

	// Goroutines cancelling by cronjob
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			d.CancelPendingAlertsForCronJob("default", fmt.Sprintf("cron-%d", idx))
		}(i)
	}

	wg.Wait()
	// No panic = success for race detection
}

func TestDispatcher_ConcurrentStatsAccess(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	var wg sync.WaitGroup

	// Goroutines dispatching alerts (which updates stats)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := testAlert("default", fmt.Sprintf("cron-%d", idx), "JobFailed", "critical")
			_ = d.Dispatch(ctx, alert, cfg)
		}(i)
	}

	// Goroutines reading stats
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = d.GetChannelStats("slack-main")
				_ = d.GetAlertCount24h()
			}
		}()
	}

	wg.Wait()
	// No panic = success for race detection
}

func TestDispatcher_ConcurrentIsSuppressed(t *testing.T) {
	d := testDispatcher(nil)

	cfg := testAlertingConfig("slack-main")

	var wg sync.WaitGroup

	// Seed with some sent alerts
	for i := 0; i < 10; i++ {
		alert := testAlert("default", fmt.Sprintf("cron-%d", i), "JobFailed", "critical")
		d.alertMu.Lock()
		d.sentAlerts[alert.Key] = time.Now()
		d.activeAlerts[alert.Key] = alert
		d.alertMu.Unlock()
	}

	// Goroutines checking suppression
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := testAlert("default", fmt.Sprintf("cron-%d", idx%15), "JobFailed", "critical")
			_, _ = d.IsSuppressed(alert, cfg)
		}(i)
	}

	// Goroutines modifying sentAlerts concurrently
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := testAlert("default", fmt.Sprintf("new-cron-%d", idx), "JobFailed", "critical")
			d.alertMu.Lock()
			d.sentAlerts[alert.Key] = time.Now()
			d.activeAlerts[alert.Key] = alert
			d.alertMu.Unlock()
		}(i)
	}

	wg.Wait()
	// No panic = success for race detection
}
