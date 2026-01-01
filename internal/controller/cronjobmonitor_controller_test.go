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

package controller

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/testutil"
)

// Helper to create a test scheme with all required types
func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = guardianv1alpha1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}

// Helper to create a test logger
func testLogger() logr.Logger {
	return log.Log.WithName("test")
}

// Helper to create a basic CronJobMonitor
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func newTestMonitor(name, namespace string) *guardianv1alpha1.CronJobMonitor {
	return &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{},
	}
}

// Helper to create a basic CronJob
func newTestCronJob(name, namespace string, labels map[string]string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "main", Image: "busybox"},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
}

// ============================================================================
// CronJobMonitorReconciler Tests (per plan section 1.5.1)
// ============================================================================

func TestReconcile_NewMonitor(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	cronJob := newTestCronJob("test-cronjob", "default", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cronJob).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Store:    &testutil.MockStore{},
		Analyzer: &testutil.MockAnalyzer{},
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Should requeue for finalizer addition (either immediate or delayed)
	//nolint:staticcheck // result.Requeue is used by the controller, test must check both modes
	assert.True(t, result.Requeue || result.RequeueAfter > 0)

	// Verify monitor still exists
	var updated guardianv1alpha1.CronJobMonitor
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)

	// Finalizer should be added
	assert.True(t, controllerutil.ContainsFinalizer(&updated, finalizerName))
}

func TestReconcile_UpdateMonitor(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Generation = 2

	cronJob := newTestCronJob("test-cronjob", "default", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cronJob).
		WithStatusSubresource(monitor).
		Build()

	mockAnalyzer := &testutil.MockAnalyzer{
		Metrics: &guardianv1alpha1.CronJobMetrics{
			SuccessRate: 100.0,
			TotalRuns:   10,
		},
	}

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Store:    &testutil.MockStore{},
		Analyzer: mockAnalyzer,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0)

	// Verify status was updated
	var updated guardianv1alpha1.CronJobMonitor
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.Equal(t, int64(2), updated.Status.ObservedGeneration)
}

func TestReconcile_DeleteMonitor(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.DeletionTimestamp = &now

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor).
		WithStatusSubresource(monitor).
		Build()

	mockDispatcher := testutil.NewMockDispatcher()

	r := &CronJobMonitorReconciler{
		Client:          fakeClient,
		Log:             testLogger(),
		Scheme:          scheme,
		AlertDispatcher: mockDispatcher,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify alerts were cleared (ClearAlertsForMonitor appends to ClearedAlerts)
	assert.NotEmpty(t, mockDispatcher.ClearedAlerts)

	// Verify object was deleted (when finalizer is removed from object with deletion timestamp,
	// Kubernetes deletes the object)
	var updated guardianv1alpha1.CronJobMonitor
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	assert.True(t, apierrors.IsNotFound(err), "monitor should be deleted after finalizer removal")
}

func TestReconcile_AddsFinalizer(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	// No finalizer added

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify finalizer was added
	var updated guardianv1alpha1.CronJobMonitor
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.True(t, controllerutil.ContainsFinalizer(&updated, finalizerName))
}

func TestReconcile_RemovesFinalizer(t *testing.T) {
	scheme := newTestScheme()

	now := metav1.Now()
	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.DeletionTimestamp = &now

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client: fakeClient,
		Log:    testLogger(),
		Scheme: scheme,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify object was deleted (when finalizer is removed from object with deletion timestamp,
	// Kubernetes deletes the object)
	var updated guardianv1alpha1.CronJobMonitor
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	assert.True(t, apierrors.IsNotFound(err), "monitor should be deleted after finalizer removal")
}

func TestReconcile_InvalidSpec(t *testing.T) {
	// Note: Current implementation always returns nil from validateSpec
	// This test verifies the flow when validation passes with invalid-looking spec
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	// Spec with invalid selector - but current validateSpec returns nil
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"invalid": "label"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	// Should still reconcile successfully
	assert.True(t, result.RequeueAfter > 0)
}

func TestFindMatchingCronJobs_Labels(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "test"},
	}

	matchingCJ := newTestCronJob("matching-cj", "default", map[string]string{"app": "test"})
	nonMatchingCJ := newTestCronJob("non-matching-cj", "default", map[string]string{"app": "other"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, matchingCJ, nonMatchingCJ).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	cronJobs, err := r.findMatchingCronJobs(context.Background(), monitor)
	require.NoError(t, err)
	assert.Len(t, cronJobs, 1)
	assert.Equal(t, "matching-cj", cronJobs[0].Name)
}

