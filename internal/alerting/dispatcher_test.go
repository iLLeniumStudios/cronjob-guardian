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
func (m *mockStore) GetExecutionsFiltered(_ context.Context, _ types.NamespacedName, _ time.Time, _ string, _, _ int) ([]store.Execution, int64, error) {
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
		globalLimiter:      rate.NewLimiter(rate.Inf, 100), // Unlimited for most tests
		cleanupDone:        make(chan struct{}),
		startupGracePeriod: 0,
		readyAt:            time.Now().Add(-time.Second), // Already past grace period
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

	// Should not error, just skip the missing channel
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
	// Should report partial failure
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send to 1 channels")

	// Successful channel should still have received the alert
	assert.Len(t, successCh.GetSentAlerts(), 1)
}

func TestDispatcher_Dispatch_DisabledAlerting(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	// Disabled config
	disabled := false
	cfg := &v1alpha1.AlertingConfig{
		Enabled:     &disabled,
		ChannelRefs: []v1alpha1.ChannelRef{{Name: "slack-main"}},
	}

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	// No alerts should be sent
	assert.Len(t, ch.GetSentAlerts(), 0)
}

func TestDispatcher_Dispatch_NilConfig(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	ctx := context.Background()
	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	// Nil config should be handled gracefully
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
		Key:      "", // Empty key should be auto-generated
		Type:     "JobFailed",
		Severity: "critical",
		CronJob:  types.NamespacedName{Namespace: "prod", Name: "daily-backup"},
	}
	cfg := testAlertingConfig("slack-main")

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	// Check the alert was tracked with generated key
	d.alertMu.RLock()
	_, exists := d.sentAlerts["prod/daily-backup/JobFailed"]
	d.alertMu.RUnlock()
	assert.True(t, exists)
}

// ==================== IsSuppressed Tests ====================

func TestDispatcher_IsSuppressed_DuplicateWithinWindow(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	// Pre-populate sent alerts
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

	// Pre-populate with alert1
	d.alertMu.Lock()
	d.sentAlerts[alert1.Key] = time.Now()
	d.activeAlerts[alert1.Key] = alert1
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(alert2, cfg)

	// Different alert should not be suppressed
	assert.False(t, suppressed)
}

func TestDispatcher_IsSuppressed_AfterSuppressionWindow(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	// Pre-populate with old timestamp (2 hours ago)
	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now().Add(-2 * time.Hour)
	d.activeAlerts[alert.Key] = alert
	d.alertMu.Unlock()

	// Default suppression is 1 hour
	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(alert, cfg)

	// Should not be suppressed after window
	assert.False(t, suppressed)
}

func TestDispatcher_IsSuppressed_CustomWindow(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	// Pre-populate with 30 min ago
	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now().Add(-30 * time.Minute)
	d.activeAlerts[alert.Key] = alert
	d.alertMu.Unlock()

	// Custom 2 hour suppression window
	cfg := testAlertingConfig("slack-main")
	cfg.SuppressDuplicatesFor = &metav1.Duration{Duration: 2 * time.Hour}

	suppressed, _ := d.IsSuppressed(alert, cfg)
	assert.True(t, suppressed)
}

func TestDispatcher_IsSuppressed_ErrorSignatureChanged(t *testing.T) {
	d := testDispatcher(nil)

	oldAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	oldAlert.Context.ExitCode = 137 // OOM

	newAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	newAlert.Context.ExitCode = 1 // App error

	// Pre-populate with old alert
	d.alertMu.Lock()
	d.sentAlerts[oldAlert.Key] = time.Now()
	d.activeAlerts[oldAlert.Key] = oldAlert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(newAlert, cfg)

	// Different error signature should bypass suppression
	assert.False(t, suppressed)
}

func TestDispatcher_IsSuppressed_SameErrorSignature(t *testing.T) {
	d := testDispatcher(nil)

	oldAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	oldAlert.Context.ExitCode = 1

	newAlert := testAlert("default", "test-cron", "JobFailed", "critical")
	newAlert.Context.ExitCode = 2 // Still in same "app-error" category

	// Pre-populate with old alert
	d.alertMu.Lock()
	d.sentAlerts[oldAlert.Key] = time.Now()
	d.activeAlerts[oldAlert.Key] = oldAlert
	d.alertMu.Unlock()

	cfg := testAlertingConfig("slack-main")
	suppressed, _ := d.IsSuppressed(newAlert, cfg)

	// Same error category should still be suppressed
	assert.True(t, suppressed)
}

// ==================== ClearAlert Tests ====================

