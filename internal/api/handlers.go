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

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// Status constants
const (
	statusSuccess = "success"
	statusFailed  = "failed"
)

// Handlers contains all API handlers
type Handlers struct {
	client              client.Client
	clientset           *kubernetes.Clientset
	store               store.Store
	config              *config.Config
	alertDispatcher     alerting.Dispatcher
	startTime           time.Time
	leaderElectionCheck func() bool
}

// NewHandlers creates a new Handlers instance
func NewHandlers(c client.Client, cs *kubernetes.Clientset, s store.Store, cfg *config.Config, ad alerting.Dispatcher, startTime time.Time, leaderCheck func() bool) *Handlers {
	return &Handlers{
		client:              c,
		clientset:           cs,
		store:               s,
		config:              cfg,
		alertDispatcher:     ad,
		startTime:           startTime,
		leaderElectionCheck: leaderCheck,
	}
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// GetHealth handles GET /api/v1/health
func (h *Handlers) GetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storageStatus := "connected"
	if h.store != nil {
		if err := h.store.Health(ctx); err != nil {
			storageStatus = "error: " + err.Error()
		}
	} else {
		storageStatus = "not configured"
	}

	uptime := time.Since(h.startTime)

	// Check leader election status
	isLeader := true // Default to true if leader election is disabled
	if h.leaderElectionCheck != nil {
		isLeader = h.leaderElectionCheck()
	}

	resp := HealthResponse{
		Status:  "healthy",
		Storage: storageStatus,
		Leader:  isLeader,
		Version: Version,
		Uptime:  uptime.Round(time.Second).String(),
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetStats handles GET /api/v1/stats
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all monitors
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.client.List(ctx, monitors); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	summary := SummaryStats{}
	totalCronJobs := int32(0)
	activeAlerts := int32(0)

	for _, m := range monitors.Items {
		if m.Status.Summary != nil {
			summary.Healthy += m.Status.Summary.Healthy
			summary.Warning += m.Status.Summary.Warning
			summary.Critical += m.Status.Summary.Critical
			summary.Suspended += m.Status.Summary.Suspended
			totalCronJobs += m.Status.Summary.TotalCronJobs
			activeAlerts += m.Status.Summary.ActiveAlerts
		}
	}

	alertsSent24h := int32(0)
	if h.alertDispatcher != nil {
		alertsSent24h = h.alertDispatcher.GetAlertCount24h()
	}

	// Count executions from store in last 24h
	executionsRecorded24h := int64(0)
	if h.store != nil {
		since := time.Now().Add(-24 * time.Hour)
		if count, err := h.store.GetExecutionCountSince(ctx, since); err == nil {
			executionsRecorded24h = count
		}
	}

	resp := StatsResponse{
		TotalMonitors:         int32(len(monitors.Items)),
		TotalCronJobs:         totalCronJobs,
		Summary:               summary,
		ActiveAlerts:          activeAlerts,
		AlertsSent24h:         alertsSent24h,
		ExecutionsRecorded24h: executionsRecorded24h,
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListMonitors handles GET /api/v1/monitors
func (h *Handlers) ListMonitors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")

	monitors := &guardianv1alpha1.CronJobMonitorList{}
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := h.client.List(ctx, monitors, opts...); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	items := make([]MonitorListItem, 0, len(monitors.Items))
	for _, m := range monitors.Items {
		item := MonitorListItem{
			Name:      m.Name,
			Namespace: m.Namespace,
			Phase:     m.Status.Phase,
		}

		if m.Status.Summary != nil {
			item.CronJobCount = m.Status.Summary.TotalCronJobs
			item.Summary = SummaryStats{
				Healthy:   m.Status.Summary.Healthy,
				Warning:   m.Status.Summary.Warning,
				Critical:  m.Status.Summary.Critical,
				Suspended: m.Status.Summary.Suspended,
			}
			item.ActiveAlerts = m.Status.Summary.ActiveAlerts
		}

		if m.Status.LastReconcileTime != nil {
			t := m.Status.LastReconcileTime.Time
			item.LastReconcile = &t
		}

		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, MonitorListResponse{Items: items})
}

// GetMonitor handles GET /api/v1/monitors/:namespace/:name
func (h *Handlers) GetMonitor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	monitor := &guardianv1alpha1.CronJobMonitor{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, monitor); err != nil {
		if client.IgnoreNotFound(err) == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Monitor %s/%s not found", namespace, name))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Return the full monitor
	writeJSON(w, http.StatusOK, monitor)
}

// ListCronJobs handles GET /api/v1/cronjobs
func (h *Handlers) ListCronJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	// Get all monitors to find monitored CronJobs
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := h.client.List(ctx, monitors, opts...); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Use a map to deduplicate CronJobs (same CronJob may appear in multiple monitors)
	// Key: "namespace/name"
	seen := make(map[string]struct{})
	items := make([]CronJobListItem, 0)
	summary := SummaryStats{}

	for _, m := range monitors.Items {
		for _, cjStatus := range m.Status.CronJobs {
			// Create unique key for deduplication
			key := cjStatus.Namespace + "/" + cjStatus.Name
			if _, exists := seen[key]; exists {
				// Already processed this CronJob from another monitor, skip
				continue
			}
			seen[key] = struct{}{}

			// Apply filters
			if statusFilter != "" && cjStatus.Status != statusFilter {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(cjStatus.Name), strings.ToLower(search)) {
				continue
			}

			// Get the actual CronJob for schedule info
			cj := &batchv1.CronJob{}
			err := h.client.Get(ctx, types.NamespacedName{Namespace: cjStatus.Namespace, Name: cjStatus.Name}, cj)

			item := CronJobListItem{
				Name:         cjStatus.Name,
				Namespace:    cjStatus.Namespace,
				Status:       cjStatus.Status,
				Suspended:    cjStatus.Suspended,
				ActiveAlerts: len(cjStatus.ActiveAlerts),
				MonitorRef:   &NamespacedRef{Namespace: m.Namespace, Name: m.Name},
			}

			if err == nil {
				item.Schedule = cj.Spec.Schedule
				if cj.Spec.TimeZone != nil {
					item.Timezone = *cj.Spec.TimeZone
				}
			}

			if cjStatus.Metrics != nil {
				item.SuccessRate = cjStatus.Metrics.SuccessRate
			}

			if cjStatus.LastSuccessfulTime != nil {
				t := cjStatus.LastSuccessfulTime.Time
				item.LastSuccess = &t
			}

			if cjStatus.LastRunDuration != nil {
				item.LastRunDuration = cjStatus.LastRunDuration.Duration.String()
			}

			if cjStatus.NextScheduledTime != nil {
				t := cjStatus.NextScheduledTime.Time
				item.NextRun = &t
			}

			items = append(items, item)

			// Update summary
			switch cjStatus.Status {
			case "healthy":
				summary.Healthy++
			case "warning":
				summary.Warning++
			case "critical":
				summary.Critical++
			}
			if cjStatus.Suspended {
				summary.Suspended++
			}
		}
	}

	writeJSON(w, http.StatusOK, CronJobListResponse{
		Items:   items,
		Summary: summary,
	})
}

// GetCronJob handles GET /api/v1/cronjobs/:namespace/:name
func (h *Handlers) GetCronJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	// Get the CronJob
	cj := &batchv1.CronJob{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("CronJob %s/%s not found", namespace, name))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := CronJobDetailResponse{
		Name:      cj.Name,
		Namespace: cj.Namespace,
		Schedule:  cj.Spec.Schedule,
		Suspended: cj.Spec.Suspend != nil && *cj.Spec.Suspend,
		Status:    "unknown",
	}

	if cj.Spec.TimeZone != nil {
		resp.Timezone = *cj.Spec.TimeZone
	}

	// Find monitor for this CronJob
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.client.List(ctx, monitors, client.InNamespace(namespace)); err == nil {
		for _, m := range monitors.Items {
			for _, cjStatus := range m.Status.CronJobs {
				if cjStatus.Name == name && cjStatus.Namespace == namespace {
					resp.MonitorRef = &NamespacedRef{Namespace: m.Namespace, Name: m.Name}
					resp.Status = cjStatus.Status

					if cjStatus.Metrics != nil {
						resp.Metrics = &CronJobMetrics{
							SuccessRate7d:      cjStatus.Metrics.SuccessRate,
							TotalRuns7d:        cjStatus.Metrics.TotalRuns,
							SuccessfulRuns7d:   cjStatus.Metrics.SuccessfulRuns,
							FailedRuns7d:       cjStatus.Metrics.FailedRuns,
							AvgDurationSeconds: cjStatus.Metrics.AvgDurationSeconds,
							P50DurationSeconds: cjStatus.Metrics.P50DurationSeconds,
							P95DurationSeconds: cjStatus.Metrics.P95DurationSeconds,
							P99DurationSeconds: cjStatus.Metrics.P99DurationSeconds,
						}
					}

					if cjStatus.NextScheduledTime != nil {
						t := cjStatus.NextScheduledTime.Time
						resp.NextRun = &t
					}

					// Convert active alerts
					resp.ActiveAlerts = make([]AlertItem, 0, len(cjStatus.ActiveAlerts))
					for _, a := range cjStatus.ActiveAlerts {
						resp.ActiveAlerts = append(resp.ActiveAlerts, AlertItem{
							Type:     a.Type,
							Severity: a.Severity,
							Message:  a.Message,
							Since:    a.Since.Time,
						})
					}

					break
				}
			}
		}
	}

	// Get last execution from store
	if h.store != nil {
		cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
		if lastExec, err := h.store.GetLastExecution(ctx, cronJobNN); err == nil && lastExec != nil {
			status := statusFailed
			if lastExec.Succeeded {
				status = statusSuccess
			}
			resp.LastExecution = &ExecutionSummary{
				JobName:   lastExec.JobName,
				Status:    status,
				StartTime: lastExec.StartTime,
				Duration:  lastExec.Duration.String(),
				ExitCode:  lastExec.ExitCode,
			}
			if !lastExec.CompletionTime.IsZero() {
				resp.LastExecution.CompletionTime = &lastExec.CompletionTime
			}
		}
	}

	// Get 30-day metrics
	if h.store != nil && resp.Metrics != nil {
		cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
		if metrics30d, err := h.store.GetMetrics(ctx, cronJobNN, 30); err == nil && metrics30d != nil {
			resp.Metrics.SuccessRate30d = metrics30d.SuccessRate
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetExecutions handles GET /api/v1/cronjobs/:namespace/:name/executions
func (h *Handlers) GetExecutions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	since := time.Time{}
	if s := r.URL.Query().Get("since"); s != "" {
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			since = parsed
		}
	}

	if h.store == nil {
		writeJSON(w, http.StatusOK, ExecutionListResponse{
			Items: []ExecutionItem{},
			Pagination: Pagination{
				Total:   0,
				Limit:   limit,
				Offset:  offset,
				HasMore: false,
			},
		})
		return
	}

	// Default to last 30 days if no since
	if since.IsZero() {
		since = time.Now().AddDate(0, 0, -30)
	}

	cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
	executions, err := h.store.GetExecutions(ctx, cronJobNN, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Apply status filter
	statusFilter := r.URL.Query().Get("status")
	filtered := make([]store.Execution, 0, len(executions))
	for _, e := range executions {
		if statusFilter != "" {
			if statusFilter == statusSuccess && !e.Succeeded {
				continue
			}
			if statusFilter == statusFailed && e.Succeeded {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	// Apply pagination
	total := int64(len(filtered))
	start := offset
	end := offset + limit
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	paged := filtered[start:end]

	items := make([]ExecutionItem, 0, len(paged))
	for _, e := range paged {
		status := statusFailed
		if e.Succeeded {
			status = statusSuccess
		}
		item := ExecutionItem{
			ID:        e.ID,
			JobName:   e.JobName,
			Status:    status,
			StartTime: e.StartTime,
			Duration:  e.Duration.String(),
			ExitCode:  e.ExitCode,
			Reason:    e.Reason,
			IsRetry:   e.IsRetry,
		}
		if !e.CompletionTime.IsZero() {
			item.CompletionTime = &e.CompletionTime
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, ExecutionListResponse{
		Items: items,
		Pagination: Pagination{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: end < len(filtered),
		},
	})
}

// GetLogs handles GET /api/v1/cronjobs/:namespace/:name/executions/:jobName/logs
func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	jobName := chi.URLParam(r, "jobName")
	container := r.URL.Query().Get("container")

	tailLines := int64(500)
	if t := r.URL.Query().Get("tailLines"); t != "" {
		if parsed, err := strconv.ParseInt(t, 10, 64); err == nil && parsed > 0 {
			tailLines = parsed
		}
	}

	if h.clientset == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Kubernetes clientset not available")
		return
	}

	// Find pod for job
	pods := &corev1.PodList{}
	if err := h.client.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels{"job-name": jobName}); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if len(pods.Items) == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("No pods found for job %s", jobName))
		return
	}

	pod := &pods.Items[0]

	// Determine container
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}

	// Get logs
	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: ptr.To(tailLines),
	}

	req := h.clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to get logs: %v", err))
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, stream)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to read logs: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, LogsResponse{
		JobName:   jobName,
		Container: container,
		Logs:      buf.String(),
		Truncated: false,
	})
}