func TestFindMatchingCronJobs_Expressions(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "env",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"prod", "staging"},
			},
		},
	}

	prodCJ := newTestCronJob("prod-cj", "default", map[string]string{"env": "prod"})
	stagingCJ := newTestCronJob("staging-cj", "default", map[string]string{"env": "staging"})
	devCJ := newTestCronJob("dev-cj", "default", map[string]string{"env": "dev"})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, prodCJ, stagingCJ, devCJ).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	cronJobs, err := r.findMatchingCronJobs(context.Background(), monitor)
	require.NoError(t, err)
	assert.Len(t, cronJobs, 2)

	names := []string{cronJobs[0].Name, cronJobs[1].Name}
	assert.Contains(t, names, "prod-cj")
	assert.Contains(t, names, "staging-cj")
}

func TestFindMatchingCronJobs_Names(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		MatchNames: []string{"cj-a", "cj-b"},
	}

	cjA := newTestCronJob("cj-a", "default", nil)
	cjB := newTestCronJob("cj-b", "default", nil)
	cjC := newTestCronJob("cj-c", "default", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cjA, cjB, cjC).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	cronJobs, err := r.findMatchingCronJobs(context.Background(), monitor)
	require.NoError(t, err)
	assert.Len(t, cronJobs, 2)

	names := []string{cronJobs[0].Name, cronJobs[1].Name}
	assert.Contains(t, names, "cj-a")
	assert.Contains(t, names, "cj-b")
}

func TestFindMatchingCronJobs_Namespaces(t *testing.T) {
	scheme := newTestScheme()

	ns1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
	ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}
	ns3 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns3"}}

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		Namespaces: []string{"ns1", "ns2"},
	}

	cjNs1 := newTestCronJob("cj-ns1", "ns1", nil)
	cjNs2 := newTestCronJob("cj-ns2", "ns2", nil)
	cjNs3 := newTestCronJob("cj-ns3", "ns3", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, ns1, ns2, ns3, cjNs1, cjNs2, cjNs3).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	cronJobs, err := r.findMatchingCronJobs(context.Background(), monitor)
	require.NoError(t, err)
	assert.Len(t, cronJobs, 2)

	namespaces := []string{cronJobs[0].Namespace, cronJobs[1].Namespace}
	assert.Contains(t, namespaces, "ns1")
	assert.Contains(t, namespaces, "ns2")
}

func TestFindMatchingCronJobs_NamespaceSelector(t *testing.T) {
	scheme := newTestScheme()

	nsProd := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "prod",
			Labels: map[string]string{"env": "production"},
		},
	}
	nsDev := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "dev",
			Labels: map[string]string{"env": "development"},
		},
	}

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "production"},
		},
	}

	cjProd := newTestCronJob("cj-prod", "prod", nil)
	cjDev := newTestCronJob("cj-dev", "dev", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, nsProd, nsDev, cjProd, cjDev).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	cronJobs, err := r.findMatchingCronJobs(context.Background(), monitor)
	require.NoError(t, err)
	assert.Len(t, cronJobs, 1)
	assert.Equal(t, "prod", cronJobs[0].Namespace)
}

func TestFindMatchingCronJobs_AllNamespaces(t *testing.T) {
	scheme := newTestScheme()

	ns1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
	ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		AllNamespaces: true,
	}

	cjNs1 := newTestCronJob("cj-ns1", "ns1", nil)
	cjNs2 := newTestCronJob("cj-ns2", "ns2", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, ns1, ns2, cjNs1, cjNs2).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	cronJobs, err := r.findMatchingCronJobs(context.Background(), monitor)
	require.NoError(t, err)
	assert.Len(t, cronJobs, 2)
}

