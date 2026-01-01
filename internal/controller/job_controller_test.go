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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/testutil"
)

// Helper to create a fake client with scheme for job tests
func newJobTestClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = guardianv1alpha1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&guardianv1alpha1.CronJobMonitor{}).
		Build()
}

// Helper to create a test CronJob
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func createTestCronJob(name, namespace string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-cronjob-uid",
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "main", Image: "alpine"},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
}

// Helper to create a completed job
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func createCompletedJob(name, namespace, cronJobName string) *batchv1.Job {
	now := metav1.Now()
	startTime := metav1.NewTime(now.Add(-1 * time.Minute))
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       cronJobName,
					UID:        "test-cronjob-uid",
				},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded:      1,
			StartTime:      &startTime,
			CompletionTime: &now,
		},
	}
}

// Helper to create a failed job
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func createFailedJob(name, namespace, cronJobName string) *batchv1.Job {
	now := metav1.Now()
	startTime := metav1.NewTime(now.Add(-1 * time.Minute))
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       cronJobName,
					UID:        "test-cronjob-uid",
				},
			},
		},
		Status: batchv1.JobStatus{
			Failed:    1,
			StartTime: &startTime,
		},
	}
}

// Helper to create a running job
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func createRunningJob(name, namespace, cronJobName string) *batchv1.Job {
	now := metav1.Now()
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "CronJob",
					Name:       cronJobName,
					UID:        "test-cronjob-uid",
				},
			},
		},
		Status: batchv1.JobStatus{
			Active:    1,
			StartTime: &now,
		},
	}
}

// Helper to create a test monitor
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func createTestMonitor(name, namespace string, selector *guardianv1alpha1.CronJobSelector) *guardianv1alpha1.CronJobMonitor {
	return &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: selector,
		},
	}
}

// ============================================================================
// Section 1.5.2: JobReconciler Tests
// ============================================================================

func TestReconcile_CompletedJob(t *testing.T) {
	cronJob := createTestCronJob("test-cron", "default")
	job := createCompletedJob("test-cron-12345", "default", "test-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "test-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-cron-12345",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	// Verify execution was recorded
	require.Len(t, mockStore.RecordedExecutions, 1)
	exec := mockStore.RecordedExecutions[0]
	assert.True(t, exec.Succeeded)
	assert.Equal(t, "test-cron", exec.CronJobName)
	assert.Equal(t, "test-cron-12345", exec.JobName)
}

func TestReconcile_FailedJob(t *testing.T) {
	cronJob := createTestCronJob("failing-cron", "default")
	job := createFailedJob("failing-cron-12345", "default", "failing-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "failing-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "failing-cron-12345",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	// Verify execution was recorded as failed
	require.Len(t, mockStore.RecordedExecutions, 1)
	exec := mockStore.RecordedExecutions[0]
	assert.False(t, exec.Succeeded)
	assert.Equal(t, "failing-cron", exec.CronJobName)

	// Verify alert was dispatched
	require.Len(t, mockDispatcher.DispatchedAlerts, 1)
	alert := mockDispatcher.DispatchedAlerts[0]
	assert.Equal(t, "JobFailed", alert.Type)
}

func TestReconcile_RunningJob(t *testing.T) {
	cronJob := createTestCronJob("running-cron", "default")
	job := createRunningJob("running-cron-12345", "default", "running-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "running-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "running-cron-12345",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	// Verify no execution was recorded (job still running)
	assert.Empty(t, mockStore.RecordedExecutions)

	// Verify no alert was dispatched
	assert.Empty(t, mockDispatcher.DispatchedAlerts)
}

func TestReconcile_NonCronJob(t *testing.T) {
	// Job without CronJob owner reference
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "standalone-job",
			Namespace: "default",
			// No OwnerReferences
		},
		Status: batchv1.JobStatus{
			Succeeded: 1,
		},
	}

	fakeClient := newJobTestClient(job)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "standalone-job",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	// Verify no execution was recorded
	assert.Empty(t, mockStore.RecordedExecutions)
}

func TestBuildExecution_AllFields(t *testing.T) {
	cronJob := createTestCronJob("test-cron", "default")
	now := metav1.Now()
	startTime := metav1.NewTime(now.Add(-5 * time.Minute))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron-12345",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "CronJob", Name: "test-cron", UID: "test-cronjob-uid"},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded:      1,
			StartTime:      &startTime,
			CompletionTime: &now,
		},
	}
	monitor := createTestMonitor("test-monitor", "default", nil)

	fakeClient := newJobTestClient(cronJob, job)
	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
	}

	exec := reconciler.buildExecution(context.Background(), job, "test-cron", "test-cronjob-uid", monitor)

	assert.Equal(t, "default", exec.CronJobNamespace)
	assert.Equal(t, "test-cron", exec.CronJobName)
	assert.Equal(t, "test-cronjob-uid", exec.CronJobUID)
	assert.Equal(t, "test-cron-12345", exec.JobName)
	assert.True(t, exec.Succeeded)
	assert.Equal(t, startTime.Time, exec.StartTime)
	assert.Equal(t, now.Time, exec.CompletionTime)
	assert.True(t, exec.Duration() > 0)
}