// TriggerCronJob handles POST /api/v1/cronjobs/:namespace/:name/trigger
func (h *Handlers) TriggerCronJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	// Get the CronJob
	cj := &batchv1.CronJob{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("CronJob %s/%s not found", namespace, name))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Create a manual job
	jobName := fmt.Sprintf("%s-manual-%d", name, time.Now().Unix())
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"guardian.illenium.net/manual": "true",
				"guardian.illenium.net/parent": name,
			},
		},
		Spec: *cj.Spec.JobTemplate.Spec.DeepCopy(),
	}

	if err := h.client.Create(ctx, job); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to create job: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, TriggerResponse{
		Success: true,
		JobName: jobName,
		Message: "Job created successfully",
	})
}

// SuspendCronJob handles POST /api/v1/cronjobs/:namespace/:name/suspend
func (h *Handlers) SuspendCronJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	cj := &batchv1.CronJob{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("CronJob %s/%s not found", namespace, name))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	cj.Spec.Suspend = ptr.To(true)
	if err := h.client.Update(ctx, cj); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to suspend: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, SimpleResponse{
		Success: true,
		Message: "CronJob suspended",
	})
}

// ResumeCronJob handles POST /api/v1/cronjobs/:namespace/:name/resume
func (h *Handlers) ResumeCronJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	cj := &batchv1.CronJob{}
	if err := h.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("CronJob %s/%s not found", namespace, name))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	cj.Spec.Suspend = ptr.To(false)
	if err := h.client.Update(ctx, cj); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("Failed to resume: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, SimpleResponse{
		Success: true,
		Message: "CronJob resumed",
	})
}