func TestProcessCronJob_CalculatesStatus(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	cronJob := newTestCronJob("test-cj", "default", nil)

	mockStore := &testutil.MockStore{
		LastSuccessExec: &store.Execution{
			CompletionTime: time.Now().Add(-1 * time.Hour),
			StartTime:      time.Now().Add(-1*time.Hour - 5*time.Minute),
		},
	}

	mockAnalyzer := &testutil.MockAnalyzer{
		Metrics: &guardianv1alpha1.CronJobMetrics{
			SuccessRate: 95.0,
			TotalRuns:   20,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cronJob).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Store:    mockStore,
		Analyzer: mockAnalyzer,
	}

	status := r.processCronJob(context.Background(), monitor, cronJob)

	assert.Equal(t, "test-cj", status.Name)
	assert.Equal(t, "default", status.Namespace)
	assert.Equal(t, statusHealthy, status.Status)
	assert.NotNil(t, status.LastSuccessfulTime)
	assert.NotNil(t, status.Metrics)
	assert.Equal(t, 95.0, status.Metrics.SuccessRate)
}

func TestProcessCronJob_ChecksSLA(t *testing.T) {
	scheme := newTestScheme()

	enabled := true
	monitor := newTestMonitor("test-monitor", "default")
	monitor.Spec.SLA = &guardianv1alpha1.SLAConfig{
		Enabled:        &enabled,
		MinSuccessRate: ptrTo(float64(99)),
	}
	cronJob := newTestCronJob("test-cj", "default", nil)

	mockAnalyzer := &testutil.MockAnalyzer{
		SLAResult: &analyzer.SLAResult{
			Passed: false,
			Violations: []analyzer.Violation{
				{Type: "SLABreached", Message: "Success rate below threshold"},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cronJob).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: mockAnalyzer,
	}

	status := r.processCronJob(context.Background(), monitor, cronJob)

	assert.Greater(t, mockAnalyzer.CheckSLACalled, 0)
	assert.Len(t, status.ActiveAlerts, 1)
	assert.Equal(t, "SLABreached", status.ActiveAlerts[0].Type)
}

func TestProcessCronJob_UpdatesMetrics(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	cronJob := newTestCronJob("test-cj", "default", nil)

	mockAnalyzer := &testutil.MockAnalyzer{
		Metrics: &guardianv1alpha1.CronJobMetrics{
			SuccessRate:        90.0,
			TotalRuns:          100,
			P50DurationSeconds: 30.0,
			P95DurationSeconds: 60.0,
			P99DurationSeconds: 90.0,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cronJob).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: mockAnalyzer,
	}

	status := r.processCronJob(context.Background(), monitor, cronJob)

	// Verify metrics are present in status
	require.NotNil(t, status.Metrics)
	assert.Equal(t, 90.0, status.Metrics.SuccessRate)
	assert.Equal(t, int32(100), status.Metrics.TotalRuns)
}

func TestHandleRemovedCronJobs(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	monitor.Status.CronJobs = []guardianv1alpha1.CronJobStatus{
		{Name: "existing-cj", Namespace: "default"},
		{Name: "removed-cj", Namespace: "default"},
	}
	monitor.Spec.DataRetention = &guardianv1alpha1.DataRetentionConfig{
		OnCronJobDeletion: "purge",
	}

	existingCJ := newTestCronJob("existing-cj", "default", nil)

	mockStore := &testutil.MockStore{}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, existingCJ).
		Build()

	r := &CronJobMonitorReconciler{
		Client: fakeClient,
		Log:    testLogger(),
		Scheme: scheme,
		Store:  mockStore,
	}

	currentCronJobs := []batchv1.CronJob{*existingCJ}
	r.handleRemovedCronJobs(context.Background(), monitor, currentCronJobs)

	assert.Equal(t, 1, mockStore.DeleteByCronJobCalled)
}

func TestUpdateStatus_Phase(t *testing.T) {
	tests := []struct {
		name          string
		summary       *guardianv1alpha1.MonitorSummary
		expectedPhase string
	}{
		{
			name: "Active phase with healthy CronJobs",
			summary: &guardianv1alpha1.MonitorSummary{
				TotalCronJobs: 5,
				Healthy:       5,
			},
			expectedPhase: phaseActive,
		},
		{
			name: "Degraded phase with warnings",
			summary: &guardianv1alpha1.MonitorSummary{
				TotalCronJobs: 5,
				Healthy:       3,
				Warning:       2,
			},
			expectedPhase: phaseDegraded,
		},
		{
			name: "Error phase with critical",
			summary: &guardianv1alpha1.MonitorSummary{
				TotalCronJobs: 5,
				Healthy:       2,
				Warning:       2,
				Critical:      1,
			},
			expectedPhase: phaseError,
		},
		{
			name: "Active phase with no CronJobs",
			summary: &guardianv1alpha1.MonitorSummary{
				TotalCronJobs: 0,
			},
			expectedPhase: phaseActive,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &CronJobMonitorReconciler{Log: testLogger()}
			phase := r.determinePhase(tc.summary)
			assert.Equal(t, tc.expectedPhase, phase)
		})
	}
}

