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

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/testutil"
)

// Helper to create a test scheme
func newTestSchedulerScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = guardianv1alpha1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	return scheme
}

// Helper to create fake client
func newTestSchedulerClient(objs ...client.Object) client.Client {
	scheme := newTestSchedulerScheme()
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&guardianv1alpha1.CronJobMonitor{}).
		Build()
}

// Helper to create a test CronJob
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func newTestSchedulerCronJob(name, namespace string, suspended bool) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspended,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{},
				},
			},
		},
	}
}

// Helper to create a test monitor with dead-man's switch enabled
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func newTestMonitorWithDeadMan(name, namespace string, cjName string) *guardianv1alpha1.CronJobMonitor {
	enabled := true
	return &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			DeadManSwitch: &guardianv1alpha1.DeadManSwitchConfig{
				Enabled: &enabled,
			},
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      cjName,
					Namespace: namespace,
				},
			},
		},
	}
}

// Helper to create a test monitor with SLA enabled
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func newTestMonitorWithSLA(name, namespace string, cjName string) *guardianv1alpha1.CronJobMonitor {
	enabled := true
	return &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			SLA: &guardianv1alpha1.SLAConfig{
				Enabled: &enabled,
			},
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      cjName,
					Namespace: namespace,
				},
			},
		},
	}
}

// ============================================================================
// Section 1.4.1: DeadManScheduler Tests
// ============================================================================

func TestDeadManScheduler_Start(t *testing.T) {
	fakeClient := newTestSchedulerClient()
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond) // Fast interval for testing

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- scheduler.Start(ctx)
	}()

	// Wait a bit for scheduler to run
	time.Sleep(50 * time.Millisecond)

	// Stop scheduler
	scheduler.Stop()

	// Check no error from Start
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		// OK, still waiting is fine
	}
}

func TestDeadManScheduler_Stop(t *testing.T) {
	fakeClient := newTestSchedulerClient()
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- scheduler.Start(ctx)
	}()

	// Wait for scheduler to start
	time.Sleep(20 * time.Millisecond)

	// Stop should be clean
	scheduler.Stop()

	// Wait for Start to return
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("scheduler did not stop in time")
	}
}

func TestDeadManScheduler_RunsAtInterval(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithDeadMan("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	// Wait for several intervals
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()

	// Verify check was called multiple times
	callCount := mockAnalyzer.CheckDeadManSwitchCalled

	assert.GreaterOrEqual(t, callCount, 3, "dead-man check should be called multiple times")
}

func TestDeadManScheduler_StartupDelay(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithDeadMan("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)
	scheduler.SetStartupDelay(50 * time.Millisecond) // Set startup delay

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startTime := time.Now()
	go func() {
		_ = scheduler.Start(ctx)
	}()

	// Wait a bit less than startup delay
	time.Sleep(30 * time.Millisecond)

	callsBeforeDelay := mockAnalyzer.CheckDeadManSwitchCalled

	// Should not have called yet
	assert.Equal(t, 0, callsBeforeDelay, "should not check before startup delay")

	// Wait for startup delay to complete plus some intervals
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()

	callsAfterDelay := mockAnalyzer.CheckDeadManSwitchCalled

	assert.Greater(t, callsAfterDelay, 0, "should check after startup delay")
	assert.GreaterOrEqual(t, time.Since(startTime), 50*time.Millisecond)
}

func TestDeadManScheduler_ChecksAllMonitors(t *testing.T) {
	cronJob1 := newTestSchedulerCronJob("cron-1", "default", false)
	cronJob2 := newTestSchedulerCronJob("cron-2", "default", false)
	monitor1 := newTestMonitorWithDeadMan("monitor-1", "default", "cron-1")
	monitor2 := newTestMonitorWithDeadMan("monitor-2", "default", "cron-2")

	fakeClient := newTestSchedulerClient(cronJob1, cronJob2, monitor1, monitor2)
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	// Wait for at least one check
	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	// Should have called check for both CronJobs
	callCount := mockAnalyzer.CheckDeadManSwitchCalled

	assert.GreaterOrEqual(t, callCount, 2, "should check all monitors")
}

func TestDeadManScheduler_DispatchesAlerts(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithDeadMan("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockAnalyzer := &testutil.MockAnalyzer{
		DeadManResult: &analyzer.DeadManResult{
			Triggered: true,
			Message:   "CronJob has not run in expected window",
		},
	}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	// Wait for check to run
	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	mockDispatcher.Lock()
	alerts := mockDispatcher.DispatchedAlerts
	mockDispatcher.Unlock()

	require.GreaterOrEqual(t, len(alerts), 1)
	assert.Equal(t, "DeadManTriggered", alerts[0].Type)
	assert.Contains(t, alerts[0].Title, "test-cron")
}

func TestDeadManScheduler_TracksSuspended(t *testing.T) {
	suspended := true
	cronJob := newTestSchedulerCronJob("suspended-cron", "default", suspended)

	enabled := true
	threshold := metav1.Duration{Duration: 100 * time.Millisecond}
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			DeadManSwitch: &guardianv1alpha1.DeadManSwitchConfig{
				Enabled: &enabled,
			},
			SuspendedHandling: &guardianv1alpha1.SuspendedHandlingConfig{
				AlertIfSuspendedFor: &threshold,
			},
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "suspended-cron",
					Namespace: "default",
				},
			},
		},
	}

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	// Wait for tracking to start
	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	// Verify suspended tracking
	scheduler.suspendedSinceMu.RLock()
	_, tracked := scheduler.suspendedSince["default/suspended-cron"]
	scheduler.suspendedSinceMu.RUnlock()

	assert.True(t, tracked, "should track suspended CronJob")
}

