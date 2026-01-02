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
	analyzerEnabled     bool
	schedulersRunning   []string
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

// SetAnalyzerEnabled sets whether the SLA analyzer is enabled
func (h *Handlers) SetAnalyzerEnabled(enabled bool) {
	h.analyzerEnabled = enabled
}

// SetSchedulersRunning sets the list of running scheduler names
func (h *Handlers) SetSchedulersRunning(schedulers []string) {
	h.schedulersRunning = schedulers
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(
		w, status, ErrorResponse{
			Error: ErrorDetail{
				Code:    code,
				Message: message,
			},
		},
	)
}

// GetHealth handles GET /api/v1/health
// @Summary      Health check
// @Description  Returns the health status of the Guardian operator
// @Tags         System
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
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

	isLeader := true
	if h.leaderElectionCheck != nil {
		isLeader = h.leaderElectionCheck()
	}

	resp := HealthResponse{
		Status:            "healthy",
		Storage:           storageStatus,
		Leader:            isLeader,
		Version:           Version,
		Uptime:            uptime.Round(time.Second).String(),
		AnalyzerEnabled:   h.analyzerEnabled,
		SchedulersRunning: h.schedulersRunning,
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetStats handles GET /api/v1/stats
// @Summary      Get statistics
// @Description  Returns aggregate statistics for all monitored CronJobs
// @Tags         System
// @Produce      json
// @Success      200  {object}  StatsResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /stats [get]
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
			summary.Running += m.Status.Summary.Running
			totalCronJobs += m.Status.Summary.TotalCronJobs
			activeAlerts += m.Status.Summary.ActiveAlerts
		}
	}

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
		ExecutionsRecorded24h: executionsRecorded24h,
	}

	writeJSON(w, http.StatusOK, resp)
}

// ListMonitors handles GET /api/v1/monitors
// @Summary      List monitors
// @Description  Returns all CronJobMonitor resources
// @Tags         Monitors
// @Produce      json
// @Param        namespace  query     string  false  "Filter by namespace"
// @Success      200  {object}  MonitorListResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /monitors [get]
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
				Running:   m.Status.Summary.Running,
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
// @Summary      Get monitor details
// @Description  Returns detailed information about a specific CronJobMonitor
// @Tags         Monitors
// @Produce      json
// @Param        namespace  path      string  true  "Monitor namespace"
// @Param        name       path      string  true  "Monitor name"
// @Success      200  {object}  object  "Monitor details (CRD format)"
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /monitors/{namespace}/{name} [get]
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

	writeJSON(w, http.StatusOK, monitor)
}

// ListCronJobs handles GET /api/v1/cronjobs
// @Summary      List CronJobs
// @Description  Returns all monitored CronJobs with their status
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  query     string  false  "Filter by namespace"
// @Param        status     query     string  false  "Filter by status (healthy, warning, critical)"
// @Success      200  {object}  CronJobListResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs [get]
func (h *Handlers) ListCronJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := r.URL.Query().Get("namespace")
	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	monitors := &guardianv1alpha1.CronJobMonitorList{}
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := h.client.List(ctx, monitors, opts...); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	seen := make(map[string]struct{})
	items := make([]CronJobListItem, 0)
	summary := SummaryStats{}

	for _, m := range monitors.Items {
		for _, cjStatus := range m.Status.CronJobs {
			key := cjStatus.Namespace + "/" + cjStatus.Name
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			if statusFilter != "" && cjStatus.Status != statusFilter {
				continue
			}
			if search != "" && !strings.Contains(strings.ToLower(cjStatus.Name), strings.ToLower(search)) {
				continue
			}

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

			if len(cjStatus.ActiveJobs) > 0 {
				item.ActiveJobs = make([]ActiveJobItem, 0, len(cjStatus.ActiveJobs))
				for _, aj := range cjStatus.ActiveJobs {
					activeJob := ActiveJobItem{
						Name:      aj.Name,
						StartTime: aj.StartTime.Time,
						PodPhase:  aj.PodPhase,
						PodName:   aj.PodName,
						Ready:     aj.Ready,
					}
					if aj.RunningDuration != nil {
						activeJob.RunningDuration = aj.RunningDuration.Duration.String()
					}
					item.ActiveJobs = append(item.ActiveJobs, activeJob)
				}
			}

			items = append(items, item)

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
			if len(cjStatus.ActiveJobs) > 0 {
				summary.Running++
			}
		}
	}

	writeJSON(
		w, http.StatusOK, CronJobListResponse{
			Items:   items,
			Summary: summary,
		},
	)
}