func TestUpdateStatus_Conditions(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	controllerutil.AddFinalizer(monitor, finalizerName)

	cronJob := newTestCronJob("test-cj", "default", nil)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(monitor, cronJob).
		WithStatusSubresource(monitor).
		Build()

	r := &CronJobMonitorReconciler{
		Client:   fakeClient,
		Log:      testLogger(),
		Scheme:   scheme,
		Analyzer: &testutil.MockAnalyzer{},
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-monitor",
			Namespace: "default",
		},
	}

	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.CronJobMonitor
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)

	// Check that Ready condition is set
	var readyCondition *metav1.Condition
	for i := range updated.Status.Conditions {
		if updated.Status.Conditions[i].Type == "Ready" {
			readyCondition = &updated.Status.Conditions[i]
			break
		}
	}
	require.NotNil(t, readyCondition)
	assert.Equal(t, metav1.ConditionTrue, readyCondition.Status)
	assert.Equal(t, "Reconciled", readyCondition.Reason)
}

func TestUpdateStatus_Summary(t *testing.T) {
	statuses := []guardianv1alpha1.CronJobStatus{
		{Name: "cj1", Status: statusHealthy, Suspended: false},
		{Name: "cj2", Status: statusHealthy, Suspended: false},
		{Name: "cj3", Status: statusWarning, Suspended: false},
		{Name: "cj4", Status: statusCritical, Suspended: true},
		{Name: "cj5", Status: statusHealthy, Suspended: true, ActiveJobs: []guardianv1alpha1.ActiveJob{{Name: "job-1"}}},
	}

	r := &CronJobMonitorReconciler{Log: testLogger()}
	summary := r.calculateSummary(statuses)

	assert.Equal(t, int32(5), summary.TotalCronJobs)
	assert.Equal(t, int32(3), summary.Healthy)
	assert.Equal(t, int32(1), summary.Warning)
	assert.Equal(t, int32(1), summary.Critical)
	assert.Equal(t, int32(2), summary.Suspended)
	assert.Equal(t, int32(1), summary.Running)
}

// ============================================================================
// Selector Matcher Tests
// ============================================================================

func TestMatchesSelector_NilSelector(t *testing.T) {
	cj := newTestCronJob("test-cj", "default", nil)
	assert.True(t, MatchesSelector(cj, nil))
}

func TestMatchesSelector_MatchLabels(t *testing.T) {
	cj := newTestCronJob("test-cj", "default", map[string]string{"app": "test", "env": "prod"})

	selector := &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "test"},
	}
	assert.True(t, MatchesSelector(cj, selector))

	selector = &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "other"},
	}
	assert.False(t, MatchesSelector(cj, selector))
}

func TestMatchesSelector_MatchExpressions(t *testing.T) {
	cj := newTestCronJob("test-cj", "default", map[string]string{"env": "prod"})

	selector := &guardianv1alpha1.CronJobSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod", "staging"}},
		},
	}
	assert.True(t, MatchesSelector(cj, selector))

	selector = &guardianv1alpha1.CronJobSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "env", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"prod"}},
		},
	}
	assert.False(t, MatchesSelector(cj, selector))
}

func TestMatchesSelector_MatchNames(t *testing.T) {
	cj := newTestCronJob("my-cronjob", "default", nil)

	selector := &guardianv1alpha1.CronJobSelector{
		MatchNames: []string{"my-cronjob", "other-cronjob"},
	}
	assert.True(t, MatchesSelector(cj, selector))

	selector = &guardianv1alpha1.CronJobSelector{
		MatchNames: []string{"other-cronjob"},
	}
	assert.False(t, MatchesSelector(cj, selector))
}

func TestMatchExpression_OpExists(t *testing.T) {
	labels := map[string]string{"key1": "value1"}

	expr := metav1.LabelSelectorRequirement{Key: "key1", Operator: metav1.LabelSelectorOpExists}
	assert.True(t, MatchExpression(labels, expr))

	expr = metav1.LabelSelectorRequirement{Key: "key2", Operator: metav1.LabelSelectorOpExists}
	assert.False(t, MatchExpression(labels, expr))
}