// ListAlerts handles GET /api/v1/alerts
func (h *Handlers) ListAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	severityFilter := r.URL.Query().Get("severity")
	typeFilter := r.URL.Query().Get("type")
	namespaceFilter := r.URL.Query().Get("namespace")
	cronjobFilter := r.URL.Query().Get("cronjob")

	// Get all monitors and collect active alerts
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.client.List(ctx, monitors); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	items := make([]AlertItem, 0)
	bySeverity := map[string]int{
		"critical": 0,
		"warning":  0,
		"info":     0,
	}

	for _, m := range monitors.Items {
		for _, cjStatus := range m.Status.CronJobs {
			// Apply filters
			if namespaceFilter != "" && cjStatus.Namespace != namespaceFilter {
				continue
			}
			if cronjobFilter != "" && cjStatus.Name != cronjobFilter {
				continue
			}

			for _, a := range cjStatus.ActiveAlerts {
				if severityFilter != "" && a.Severity != severityFilter {
					continue
				}
				if typeFilter != "" && a.Type != typeFilter {
					continue
				}

				item := AlertItem{
					ID:       fmt.Sprintf("%s-%s-%s", cjStatus.Namespace, cjStatus.Name, a.Type),
					Type:     a.Type,
					Severity: a.Severity,
					Title:    fmt.Sprintf("%s: %s/%s", a.Type, cjStatus.Namespace, cjStatus.Name),
					Message:  a.Message,
					CronJob:  &NamespacedRef{Namespace: cjStatus.Namespace, Name: cjStatus.Name},
					Monitor:  &NamespacedRef{Namespace: m.Namespace, Name: m.Name},
					Since:    a.Since.Time,
				}
				if a.LastNotified != nil {
					t := a.LastNotified.Time
					item.LastNotified = &t
				}

				items = append(items, item)
				bySeverity[a.Severity]++
			}
		}
	}

	writeJSON(w, http.StatusOK, AlertListResponse{
		Items:      items,
		Total:      len(items),
		BySeverity: bySeverity,
	})
}

