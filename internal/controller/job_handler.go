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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/remediation"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// Data retention action constants
const (
	retentionRetain = "retain"
	retentionReset  = "reset"
)

// JobHandler handles Job events for execution tracking
type JobHandler struct {
	client.Client
	Log               logr.Logger // Required - must be injected
	Scheme            *runtime.Scheme
	Clientset         *kubernetes.Clientset
	Store             store.Store
	Config            *config.Config
	AlertDispatcher   alerting.Dispatcher
	RemediationEngine remediation.Engine
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

// Reconcile handles Job completion/failure events
func (h *JobHandler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := h.Log.WithValues("job", req.NamespacedName)
	log.V(1).Info("reconciling job")

	job := &batchv1.Job{}
	if err := h.Get(ctx, req.NamespacedName, job); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.V(1).Info("job not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get job")
		return ctrl.Result{}, err
	}

	completionTime := "nil"
	if job.Status.CompletionTime != nil {
		completionTime = job.Status.CompletionTime.Format(time.RFC3339)
	}
	log.V(1).Info("job fetched",
		"succeeded", job.Status.Succeeded,
		"failed", job.Status.Failed,
		"active", job.Status.Active,
		"completionTime", completionTime)

	// Get parent CronJob
	cronJobName := h.getCronJobOwner(job)
	if cronJobName == "" {
		log.V(1).Info("job not owned by a CronJob, skipping")
		return ctrl.Result{}, nil
	}
	log = log.WithValues("cronJob", cronJobName)

	// Check job status - skip if still running
	if job.Status.CompletionTime == nil && job.Status.Failed == 0 {
		log.V(1).Info("job still running, nothing to record yet")
		return ctrl.Result{}, nil
	}

	// Find ALL monitors whose selector matches this CronJob (real-time evaluation)
	monitors := h.findMonitorsForCronJob(ctx, job.Namespace, cronJobName)
	if len(monitors) == 0 {
		log.V(1).Info("no monitors found for CronJob, skipping")
		return ctrl.Result{}, nil
	}
	log.V(1).Info("found matching monitors", "count", len(monitors))

	// Get the parent CronJob to extract its UID
	cronJob := &batchv1.CronJob{}
	cronJobNN := types.NamespacedName{Namespace: job.Namespace, Name: cronJobName}
	cronJobUID := ""
	if err := h.Get(ctx, cronJobNN, cronJob); err == nil {
		cronJobUID = string(cronJob.UID)
		log.V(1).Info("got CronJob UID", "uid", cronJobUID)
	} else {
		log.V(1).Info("could not get CronJob (may be deleted)", "error", err)
	}

	// Check for CronJob recreation (UID change) - use first monitor for config
	if h.Store != nil && cronJobUID != "" {
		h.handleRecreationCheck(ctx, log, monitors[0], cronJobNN, cronJobUID)
	}

	// Record execution ONCE (keyed by CronJob, not monitor)
	// Use first monitor for config (logs/events storage settings)
	exec := h.buildExecution(ctx, job, cronJobName, cronJobUID, monitors[0])
	log.V(1).Info("built execution record",
		"succeeded", exec.Succeeded,
		"duration", exec.Duration,
		"exitCode", exec.ExitCode,
		"reason", exec.Reason,
		"cronJobUID", exec.CronJobUID,
		"hasLogs", exec.Logs != "",
		"hasEvents", exec.Events != "")

	if h.Store != nil {
		log.V(1).Info("recording execution to store")
		if err := h.Store.RecordExecution(ctx, exec); err != nil {
			log.Error(err, "failed to record execution")
		} else {
			log.Info("execution recorded",
				"cronJob", cronJobName,
				"job", job.Name,
				"succeeded", exec.Succeeded,
				"duration", exec.Duration.Round(time.Millisecond))
		}
	} else {
		log.V(1).Info("store not configured, skipping execution recording")
	}

	// Handle completion for ALL matching monitors
	if job.Status.Succeeded > 0 {
		log.Info("job succeeded", "cronJob", cronJobName, "job", job.Name)
		for _, monitor := range monitors {
			monitorLog := log.WithValues("monitor", monitor.Name)
			h.handleSuccess(ctx, monitorLog, monitor, job, cronJobName)
		}
	} else if job.Status.Failed > 0 {
		log.Info("job failed", "cronJob", cronJobName, "job", job.Name, "exitCode", exec.ExitCode, "reason", exec.Reason)
		for _, monitor := range monitors {
			monitorLog := log.WithValues("monitor", monitor.Name)
			h.handleFailure(ctx, monitorLog, monitor, job, cronJobName)
		}
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

// findMonitorsForCronJob finds ALL monitors whose selector matches the given CronJob.
// This uses real-time selector evaluation (not cached status) to avoid race conditions.
func (h *JobHandler) findMonitorsForCronJob(ctx context.Context, namespace, cronJobName string) []*guardianv1alpha1.CronJobMonitor {
	log := h.Log.V(1)

	// Get the CronJob to check its labels
	cronJob := &batchv1.CronJob{}
	if err := h.Get(ctx, types.NamespacedName{Namespace: namespace, Name: cronJobName}, cronJob); err != nil {
		log.Error(err, "failed to get CronJob", "cronJob", cronJobName)
		return nil
	}

	// List all monitors in namespace
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.List(ctx, monitors, client.InNamespace(namespace)); err != nil {
		log.Error(err, "failed to list monitors", "namespace", namespace)
		return nil
	}

	log.Info("searching for monitors", "namespace", namespace, "cronJob", cronJobName, "monitorCount", len(monitors.Items))

	// Find ALL monitors whose selector matches this CronJob
	var matching []*guardianv1alpha1.CronJobMonitor
	for i := range monitors.Items {
		monitor := &monitors.Items[i]
		if MatchesSelector(cronJob, monitor.Spec.Selector) {
			log.Info("found matching monitor", "monitor", monitor.Name)
			matching = append(matching, monitor)
		}
	}

	return matching
}

func (h *JobHandler) buildExecution(ctx context.Context, job *batchv1.Job, cronJobName, cronJobUID string, monitor *guardianv1alpha1.CronJobMonitor) store.Execution {
	exec := store.Execution{
		CronJobNamespace: job.Namespace,
		CronJobName:      cronJobName,
		CronJobUID:       cronJobUID,
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
	pod := h.getJobPod(ctx, job)
	if pod != nil {
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

	// Store logs if configured
	if h.shouldStoreLogs(monitor) {
		maxSizeKB := h.getMaxLogSizeKB(monitor)
		exec.Logs = h.collectAndTruncateLogs(ctx, pod, maxSizeKB)
	}

	// Store events if configured
	if h.shouldStoreEvents(monitor) {
		events := h.collectEvents(ctx, job)
		if len(events) > 0 {
			eventsJSON, _ := json.Marshal(events)
			exec.Events = string(eventsJSON)
		}
	}

	return exec
}

func (h *JobHandler) getJobPod(ctx context.Context, job *batchv1.Job) *corev1.Pod {
	pods := &corev1.PodList{}
	if err := h.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{"job-name": job.Name}); err != nil {
		h.Log.V(1).Error(err, "failed to list pods for job", "job", job.Name)
		return nil
	}

	if len(pods.Items) > 0 {
		h.Log.V(1).Info("found pod for job", "job", job.Name, "pod", pods.Items[0].Name)
		return &pods.Items[0]
	}
	h.Log.V(1).Info("no pod found for job", "job", job.Name)
	return nil
}

// handleRecreationCheck checks if a CronJob was recreated (UID changed) and handles per config
func (h *JobHandler) handleRecreationCheck(ctx context.Context, log logr.Logger, monitor *guardianv1alpha1.CronJobMonitor, cronJob types.NamespacedName, currentUID string) {
	// Get existing UIDs for this CronJob
	uids, err := h.Store.GetCronJobUIDs(ctx, cronJob)
	if err != nil {
		log.V(1).Error(err, "failed to get CronJob UIDs from store")
		return
	}

	// Check if there are different UIDs (indicating recreation)
	for _, uid := range uids {
		if uid != "" && uid != currentUID {
			log.Info("detected CronJob recreation", "oldUID", uid, "newUID", currentUID)

			// Check onRecreation config
			onRecreation := retentionRetain // default
			if monitor.Spec.DataRetention != nil && monitor.Spec.DataRetention.OnRecreation != "" {
				onRecreation = monitor.Spec.DataRetention.OnRecreation
			}

			if onRecreation == retentionReset {
				// Delete executions from the old UID
				deleted, err := h.Store.DeleteExecutionsByUID(ctx, cronJob, uid)
				if err != nil {
					log.Error(err, "failed to delete old UID executions", "uid", uid)
				} else {
					log.Info("deleted executions from old CronJob UID", "uid", uid, "count", deleted)
				}
			} else {
				log.V(1).Info("retaining old UID executions per config", "uid", uid, "onRecreation", onRecreation)
			}
		}
	}
}

// shouldStoreLogs determines if logs should be stored for this execution
func (h *JobHandler) shouldStoreLogs(monitor *guardianv1alpha1.CronJobMonitor) bool {
	// Check monitor-level config first (takes priority)
	if monitor.Spec.DataRetention != nil && monitor.Spec.DataRetention.StoreLogs != nil {
		return *monitor.Spec.DataRetention.StoreLogs
	}

	// Fall back to global config
	if h.Config != nil {
		return h.Config.Storage.LogStorageEnabled
	}

	return false // Default to not storing logs
}

// shouldStoreEvents determines if events should be stored for this execution
func (h *JobHandler) shouldStoreEvents(monitor *guardianv1alpha1.CronJobMonitor) bool {
	// Check monitor-level config first (takes priority)
	if monitor.Spec.DataRetention != nil && monitor.Spec.DataRetention.StoreEvents != nil {
		return *monitor.Spec.DataRetention.StoreEvents
	}

	// Fall back to global config
	if h.Config != nil {
		return h.Config.Storage.EventStorageEnabled
	}

	return false // Default to not storing events
}

// getMaxLogSizeKB returns the max log size in KB for this monitor
func (h *JobHandler) getMaxLogSizeKB(monitor *guardianv1alpha1.CronJobMonitor) int {
	// Check monitor-level config first
	if monitor.Spec.DataRetention != nil && monitor.Spec.DataRetention.MaxLogSizeKB != nil {
		return int(*monitor.Spec.DataRetention.MaxLogSizeKB)
	}

	// Fall back to global config
	if h.Config != nil && h.Config.Storage.MaxLogSizeKB > 0 {
		return h.Config.Storage.MaxLogSizeKB
	}

	return 100 // Default 100KB
}

// collectAndTruncateLogs collects logs and truncates to max size
func (h *JobHandler) collectAndTruncateLogs(ctx context.Context, pod *corev1.Pod, maxSizeKB int) string {
	if h.Clientset == nil || pod == nil {
		return ""
	}

	containerName := ""
	if len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	opts := &corev1.PodLogOptions{
		Container: containerName,
	}

	req := h.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		h.Log.V(1).Error(err, "failed to stream pod logs for storage", "pod", pod.Name)
		return ""
	}
	defer func() {
		_ = stream.Close()
	}()

	// Read with size limit
	maxBytes := maxSizeKB * 1024
	buf := make([]byte, maxBytes+1)
	n, err := io.ReadFull(stream, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		h.Log.V(1).Error(err, "failed to read pod logs for storage", "pod", pod.Name)
		return ""
	}

	logs := string(buf[:n])
	if n > maxBytes {
		// Truncate and add indicator
		logs = logs[:maxBytes-50] + "\n... [truncated to " + fmt.Sprintf("%d", maxSizeKB) + "KB]"
	}

	return logs
}

func (h *JobHandler) handleSuccess(ctx context.Context, log logr.Logger, _ *guardianv1alpha1.CronJobMonitor, job *batchv1.Job, cronJobName string) {
	// Reset retry counter on success
	if h.RemediationEngine != nil {
		log.V(1).Info("resetting retry counter")
		h.RemediationEngine.ResetRetryCount(job.Namespace, cronJobName)
	}

	// Clear any active failure alerts for this CronJob
	if h.AlertDispatcher != nil {
		alertKey := fmt.Sprintf("%s/%s/JobFailed", job.Namespace, cronJobName)
		log.V(1).Info("clearing failure alert", "alertKey", alertKey)
		_ = h.AlertDispatcher.ClearAlert(ctx, alertKey)
	}
}

func (h *JobHandler) handleFailure(ctx context.Context, log logr.Logger, monitor *guardianv1alpha1.CronJobMonitor, job *batchv1.Job, cronJobName string) {
	// Collect context
	alertCtx := alerting.AlertContext{}

	// Safe access to alerting config
	var includeCtx *guardianv1alpha1.AlertContext
	if monitor.Spec.Alerting != nil {
		includeCtx = monitor.Spec.Alerting.IncludeContext
	}

	if includeCtx != nil && isEnabled(includeCtx.Logs) {
		log.V(1).Info("collecting logs for alert")
		alertCtx.Logs = h.collectLogs(ctx, job, includeCtx)
		log.V(1).Info("collected logs", "logLength", len(alertCtx.Logs))
	}

	if includeCtx != nil && isEnabled(includeCtx.Events) {
		log.V(1).Info("collecting events for alert")
		alertCtx.Events = h.collectEvents(ctx, job)
		log.V(1).Info("collected events", "eventCount", len(alertCtx.Events))
	}

	// Get exit code and reason
	if pod := h.getJobPod(ctx, job); pod != nil {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				alertCtx.ExitCode = cs.State.Terminated.ExitCode
				alertCtx.Reason = cs.State.Terminated.Reason
				log.V(1).Info("got termination info from pod",
					"exitCode", alertCtx.ExitCode,
					"reason", alertCtx.Reason)
				break
			}
		}
	}

	if includeCtx != nil && isEnabled(includeCtx.SuggestedFixes) {
		alertCtx.SuggestedFix = h.getSuggestedFix(job, alertCtx)
		log.V(1).Info("generated suggested fix", "fix", alertCtx.SuggestedFix)
	}

	// Determine severity (with nil safety)
	severity := statusCritical
	if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
		severity = getSeverity(monitor.Spec.Alerting.SeverityOverrides.JobFailed, statusCritical)
	}
	log.V(1).Info("determined alert severity", "severity", severity)

	// Create alert
	alert := alerting.Alert{
		Key:      fmt.Sprintf("%s/%s/JobFailed", job.Namespace, cronJobName),
		Type:     "JobFailed",
		Severity: severity,
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
		log.Info("dispatching failure alert",
			"alertKey", alert.Key,
			"severity", alert.Severity,
			"exitCode", alertCtx.ExitCode,
			"reason", alertCtx.Reason)
		if err := h.AlertDispatcher.Dispatch(ctx, alert, monitor.Spec.Alerting); err != nil {
			log.Error(err, "failed to dispatch alert")
		} else {
			log.V(1).Info("alert dispatched successfully")
		}
	} else {
		log.V(1).Info("alert dispatcher not configured, skipping alert dispatch")
	}

	// Check for auto-retry
	if monitor.Spec.Remediation != nil && isEnabled(monitor.Spec.Remediation.Enabled) {
		log.V(1).Info("remediation enabled, checking auto-retry")
		if monitor.Spec.Remediation.AutoRetry != nil && monitor.Spec.Remediation.AutoRetry.Enabled {
			if h.RemediationEngine != nil {
				log.Info("attempting auto-retry", "job", job.Name)
				result, err := h.RemediationEngine.TryRetry(ctx, monitor, job, cronJobName)
				if err != nil {
					log.Error(err, "failed to retry job")
				} else if result.Success {
					log.Info("retry initiated",
						"retryJob", result.JobName,
						"message", result.Message,
						"dryRun", result.DryRun)
				} else {
					log.Info("retry not initiated", "message", result.Message)
				}
			} else {
				log.V(1).Info("remediation engine not configured, skipping auto-retry")
			}
		} else {
			log.V(1).Info("auto-retry not enabled in monitor spec")
		}
	} else {
		log.V(1).Info("remediation not enabled for this monitor")
	}
}

