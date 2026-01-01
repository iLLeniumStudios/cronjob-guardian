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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CronJobMonitorSpec defines the desired state of CronJobMonitor
type CronJobMonitorSpec struct {
	// Selector specifies which CronJobs to monitor
	// +optional
	Selector *CronJobSelector `json:"selector,omitempty"`

	// DeadManSwitch configures dead-man's switch alerting
	// +optional
	DeadManSwitch *DeadManSwitchConfig `json:"deadManSwitch,omitempty"`

	// SLA configures SLA tracking and alerting
	// +optional
	SLA *SLAConfig `json:"sla,omitempty"`

	// SuspendedHandling configures behavior for suspended CronJobs
	// +optional
	SuspendedHandling *SuspendedHandlingConfig `json:"suspendedHandling,omitempty"`

	// MaintenanceWindows defines scheduled maintenance periods
	// +optional
	MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows,omitempty"`

	// Alerting configures alert channels and behavior
	// +optional
	Alerting *AlertingConfig `json:"alerting,omitempty"`

	// DataRetention configures data lifecycle management
	// +optional
	DataRetention *DataRetentionConfig `json:"dataRetention,omitempty"`
}

// CronJobSelector specifies which CronJobs to monitor
type CronJobSelector struct {
	// MatchLabels selects CronJobs by labels
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions selects CronJobs by label expressions
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`

	// MatchNames explicitly lists CronJob names to monitor (only valid when watching a single namespace)
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`

	// Namespaces explicitly lists namespaces to watch for CronJobs.
	// If empty and namespaceSelector is not set, watches only the monitor's namespace.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// NamespaceSelector selects namespaces by labels.
	// CronJobs in matching namespaces will be monitored.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// AllNamespaces watches CronJobs in all namespaces (except globally ignored ones).
	// Takes precedence over namespaces and namespaceSelector.
	// +optional
	AllNamespaces bool `json:"allNamespaces,omitempty"`
}

// DeadManSwitchConfig configures dead-man's switch behavior
type DeadManSwitchConfig struct {
	// Enabled turns on dead-man's switch monitoring (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MaxTimeSinceLastSuccess alerts if no success within this duration
	// Example: "25h" for daily jobs with 1h buffer
	// +optional
	MaxTimeSinceLastSuccess *metav1.Duration `json:"maxTimeSinceLastSuccess,omitempty"`

	// AutoFromSchedule auto-calculates expected interval from cron schedule
	// +optional
	AutoFromSchedule *AutoScheduleConfig `json:"autoFromSchedule,omitempty"`
}

// AutoScheduleConfig configures automatic schedule detection
type AutoScheduleConfig struct {
	// Enabled turns on auto-detection (default: false)
	Enabled bool `json:"enabled"`

	// Buffer adds extra time to expected interval (default: 1h)
	// +optional
	Buffer *metav1.Duration `json:"buffer,omitempty"`

	// MissedScheduleThreshold alerts after this many missed schedules (default: 1)
	// +optional
	MissedScheduleThreshold *int32 `json:"missedScheduleThreshold,omitempty"`
}

// SLAConfig configures SLA tracking
type SLAConfig struct {
	// Enabled turns on SLA tracking (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinSuccessRate is minimum acceptable success rate percentage (default: 95)
	// +optional
	MinSuccessRate *float64 `json:"minSuccessRate,omitempty"`

	// WindowDays is the rolling window for success rate calculation (default: 7)
	// +optional
	WindowDays *int32 `json:"windowDays,omitempty"`

	// MaxDuration alerts if job exceeds this duration
	// +optional
	MaxDuration *metav1.Duration `json:"maxDuration,omitempty"`

	// DurationRegressionThreshold alerts if P95 increases by this percentage (default: 50)
	// +optional
	DurationRegressionThreshold *int32 `json:"durationRegressionThreshold,omitempty"`

	// DurationBaselineWindowDays for baseline calculation (default: 14)
	// +optional
	DurationBaselineWindowDays *int32 `json:"durationBaselineWindowDays,omitempty"`
}

// SuspendedHandlingConfig configures behavior for suspended CronJobs
type SuspendedHandlingConfig struct {
	// PauseMonitoring pauses monitoring when CronJob is suspended (default: true)
	// +optional
	PauseMonitoring *bool `json:"pauseMonitoring,omitempty"`

	// AlertIfSuspendedFor alerts if suspended longer than this duration
	// +optional
	AlertIfSuspendedFor *metav1.Duration `json:"alertIfSuspendedFor,omitempty"`
}

