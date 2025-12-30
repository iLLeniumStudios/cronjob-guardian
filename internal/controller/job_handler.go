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
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/remediation"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// JobHandler handles Job events for execution tracking
type JobHandler struct {
	client.Client
	Scheme            *runtime.Scheme
	Clientset         *kubernetes.Clientset
	Store             store.Store
	AlertDispatcher   alerting.Dispatcher
	RemediationEngine remediation.Engine
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

// Reconcile handles Job completion/failure events
func (h *JobHandler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	job := &batchv1.Job{}
	if err := h.Get(ctx, req.NamespacedName, job); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get parent CronJob
	cronJobName := h.getCronJobOwner(job)
	if cronJobName == "" {
		return ctrl.Result{}, nil // Not owned by a CronJob
	}

	// Check if this CronJob is monitored
	monitor := h.findMonitorForCronJob(ctx, job.Namespace, cronJobName)
	if monitor == nil {
		return ctrl.Result{}, nil // Not monitored
	}

	// Check job status
	if job.Status.CompletionTime == nil && job.Status.Failed == 0 {
		// Still running, nothing to record yet
		return ctrl.Result{}, nil
	}

	// Record execution
	exec := h.buildExecution(ctx, job, cronJobName)
	if h.Store != nil {
		if err := h.Store.RecordExecution(ctx, exec); err != nil {
			logger.Error(err, "failed to record execution")
		}
	}

	// Handle completion
	if job.Status.Succeeded > 0 {
		h.handleSuccess(ctx, monitor, job, cronJobName)
	} else if job.Status.Failed > 0 {
		h.handleFailure(ctx, monitor, job, cronJobName)
	}

	return ctrl.Result{}, nil
}

func (h *JobHandler) getCronJobOwner(job *batchv1.Job) string {
	for _, ref := range job.OwnerReferences {
		if ref.Kind == "CronJob" {
			return ref.Name
		}
	}
	return ""
}

func (h *JobHandler) findMonitorForCronJob(ctx context.Context, namespace, cronJobName string) *guardianv1alpha1.CronJobMonitor {
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.List(ctx, monitors, client.InNamespace(namespace)); err != nil {
		return nil
	}

	for _, monitor := range monitors.Items {
		for _, cj := range monitor.Status.CronJobs {
			if cj.Name == cronJobName && cj.Namespace == namespace {
				return &monitor
			}
		}
	}
	return nil
}

func (h *JobHandler) buildExecution(ctx context.Context, job *batchv1.Job, cronJobName string) store.Execution {
	exec := store.Execution{
		CronJobNamespace: job.Namespace,
		CronJobName:      cronJobName,
		JobName:          job.Name,
		Succeeded:        job.Status.Succeeded > 0,
	}

	if job.Status.StartTime != nil {
		exec.StartTime = job.Status.StartTime.Time
	}

	if job.Status.CompletionTime != nil {
		exec.CompletionTime = job.Status.CompletionTime.Time
		exec.Duration = exec.CompletionTime.Sub(exec.StartTime)
	}

	// Get exit code from pod
	if pod := h.getJobPod(ctx, job); pod != nil {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				exec.ExitCode = cs.State.Terminated.ExitCode
				exec.Reason = cs.State.Terminated.Reason
				break
			}
		}
	}

	// Check if this is a retry
	if job.Labels["guardian.illenium.net/retry"] == "true" {
		exec.IsRetry = true
		exec.RetryOf = job.Annotations["guardian.illenium.net/retry-of"]
	}

	return exec
}

func (h *JobHandler) getJobPod(ctx context.Context, job *batchv1.Job) *corev1.Pod {
	pods := &corev1.PodList{}
	if err := h.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{"job-name": job.Name}); err != nil {
		return nil
	}

	if len(pods.Items) > 0 {
		return &pods.Items[0]
	}
	return nil
}

func (h *JobHandler) handleSuccess(ctx context.Context, _ *guardianv1alpha1.CronJobMonitor, job *batchv1.Job, cronJobName string) {
	// Reset retry counter on success
	if h.RemediationEngine != nil {
		h.RemediationEngine.ResetRetryCount(job.Namespace, cronJobName)
	}

	// Clear any active failure alerts for this CronJob
	if h.AlertDispatcher != nil {
		alertKey := fmt.Sprintf("%s/%s/JobFailed", job.Namespace, cronJobName)
		_ = h.AlertDispatcher.ClearAlert(ctx, alertKey)
	}
}