func (h *JobHandler) collectLogs(ctx context.Context, job *batchv1.Job, alertCtx *guardianv1alpha1.AlertContext) string {
	if h.Clientset == nil {
		h.Log.V(1).Info("clientset not configured, cannot collect logs")
		return ""
	}

	pod := h.getJobPod(ctx, job)
	if pod == nil {
		return ""
	}

	containerName := ""
	if alertCtx != nil {
		containerName = alertCtx.LogContainerName
	}
	if containerName == "" && len(pod.Spec.Containers) > 0 {
		containerName = pod.Spec.Containers[0].Name
	}

	logLines := int32(50)
	if alertCtx != nil && alertCtx.LogLines != nil {
		logLines = *alertCtx.LogLines
	}
	tailLines := int64(logLines)
	opts := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: ptr.To(tailLines),
	}

	h.Log.V(1).Info("fetching pod logs",
		"pod", pod.Name,
		"container", containerName,
		"tailLines", tailLines)

	req := h.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		h.Log.V(1).Error(err, "failed to stream pod logs", "pod", pod.Name)
		return ""
	}
	defer func() {
		_ = stream.Close()
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, stream)
	if err != nil {
		h.Log.V(1).Error(err, "failed to read pod logs", "pod", pod.Name)
		return ""
	}
	return buf.String()
}