// MaintenanceWindow defines a scheduled maintenance period
type MaintenanceWindow struct {
	// Name identifies this maintenance window
	Name string `json:"name"`

	// Schedule is a cron expression for when window starts
	Schedule string `json:"schedule"`

	// Duration of the maintenance window
	Duration metav1.Duration `json:"duration"`

	// Timezone for the schedule (default: UTC)
	// +optional
	Timezone string `json:"timezone,omitempty"`

	// SuppressAlerts during this window (default: true)
	// +optional
	SuppressAlerts *bool `json:"suppressAlerts,omitempty"`
}

// AlertingConfig configures alerting behavior
type AlertingConfig struct {
	// Enabled turns on alerting (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// ChannelRefs references cluster-scoped AlertChannel CRs
	// +optional
	ChannelRefs []ChannelRef `json:"channelRefs,omitempty"`

	// IncludeContext specifies what context to include in alerts
	// +optional
	IncludeContext *AlertContext `json:"includeContext,omitempty"`

	// SuppressDuplicatesFor prevents re-alerting within this window (default: 1h)
	// +optional
	SuppressDuplicatesFor *metav1.Duration `json:"suppressDuplicatesFor,omitempty"`

	// AlertDelay delays alert dispatch to allow transient issues to resolve.
	// If the issue resolves (e.g., next job succeeds) before the delay expires,
	// the alert is cancelled and never sent. Useful for flaky jobs.
	// Example: "5m" waits 5 minutes before sending failure alerts.
	// +optional
	AlertDelay *metav1.Duration `json:"alertDelay,omitempty"`

	// SeverityOverrides customizes severity for alert types
	// +optional
	SeverityOverrides *SeverityOverrides `json:"severityOverrides,omitempty"`

	// SuggestedFixPatterns defines custom fix patterns for this monitor
	// These are merged with built-in patterns, with custom patterns taking priority
	// +optional
	SuggestedFixPatterns []SuggestedFixPattern `json:"suggestedFixPatterns,omitempty"`
}

// ChannelRef references an AlertChannel CR
type ChannelRef struct {
	// Name of the AlertChannel CR
	Name string `json:"name"`

	// Severities to send to this channel (empty = all)
	// +optional
	Severities []string `json:"severities,omitempty"`
}

// AlertContext specifies what context to include in alerts
type AlertContext struct {
	// Logs includes pod logs (default: true)
	// +optional
	Logs *bool `json:"logs,omitempty"`

	// LogLines is number of log lines to include (default: 50)
	// +optional
	LogLines *int32 `json:"logLines,omitempty"`

	// LogContainerName specifies container for logs (default: first container)
	// +optional
	LogContainerName string `json:"logContainerName,omitempty"`

	// IncludeInitContainerLogs includes init container logs (default: false)
	// +optional
	IncludeInitContainerLogs *bool `json:"includeInitContainerLogs,omitempty"`

	// Events includes Kubernetes events (default: true)
	// +optional
	Events *bool `json:"events,omitempty"`

	// PodStatus includes pod status details (default: true)
	// +optional
	PodStatus *bool `json:"podStatus,omitempty"`

	// SuggestedFixes includes fix suggestions (default: true)
	// +optional
	SuggestedFixes *bool `json:"suggestedFixes,omitempty"`
}

// SeverityOverrides customizes alert severities
// Only critical and warning are valid - alerts are actionable notifications
type SeverityOverrides struct {
	// +kubebuilder:validation:Enum=critical;warning
	// +optional
	MissedSchedule string `json:"missedSchedule,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning
	// +optional
	JobFailed string `json:"jobFailed,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning
	// +optional
	SLABreached string `json:"slaBreached,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning
	// +optional
	DeadManTriggered string `json:"deadManTriggered,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning
	// +optional
	DurationRegression string `json:"durationRegression,omitempty"`
}

// SuggestedFixPattern defines a pattern for suggesting fixes based on failure context
type SuggestedFixPattern struct {
	// Name identifies this pattern (for overriding built-ins like "oom-killed")
	Name string `json:"name"`

	// Match criteria - at least one must be specified
	Match PatternMatch `json:"match"`

	// Suggestion is the fix text (supports Go templates)
	// Available variables: {{.Namespace}}, {{.Name}}, {{.ExitCode}}, {{.Reason}}, {{.JobName}}
	Suggestion string `json:"suggestion"`

	// Priority determines order (higher = checked first, default: 0)
	// Built-in patterns use priorities 1-100, use >100 to override
	// +optional
	Priority *int32 `json:"priority,omitempty"`
}

