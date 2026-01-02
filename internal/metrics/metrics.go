package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// CronJobSuccessRate tracks the success rate of monitored CronJobs
	CronJobSuccessRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cronjob_guardian_success_rate",
			Help: "Success rate of monitored CronJobs (0-100)",
		},
		[]string{"namespace", "cronjob", "monitor"},
	)

	// CronJobDurationSeconds tracks duration metrics for monitored CronJobs
	CronJobDurationSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cronjob_guardian_duration_seconds",
			Help: "Duration metrics for monitored CronJobs",
		},
		[]string{"namespace", "cronjob", "percentile"},
	)

	// AlertsTotal tracks the total number of alerts successfully sent
	AlertsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cronjob_guardian_alerts_total",
			Help: "Total number of alerts successfully sent",
		},
		[]string{"namespace", "cronjob", "type", "severity", "channel"},
	)

	// AlertsFailedTotal tracks the total number of alerts that failed to send
	AlertsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cronjob_guardian_alerts_failed_total",
			Help: "Total number of alerts that failed to send",
		},
		[]string{"namespace", "cronjob", "type", "severity", "channel"},
	)

	// ExecutionsTotal tracks the total number of job executions recorded
	ExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cronjob_guardian_executions_total",
			Help: "Total number of job executions recorded",
		},
		[]string{"namespace", "cronjob", "status"},
	)

	// ActiveAlerts tracks the number of currently active alerts
	ActiveAlerts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cronjob_guardian_active_alerts",
			Help: "Number of currently active alerts",
		},
		[]string{"namespace", "cronjob", "severity"},
	)
)

func init() {
	// Register all metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(
		CronJobSuccessRate,
		CronJobDurationSeconds,
		AlertsTotal,
		AlertsFailedTotal,
		ExecutionsTotal,
		ActiveAlerts,
	)
}

// RecordExecution records a job execution metric
func RecordExecution(namespace, cronjob, status string) {
	ExecutionsTotal.WithLabelValues(namespace, cronjob, status).Inc()
}

// RecordAlert records a successful alert sent metric
func RecordAlert(namespace, cronjob, alertType, severity, channel string) {
	AlertsTotal.WithLabelValues(namespace, cronjob, alertType, severity, channel).Inc()
}

// RecordAlertFailed records a failed alert send metric
func RecordAlertFailed(namespace, cronjob, alertType, severity, channel string) {
	AlertsFailedTotal.WithLabelValues(namespace, cronjob, alertType, severity, channel).Inc()
}

// UpdateSuccessRate updates the success rate gauge for a CronJob
func UpdateSuccessRate(namespace, cronjob, monitor string, rate float64) {
	CronJobSuccessRate.WithLabelValues(namespace, cronjob, monitor).Set(rate)
}

// UpdateDuration updates duration percentile gauges for a CronJob
func UpdateDuration(namespace, cronjob, percentile string, seconds float64) {
	CronJobDurationSeconds.WithLabelValues(namespace, cronjob, percentile).Set(seconds)
}

// UpdateActiveAlerts updates the active alerts gauge for a CronJob
func UpdateActiveAlerts(namespace, cronjob, severity string, count float64) {
	ActiveAlerts.WithLabelValues(namespace, cronjob, severity).Set(count)
}

// ResetCronJobMetrics resets all metrics for a specific CronJob (e.g., when it's deleted)
func ResetCronJobMetrics(namespace, cronjob string) {
	// Delete all label combinations for this CronJob
	CronJobSuccessRate.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "cronjob": cronjob})
	CronJobDurationSeconds.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "cronjob": cronjob})
	ActiveAlerts.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "cronjob": cronjob})
}