func TestDeadManScheduler_SuspendedAlertTiming(t *testing.T) {
	suspended := true
	cronJob := newTestSchedulerCronJob("suspended-cron", "default", suspended)

	enabled := true
	threshold := metav1.Duration{Duration: 50 * time.Millisecond} // Very short for testing
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			DeadManSwitch: &guardianv1alpha1.DeadManSwitchConfig{
				Enabled: &enabled,
			},
			SuspendedHandling: &guardianv1alpha1.SuspendedHandlingConfig{
				AlertIfSuspendedFor: &threshold,
			},
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "suspended-cron",
					Namespace: "default",
				},
			},
		},
	}

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewDeadManScheduler(fakeClient, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	// Pre-populate suspended since to simulate already tracking
	scheduler.suspendedSince["default/suspended-cron"] = time.Now().Add(-100 * time.Millisecond) // Already past threshold

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	// Wait for alert to be dispatched
	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	mockDispatcher.Lock()
	alerts := mockDispatcher.DispatchedAlerts
	mockDispatcher.Unlock()

	// Should have SuspendedTooLong alert
	var hasSuspendedAlert bool
	for _, a := range alerts {
		if a.Type == "SuspendedTooLong" {
			hasSuspendedAlert = true
			break
		}
	}
	assert.True(t, hasSuspendedAlert, "should dispatch SuspendedTooLong alert")
}

// ============================================================================
// Section 1.4.2: SLARecalcScheduler Tests
// ============================================================================