// GetAlertHistory handles GET /api/v1/alerts/history
func (h *Handlers) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.store == nil {
		writeJSON(w, http.StatusOK, AlertHistoryResponse{
			Items: []AlertHistoryItem{},
			Pagination: Pagination{
				Total:   0,
				Limit:   50,
				Offset:  0,
				HasMore: false,
			},
		})
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	query := store.AlertHistoryQuery{
		Limit:    limit,
		Offset:   offset,
		Severity: r.URL.Query().Get("severity"),
	}

	if s := r.URL.Query().Get("since"); s != "" {
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			query.Since = &parsed
		}
	}

	alerts, total, err := h.store.ListAlertHistory(ctx, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	items := make([]AlertHistoryItem, 0, len(alerts))
	for _, a := range alerts {
		item := AlertHistoryItem{
			ID:               strconv.FormatInt(a.ID, 10),
			Type:             a.Type,
			Severity:         a.Severity,
			Title:            a.Title,
			Message:          a.Message,
			OccurredAt:       a.OccurredAt,
			ResolvedAt:       a.ResolvedAt,
			ChannelsNotified: a.ChannelsNotified,
		}
		if a.CronJobNamespace != "" || a.CronJobName != "" {
			item.CronJob = &NamespacedRef{
				Namespace: a.CronJobNamespace,
				Name:      a.CronJobName,
			}
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, AlertHistoryResponse{
		Items: items,
		Pagination: Pagination{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: int64(offset+limit) < total,
		},
	})
}

// ListChannels handles GET /api/v1/channels
func (h *Handlers) ListChannels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	channels := &guardianv1alpha1.AlertChannelList{}
	if err := h.client.List(ctx, channels); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Fetch channel alert stats from store
	var channelStats map[string]store.ChannelAlertStats
	if h.store != nil {
		var err error
		channelStats, err = h.store.GetChannelAlertStats(ctx)
		if err != nil {
			// Non-fatal, just log and continue with empty stats
			channelStats = make(map[string]store.ChannelAlertStats)
		}
	}

	items := make([]ChannelListItem, 0, len(channels.Items))
	ready := 0
	notReady := 0

	for _, ch := range channels.Items {
		stats := ChannelStats{}
		if s, ok := channelStats[ch.Name]; ok {
			stats.AlertsSent24h = int32(s.AlertsSent24h)
			stats.AlertsSentTotal = s.AlertsSentTotal
		}

		item := ChannelListItem{
			Name:  ch.Name,
			Type:  ch.Spec.Type,
			Ready: ch.Status.Ready,
			Stats: stats,
		}

		// Build config summary (without sensitive data)
		item.Config = make(map[string]any)
		switch ch.Spec.Type {
		case "slack":
			if ch.Spec.Slack != nil {
				item.Config["channel"] = ch.Spec.Slack.DefaultChannel
			}
		case "email":
			if ch.Spec.Email != nil {
				item.Config["to"] = ch.Spec.Email.To
			}
		}

		if ch.Status.LastTestTime != nil {
			item.LastTest = &TestResult{
				Time:   ch.Status.LastTestTime.Time,
				Result: ch.Status.LastTestResult,
			}
		}

		items = append(items, item)

		if ch.Status.Ready {
			ready++
		} else {
			notReady++
		}
	}

	writeJSON(w, http.StatusOK, ChannelListResponse{
		Items: items,
		Summary: ChannelSummary{
			Total:    len(channels.Items),
			Ready:    ready,
			NotReady: notReady,
		},
	})
}