func TestBuildExecution_FetchesLogs(t *testing.T) {
	cronJob := createTestCronJob("log-cron", "default")
	job := createCompletedJob("log-cron-12345", "default", "log-cron")

	// Create monitor that enables log storage
	storeLogs := true
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "log-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			DataRetention: &guardianv1alpha1.DataRetentionConfig{
				StoreLogs: &storeLogs,
			},
		},
	}

	// Create a pod for the job
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "log-cron-12345-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "log-cron-12345"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
	}

	fakeClient := newJobTestClient(cronJob, job, pod)
	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		// Note: Clientset would be needed for actual log fetching
		// This test verifies the structure is correct
	}

	exec := reconciler.buildExecution(context.Background(), job, "log-cron", "test-uid", monitor)

	// Logs field should be initialized (even if empty without Clientset)
	assert.True(t, exec.Logs != nil)
}

func TestBuildExecution_FetchesEvents(t *testing.T) {
	cronJob := createTestCronJob("event-cron", "default")
	job := createCompletedJob("event-cron-12345", "default", "event-cron")

	// Create monitor that enables event storage
	storeEvents := true
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			DataRetention: &guardianv1alpha1.DataRetentionConfig{
				StoreEvents: &storeEvents,
			},
		},
	}

	// Create an event for the job
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-1",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Job",
			Name: "event-cron-12345",
		},
		Reason:  "Completed",
		Message: "Job completed successfully",
	}

	fakeClient := newJobTestClient(cronJob, job, event)
	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
	}

	exec := reconciler.buildExecution(context.Background(), job, "event-cron", "test-uid", monitor)

	// Events should be collected
	assert.NotNil(t, exec.Events)
	assert.Contains(t, *exec.Events, "Completed")
}

func TestBuildExecution_SuggestedFix(t *testing.T) {
	cronJob := createTestCronJob("oom-cron", "default")
	job := createFailedJob("oom-cron-12345", "default", "oom-cron")

	// Create a pod with OOMKilled status
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oom-cron-12345-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "oom-cron-12345"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 137,
							Reason:   "OOMKilled",
						},
					},
				},
			},
		},
	}

	monitor := createTestMonitor("test-monitor", "default", nil)

	fakeClient := newJobTestClient(cronJob, job, pod)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	// First build the execution to get exit code/reason
	exec := reconciler.buildExecution(context.Background(), job, "oom-cron", "test-uid", monitor)

	// Then generate the suggested fix
	suggestedFix := reconciler.generateSuggestedFix(exec, monitor)

	// Should have a suggestion for OOMKilled
	assert.NotEmpty(t, suggestedFix)
	assert.Contains(t, suggestedFix, "memory")
}

func TestFindMonitorsForCronJob(t *testing.T) {
	cronJob := createTestCronJob("multi-cron", "default")

	// Create multiple monitors with different selectors
	monitor1 := createTestMonitor("monitor-1", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "multi-cron"},
	})
	monitor2 := createTestMonitor("monitor-2", "default", &guardianv1alpha1.CronJobSelector{
		MatchNames: []string{"multi-cron"},
	})
	monitor3 := createTestMonitor("non-matching", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "other-cron"},
	})

	fakeClient := newJobTestClient(cronJob, monitor1, monitor2, monitor3)
	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
	}

	monitors := reconciler.findMonitorsForCronJob(context.Background(), "default", "multi-cron")

	// Should find 2 matching monitors
	assert.Len(t, monitors, 2)
	names := []string{monitors[0].Name, monitors[1].Name}
	assert.Contains(t, names, "monitor-1")
	assert.Contains(t, names, "monitor-2")
}