// PatternMatch defines what to match against for suggested fixes
type PatternMatch struct {
	// ExitCode matches specific exit codes (e.g., 137 for OOM)
	// +optional
	ExitCode *int32 `json:"exitCode,omitempty"`

	// ExitCodeRange matches a range [min, max] inclusive
	// +optional
	ExitCodeRange *ExitCodeRange `json:"exitCodeRange,omitempty"`

	// Reason matches container termination reason (exact match, case-insensitive)
	// +optional
	Reason string `json:"reason,omitempty"`

	// ReasonPattern matches reason using regex
	// +optional
	ReasonPattern string `json:"reasonPattern,omitempty"`

	// LogPattern matches log content using regex
	// +optional
	LogPattern string `json:"logPattern,omitempty"`

	// EventPattern matches event messages using regex
	// +optional
	EventPattern string `json:"eventPattern,omitempty"`
}

// ExitCodeRange defines a range of exit codes [Min, Max] inclusive
type ExitCodeRange struct {
	Min int32 `json:"min"`
	Max int32 `json:"max"`
}

// DataRetentionConfig configures data lifecycle management for this monitor
type DataRetentionConfig struct {
	// RetentionDays overrides global retention for this monitor's execution history
	// If not set, uses global history-retention.default-days setting
	// +optional
	RetentionDays *int32 `json:"retentionDays,omitempty"`

	// OnCronJobDeletion defines behavior when a monitored CronJob is deleted
	// +kubebuilder:validation:Enum=retain;purge;purge-after-days
	// +optional
	OnCronJobDeletion string `json:"onCronJobDeletion,omitempty"`

	// PurgeAfterDays specifies how long to wait before purging data
	// Only used when onCronJobDeletion is "purge-after-days"
	// +optional
	PurgeAfterDays *int32 `json:"purgeAfterDays,omitempty"`

	// OnRecreation defines behavior when a CronJob is recreated (detected via UID change)
	// "retain" keeps old history, "reset" deletes history from the old UID
	// +kubebuilder:validation:Enum=retain;reset
	// +optional
	OnRecreation string `json:"onRecreation,omitempty"`

	// StoreLogs enables storing job logs in the database
	// If nil, uses global --storage.log-storage-enabled setting
	// +optional
	StoreLogs *bool `json:"storeLogs,omitempty"`

	// LogRetentionDays specifies how long to keep stored logs
	// If not set, uses the same value as retentionDays
	// +optional
	LogRetentionDays *int32 `json:"logRetentionDays,omitempty"`

	// MaxLogSizeKB is the maximum log size to store per execution in KB
	// If not set, uses global --storage.max-log-size-kb setting
	// +optional
	MaxLogSizeKB *int32 `json:"maxLogSizeKB,omitempty"`

	// StoreEvents enables storing Kubernetes events in the database
	// If nil, uses global --storage.event-storage-enabled setting
	// +optional
	StoreEvents *bool `json:"storeEvents,omitempty"`
}