// GetChannel handles GET /api/v1/channels/:name
func (h *Handlers) GetChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")

	channel := &guardianv1alpha1.AlertChannel{}
	if err := h.client.Get(ctx, types.NamespacedName{Name: name}, channel); err != nil {
		if client.IgnoreNotFound(err) == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Channel %s not found", name))
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Return the full channel (secrets redacted by Kubernetes)
	writeJSON(w, http.StatusOK, channel)
}

// TestChannel handles POST /api/v1/channels/:name/test
func (h *Handlers) TestChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")

	if h.alertDispatcher == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Alert dispatcher not available")
		return
	}

	testAlert := alerting.Alert{
		Key:       "api-test-" + name,
		Type:      "Test",
		Severity:  "info",
		Title:     "CronJob Guardian Test Alert",
		Message:   "This is a test alert from the API.",
		CronJob:   types.NamespacedName{Namespace: "test", Name: "test-cronjob"},
		Timestamp: time.Now(),
	}

	if err := h.alertDispatcher.SendToChannel(ctx, name, testAlert); err != nil {
		writeJSON(w, http.StatusOK, SimpleResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, SimpleResponse{
		Success: true,
		Message: "Test alert sent successfully",
	})
}

// ConfigResponse represents the operator configuration for the API
type ConfigResponse struct {
	LogLevel          string                        `json:"logLevel"`
	Storage           config.StorageConfig          `json:"storage"`
	HistoryRetention  config.HistoryRetentionConfig `json:"historyRetention"`
	RateLimits        config.RateLimitsConfig       `json:"rateLimits"`
	IgnoredNamespaces []string                      `json:"ignoredNamespaces"`
	API               config.APIConfig              `json:"api"`
	Scheduler         config.SchedulerConfig        `json:"scheduler"`
}