func (h *JobHandler) collectEvents(ctx context.Context, job *batchv1.Job) []string {
	events := &corev1.EventList{}
	if err := h.List(ctx, events, client.InNamespace(job.Namespace)); err != nil {
		h.Log.V(1).Error(err, "failed to list events", "namespace", job.Namespace)
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
	h.Log.V(1).Info("collected events for job", "job", job.Name, "eventCount", len(result))
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

// isJobComplete checks if a job has completed (succeeded or failed)
func isJobComplete(job *batchv1.Job) bool {
	return job.Status.CompletionTime != nil || job.Status.Failed > 0
}

// SetupWithManager sets up the job handler with the Manager.
func (h *JobHandler) SetupWithManager(mgr ctrl.Manager) error {
	h.Log.Info("setting up job handler controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				// Only process creates if job is already complete (rare but possible)
				job, ok := e.Object.(*batchv1.Job)
				if !ok || !h.isOwnedByCronJob(e.Object) {
					return false
				}
				complete := isJobComplete(job)
				h.Log.V(1).Info("job create event",
					"job", e.Object.GetName(),
					"namespace", e.Object.GetNamespace(),
					"complete", complete)
				return complete
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Only process if job transitions to complete
				if !h.isOwnedByCronJob(e.ObjectNew) {
					return false
				}
				oldJob, ok1 := e.ObjectOld.(*batchv1.Job)
				newJob, ok2 := e.ObjectNew.(*batchv1.Job)
				if !ok1 || !ok2 {
					return false
				}
				// Only reconcile when job transitions to complete
				wasComplete := isJobComplete(oldJob)
				nowComplete := isJobComplete(newJob)
				shouldProcess := nowComplete && !wasComplete
				h.Log.V(1).Info("job update event",
					"job", e.ObjectNew.GetName(),
					"namespace", e.ObjectNew.GetNamespace(),
					"wasComplete", wasComplete,
					"nowComplete", nowComplete,
					"processing", shouldProcess)
				return shouldProcess
			},
			DeleteFunc: func(_ event.DeleteEvent) bool {
				return false // Don't process deletes
			},
		}).
		Named("jobhandler").
		Complete(h)
}
