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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/testutil"
)

// Test scheme
var testScheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(testScheme)
	_ = guardianv1alpha1.AddToScheme(testScheme)
	_ = batchv1.AddToScheme(testScheme)
}

// Helper to create fake client
func newTestAPIClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(objs...).
		Build()
}

// Helper to create handlers with test dependencies
func newTestHandlers(c client.Client, s store.Store, cfg *config.Config, disp alerting.Dispatcher) *Handlers {
	h := NewHandlers(c, nil, s, cfg, disp, time.Now(), func() bool { return true })
	return h
}

// Helper to create a chi router with URL params
func chiRouterWithParams(handler http.HandlerFunc, params map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := chi.NewRouteContext()
		for k, v := range params {
			ctx.URLParams.Add(k, v)
		}
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, ctx))
		handler.ServeHTTP(w, r)
	}
}

// ============================================================================
// Health Handler Tests
// ============================================================================

func TestHealthHandler_Healthy(t *testing.T) {
	mockStore := &testutil.MockStore{}
	cfg := &config.Config{LogLevel: "info"}
	h := newTestHandlers(newTestAPIClient(), mockStore, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	h.GetHealth(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result HealthResponse
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, "connected", result.Storage)
	assert.True(t, result.Leader)
}

func TestHealthHandler_StorageInfo(t *testing.T) {
	mockStore := &testutil.MockStore{}
	cfg := &config.Config{
		LogLevel: "info",
		Storage:  config.StorageConfig{Type: "sqlite"},
	}
	h := newTestHandlers(newTestAPIClient(), mockStore, cfg, nil)
	h.SetSchedulersRunning([]string{"dead-man", "sla-recalc"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	h.GetHealth(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	var result HealthResponse
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "connected", result.Storage)
	assert.Contains(t, result.SchedulersRunning, "dead-man")
	assert.Contains(t, result.SchedulersRunning, "sla-recalc")
}

func TestHealthHandler_LeaderStatus(t *testing.T) {
	t.Run("leader", func(t *testing.T) {
		h := NewHandlers(newTestAPIClient(), nil, nil, nil, nil, time.Now(), func() bool { return true })

		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()

		h.GetHealth(w, req)

		var result HealthResponse
		_ = json.NewDecoder(w.Body).Decode(&result)
		assert.True(t, result.Leader)
	})

	t.Run("not leader", func(t *testing.T) {
		h := NewHandlers(newTestAPIClient(), nil, nil, nil, nil, time.Now(), func() bool { return false })

		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()

		h.GetHealth(w, req)

		var result HealthResponse
		_ = json.NewDecoder(w.Body).Decode(&result)
		assert.False(t, result.Leader)
	})
}

// ============================================================================
// Stats Handler Tests
// ============================================================================

func TestStatsHandler_Counts(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			Summary: &guardianv1alpha1.MonitorSummary{
				TotalCronJobs: 5,
				Healthy:       3,
				Warning:       1,
				Critical:      1,
				ActiveAlerts:  2,
			},
		},
	}

	mockStore := &testutil.MockStore{ExecutionCountSince: 100}
	h := newTestHandlers(newTestAPIClient(monitor), mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result StatsResponse
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, int32(1), result.TotalMonitors)
	assert.Equal(t, int32(5), result.TotalCronJobs)
	assert.Equal(t, int64(100), result.ExecutionsRecorded24h)
}

func TestStatsHandler_Summary(t *testing.T) {
	monitor1 := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-1",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			Summary: &guardianv1alpha1.MonitorSummary{
				Healthy:  3,
				Warning:  1,
				Critical: 0,
			},
		},
	}
	monitor2 := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-2",
			Namespace: "prod",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			Summary: &guardianv1alpha1.MonitorSummary{
				Healthy:  2,
				Warning:  0,
				Critical: 2,
			},
		},
	}

	h := newTestHandlers(newTestAPIClient(monitor1, monitor2), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	var result StatsResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Equal(t, int32(2), result.TotalMonitors)
	assert.Equal(t, int32(5), result.Summary.Healthy)
	assert.Equal(t, int32(1), result.Summary.Warning)
	assert.Equal(t, int32(2), result.Summary.Critical)
}

// ============================================================================
// Monitor List Handler Tests
// ============================================================================