// GetConfig handles GET /api/v1/config
func (h *Handlers) GetConfig(w http.ResponseWriter, _ *http.Request) {
	if h.config == nil {
		writeJSON(w, http.StatusOK, ConfigResponse{})
		return
	}

	resp := ConfigResponse{
		LogLevel:          h.config.LogLevel,
		Storage:           h.config.Storage,
		HistoryRetention:  h.config.HistoryRetention,
		RateLimits:        h.config.RateLimits,
		IgnoredNamespaces: h.config.IgnoredNamespaces,
		API:               h.config.API,
		Scheduler:         h.config.Scheduler,
	}

	writeJSON(w, http.StatusOK, resp)
}

// DeleteCronJobHistory handles DELETE /api/v1/cronjobs/:namespace/:name/history
func (h *Handlers) DeleteCronJobHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Store not configured")
		return
	}

	cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
	deleted, err := h.store.DeleteExecutionsByCronJob(ctx, cronJobNN)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, DeleteHistoryResponse{
		Success:        true,
		RecordsDeleted: deleted,
		Message:        fmt.Sprintf("Deleted %d execution records for %s/%s", deleted, namespace, name),
	})
}

// GetStorageStats handles GET /api/v1/admin/storage-stats
func (h *Handlers) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Store not configured")
		return
	}

	count, err := h.store.GetExecutionCount(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	storageType := "unknown"
	if h.config != nil {
		storageType = h.config.Storage.Type
	}

	healthy := h.store.Health(ctx) == nil

	writeJSON(w, http.StatusOK, StorageStatsResponse{
		ExecutionCount:    count,
		StorageType:       storageType,
		Healthy:           healthy,
		RetentionDays:     h.config.HistoryRetention.DefaultDays,
		LogStorageEnabled: h.config.Storage.LogStorageEnabled,
	})
}

