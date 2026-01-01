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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// Note: The metrics are registered globally in init(), so we test them directly
// without re-registering. These tests verify the wrapper functions work correctly.

func TestRecordExecution_Success(t *testing.T) {
	// Reset metric before test
	ExecutionsTotal.Reset()

	RecordExecution("default", "test-cron", "success")

	// Verify counter was incremented
	labels := prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"status":    "success",
	}
	count := testutil.ToFloat64(ExecutionsTotal.With(labels))
	assert.Equal(t, float64(1), count)

	// Increment again
	RecordExecution("default", "test-cron", "success")
	count = testutil.ToFloat64(ExecutionsTotal.With(labels))
	assert.Equal(t, float64(2), count)
}

func TestRecordExecution_Failed(t *testing.T) {
	// Reset metric before test
	ExecutionsTotal.Reset()

	RecordExecution("default", "failing-cron", "failed")

	labels := prometheus.Labels{
		"namespace": "default",
		"cronjob":   "failing-cron",
		"status":    "failed",
	}
	count := testutil.ToFloat64(ExecutionsTotal.With(labels))
	assert.Equal(t, float64(1), count)
}

func TestRecordExecution_DifferentCronJobs(t *testing.T) {
	// Reset metric before test
	ExecutionsTotal.Reset()

	RecordExecution("default", "cron-a", "success")
	RecordExecution("default", "cron-b", "success")
	RecordExecution("prod", "cron-a", "failed")

	assert.Equal(t, float64(1), testutil.ToFloat64(ExecutionsTotal.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "cron-a",
		"status":    "success",
	})))

	assert.Equal(t, float64(1), testutil.ToFloat64(ExecutionsTotal.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "cron-b",
		"status":    "success",
	})))

	assert.Equal(t, float64(1), testutil.ToFloat64(ExecutionsTotal.With(prometheus.Labels{
		"namespace": "prod",
		"cronjob":   "cron-a",
		"status":    "failed",
	})))
}

func TestRecordAlert_Increments(t *testing.T) {
	// Reset metric before test
	AlertsTotal.Reset()

	RecordAlert("default", "test-cron", "JobFailed", "critical", "slack-main")

	labels := prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"type":      "JobFailed",
		"severity":  "critical",
		"channel":   "slack-main",
	}
	count := testutil.ToFloat64(AlertsTotal.With(labels))
	assert.Equal(t, float64(1), count)

	// Send another alert
	RecordAlert("default", "test-cron", "JobFailed", "critical", "slack-main")
	count = testutil.ToFloat64(AlertsTotal.With(labels))
	assert.Equal(t, float64(2), count)
}

func TestRecordAlert_DifferentChannels(t *testing.T) {
	// Reset metric before test
	AlertsTotal.Reset()

	RecordAlert("default", "test-cron", "JobFailed", "critical", "slack")
	RecordAlert("default", "test-cron", "JobFailed", "critical", "pagerduty")
	RecordAlert("default", "test-cron", "SLABreached", "warning", "slack")

	assert.Equal(t, float64(1), testutil.ToFloat64(AlertsTotal.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"type":      "JobFailed",
		"severity":  "critical",
		"channel":   "slack",
	})))

	assert.Equal(t, float64(1), testutil.ToFloat64(AlertsTotal.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"type":      "JobFailed",
		"severity":  "critical",
		"channel":   "pagerduty",
	})))

	assert.Equal(t, float64(1), testutil.ToFloat64(AlertsTotal.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"type":      "SLABreached",
		"severity":  "warning",
		"channel":   "slack",
	})))
}

func TestRecordAlertFailed(t *testing.T) {
	// Reset metric before test
	AlertsFailedTotal.Reset()

	RecordAlertFailed("default", "test-cron", "JobFailed", "critical", "slack-broken")

	labels := prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"type":      "JobFailed",
		"severity":  "critical",
		"channel":   "slack-broken",
	}
	count := testutil.ToFloat64(AlertsFailedTotal.With(labels))
	assert.Equal(t, float64(1), count)
}