func TestMonitorListHandler_Pagination(t *testing.T) {
	// Create multiple monitors
	monitors := make([]client.Object, 10)
	for i := 0; i < 10; i++ {
		monitors[i] = &guardianv1alpha1.CronJobMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitor-" + string(rune('a'+i)),
				Namespace: "default",
			},
			Status: guardianv1alpha1.CronJobMonitorStatus{
				Phase: "Active",
			},
		}
	}

	h := newTestHandlers(newTestAPIClient(monitors...), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil)
	w := httptest.NewRecorder()

	h.ListMonitors(w, req)

	var result MonitorListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	// Should return all monitors (no pagination in this handler)
	assert.Len(t, result.Items, 10)
}

func TestMonitorListHandler_Empty(t *testing.T) {
	h := newTestHandlers(newTestAPIClient(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors", nil)
	w := httptest.NewRecorder()

	h.ListMonitors(w, req)

	var result MonitorListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Empty(t, result.Items)
}

// ============================================================================
// Monitor Detail Handler Tests
// ============================================================================

func TestMonitorDetailHandler_Found(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.CronJobMonitorSpec{
			Selector: &guardianv1alpha1.CronJobSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			Phase: "Active",
		},
	}

	h := newTestHandlers(newTestAPIClient(monitor), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/default/test-monitor", nil)
	handler := chiRouterWithParams(h.GetMonitor, map[string]string{
		"namespace": "default",
		"name":      "test-monitor",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result guardianv1alpha1.CronJobMonitor
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "test-monitor", result.Name)
	assert.Equal(t, "default", result.Namespace)
}

func TestMonitorDetailHandler_NotFound(t *testing.T) {
	h := newTestHandlers(newTestAPIClient(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitors/default/nonexistent", nil)
	handler := chiRouterWithParams(h.GetMonitor, map[string]string{
		"namespace": "default",
		"name":      "nonexistent",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var result ErrorResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "NOT_FOUND", result.Error.Code)
}

// ============================================================================
// CronJob List Handler Tests
// ============================================================================

func TestCronJobListHandler_All(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "cron-1",
					Namespace: "default",
					Status:    "healthy",
				},
				{
					Name:      "cron-2",
					Namespace: "default",
					Status:    "warning",
				},
			},
		},
	}

	cronJob1 := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cron-1",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
		},
	}

	cronJob2 := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cron-2",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *",
		},
	}

	h := newTestHandlers(newTestAPIClient(monitor, cronJob1, cronJob2), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cronjobs", nil)
	w := httptest.NewRecorder()

	h.ListCronJobs(w, req)

	var result CronJobListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 2)
	assert.Equal(t, int32(1), result.Summary.Healthy)
	assert.Equal(t, int32(1), result.Summary.Warning)
}

// ============================================================================
// CronJob Detail Handler Tests
// ============================================================================

func TestCronJobDetailHandler_Found(t *testing.T) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
		},
	}

	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "test-cron",
					Namespace: "default",
					Status:    "healthy",
				},
			},
		},
	}

	h := newTestHandlers(newTestAPIClient(cronJob, monitor), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cronjobs/default/test-cron", nil)
	handler := chiRouterWithParams(h.GetCronJob, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result CronJobDetailResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, "test-cron", result.Name)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, "*/5 * * * *", result.Schedule)
}