// PruneRequest represents a prune request body
type PruneRequest struct {
	OlderThanDays int  `json:"olderThanDays"`
	DryRun        bool `json:"dryRun"`
	PruneLogsOnly bool `json:"pruneLogsOnly"`
}

// TriggerPrune handles POST /api/v1/admin/prune
func (h *Handlers) TriggerPrune(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Store not configured")
		return
	}

	// Parse request body
	var req PruneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to retention config
		if h.config != nil {
			req.OlderThanDays = h.config.HistoryRetention.DefaultDays
		} else {
			req.OlderThanDays = 30
		}
	}

	if req.OlderThanDays <= 0 {
		req.OlderThanDays = 30
	}

	cutoff := time.Now().AddDate(0, 0, -req.OlderThanDays)

	if req.DryRun {
		writeJSON(w, http.StatusOK, PruneResponse{
			Success:       true,
			RecordsPruned: 0,
			DryRun:        true,
			Cutoff:        cutoff,
			OlderThanDays: req.OlderThanDays,
			Message:       "Dry run - no records deleted",
		})
		return
	}

	var count int64
	var err error
	if req.PruneLogsOnly {
		count, err = h.store.PruneLogs(ctx, cutoff)
	} else {
		count, err = h.store.Prune(ctx, cutoff)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	message := fmt.Sprintf("Pruned %d execution records older than %d days", count, req.OlderThanDays)
	if req.PruneLogsOnly {
		message = fmt.Sprintf("Pruned logs from %d execution records older than %d days", count, req.OlderThanDays)
	}

	writeJSON(w, http.StatusOK, PruneResponse{
		Success:       true,
		RecordsPruned: count,
		DryRun:        false,
		Cutoff:        cutoff,
		OlderThanDays: req.OlderThanDays,
		Message:       message,
	})
}

// GetExecutionWithLogs handles GET /api/v1/cronjobs/:namespace/:name/executions/:id
// Returns execution details including stored logs if available
func (h *Handlers) GetExecutionWithLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	jobName := chi.URLParam(r, "jobName")

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Store not configured")
		return
	}

	// Get executions and find the matching one
	cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
	since := time.Now().AddDate(0, 0, -90) // Look back 90 days
	executions, err := h.store.GetExecutions(ctx, cronJobNN, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Find the execution by job name
	for _, e := range executions {
		if e.JobName == jobName {
			status := statusFailed
			if e.Succeeded {
				status = statusSuccess
			}

			resp := ExecutionDetailResponse{
				ID:               e.ID,
				CronJobNamespace: e.CronJobNamespace,
				CronJobName:      e.CronJobName,
				CronJobUID:       e.CronJobUID,
				JobName:          e.JobName,
				Status:           status,
				StartTime:        e.StartTime,
				Duration:         e.Duration.String(),
				ExitCode:         e.ExitCode,
				Reason:           e.Reason,
				IsRetry:          e.IsRetry,
				RetryOf:          e.RetryOf,
				StoredLogs:       e.Logs,
				StoredEvents:     e.Events,
			}
			if !e.CompletionTime.IsZero() {
				resp.CompletionTime = &e.CompletionTime
			}

			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	writeError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Execution %s not found", jobName))
}