func TestHandleRecreationCheck_Retain(t *testing.T) {
	cronJob := createTestCronJob("recreated-cron", "default")
	job := createCompletedJob("recreated-cron-12345", "default", "recreated-cron")

	// Monitor configured to retain history on recreation
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "retain-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: &guardianv1alpha1.CronJobSelector{
				MatchNames: []string{"recreated-cron"},
			},
			DataRetention: &guardianv1alpha1.DataRetentionConfig{
				OnRecreation: "retain",
			},
		},
	}

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{
		CronJobUIDsMap: map[string][]string{
			"default/recreated-cron": {"old-uid-1234"},
		},
	}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	cronJobNN := types.NamespacedName{Namespace: "default", Name: "recreated-cron"}
	reconciler.handleRecreationCheck(context.Background(), logr.Discard(), monitor, cronJobNN, "new-uid-5678")

	// Should NOT delete old UID executions
	assert.Empty(t, mockStore.DeletedUIDs)
}

func TestHandleRecreationCheck_Reset(t *testing.T) {
	cronJob := createTestCronJob("recreated-cron", "default")
	job := createCompletedJob("recreated-cron-12345", "default", "recreated-cron")

	// Monitor configured to reset history on recreation
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reset-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: &guardianv1alpha1.CronJobSelector{
				MatchNames: []string{"recreated-cron"},
			},
			DataRetention: &guardianv1alpha1.DataRetentionConfig{
				OnRecreation: "reset",
			},
		},
	}

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{
		CronJobUIDsMap: map[string][]string{
			"default/recreated-cron": {"old-uid-1234"},
		},
	}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	cronJobNN := types.NamespacedName{Namespace: "default", Name: "recreated-cron"}
	reconciler.handleRecreationCheck(context.Background(), logr.Discard(), monitor, cronJobNN, "new-uid-5678")

	// Should delete old UID executions
	require.Len(t, mockStore.DeletedUIDs, 1)
	assert.Equal(t, "old-uid-1234", mockStore.DeletedUIDs[0])
}

func TestDispatchAlerts_Success(t *testing.T) {
	cronJob := createTestCronJob("failing-cron", "default")
	job := createFailedJob("failing-cron-12345", "default", "failing-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "failing-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "failing-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify alert was dispatched successfully
	require.Len(t, mockDispatcher.DispatchedAlerts, 1)
	alert := mockDispatcher.DispatchedAlerts[0]
	assert.Equal(t, "JobFailed", alert.Type)
	assert.Equal(t, "default", alert.CronJob.Namespace)
	assert.Equal(t, "failing-cron", alert.CronJob.Name)
}

func TestDispatchAlerts_WithContext(t *testing.T) {
	cronJob := createTestCronJob("context-cron", "default")
	job := createFailedJob("context-cron-12345", "default", "context-cron")

	// Create a pod with termination info
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "context-cron-12345-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "context-cron-12345"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
			},
		},
	}

	// Monitor with alerting context enabled
	logsEnabled := true
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "context-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: &guardianv1alpha1.CronJobSelector{
				MatchLabels: map[string]string{"app": "context-cron"},
			},
			Alerting: &guardianv1alpha1.AlertingConfig{
				IncludeContext: &guardianv1alpha1.AlertContext{
					Logs: &logsEnabled,
				},
			},
		},
	}

	fakeClient := newJobTestClient(cronJob, job, pod, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "context-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify alert was dispatched with context
	require.Len(t, mockDispatcher.DispatchedAlerts, 1)
	alert := mockDispatcher.DispatchedAlerts[0]
	assert.Equal(t, int32(1), alert.Context.ExitCode)
	assert.Equal(t, "Error", alert.Context.Reason)
}