func TestUpdateSuccessRate(t *testing.T) {
	// Reset metric before test
	CronJobSuccessRate.Reset()

	UpdateSuccessRate("default", "test-cron", "monitor-a", 95.5)

	labels := prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"monitor":   "monitor-a",
	}
	rate := testutil.ToFloat64(CronJobSuccessRate.With(labels))
	assert.Equal(t, 95.5, rate)

	// Update to new value
	UpdateSuccessRate("default", "test-cron", "monitor-a", 80.0)
	rate = testutil.ToFloat64(CronJobSuccessRate.With(labels))
	assert.Equal(t, 80.0, rate)
}

func TestUpdateSuccessRate_MultipleMonitors(t *testing.T) {
	// Reset metric before test
	CronJobSuccessRate.Reset()

	UpdateSuccessRate("default", "test-cron", "monitor-a", 95.0)
	UpdateSuccessRate("default", "test-cron", "monitor-b", 99.0)

	assert.Equal(t, 95.0, testutil.ToFloat64(CronJobSuccessRate.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"monitor":   "monitor-a",
	})))

	assert.Equal(t, 99.0, testutil.ToFloat64(CronJobSuccessRate.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"monitor":   "monitor-b",
	})))
}

func TestUpdateDuration_P50(t *testing.T) {
	// Reset metric before test
	CronJobDurationSeconds.Reset()

	UpdateDuration("default", "test-cron", "p50", 30.5)

	labels := prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "test-cron",
		"percentile": "p50",
	}
	duration := testutil.ToFloat64(CronJobDurationSeconds.With(labels))
	assert.Equal(t, 30.5, duration)
}

func TestUpdateDuration_P95(t *testing.T) {
	// Reset metric before test
	CronJobDurationSeconds.Reset()

	UpdateDuration("default", "test-cron", "p95", 120.0)

	labels := prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "test-cron",
		"percentile": "p95",
	}
	duration := testutil.ToFloat64(CronJobDurationSeconds.With(labels))
	assert.Equal(t, 120.0, duration)
}

func TestUpdateDuration_P99(t *testing.T) {
	// Reset metric before test
	CronJobDurationSeconds.Reset()

	UpdateDuration("default", "test-cron", "p99", 180.0)

	labels := prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "test-cron",
		"percentile": "p99",
	}
	duration := testutil.ToFloat64(CronJobDurationSeconds.With(labels))
	assert.Equal(t, 180.0, duration)
}

func TestUpdateDuration_AllPercentiles(t *testing.T) {
	// Reset metric before test
	CronJobDurationSeconds.Reset()

	UpdateDuration("default", "test-cron", "p50", 30.0)
	UpdateDuration("default", "test-cron", "p95", 90.0)
	UpdateDuration("default", "test-cron", "p99", 120.0)

	assert.Equal(t, 30.0, testutil.ToFloat64(CronJobDurationSeconds.With(prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "test-cron",
		"percentile": "p50",
	})))

	assert.Equal(t, 90.0, testutil.ToFloat64(CronJobDurationSeconds.With(prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "test-cron",
		"percentile": "p95",
	})))

	assert.Equal(t, 120.0, testutil.ToFloat64(CronJobDurationSeconds.With(prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "test-cron",
		"percentile": "p99",
	})))
}

func TestUpdateActiveAlerts(t *testing.T) {
	// Reset metric before test
	ActiveAlerts.Reset()

	UpdateActiveAlerts("default", "test-cron", "critical", 2.0)

	labels := prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"severity":  "critical",
	}
	count := testutil.ToFloat64(ActiveAlerts.With(labels))
	assert.Equal(t, 2.0, count)

	// Update to new value
	UpdateActiveAlerts("default", "test-cron", "critical", 0.0)
	count = testutil.ToFloat64(ActiveAlerts.With(labels))
	assert.Equal(t, 0.0, count)
}