func TestMatchExpression_OpDoesNotExist(t *testing.T) {
	labels := map[string]string{"key1": "value1"}

	expr := metav1.LabelSelectorRequirement{Key: "key2", Operator: metav1.LabelSelectorOpDoesNotExist}
	assert.True(t, MatchExpression(labels, expr))

	expr = metav1.LabelSelectorRequirement{Key: "key1", Operator: metav1.LabelSelectorOpDoesNotExist}
	assert.False(t, MatchExpression(labels, expr))
}

// Helper function for creating pointers
func ptrTo[T any](v T) *T {
	return &v
}

// ============================================================================
// findMonitorsForCronJob Tests
// ============================================================================

func TestFindMonitorsForCronJob_ReturnsMatchingMonitors(t *testing.T) {
	scheme := newTestScheme()

	cronJob := newTestCronJob("test-cj", "default", map[string]string{"app": "test"})

	monitor1 := newTestMonitor("monitor1", "default")
	monitor1.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "test"},
	}

	monitor2 := newTestMonitor("monitor2", "default")
	monitor2.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "other"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cronJob, monitor1, monitor2).
		Build()

	r := &CronJobMonitorReconciler{
		Client: fakeClient,
		Log:    testLogger(),
		Scheme: scheme,
	}

	requests := r.findMonitorsForCronJob(context.Background(), cronJob)

	assert.Len(t, requests, 1)
	assert.Equal(t, "monitor1", requests[0].Name)
}

func TestMonitorWatchesNamespace_NoSelector(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &CronJobMonitorReconciler{
		Client: fakeClient,
		Log:    testLogger(),
		Scheme: scheme,
	}

	// Should only watch its own namespace
	assert.True(t, r.monitorWatchesNamespace(context.Background(), monitor, "default"))
	assert.False(t, r.monitorWatchesNamespace(context.Background(), monitor, "other"))
}

func TestMonitorWatchesNamespace_AllNamespaces(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		AllNamespaces: true,
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &CronJobMonitorReconciler{
		Client: fakeClient,
		Log:    testLogger(),
		Scheme: scheme,
	}

	assert.True(t, r.monitorWatchesNamespace(context.Background(), monitor, "default"))
	assert.True(t, r.monitorWatchesNamespace(context.Background(), monitor, "any-namespace"))
}

func TestMonitorWatchesNamespace_ExplicitNamespaces(t *testing.T) {
	scheme := newTestScheme()

	monitor := newTestMonitor("test-monitor", "default")
	monitor.Spec.Selector = &guardianv1alpha1.CronJobSelector{
		Namespaces: []string{"ns1", "ns2"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	r := &CronJobMonitorReconciler{
		Client: fakeClient,
		Log:    testLogger(),
		Scheme: scheme,
	}

	assert.True(t, r.monitorWatchesNamespace(context.Background(), monitor, "ns1"))
	assert.True(t, r.monitorWatchesNamespace(context.Background(), monitor, "ns2"))
	assert.False(t, r.monitorWatchesNamespace(context.Background(), monitor, "ns3"))
}

// ============================================================================
// Helper Functions Tests
// ============================================================================

func TestCalculateNextRun(t *testing.T) {
	result := calculateNextRun("0 * * * *", nil)
	require.NotNil(t, result)
	// Next run should be in the future
	assert.True(t, result.After(time.Now()))
}

func TestCalculateNextRun_WithTimezone(t *testing.T) {
	tz := "America/New_York"
	result := calculateNextRun("0 9 * * *", &tz)
	require.NotNil(t, result)
	assert.True(t, result.After(time.Now()))
}

func TestCalculateNextRun_InvalidSchedule(t *testing.T) {
	result := calculateNextRun("invalid", nil)
	assert.Nil(t, result)
}

func TestIsEnabled(t *testing.T) {
	assert.True(t, isEnabled(nil))

	enabled := true
	assert.True(t, isEnabled(&enabled))

	disabled := false
	assert.False(t, isEnabled(&disabled))
}

func TestGetSeverity(t *testing.T) {
	assert.Equal(t, "critical", getSeverity("critical", "warning"))
	assert.Equal(t, "warning", getSeverity("", "warning"))
}