func TestCronJobDetailHandler_WithMetrics(t *testing.T) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
		},
	}

	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "test-cron",
					Namespace: "default",
					Status:    "healthy",
					Metrics: &guardianv1alpha1.CronJobMetrics{
						SuccessRate:        95.5,
						TotalRuns:          100,
						SuccessfulRuns:     95,
						FailedRuns:         5,
						AvgDurationSeconds: 30.0,
						P50DurationSeconds: 25.0,
						P95DurationSeconds: 50.0,
						P99DurationSeconds: 60.0,
					},
				},
			},
		},
	}

	mockStore := &testutil.MockStore{
		Metrics: &store.Metrics{
			SuccessRate: 90.0,
		},
	}

	h := newTestHandlers(newTestAPIClient(cronJob, monitor), mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cronjobs/default/test-cron", nil)
	handler := chiRouterWithParams(h.GetCronJob, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result CronJobDetailResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	require.NotNil(t, result.Metrics)
	assert.Equal(t, 95.5, result.Metrics.SuccessRate7d)
	assert.Equal(t, 90.0, result.Metrics.SuccessRate30d)
	assert.Equal(t, int32(100), result.Metrics.TotalRuns7d)
}

// ============================================================================
// Alert Handler Tests
// ============================================================================

func TestAlertsHandler_Pagination(t *testing.T) {
	mockStore := &testutil.MockStore{
		AlertHistory: []store.AlertHistory{
			{ID: 1, Type: "JobFailed", Severity: "critical"},
			{ID: 2, Type: "SLABreached", Severity: "warning"},
		},
		AlertHistoryTotal: 100,
	}

	h := newTestHandlers(newTestAPIClient(), mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/history?limit=10&offset=20", nil)
	w := httptest.NewRecorder()

	h.GetAlertHistory(w, req)

	var result AlertHistoryResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(100), result.Pagination.Total)
	assert.Equal(t, 10, result.Pagination.Limit)
	assert.Equal(t, 20, result.Pagination.Offset)
	assert.True(t, result.Pagination.HasMore)
}

func TestAlertsHandler_FilterByType(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "cron-1",
					Namespace: "default",
					ActiveAlerts: []guardianv1alpha1.ActiveAlert{
						{Type: "JobFailed", Severity: "critical", Since: metav1.Now()},
						{Type: "SLABreached", Severity: "warning", Since: metav1.Now()},
					},
				},
			},
		},
	}

	h := newTestHandlers(newTestAPIClient(monitor), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?type=JobFailed", nil)
	w := httptest.NewRecorder()

	h.ListAlerts(w, req)

	var result AlertListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 1)
	assert.Equal(t, "JobFailed", result.Items[0].Type)
}

func TestAlertsHandler_FilterBySeverity(t *testing.T) {
	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-monitor",
			Namespace: "default",
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			CronJobs: []guardianv1alpha1.CronJobStatus{
				{
					Name:      "cron-1",
					Namespace: "default",
					ActiveAlerts: []guardianv1alpha1.ActiveAlert{
						{Type: "JobFailed", Severity: "critical", Since: metav1.Now()},
						{Type: "SLABreached", Severity: "warning", Since: metav1.Now()},
					},
				},
			},
		},
	}

	h := newTestHandlers(newTestAPIClient(monitor), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?severity=critical", nil)
	w := httptest.NewRecorder()

	h.ListAlerts(w, req)

	var result AlertListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 1)
	assert.Equal(t, "critical", result.Items[0].Severity)
}

func TestAlertsHandler_FilterByTime(t *testing.T) {
	now := time.Now()
	mockStore := &testutil.MockStore{
		AlertHistory: []store.AlertHistory{
			{
				ID:         1,
				Type:       "JobFailed",
				OccurredAt: now.Add(-1 * time.Hour),
			},
		},
		AlertHistoryTotal: 1,
	}

	h := newTestHandlers(newTestAPIClient(), mockStore, nil, nil)

	since := now.Add(-2 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/history?since="+since, nil)
	w := httptest.NewRecorder()

	h.GetAlertHistory(w, req)

	var result AlertHistoryResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 1)
}

// ============================================================================
// Test Alert Handler Tests
// ============================================================================