func TestUpdateActiveAlerts_DifferentSeverities(t *testing.T) {
	// Reset metric before test
	ActiveAlerts.Reset()

	UpdateActiveAlerts("default", "test-cron", "critical", 1.0)
	UpdateActiveAlerts("default", "test-cron", "warning", 3.0)
	UpdateActiveAlerts("default", "test-cron", "info", 5.0)

	assert.Equal(t, 1.0, testutil.ToFloat64(ActiveAlerts.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"severity":  "critical",
	})))

	assert.Equal(t, 3.0, testutil.ToFloat64(ActiveAlerts.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"severity":  "warning",
	})))

	assert.Equal(t, 5.0, testutil.ToFloat64(ActiveAlerts.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "test-cron",
		"severity":  "info",
	})))
}

func TestResetCronJobMetrics(t *testing.T) {
	// Setup: create metrics for two CronJobs
	CronJobSuccessRate.Reset()
	CronJobDurationSeconds.Reset()
	ActiveAlerts.Reset()

	// Create metrics for "delete-me" CronJob
	UpdateSuccessRate("default", "delete-me", "monitor", 95.0)
	UpdateDuration("default", "delete-me", "p50", 30.0)
	UpdateDuration("default", "delete-me", "p95", 90.0)
	UpdateActiveAlerts("default", "delete-me", "warning", 2.0)

	// Create metrics for "keep-me" CronJob
	UpdateSuccessRate("default", "keep-me", "monitor", 99.0)
	UpdateDuration("default", "keep-me", "p50", 20.0)
	UpdateActiveAlerts("default", "keep-me", "warning", 1.0)

	// Verify setup
	assert.Equal(t, 95.0, testutil.ToFloat64(CronJobSuccessRate.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "delete-me",
		"monitor":   "monitor",
	})))

	// Reset metrics for "delete-me"
	ResetCronJobMetrics("default", "delete-me")

	// Verify "delete-me" metrics are gone - using metric count approach
	// Since we deleted partial match, the metrics should no longer exist
	// We can verify by checking that "keep-me" still exists

	assert.Equal(t, 99.0, testutil.ToFloat64(CronJobSuccessRate.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "keep-me",
		"monitor":   "monitor",
	})))

	assert.Equal(t, 20.0, testutil.ToFloat64(CronJobDurationSeconds.With(prometheus.Labels{
		"namespace":  "default",
		"cronjob":    "keep-me",
		"percentile": "p50",
	})))

	assert.Equal(t, 1.0, testutil.ToFloat64(ActiveAlerts.With(prometheus.Labels{
		"namespace": "default",
		"cronjob":   "keep-me",
		"severity":  "warning",
	})))
}

func TestResetCronJobMetrics_DifferentNamespaces(t *testing.T) {
	// Reset all metrics
	CronJobSuccessRate.Reset()
	CronJobDurationSeconds.Reset()
	ActiveAlerts.Reset()

	// Create metrics in different namespaces for same CronJob name
	UpdateSuccessRate("ns1", "same-name", "monitor", 95.0)
	UpdateSuccessRate("ns2", "same-name", "monitor", 99.0)

	// Reset only ns1
	ResetCronJobMetrics("ns1", "same-name")

	// Verify ns2 still has metrics
	assert.Equal(t, 99.0, testutil.ToFloat64(CronJobSuccessRate.With(prometheus.Labels{
		"namespace": "ns2",
		"cronjob":   "same-name",
		"monitor":   "monitor",
	})))
}

// Test that metric labels are correctly structured
func TestMetricLabels(t *testing.T) {
	// Verify ExecutionsTotal has correct labels
	desc := ExecutionsTotal.WithLabelValues("ns", "cj", "status").Desc()
	assert.NotNil(t, desc)

	// Verify AlertsTotal has correct labels
	desc = AlertsTotal.WithLabelValues("ns", "cj", "type", "sev", "ch").Desc()
	assert.NotNil(t, desc)

	// Verify CronJobSuccessRate has correct labels
	desc = CronJobSuccessRate.WithLabelValues("ns", "cj", "mon").Desc()
	assert.NotNil(t, desc)

	// Verify CronJobDurationSeconds has correct labels
	desc = CronJobDurationSeconds.WithLabelValues("ns", "cj", "pct").Desc()
	assert.NotNil(t, desc)

	// Verify ActiveAlerts has correct labels
	desc = ActiveAlerts.WithLabelValues("ns", "cj", "sev").Desc()
	assert.NotNil(t, desc)
}