// GetCronJob handles GET /api/v1/cronjobs/:namespace/:name
// @Summary      Get CronJob details
// @Description  Returns detailed information about a specific CronJob
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Success      200  {object}  CronJobDetailResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name} [get]
func (h *Handlers) GetCronJob(w http.ResponseWriter, r *http.Request) {
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

	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.client.List(ctx, monitors); err == nil {
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

					if len(cjStatus.ActiveJobs) > 0 {
						resp.ActiveJobs = make([]ActiveJobItem, 0, len(cjStatus.ActiveJobs))
						for _, aj := range cjStatus.ActiveJobs {
							activeJob := ActiveJobItem{
								Name:      aj.Name,
								StartTime: aj.StartTime.Time,
								PodPhase:  aj.PodPhase,
								PodName:   aj.PodName,
								Ready:     aj.Ready,
							}
							if aj.RunningDuration != nil {
								activeJob.RunningDuration = aj.RunningDuration.Duration.String()
							}
							resp.ActiveJobs = append(resp.ActiveJobs, activeJob)
						}
					}

					resp.ActiveAlerts = make([]AlertItem, 0, len(cjStatus.ActiveAlerts))
					for _, a := range cjStatus.ActiveAlerts {
						item := AlertItem{
							Type:     a.Type,
							Severity: a.Severity,
							Message:  a.Message,
							Since:    a.Since.Time,
						}
						if a.ExitCode != 0 || a.Reason != "" || a.SuggestedFix != "" {
							item.Context = &AlertContextResponse{
								ExitCode:     a.ExitCode,
								Reason:       a.Reason,
								SuggestedFix: a.SuggestedFix,
							}
						}
						resp.ActiveAlerts = append(resp.ActiveAlerts, item)
					}

					break
				}
			}
		}
	}

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
				Duration:  lastExec.Duration().String(),
				ExitCode:  lastExec.ExitCode,
			}
			if !lastExec.CompletionTime.IsZero() {
				resp.LastExecution.CompletionTime = &lastExec.CompletionTime
			}
		}
	}

	if h.store != nil && resp.Metrics != nil {
		cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
		if metrics30d, err := h.store.GetMetrics(ctx, cronJobNN, 30); err == nil && metrics30d != nil {
			resp.Metrics.SuccessRate30d = metrics30d.SuccessRate
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetExecutions handles GET /api/v1/cronjobs/:namespace/:name/executions
// @Summary      Get execution history
// @Description  Returns paginated execution history for a CronJob
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true   "CronJob namespace"
// @Param        name       path      string  true   "CronJob name"
// @Param        limit      query     int     false  "Page size" default(50)
// @Param        offset     query     int     false  "Page offset" default(0)
// @Param        status     query     string  false  "Filter by status (success, failed)"
// @Param        since      query     string  false  "Filter since timestamp (RFC3339)"
// @Success      200  {object}  ExecutionListResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/executions [get]
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
		writeJSON(
			w, http.StatusOK, ExecutionListResponse{
				Items: []ExecutionItem{},
				Pagination: Pagination{
					Total:   0,
					Limit:   limit,
					Offset:  offset,
					HasMore: false,
				},
			},
		)
		return
	}

	if since.IsZero() {
		since = time.Now().AddDate(0, 0, -30)
	}

	cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
	statusFilter := r.URL.Query().Get("status")

	var paged []store.Execution
	var total int64

	var err error
	paged, total, err = h.store.GetExecutionsFiltered(ctx, cronJobNN, since, statusFilter, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

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
			Duration:  e.Duration().String(),
			ExitCode:  e.ExitCode,
			Reason:    e.Reason,
			IsRetry:   e.IsRetry,
		}
		if !e.CompletionTime.IsZero() {
			item.CompletionTime = &e.CompletionTime
		}
		items = append(items, item)
	}

	writeJSON(
		w, http.StatusOK, ExecutionListResponse{
			Items: items,
			Pagination: Pagination{
				Total:   total,
				Limit:   limit,
				Offset:  offset,
				HasMore: int64(offset+limit) < total,
			},
		},
	)
}