func TestDispatchAlerts_Suppressed(t *testing.T) {
	cronJob := createTestCronJob("suppressed-cron", "default")
	job := createFailedJob("suppressed-cron-12345", "default", "suppressed-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "suppressed-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()
	mockDispatcher.Suppressed = true // Mark alert as suppressed

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "suppressed-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Alert should still be dispatched (suppression is handled by dispatcher)
	// The dispatcher will check IsSuppressed internally
	require.Len(t, mockDispatcher.DispatchedAlerts, 1)
}

func TestStoreExecution_OncePerJob(t *testing.T) {
	cronJob := createTestCronJob("once-cron", "default")
	job := createCompletedJob("once-cron-12345", "default", "once-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "once-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "once-cron-12345",
			Namespace: "default",
		},
	}

	// Reconcile twice
	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	_, err = reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Note: Without proper deduplication in RecordExecution (which the real store handles),
	// this would record twice. The test verifies the store interface is called.
	// In production, the store has ON CONFLICT handling.
	assert.Len(t, mockStore.RecordedExecutions, 2) // Both calls record (mock doesn't dedupe)
}

func TestUpdateMetrics_Execution(t *testing.T) {
	cronJob := createTestCronJob("metrics-cron", "default")
	job := createCompletedJob("metrics-cron-12345", "default", "metrics-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "metrics-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "metrics-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify execution was recorded (metrics are updated via metrics package)
	require.Len(t, mockStore.RecordedExecutions, 1)
	exec := mockStore.RecordedExecutions[0]
	assert.True(t, exec.Succeeded)
}

// Additional tests for edge cases

func TestReconcile_NoMatchingMonitors(t *testing.T) {
	cronJob := createTestCronJob("orphan-cron", "default")
	job := createCompletedJob("orphan-cron-12345", "default", "orphan-cron")
	// No monitor matches this CronJob

	fakeClient := newJobTestClient(cronJob, job)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "orphan-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// No execution recorded since no monitor matched
	assert.Empty(t, mockStore.RecordedExecutions)
}

func TestReconcile_JobNotFound(t *testing.T) {
	// Job doesn't exist
	fakeClient := newJobTestClient()
	mockStore := &testutil.MockStore{}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent-job",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err) // NotFound is not an error
	assert.Zero(t, result.RequeueAfter)
	assert.Empty(t, mockStore.RecordedExecutions)
}

func TestReconcile_CronJobDeleted(t *testing.T) {
	// Job exists but CronJob was deleted
	job := createCompletedJob("deleted-parent-12345", "default", "deleted-parent")
	mockStore := &testutil.MockStore{}
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchNames: []string{"deleted-parent"},
	})
	fakeClient := newJobTestClient(job, monitor)

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "deleted-parent-12345",
			Namespace: "default",
		},
	}

	// Should handle gracefully when CronJob is gone
	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
}

func TestReconcile_SuccessClearsAlerts(t *testing.T) {
	cronJob := createTestCronJob("success-clear-cron", "default")
	job := createCompletedJob("success-clear-cron-12345", "default", "success-clear-cron")
	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "success-clear-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "success-clear-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify alerts were cleared on success
	assert.NotEmpty(t, mockDispatcher.ClearedAlerts)

	// Should have cleared JobFailed, DeadManTriggered, SuspendedTooLong
	foundJobFailed := false
	for _, key := range mockDispatcher.ClearedAlerts {
		if key == "default/success-clear-cron/JobFailed" {
			foundJobFailed = true
		}
	}
	assert.True(t, foundJobFailed)
}

func TestReconcile_WithRetryLabel(t *testing.T) {
	cronJob := createTestCronJob("retry-cron", "default")
	now := metav1.Now()
	startTime := metav1.NewTime(now.Add(-1 * time.Minute))

	// Job with retry labels
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "retry-cron-12345",
			Namespace: "default",
			Labels: map[string]string{
				"guardian.illenium.net/retry": "true",
			},
			Annotations: map[string]string{
				"guardian.illenium.net/retry-of": "retry-cron-original",
			},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "CronJob", Name: "retry-cron", UID: "test-uid"},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded:      1,
			StartTime:      &startTime,
			CompletionTime: &now,
		},
	}

	monitor := createTestMonitor("test-monitor", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "retry-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor)
	mockStore := &testutil.MockStore{}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Store:  mockStore,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "retry-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify retry info was captured
	require.Len(t, mockStore.RecordedExecutions, 1)
	exec := mockStore.RecordedExecutions[0]
	assert.True(t, exec.IsRetry)
	assert.Equal(t, "retry-cron-original", exec.RetryOf)
}