func TestDispatcher_ClearAlert_RemovesFromActive(t *testing.T) {
	d := testDispatcher(nil)

	alert := testAlert("default", "test-cron", "JobFailed", "critical")

	// Pre-populate
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

	// Add multiple alerts for same CronJob
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

	// test-cron alerts should be cleared
	assert.NotContains(t, d.activeAlerts, "default/test-cron/JobFailed")
	assert.NotContains(t, d.activeAlerts, "default/test-cron/SLABreached")
	assert.NotContains(t, d.activeAlerts, "default/test-cron/DeadManTriggered")

	// other-cron should remain
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

	// Should not be sent immediately
	assert.Len(t, ch.GetSentAlerts(), 0)

	// Check it's pending
	d.pendingMu.RLock()
	_, pending := d.pendingAlerts[alert.Key]
	d.pendingMu.RUnlock()
	assert.True(t, pending)

	// Wait for delay to expire
	time.Sleep(200 * time.Millisecond)

	// Should now be sent
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
	// No AlertDelay set

	err := d.Dispatch(ctx, alert, cfg)
	require.NoError(t, err)

	// Should be sent immediately
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

	// Cancel before delay expires
	cancelled := d.CancelPendingAlert(alert.Key)
	assert.True(t, cancelled)

	// Wait for original delay to pass
	time.Sleep(600 * time.Millisecond)

	// Should NOT have been sent
	assert.Len(t, ch.GetSentAlerts(), 0)
}

func TestDispatcher_CancelPendingAlertsForCronJob(t *testing.T) {
	d := testDispatcher(nil)

	// Add multiple pending alerts
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

	// Set up a generous rate limiter (100/min)
	d.SetGlobalRateLimits(config.RateLimitsConfig{
		MaxAlertsPerMinute: 100,
	})

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	// Multiple alerts should succeed when under limit
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

	// Set up a very restrictive rate limiter with no burst
	d.globalLimiter = rate.NewLimiter(rate.Limit(1.0/60.0), 1) // 1/min, burst 1

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	// First alert should succeed (uses burst)
	alert1 := testAlert("default", "cron-a", "JobFailed", "critical")
	err := d.Dispatch(ctx, alert1, cfg)
	require.NoError(t, err)

	// Second alert should be rate limited
	alert2 := testAlert("default", "cron-b", "JobFailed", "critical")
	err = d.Dispatch(ctx, alert2, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
}

func TestRateLimiter_BurstHandling(t *testing.T) {
	mockStore := newMockStore()
	d := testDispatcher(mockStore)

	// Set rate limiter with burst of 5
	d.globalLimiter = rate.NewLimiter(rate.Limit(1.0/60.0), 5) // 1/min, burst 5

	ch := newMockChannel("slack-main", "slack")
	d.channels["slack-main"] = ch

	ctx := context.Background()
	cfg := testAlertingConfig("slack-main")

	// First 5 alerts should succeed (uses burst)
	for i := 0; i < 5; i++ {
		alert := testAlert("default", "cron-"+string(rune('a'+i)), "JobFailed", "critical")
		err := d.Dispatch(ctx, alert, cfg)
		require.NoError(t, err)
	}

	// 6th alert should be rate limited
	alert6 := testAlert("default", "cron-f", "JobFailed", "critical")
	err := d.Dispatch(ctx, alert6, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")

	assert.Len(t, ch.GetSentAlerts(), 5)
}

func TestSetGlobalRateLimits(t *testing.T) {
	d := testDispatcher(nil)

	// Set with specific value
	d.SetGlobalRateLimits(config.RateLimitsConfig{
		MaxAlertsPerMinute: 30,
	})

	assert.NotNil(t, d.globalLimiter)
}

func TestSetGlobalRateLimits_DefaultValue(t *testing.T) {
	d := testDispatcher(nil)

	// Set with zero value should use default (50/min)
	d.SetGlobalRateLimits(config.RateLimitsConfig{
		MaxAlertsPerMinute: 0,
	})

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

	// Alert should NOT be sent during grace period
	assert.Len(t, ch.GetSentAlerts(), 0)

	// But should be tracked for deduplication
	d.alertMu.RLock()
	_, exists := d.sentAlerts[alert.Key]
	d.alertMu.RUnlock()
	assert.True(t, exists)
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

	// Alert should be sent after grace period
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

	// Wait for async persist
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

	// Wait for async persist
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

	// Send multiple failing alerts
	for i := 0; i < 3; i++ {
		alert := testAlert("default", "cron-"+string(rune('a'+i)), "JobFailed", "critical")
		_ = d.Dispatch(ctx, alert, cfg)
	}

	// Wait for async persist
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

	// Pre-populate with failures
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

	// Wait for async persist
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

	// Critical alert should go to pagerduty only
	critAlert := testAlert("default", "cron-a", "JobFailed", "critical")
	err := d.Dispatch(ctx, critAlert, cfg)
	require.NoError(t, err)

	assert.Len(t, criticalCh.GetSentAlerts(), 1)
	assert.Len(t, warningCh.GetSentAlerts(), 0)

	criticalCh.Reset()
	warningCh.Reset()

	// Warning alert should go to slack only
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
			{Name: "slack"}, // No Severities = all severities
		},
	}

	ctx := context.Background()

	// All severities should be routed
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
		t.Run(fmt.Sprintf("%d", tc.code), func(t *testing.T) {
			result := exitCodeCategory(tc.code)
			assert.Equal(t, tc.expected, result)
		})
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
			expected: false, // Empty doesn't trigger change
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := errorSignatureChanged(tc.old, tc.new)
			assert.Equal(t, tc.expected, result)
		})
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

	// Should not store if all channels fail
	assert.Len(t, mockStore.alerts, 0)
}