// GetLogs handles GET /api/v1/cronjobs/:namespace/:name/executions/:jobName/logs
// @Summary      Get execution logs
// @Description  Returns container logs from a job execution
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Param        jobName    path      string  true  "Job name"
// @Success      200  {object}  LogsResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/executions/{jobName}/logs [get]
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

	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}

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

	writeJSON(
		w, http.StatusOK, LogsResponse{
			JobName:   jobName,
			Container: container,
			Logs:      buf.String(),
			Truncated: false,
		},
	)
}

// TriggerCronJob handles POST /api/v1/cronjobs/:namespace/:name/trigger
// @Summary      Trigger CronJob
// @Description  Manually triggers a CronJob to run immediately
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Success      200  {object}  TriggerResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/trigger [post]
func (h *Handlers) TriggerCronJob(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(
		w, http.StatusOK, TriggerResponse{
			Success: true,
			JobName: jobName,
			Message: "Job created successfully",
		},
	)
}

// SuspendCronJob handles POST /api/v1/cronjobs/:namespace/:name/suspend
// @Summary      Suspend CronJob
// @Description  Suspends a CronJob to prevent scheduled runs
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Success      200  {object}  SimpleResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/suspend [post]
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

	writeJSON(
		w, http.StatusOK, SimpleResponse{
			Success: true,
			Message: "CronJob suspended",
		},
	)
}

// ResumeCronJob handles POST /api/v1/cronjobs/:namespace/:name/resume
// @Summary      Resume CronJob
// @Description  Resumes a suspended CronJob to allow scheduled runs
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Success      200  {object}  SimpleResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/resume [post]
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

	writeJSON(
		w, http.StatusOK, SimpleResponse{
			Success: true,
			Message: "CronJob resumed",
		},
	)
}

// ListAlerts handles GET /api/v1/alerts
// @Summary      List active alerts
// @Description  Returns all active alerts across all monitored CronJobs
// @Tags         Alerts
// @Produce      json
// @Param        severity   query     string  false  "Filter by severity (critical, warning)"
// @Param        type       query     string  false  "Filter by alert type"
// @Param        namespace  query     string  false  "Filter by CronJob namespace"
// @Param        cronjob    query     string  false  "Filter by CronJob name"
// @Success      200  {object}  AlertListResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /alerts [get]
func (h *Handlers) ListAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	severityFilter := r.URL.Query().Get("severity")
	typeFilter := r.URL.Query().Get("type")
	namespaceFilter := r.URL.Query().Get("namespace")
	cronjobFilter := r.URL.Query().Get("cronjob")

	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := h.client.List(ctx, monitors); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	alertMap := make(map[string]AlertItem)
	severityOrder := map[string]int{"critical": 2, "warning": 1, "info": 0}

	for _, m := range monitors.Items {
		for _, cjStatus := range m.Status.CronJobs {
			if namespaceFilter != "" && cjStatus.Namespace != namespaceFilter {
				continue
			}
			if cronjobFilter != "" && cjStatus.Name != cronjobFilter {
				continue
			}

			for _, a := range cjStatus.ActiveAlerts {
				if typeFilter != "" && a.Type != typeFilter {
					continue
				}

				alertID := fmt.Sprintf("%s-%s-%s", cjStatus.Namespace, cjStatus.Name, a.Type)

				item := AlertItem{
					ID:       alertID,
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
				if a.ExitCode != 0 || a.Reason != "" || a.SuggestedFix != "" {
					item.Context = &AlertContextResponse{
						ExitCode:     a.ExitCode,
						Reason:       a.Reason,
						SuggestedFix: a.SuggestedFix,
					}
				}

				if existing, exists := alertMap[alertID]; exists {
					if severityOrder[a.Severity] > severityOrder[existing.Severity] {
						alertMap[alertID] = item
					} else if severityOrder[a.Severity] == severityOrder[existing.Severity] && a.Since.Time.Before(existing.Since) {
						alertMap[alertID] = item
					}
				} else {
					alertMap[alertID] = item
				}
			}
		}
	}

	items := make([]AlertItem, 0, len(alertMap))
	bySeverity := map[string]int{
		"critical": 0,
		"warning":  0,
	}

	for _, item := range alertMap {
		if severityFilter != "" && item.Severity != severityFilter {
			continue
		}
		items = append(items, item)
		bySeverity[item.Severity]++
	}

	writeJSON(
		w, http.StatusOK, AlertListResponse{
			Items:      items,
			Total:      len(items),
			BySeverity: bySeverity,
		},
	)
}