func TestSLARecalcScheduler_Start(t *testing.T) {
	fakeClient := newTestSchedulerClient()
	mockStore := &testutil.MockStore{}
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewSLARecalcScheduler(fakeClient, mockStore, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- scheduler.Start(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	scheduler.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		// OK
	}
}

func TestSLARecalcScheduler_Stop(t *testing.T) {
	fakeClient := newTestSchedulerClient()
	mockStore := &testutil.MockStore{}
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewSLARecalcScheduler(fakeClient, mockStore, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- scheduler.Start(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	scheduler.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("scheduler did not stop in time")
	}
}

func TestSLARecalcScheduler_ChecksSLA(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithSLA("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockStore := &testutil.MockStore{}
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewSLARecalcScheduler(fakeClient, mockStore, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	callCount := mockAnalyzer.CheckSLACalled

	assert.GreaterOrEqual(t, callCount, 1, "should check SLA")
}

func TestSLARecalcScheduler_ChecksRegression(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithSLA("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockStore := &testutil.MockStore{}
	mockAnalyzer := &testutil.MockAnalyzer{}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewSLARecalcScheduler(fakeClient, mockStore, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	callCount := mockAnalyzer.CheckRegressionCalled

	assert.GreaterOrEqual(t, callCount, 1, "should check regression")
}

func TestSLARecalcScheduler_DispatchesOnBreach(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithSLA("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockStore := &testutil.MockStore{}
	mockAnalyzer := &testutil.MockAnalyzer{
		SLAResult: &analyzer.SLAResult{
			Passed: false,
			Violations: []analyzer.Violation{
				{Type: "SuccessRate", Message: "Success rate below threshold"},
			},
		},
	}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewSLARecalcScheduler(fakeClient, mockStore, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	mockDispatcher.Lock()
	alerts := mockDispatcher.DispatchedAlerts
	mockDispatcher.Unlock()

	require.GreaterOrEqual(t, len(alerts), 1)
	assert.Equal(t, "SLABreached", alerts[0].Type)
}

func TestSLARecalcScheduler_ClearsOnRecovery(t *testing.T) {
	cronJob := newTestSchedulerCronJob("test-cron", "default", false)
	monitor := newTestMonitorWithSLA("test-monitor", "default", "test-cron")

	fakeClient := newTestSchedulerClient(cronJob, monitor)
	mockStore := &testutil.MockStore{}
	mockAnalyzer := &testutil.MockAnalyzer{
		SLAResult: &analyzer.SLAResult{Passed: true}, // SLA passes
	}
	mockDispatcher := testutil.NewMockDispatcher()

	scheduler := NewSLARecalcScheduler(fakeClient, mockStore, mockAnalyzer, mockDispatcher)
	scheduler.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = scheduler.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	// Verify alert was cleared
	mockDispatcher.Lock()
	clearedAlerts := mockDispatcher.ClearedAlerts
	mockDispatcher.Unlock()

	assert.GreaterOrEqual(t, len(clearedAlerts), 1, "should clear SLA alerts on recovery")

	// Verify store was notified
	mockStore.Lock()
	resolveCalled := mockStore.ResolveAlertCalls
	mockStore.Unlock()

	assert.GreaterOrEqual(t, resolveCalled, 1, "should resolve alert in store")
}

// ============================================================================
// Section 1.4.3: HistoryPruner Tests
// ============================================================================

func TestHistoryPruner_Start(t *testing.T) {
	mockStore := &testutil.MockStore{PrunedCount: 0}

	pruner := NewHistoryPruner(mockStore, 7)
	pruner.SetInterval(10 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- pruner.Start(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	pruner.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		// OK
	}
}

func TestHistoryPruner_Stop(t *testing.T) {
	mockStore := &testutil.MockStore{}

	pruner := NewHistoryPruner(mockStore, 7)
	pruner.SetInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- pruner.Start(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	pruner.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("pruner did not stop in time")
	}
}

func TestHistoryPruner_PrunesAtInterval(t *testing.T) {
	mockStore := &testutil.MockStore{PrunedCount: 5}

	pruner := NewHistoryPruner(mockStore, 7)
	pruner.SetInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = pruner.Start(ctx)
	}()

	// Wait for multiple intervals (first run is immediate on start)
	time.Sleep(80 * time.Millisecond)
	pruner.Stop()

	mockStore.Lock()
	callCount := mockStore.PruneCalled
	mockStore.Unlock()

	// Should have called at least 2-3 times (immediate + intervals)
	assert.GreaterOrEqual(t, callCount, 2, "should prune at intervals")
}

func TestHistoryPruner_UsesRetentionDays(t *testing.T) {
	mockStore := &testutil.MockStore{PrunedCount: 10}
	retentionDays := 14

	pruner := NewHistoryPruner(mockStore, retentionDays)
	pruner.SetInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = pruner.Start(ctx)
	}()

	// Wait for first prune (runs immediately)
	time.Sleep(30 * time.Millisecond)
	pruner.Stop()

	mockStore.Lock()
	cutoff := mockStore.PruneCutoff
	mockStore.Unlock()

	// Cutoff should be approximately 14 days ago
	expectedCutoff := time.Now().AddDate(0, 0, -retentionDays)
	assert.WithinDuration(t, expectedCutoff, cutoff, 1*time.Second)
}

func TestHistoryPruner_SeparateLogRetention(t *testing.T) {
	mockStore := &testutil.MockStore{
		PrunedCount:     5,
		PrunedLogsCount: 3,
	}

	pruner := NewHistoryPruner(mockStore, 30) // 30 days for executions
	pruner.SetLogRetentionDays(7)             // 7 days for logs
	pruner.SetInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = pruner.Start(ctx)
	}()

	// Wait for first prune
	time.Sleep(30 * time.Millisecond)
	pruner.Stop()

	mockStore.Lock()
	pruneCalled := mockStore.PruneCalled
	logPruneCalled := mockStore.PruneLogsCalled
	prunedCutoff := mockStore.PruneCutoff
	logPrunedCutoff := mockStore.LogPruneCutoff
	mockStore.Unlock()

	// Both prune and log prune should be called
	assert.GreaterOrEqual(t, pruneCalled, 1)
	assert.GreaterOrEqual(t, logPruneCalled, 1)

	// Execution cutoff should be ~30 days ago
	expectedExecCutoff := time.Now().AddDate(0, 0, -30)
	assert.WithinDuration(t, expectedExecCutoff, prunedCutoff, 1*time.Second)

	// Log cutoff should be ~7 days ago
	expectedLogCutoff := time.Now().AddDate(0, 0, -7)
	assert.WithinDuration(t, expectedLogCutoff, logPrunedCutoff, 1*time.Second)
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestIsEnabled(t *testing.T) {
	t.Run("nil returns true", func(t *testing.T) {
		assert.True(t, isEnabled(nil))
	})

	t.Run("true returns true", func(t *testing.T) {
		b := true
		assert.True(t, isEnabled(&b))
	})

	t.Run("false returns false", func(t *testing.T) {
		b := false
		assert.False(t, isEnabled(&b))
	})
}

func TestGetOrDefault(t *testing.T) {
	t.Run("nil pointer returns default", func(t *testing.T) {
		result := getOrDefault[int32](nil, 10)
		assert.Equal(t, int32(10), result)
	})

	t.Run("non-nil pointer returns value", func(t *testing.T) {
		v := int32(42)
		result := getOrDefault(&v, 10)
		assert.Equal(t, int32(42), result)
	})
}

func TestGetSeverity(t *testing.T) {
	t.Run("empty override returns default", func(t *testing.T) {
		result := getSeverity("", "warning")
		assert.Equal(t, "warning", result)
	})

	t.Run("non-empty override returns override", func(t *testing.T) {
		result := getSeverity("critical", "warning")
		assert.Equal(t, "critical", result)
	})
}

func TestHasActiveAlert(t *testing.T) {
	alerts := []guardianv1alpha1.ActiveAlert{
		{Type: "DeadManTriggered", Severity: "critical"},
		{Type: "SLABreached", Severity: "warning"},
	}

	t.Run("finds existing alert type", func(t *testing.T) {
		assert.True(t, hasActiveAlert(alerts, "DeadManTriggered"))
	})

	t.Run("returns false for missing alert type", func(t *testing.T) {
		assert.False(t, hasActiveAlert(alerts, "NonExistent"))
	})

	t.Run("returns false for empty alerts", func(t *testing.T) {
		assert.False(t, hasActiveAlert(nil, "DeadManTriggered"))
	})
}

func TestInMaintenanceWindow(t *testing.T) {
	t.Run("empty windows returns false", func(t *testing.T) {
		result := inMaintenanceWindow(nil, time.Now(), "")
		assert.False(t, result)
	})

	t.Run("no matching schedule returns false", func(t *testing.T) {
		windows := []guardianv1alpha1.MaintenanceWindow{
			{
				Schedule: "0 3 * * 0", // Sunday at 3am
				Duration: metav1.Duration{Duration: 1 * time.Hour},
			},
		}
		// Use a Tuesday at 10am
		testTime := time.Date(2025, 1, 7, 10, 0, 0, 0, time.UTC) // Tuesday
		result := inMaintenanceWindow(windows, testTime, "UTC")
		assert.False(t, result)
	})
}