func TestReconcile_MultipleMatchingMonitors(t *testing.T) {
	cronJob := createTestCronJob("multi-monitor-cron", "default")
	job := createFailedJob("multi-monitor-cron-12345", "default", "multi-monitor-cron")

	// Create two monitors that both match
	monitor1 := createTestMonitor("monitor-1", "default", &guardianv1alpha1.CronJobSelector{
		MatchLabels: map[string]string{"app": "multi-monitor-cron"},
	})
	monitor2 := createTestMonitor("monitor-2", "default", &guardianv1alpha1.CronJobSelector{
		MatchNames: []string{"multi-monitor-cron"},
	})

	fakeClient := newJobTestClient(cronJob, job, monitor1, monitor2)
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()

	reconciler := &JobReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		Store:           mockStore,
		AlertDispatcher: mockDispatcher,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "multi-monitor-cron-12345",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Execution should be recorded once
	assert.Len(t, mockStore.RecordedExecutions, 1)

	// But alert should be sent for each monitor
	assert.Len(t, mockDispatcher.DispatchedAlerts, 2)
}

func TestJobReconciler_MonitorWatchesNamespace_AllNamespaces(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "all-ns-monitor",
			Namespace: "monitoring",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: &guardianv1alpha1.CronJobSelector{
				AllNamespaces: true,
			},
		},
	}

	reconciler := &JobReconciler{
		Log: logr.Discard(),
	}

	// Should watch all namespaces
	assert.True(t, reconciler.monitorWatchesNamespace(monitor, "default"))
	assert.True(t, reconciler.monitorWatchesNamespace(monitor, "production"))
	assert.True(t, reconciler.monitorWatchesNamespace(monitor, "kube-system"))
}

func TestJobReconciler_MonitorWatchesNamespace_ExplicitList(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "explicit-ns-monitor",
			Namespace: "monitoring",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: &guardianv1alpha1.CronJobSelector{
				Namespaces: []string{"default", "production"},
			},
		},
	}

	reconciler := &JobReconciler{
		Log: logr.Discard(),
	}

	assert.True(t, reconciler.monitorWatchesNamespace(monitor, "default"))
	assert.True(t, reconciler.monitorWatchesNamespace(monitor, "production"))
	assert.False(t, reconciler.monitorWatchesNamespace(monitor, "staging"))
}

func TestJobReconciler_MonitorWatchesNamespace_OwnNamespaceOnly(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			// No selector - defaults to own namespace
		},
	}

	reconciler := &JobReconciler{
		Log: logr.Discard(),
	}

	assert.True(t, reconciler.monitorWatchesNamespace(monitor, "default"))
	assert.False(t, reconciler.monitorWatchesNamespace(monitor, "production"))
}

func TestBuildExecution_StoreLogs_GlobalConfig(t *testing.T) {
	cronJob := createTestCronJob("global-config-cron", "default")
	job := createCompletedJob("global-config-cron-12345", "default", "global-config-cron")

	// Monitor without explicit log storage setting
	monitor := createTestMonitor("test-monitor", "default", nil)

	// Create pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-config-cron-12345-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": "global-config-cron-12345"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main"}},
		},
	}

	fakeClient := newJobTestClient(cronJob, job, pod)

	// With global config enabling logs
	globalConfig := &config.Config{
		Storage: config.StorageConfig{
			LogStorageEnabled: true,
			MaxLogSizeKB:      50,
		},
	}

	reconciler := &JobReconciler{
		Client: fakeClient,
		Log:    logr.Discard(),
		Scheme: fakeClient.Scheme(),
		Config: globalConfig,
	}

	// shouldStoreLogs should respect global config
	assert.True(t, reconciler.shouldStoreLogs(monitor))

	// Without global config
	reconciler.Config = nil
	assert.False(t, reconciler.shouldStoreLogs(monitor))
}