// GetAlertHistory handles GET /api/v1/alerts/history
// @Summary      Get alert history
// @Description  Returns paginated history of past alerts from the store
// @Tags         Alerts
// @Produce      json
// @Param        limit     query     int     false  "Page size"          default(50)
// @Param        offset    query     int     false  "Page offset"        default(0)
// @Param        severity  query     string  false  "Filter by severity"
// @Param        since     query     string  false  "Filter since timestamp (RFC3339)"
// @Success      200  {object}  AlertHistoryResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /alerts/history [get]
func (h *Handlers) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.store == nil {
		writeJSON(
			w, http.StatusOK, AlertHistoryResponse{
				Items: []AlertHistoryItem{},
				Pagination: Pagination{
					Total:   0,
					Limit:   50,
					Offset:  0,
					HasMore: false,
				},
			},
		)
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
			ChannelsNotified: a.GetChannelsNotified(),
			ExitCode:         a.ExitCode,
			Reason:           a.Reason,
			SuggestedFix:     a.SuggestedFix,
		}
		if a.CronJobNamespace != "" || a.CronJobName != "" {
			item.CronJob = &NamespacedRef{
				Namespace: a.CronJobNamespace,
				Name:      a.CronJobName,
			}
		}
		items = append(items, item)
	}

	writeJSON(
		w, http.StatusOK, AlertHistoryResponse{
			Items: items,
			Pagination: Pagination{
				Total:   total,
				Limit:   limit,
				Offset:  offset,
				HasMore: int64(offset+limit) < total,
			},
		},
	)
}

// ListChannels handles GET /api/v1/channels
// @Summary      List alert channels
// @Description  Returns all configured alert channels with their status and stats
// @Tags         Channels
// @Produce      json
// @Success      200  {object}  ChannelListResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /channels [get]
func (h *Handlers) ListChannels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	channels := &guardianv1alpha1.AlertChannelList{}
	if err := h.client.List(ctx, channels); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	var channelStats map[string]store.ChannelAlertStats
	if h.store != nil {
		var err error
		channelStats, err = h.store.GetChannelAlertStats(ctx)
		if err != nil {
			channelStats = make(map[string]store.ChannelAlertStats)
		}
	}

	items := make([]ChannelListItem, 0, len(channels.Items))
	ready := 0
	notReady := 0

	for _, ch := range channels.Items {
		stats := ChannelStats{}
		if s, ok := channelStats[ch.Name]; ok {
			stats.AlertsSentTotal = s.AlertsSentTotal
		}

		stats.AlertsFailedTotal = ch.Status.AlertsFailedTotal
		stats.ConsecutiveFailures = ch.Status.ConsecutiveFailures
		if ch.Status.LastAlertTime != nil {
			t := ch.Status.LastAlertTime.Time
			stats.LastAlertTime = &t
		}
		if ch.Status.LastFailedTime != nil {
			t := ch.Status.LastFailedTime.Time
			stats.LastFailedTime = &t
		}
		if ch.Status.LastFailedError != "" {
			stats.LastFailedError = ch.Status.LastFailedError
		}

		item := ChannelListItem{
			Name:  ch.Name,
			Type:  ch.Spec.Type,
			Ready: ch.Status.Ready,
			Stats: stats,
		}

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

	writeJSON(
		w, http.StatusOK, ChannelListResponse{
			Items: items,
			Summary: ChannelSummary{
				Total:    len(channels.Items),
				Ready:    ready,
				NotReady: notReady,
			},
		},
	)
}