func (h *JobHandler) handleFailure(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, job *batchv1.Job, cronJobName string) {
	logger := log.FromContext(ctx)

	// Collect context
	alertCtx := alerting.AlertContext{}

	includeCtx := monitor.Spec.Alerting.IncludeContext
	if includeCtx != nil && isEnabled(includeCtx.Logs) {
		alertCtx.Logs = h.collectLogs(ctx, job, includeCtx)
	}

	if includeCtx != nil && isEnabled(includeCtx.Events) {
		alertCtx.Events = h.collectEvents(ctx, job)
	}

	// Get exit code and reason
	if pod := h.getJobPod(ctx, job); pod != nil {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				alertCtx.ExitCode = cs.State.Terminated.ExitCode
				alertCtx.Reason = cs.State.Terminated.Reason
				break
			}
		}
	}

	if includeCtx != nil && isEnabled(includeCtx.SuggestedFixes) {
		alertCtx.SuggestedFix = h.getSuggestedFix(job, alertCtx)
	}

	// Create alert
	alert := alerting.Alert{
		Key:      fmt.Sprintf("%s/%s/JobFailed", job.Namespace, cronJobName),
		Type:     "JobFailed",
		Severity: getSeverity(monitor.Spec.Alerting.SeverityOverrides.JobFailed, "critical"),
		Title:    fmt.Sprintf("CronJob %s/%s failed", job.Namespace, cronJobName),
		Message:  h.buildFailureMessage(job, alertCtx),
		CronJob: types.NamespacedName{
			Namespace: job.Namespace,
			Name:      cronJobName,
		},
		Context: alertCtx,
		MonitorRef: types.NamespacedName{
			Namespace: monitor.Namespace,
			Name:      monitor.Name,
		},
		Timestamp: time.Now(),
	}

	// Dispatch alert
	if h.AlertDispatcher != nil {
		if err := h.AlertDispatcher.Dispatch(ctx, alert, monitor.Spec.Alerting); err != nil {
			logger.Error(err, "failed to dispatch alert")
		}
	}

	// Check for auto-retry
	if monitor.Spec.Remediation != nil && isEnabled(monitor.Spec.Remediation.Enabled) {
		if monitor.Spec.Remediation.AutoRetry != nil && monitor.Spec.Remediation.AutoRetry.Enabled {
			if h.RemediationEngine != nil {
				result, err := h.RemediationEngine.TryRetry(ctx, monitor, job, cronJobName)
				if err != nil {
					logger.Error(err, "failed to retry job")
				} else if result.Success {
					logger.Info("initiated retry", "job", result.JobName, "message", result.Message)
				}
			}
		}
	}
}

func (h *JobHandler) collectLogs(ctx context.Context, job *batchv1.Job, config *guardianv1alpha1.AlertContext) string {
	if h.Clientset == nil {
		return ""
	}

	pod := h.getJobPod(ctx, job)
	if pod == nil {
		return ""
	}

	containerName := ""
	if config != nil {
		containerName = config.LogContainerName
	}
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	logLines := int32(50)
	if config != nil && config.LogLines != nil {
		logLines = *config.LogLines
	}
	tailLines := int64(logLines)
	opts := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: ptr.To(tailLines),
	}

	req := h.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return ""
	}
	defer func() {
		_ = stream.Close()
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, stream)
	if err != nil {
		return ""
	}
	return buf.String()
}

func (h *JobHandler) collectEvents(ctx context.Context, job *batchv1.Job) []string {
	events := &corev1.EventList{}
	if err := h.List(ctx, events, client.InNamespace(job.Namespace)); err != nil {
		return nil
	}

	var result []string
	for _, e := range events.Items {
		if e.InvolvedObject.Kind == "Job" && e.InvolvedObject.Name == job.Name {
			result = append(result, fmt.Sprintf("%s: %s", e.Reason, e.Message))
		}
		if e.InvolvedObject.Kind == "Pod" && strings.HasPrefix(e.InvolvedObject.Name, job.Name) {
			result = append(result, fmt.Sprintf("%s: %s", e.Reason, e.Message))
		}
	}
	return result
}

func (h *JobHandler) buildFailureMessage(job *batchv1.Job, ctx alerting.AlertContext) string {
	msg := fmt.Sprintf("Job %s failed", job.Name)
	if ctx.Reason != "" {
		msg += fmt.Sprintf(" with reason: %s", ctx.Reason)
	}
	if ctx.ExitCode != 0 {
		msg += fmt.Sprintf(" (exit code: %d)", ctx.ExitCode)
	}
	return msg
}

var suggestedFixes = map[string]string{
	"OOMKilled":                  "Container ran out of memory. Increase resources.limits.memory.",
	"ImagePullBackOff":           "Failed to pull image. Check image name/tag and registry credentials.",
	"CrashLoopBackOff":           "Container keeps crashing. Check application logs for startup errors.",
	"CreateContainerConfigError": "Config error. Check Secret/ConfigMap references exist.",
	"DeadlineExceeded":           "Job exceeded activeDeadlineSeconds. Job may be too slow or deadline too aggressive.",
	"BackoffLimitExceeded":       "Job failed too many times. Check underlying failure cause.",
	"Evicted":                    "Pod was evicted. Check node resources and pod priority.",
	"FailedScheduling":           "Could not schedule pod. Check node resources and affinity rules.",
}

func (h *JobHandler) getSuggestedFix(_ *batchv1.Job, ctx alerting.AlertContext) string {
	// Check reason from container status
	if ctx.Reason != "" {
		if fix, ok := suggestedFixes[ctx.Reason]; ok {
			return fix
		}
	}

	// Check events for common issues
	for _, evt := range ctx.Events {
		for pattern, fix := range suggestedFixes {
			if strings.Contains(evt, pattern) {
				return fix
			}
		}
	}

	return "Check job logs and events for details."
}

func (h *JobHandler) isOwnedByCronJob(obj client.Object) bool {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return false
	}
	for _, ref := range job.OwnerReferences {
		if ref.Kind == "CronJob" {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the job handler with the Manager.
func (h *JobHandler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return h.isOwnedByCronJob(e.Object)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return h.isOwnedByCronJob(e.ObjectNew)
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false // Don't process deletes
			},
		}).
		Named("jobhandler").
		Complete(h)
}