// CronJobMonitorStatus defines the observed state of CronJobMonitor
type CronJobMonitorStatus struct {
	// ObservedGeneration is the generation last processed
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase indicates the monitor's operational state
	// +kubebuilder:validation:Enum=Initializing;Active;Degraded;Error
	// +optional
	Phase string `json:"phase,omitempty"`

	// LastReconcileTime is when the controller last reconciled
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// Summary provides aggregate counts
	// +optional
	Summary *MonitorSummary `json:"summary,omitempty"`

	// CronJobs contains per-CronJob status
	// +optional
	CronJobs []CronJobStatus `json:"cronJobs,omitempty"`

	// Conditions represent the latest observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// MonitorSummary provides aggregate counts
type MonitorSummary struct {
	TotalCronJobs int32 `json:"totalCronJobs"`
	Healthy       int32 `json:"healthy"`
	Warning       int32 `json:"warning"`
	Critical      int32 `json:"critical"`
	Suspended     int32 `json:"suspended"`
	Running       int32 `json:"running"`
	ActiveAlerts  int32 `json:"activeAlerts"`
}

// CronJobStatus contains status for a single CronJob
type CronJobStatus struct {
	// Name of the CronJob
	Name string `json:"name"`

	// Namespace of the CronJob
	Namespace string `json:"namespace"`

	// Status indicates health
	// +kubebuilder:validation:Enum=healthy;warning;critical;suspended;unknown
	Status string `json:"status"`

	// Suspended indicates if the CronJob is suspended
	Suspended bool `json:"suspended"`

	// LastSuccessfulTime is when the last Job succeeded
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`

	// LastFailedTime is when the last Job failed
	// +optional
	LastFailedTime *metav1.Time `json:"lastFailedTime,omitempty"`

	// LastRunDuration is the duration of the last completed Job
	// +optional
	LastRunDuration *metav1.Duration `json:"lastRunDuration,omitempty"`

	// NextScheduledTime is when the next Job will be created
	// +optional
	NextScheduledTime *metav1.Time `json:"nextScheduledTime,omitempty"`

	// Metrics contains SLA metrics
	// +optional
	Metrics *CronJobMetrics `json:"metrics,omitempty"`

	// ActiveJobs lists currently running jobs for this CronJob
	// +optional
	ActiveJobs []ActiveJob `json:"activeJobs,omitempty"`

	// ActiveAlerts lists current alerts for this CronJob
	// +optional
	ActiveAlerts []ActiveAlert `json:"activeAlerts,omitempty"`
}

// CronJobMetrics contains SLA metrics for a CronJob
type CronJobMetrics struct {
	SuccessRate    float64 `json:"successRate"`
	TotalRuns      int32   `json:"totalRuns"`
	SuccessfulRuns int32   `json:"successfulRuns"`
	FailedRuns     int32   `json:"failedRuns"`
	// Duration in seconds
	// +optional
	AvgDurationSeconds float64 `json:"avgDurationSeconds,omitempty"`
	// +optional
	P50DurationSeconds float64 `json:"p50DurationSeconds,omitempty"`
	// +optional
	P95DurationSeconds float64 `json:"p95DurationSeconds,omitempty"`
	// +optional
	P99DurationSeconds float64 `json:"p99DurationSeconds,omitempty"`
}

// ActiveAlert represents an active alert
type ActiveAlert struct {
	// Type of alert
	Type string `json:"type"`

	// Severity of alert
	Severity string `json:"severity"`

	// Message describes the alert
	Message string `json:"message"`

	// Since is when the alert became active
	Since metav1.Time `json:"since"`

	// LastNotified is when the alert was last sent
	// +optional
	LastNotified *metav1.Time `json:"lastNotified,omitempty"`

	// ExitCode from the failed container (for JobFailed alerts)
	// +optional
	ExitCode int32 `json:"exitCode,omitempty"`

	// Reason for the failure (e.g., OOMKilled, Error)
	// +optional
	Reason string `json:"reason,omitempty"`

	// SuggestedFix provides actionable guidance for resolving the alert
	// +optional
	SuggestedFix string `json:"suggestedFix,omitempty"`
}

// ActiveJob represents a currently running job
type ActiveJob struct {
	// Name of the Job
	Name string `json:"name"`

	// StartTime is when the Job started
	StartTime metav1.Time `json:"startTime"`

	// RunningDuration is how long the job has been running
	// +optional
	RunningDuration *metav1.Duration `json:"runningDuration,omitempty"`

	// PodPhase is the current phase of the job's pod (Pending, Running, etc.)
	// +optional
	PodPhase string `json:"podPhase,omitempty"`

	// PodName is the name of the pod running the job
	// +optional
	PodName string `json:"podName,omitempty"`

	// Ready indicates how many pods are ready vs total
	// +optional
	Ready string `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CronJobs",type=integer,JSONPath=`.status.summary.totalCronJobs`
// +kubebuilder:printcolumn:name="Healthy",type=integer,JSONPath=`.status.summary.healthy`
// +kubebuilder:printcolumn:name="Warning",type=integer,JSONPath=`.status.summary.warning`
// +kubebuilder:printcolumn:name="Critical",type=integer,JSONPath=`.status.summary.critical`
// +kubebuilder:printcolumn:name="Alerts",type=integer,JSONPath=`.status.summary.activeAlerts`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CronJobMonitor is the Schema for the cronjobmonitors API.
type CronJobMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronJobMonitorSpec   `json:"spec,omitempty"`
	Status CronJobMonitorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CronJobMonitorList contains a list of CronJobMonitor.
type CronJobMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronJobMonitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronJobMonitor{}, &CronJobMonitorList{})
}