// GetChannel handles GET /api/v1/channels/:name
// @Summary      Get channel details
// @Description  Returns details for a specific alert channel
// @Tags         Channels
// @Produce      json
// @Param        name  path      string  true  "Channel name"
// @Success      200  {object}  guardianv1alpha1.AlertChannel
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /channels/{name} [get]
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

	writeJSON(w, http.StatusOK, channel)
}

// TestChannel handles POST /api/v1/channels/:name/test
// @Summary      Test alert channel
// @Description  Sends a test alert to verify channel configuration
// @Tags         Channels
// @Produce      json
// @Param        name  path      string  true  "Channel name"
// @Success      200  {object}  SimpleResponse
// @Failure      503  {object}  ErrorResponse
// @Router       /channels/{name}/test [post]
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
		writeJSON(
			w, http.StatusOK, SimpleResponse{
				Success: false,
				Error:   err.Error(),
			},
		)
		return
	}

	writeJSON(
		w, http.StatusOK, SimpleResponse{
			Success: true,
			Message: "Test alert sent successfully",
		},
	)
}

// ConfigResponse represents the operator configuration for the API
type ConfigResponse struct {
	LogLevel         string                        `json:"logLevel"`
	Storage          config.StorageConfig          `json:"storage"`
	HistoryRetention config.HistoryRetentionConfig `json:"historyRetention"`
	RateLimits       config.RateLimitsConfig       `json:"rateLimits"`
	UI               config.UIConfig               `json:"ui"`
	Scheduler        config.SchedulerConfig        `json:"scheduler"`
}

// GetConfig handles GET /api/v1/config
// @Summary      Get operator configuration
// @Description  Returns the current operator configuration (sensitive values redacted)
// @Tags         Config
// @Produce      json
// @Success      200  {object}  ConfigResponse
// @Router       /config [get]
func (h *Handlers) GetConfig(w http.ResponseWriter, _ *http.Request) {
	if h.config == nil {
		writeJSON(w, http.StatusOK, ConfigResponse{})
		return
	}

	resp := ConfigResponse{
		LogLevel:         h.config.LogLevel,
		Storage:          h.config.Storage,
		HistoryRetention: h.config.HistoryRetention,
		RateLimits:       h.config.RateLimits,
		UI:               h.config.UI,
		Scheduler:        h.config.Scheduler,
	}

	writeJSON(w, http.StatusOK, resp)
}

// DeleteCronJobHistory handles DELETE /api/v1/cronjobs/:namespace/:name/history
// @Summary      Delete CronJob history
// @Description  Deletes all execution history for a specific CronJob
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Success      200  {object}  DeleteHistoryResponse
// @Failure      503  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/history [delete]
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

	writeJSON(
		w, http.StatusOK, DeleteHistoryResponse{
			Success:        true,
			RecordsDeleted: deleted,
			Message:        fmt.Sprintf("Deleted %d execution records for %s/%s", deleted, namespace, name),
		},
	)
}

// GetStorageStats handles GET /api/v1/admin/storage-stats
// @Summary      Get storage statistics
// @Description  Returns storage backend statistics and health
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  StorageStatsResponse
// @Failure      503  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/storage-stats [get]
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

	writeJSON(
		w, http.StatusOK, StorageStatsResponse{
			ExecutionCount:    count,
			StorageType:       storageType,
			Healthy:           healthy,
			RetentionDays:     h.config.HistoryRetention.DefaultDays,
			LogStorageEnabled: h.config.Storage.LogStorageEnabled,
		},
	)
}

// PruneRequest represents a prune request body
type PruneRequest struct {
	OlderThanDays int  `json:"olderThanDays"`
	DryRun        bool `json:"dryRun"`
	PruneLogsOnly bool `json:"pruneLogsOnly"`
}

// TriggerPrune handles POST /api/v1/admin/prune
// @Summary      Trigger history pruning
// @Description  Manually triggers pruning of old execution records
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        request  body      PruneRequest  false  "Prune options"
// @Success      200  {object}  PruneResponse
// @Failure      503  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/prune [post]
func (h *Handlers) TriggerPrune(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Store not configured")
		return
	}

	var req PruneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		writeJSON(
			w, http.StatusOK, PruneResponse{
				Success:       true,
				RecordsPruned: 0,
				DryRun:        true,
				Cutoff:        cutoff,
				OlderThanDays: req.OlderThanDays,
				Message:       "Dry run - no records deleted",
			},
		)
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

	writeJSON(
		w, http.StatusOK, PruneResponse{
			Success:       true,
			RecordsPruned: count,
			DryRun:        false,
			Cutoff:        cutoff,
			OlderThanDays: req.OlderThanDays,
			Message:       message,
		},
	)
}