func TestTestAlertHandler_Success(t *testing.T) {
	mockDispatcher := testutil.NewMockDispatcher()
	h := newTestHandlers(newTestAPIClient(), nil, nil, mockDispatcher)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/slack-channel/test", nil)
	handler := chiRouterWithParams(h.TestChannel, map[string]string{
		"name": "slack-channel",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result SimpleResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.Equal(t, "Test alert sent successfully", result.Message)
}

func TestTestAlertHandler_InvalidChannel(t *testing.T) {
	mockDispatcher := testutil.NewMockDispatcher()
	mockDispatcher.SendToChannelError = assert.AnError
	h := newTestHandlers(newTestAPIClient(), nil, nil, mockDispatcher)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/nonexistent/test", nil)
	handler := chiRouterWithParams(h.TestChannel, map[string]string{
		"name": "nonexistent",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result SimpleResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)
}

// ============================================================================
// Execution Handler Tests
// ============================================================================

func TestGetExecutions_Pagination(t *testing.T) {
	mockStore := &testutil.MockStore{
		ExecutionsFiltered: []store.Execution{
			{ID: 1, JobName: "job-1", Succeeded: true},
			{ID: 2, JobName: "job-2", Succeeded: false},
		},
		ExecutionsTotal: 50,
	}

	h := newTestHandlers(newTestAPIClient(), mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cronjobs/default/test-cron/executions?limit=10&offset=20", nil)
	handler := chiRouterWithParams(h.GetExecutions, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result ExecutionListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(50), result.Pagination.Total)
	assert.Equal(t, 10, result.Pagination.Limit)
	assert.Equal(t, 20, result.Pagination.Offset)
}

func TestGetExecutions_NoStore(t *testing.T) {
	h := newTestHandlers(newTestAPIClient(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cronjobs/default/test-cron/executions", nil)
	handler := chiRouterWithParams(h.GetExecutions, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result ExecutionListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Empty(t, result.Items)
	assert.Equal(t, int64(0), result.Pagination.Total)
}

// ============================================================================
// Channel Handler Tests
// ============================================================================

func TestListChannels(t *testing.T) {
	channel1 := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slack-channel",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "slack",
		},
		Status: guardianv1alpha1.AlertChannelStatus{
			Ready: true,
		},
	}
	channel2 := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "email-channel",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "email",
		},
		Status: guardianv1alpha1.AlertChannelStatus{
			Ready: false,
		},
	}

	mockStore := &testutil.MockStore{
		ChannelAlertStats: map[string]store.ChannelAlertStats{
			"slack-channel": {AlertsSentTotal: 100},
		},
	}

	h := newTestHandlers(newTestAPIClient(channel1, channel2), mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/channels", nil)
	w := httptest.NewRecorder()

	h.ListChannels(w, req)

	var result ChannelListResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Len(t, result.Items, 2)
	assert.Equal(t, 2, result.Summary.Total)
	assert.Equal(t, 1, result.Summary.Ready)
	assert.Equal(t, 1, result.Summary.NotReady)
}

// ============================================================================
// Config Handler Tests
// ============================================================================

func TestGetConfig(t *testing.T) {
	cfg := &config.Config{
		LogLevel: "debug",
		Storage: config.StorageConfig{
			Type: "sqlite",
		},
		HistoryRetention: config.HistoryRetentionConfig{
			DefaultDays: 30,
		},
	}

	h := newTestHandlers(newTestAPIClient(), nil, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()

	h.GetConfig(w, req)

	var result ConfigResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Equal(t, "debug", result.LogLevel)
	assert.Equal(t, "sqlite", result.Storage.Type)
	assert.Equal(t, 30, result.HistoryRetention.DefaultDays)
}

// ============================================================================
// Admin Handler Tests
// ============================================================================

func TestGetStorageStats(t *testing.T) {
	mockStore := &testutil.MockStore{
		ExecutionCount: 1000,
	}
	cfg := &config.Config{
		Storage: config.StorageConfig{
			Type:              "sqlite",
			LogStorageEnabled: true,
		},
		HistoryRetention: config.HistoryRetentionConfig{
			DefaultDays: 30,
		},
	}

	h := newTestHandlers(newTestAPIClient(), mockStore, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage-stats", nil)
	w := httptest.NewRecorder()

	h.GetStorageStats(w, req)

	var result StorageStatsResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.Equal(t, int64(1000), result.ExecutionCount)
	assert.Equal(t, "sqlite", result.StorageType)
	assert.True(t, result.Healthy)
	assert.Equal(t, 30, result.RetentionDays)
	assert.True(t, result.LogStorageEnabled)
}

func TestTriggerPrune(t *testing.T) {
	mockStore := &testutil.MockStore{
		PrunedCount: 50,
	}
	cfg := &config.Config{
		HistoryRetention: config.HistoryRetentionConfig{
			DefaultDays: 30,
		},
	}

	h := newTestHandlers(newTestAPIClient(), mockStore, cfg, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/prune", strings.NewReader(`{"olderThanDays": 7}`))
	w := httptest.NewRecorder()

	h.TriggerPrune(w, req)

	var result PruneResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.Equal(t, int64(50), result.RecordsPruned)
	assert.Equal(t, 7, result.OlderThanDays)
}

func TestTriggerPrune_DryRun(t *testing.T) {
	mockStore := &testutil.MockStore{}
	cfg := &config.Config{}

	h := newTestHandlers(newTestAPIClient(), mockStore, cfg, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/prune", strings.NewReader(`{"olderThanDays": 7, "dryRun": true}`))
	w := httptest.NewRecorder()

	h.TriggerPrune(w, req)

	var result PruneResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.True(t, result.DryRun)
	assert.Equal(t, int64(0), result.RecordsPruned)
}

// ============================================================================
// Delete History Handler Tests
// ============================================================================

func TestDeleteCronJobHistory(t *testing.T) {
	mockStore := &testutil.MockStore{
		DeletedCount: 25,
	}

	h := newTestHandlers(newTestAPIClient(), mockStore, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cronjobs/default/test-cron/history", nil)
	handler := chiRouterWithParams(h.DeleteCronJobHistory, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result DeleteHistoryResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.Equal(t, int64(25), result.RecordsDeleted)
}

// ============================================================================
// Pattern Test Handler Tests
// ============================================================================

func TestTestPattern_Match(t *testing.T) {
	h := newTestHandlers(newTestAPIClient(), nil, nil, nil)

	body := `{
		"pattern": {
			"name": "oom-test",
			"match": {"reason": "OOMKilled"},
			"suggestion": "Increase memory limits"
		},
		"testData": {
			"reason": "OOMKilled",
			"namespace": "default",
			"name": "test-cron",
			"jobName": "test-job"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/patterns/test", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.TestPattern(w, req)

	var result PatternTestResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Matched)
	assert.Equal(t, "Increase memory limits", result.RenderedSuggestion)
}

func TestTestPattern_NoMatch(t *testing.T) {
	h := newTestHandlers(newTestAPIClient(), nil, nil, nil)

	// Use a reason that doesn't match any built-in pattern or our test pattern
	body := `{
		"pattern": {
			"name": "oom-test",
			"match": {"reason": "OOMKilled"},
			"suggestion": "Increase memory limits"
		},
		"testData": {
			"reason": "SomeUnknownReason",
			"exitCode": 0,
			"namespace": "default",
			"name": "test-cron",
			"jobName": "test-job"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/patterns/test", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.TestPattern(w, req)

	var result PatternTestResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.False(t, result.Matched)
}

// ============================================================================
// Trigger CronJob Handler Tests
// ============================================================================

func TestTriggerCronJob(t *testing.T) {
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "busybox",
								},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}

	h := newTestHandlers(newTestAPIClient(cronJob), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cronjobs/default/test-cron/trigger", nil)
	handler := chiRouterWithParams(h.TriggerCronJob, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result TriggerResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.Contains(t, result.JobName, "test-cron-manual-")
}

func TestTriggerCronJob_NotFound(t *testing.T) {
	h := newTestHandlers(newTestAPIClient(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cronjobs/default/nonexistent/trigger", nil)
	handler := chiRouterWithParams(h.TriggerCronJob, map[string]string{
		"namespace": "default",
		"name":      "nonexistent",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ============================================================================
// Suspend/Resume CronJob Handler Tests
// ============================================================================

func TestSuspendCronJob(t *testing.T) {
	suspended := false
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspended,
		},
	}

	fakeClient := newTestAPIClient(cronJob)
	h := newTestHandlers(fakeClient, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cronjobs/default/test-cron/suspend", nil)
	handler := chiRouterWithParams(h.SuspendCronJob, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result SimpleResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.Equal(t, "CronJob suspended", result.Message)

	// Verify CronJob is now suspended
	var updated batchv1.CronJob
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-cron"}, &updated)
	assert.True(t, *updated.Spec.Suspend)
}

func TestResumeCronJob(t *testing.T) {
	suspended := true
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			Suspend:  &suspended,
		},
	}

	fakeClient := newTestAPIClient(cronJob)
	h := newTestHandlers(fakeClient, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cronjobs/default/test-cron/resume", nil)
	handler := chiRouterWithParams(h.ResumeCronJob, map[string]string{
		"namespace": "default",
		"name":      "test-cron",
	})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var result SimpleResponse
	_ = json.NewDecoder(w.Body).Decode(&result)

	assert.True(t, result.Success)
	assert.Equal(t, "CronJob resumed", result.Message)

	// Verify CronJob is now not suspended
	var updated batchv1.CronJob
	_ = fakeClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "test-cron"}, &updated)
	assert.False(t, *updated.Spec.Suspend)
}

// ============================================================================
// Write JSON Helper Tests
// ============================================================================

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	writeJSON(w, http.StatusOK, data)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `"key":"value"`)
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid input")

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var result ErrorResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, "BAD_REQUEST", result.Error.Code)
	assert.Equal(t, "Invalid input", result.Error.Message)
}