// GetExecutionWithLogs handles GET /api/v1/cronjobs/:namespace/:name/executions/:jobName
// @Summary      Get execution details with logs
// @Description  Returns full execution details including stored logs and events
// @Tags         CronJobs
// @Produce      json
// @Param        namespace  path      string  true  "CronJob namespace"
// @Param        name       path      string  true  "CronJob name"
// @Param        jobName    path      string  true  "Job name (execution ID)"
// @Success      200  {object}  ExecutionDetailResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      503  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /cronjobs/{namespace}/{name}/executions/{jobName} [get]
func (h *Handlers) GetExecutionWithLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	jobName := chi.URLParam(r, "jobName")

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Store not configured")
		return
	}

	cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}
	since := time.Now().AddDate(0, 0, -90) // Look back 90 days
	executions, err := h.store.GetExecutions(ctx, cronJobNN, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

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
				Duration:         e.Duration().String(),
				ExitCode:         e.ExitCode,
				Reason:           e.Reason,
				IsRetry:          e.IsRetry,
				RetryOf:          e.RetryOf,
				StoredLogs:       ptr.Deref(e.Logs, ""),
				StoredEvents:     ptr.Deref(e.Events, ""),
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

// TestPattern handles POST /api/v1/patterns/test
// @Summary      Test suggested fix pattern
// @Description  Tests a suggested fix pattern against sample data to verify matching
// @Tags         Patterns
// @Accept       json
// @Produce      json
// @Param        request  body      PatternTestRequest  true  "Pattern and test data"
// @Success      200  {object}  PatternTestResponse
// @Failure      400  {object}  ErrorResponse
// @Router       /patterns/test [post]
func (h *Handlers) TestPattern(w http.ResponseWriter, r *http.Request) {
	var req PatternTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Pattern.Name == "" || req.Pattern.Suggestion == "" {
		writeJSON(
			w, http.StatusOK, PatternTestResponse{
				Matched: false,
				Error:   "pattern name and suggestion are required",
			},
		)
		return
	}

	pattern := guardianv1alpha1.SuggestedFixPattern{
		Name:       req.Pattern.Name,
		Suggestion: req.Pattern.Suggestion,
		Priority:   req.Pattern.Priority,
		Match: guardianv1alpha1.PatternMatch{
			ExitCode:      req.Pattern.Match.ExitCode,
			Reason:        req.Pattern.Match.Reason,
			ReasonPattern: req.Pattern.Match.ReasonPattern,
			LogPattern:    req.Pattern.Match.LogPattern,
			EventPattern:  req.Pattern.Match.EventPattern,
		},
	}
	if req.Pattern.Match.ExitCodeRange != nil {
		pattern.Match.ExitCodeRange = &guardianv1alpha1.ExitCodeRange{
			Min: req.Pattern.Match.ExitCodeRange.Min,
			Max: req.Pattern.Match.ExitCodeRange.Max,
		}
	}

	matchCtx := alerting.MatchContext{
		Namespace: req.TestData.Namespace,
		Name:      req.TestData.Name,
		JobName:   req.TestData.JobName,
		ExitCode:  req.TestData.ExitCode,
		Reason:    req.TestData.Reason,
		Logs:      req.TestData.Logs,
		Events:    req.TestData.Events,
	}

	engine := alerting.NewSuggestedFixEngine()

	priority := int32(1000)
	pattern.Priority = &priority
	patterns := []guardianv1alpha1.SuggestedFixPattern{pattern}
	suggestion := engine.GetBestSuggestion(matchCtx, patterns)

	fallback := "Check job logs and events for details."
	matched := suggestion != fallback

	resp := PatternTestResponse{
		Matched: matched,
	}
	if matched {
		resp.RenderedSuggestion = suggestion
	}

	writeJSON(w, http.StatusOK, resp)
}
