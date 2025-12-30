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

	// Dependencies lists upstream CronJobs this depends on
	// +optional
	Dependencies []DependencyConfig `json:"dependencies,omitempty"`

	// SuspendedHandling configures behavior for suspended CronJobs
	// +optional
	SuspendedHandling *SuspendedHandlingConfig `json:"suspendedHandling,omitempty"`

	// MaintenanceWindows defines scheduled maintenance periods
	// +optional
	MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows,omitempty"`

	// Remediation configures auto-remediation actions
	// +optional
	Remediation *RemediationConfig `json:"remediation,omitempty"`

	// Alerting configures alert channels and behavior
	// +optional
	Alerting *AlertingConfig `json:"alerting,omitempty"`

	// Timezone for schedule interpretation (default: UTC)
	// +optional
	Timezone string `json:"timezone,omitempty"`
}

// CronJobSelector specifies which CronJobs to monitor
type CronJobSelector struct {
	// MatchLabels selects CronJobs by labels
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchExpressions selects CronJobs by label expressions
	// +optional
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`

	// MatchNames explicitly lists CronJob names to monitor
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`
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

	// DurationPercentiles to track (default: [50, 95, 99])
	// +optional
	DurationPercentiles []int32 `json:"durationPercentiles,omitempty"`

	// DurationRegressionThreshold alerts if P95 increases by this percentage (default: 50)
	// +optional
	DurationRegressionThreshold *int32 `json:"durationRegressionThreshold,omitempty"`

	// DurationBaselineWindowDays for baseline calculation (default: 14)
	// +optional
	DurationBaselineWindowDays *int32 `json:"durationBaselineWindowDays,omitempty"`
}

// DependencyConfig defines an upstream CronJob dependency
type DependencyConfig struct {
	// Name of the upstream CronJob
	Name string `json:"name"`

	// Namespace of the upstream CronJob (defaults to same namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// AlertOnFailure sends alert when upstream fails (default: true)
	// +optional
	AlertOnFailure *bool `json:"alertOnFailure,omitempty"`

	// SuppressDownstreamAlerts suppresses alerts for this job if upstream failed (default: true)
	// +optional
	SuppressDownstreamAlerts *bool `json:"suppressDownstreamAlerts,omitempty"`
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

	// SuppressRemediation during this window (default: true)
	// +optional
	SuppressRemediation *bool `json:"suppressRemediation,omitempty"`
}

// RemediationConfig configures auto-remediation
type RemediationConfig struct {
	// Enabled turns on remediation (default: false)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// DryRun logs actions without executing (default: false)
	// +optional
	DryRun *bool `json:"dryRun,omitempty"`

	// KillStuckJobs configures stuck job termination
	// +optional
	KillStuckJobs *KillStuckJobsConfig `json:"killStuckJobs,omitempty"`

	// AutoRetry configures automatic job retry
	// +optional
	AutoRetry *AutoRetryConfig `json:"autoRetry,omitempty"`
}

// KillStuckJobsConfig configures stuck job termination
type KillStuckJobsConfig struct {
	// Enabled turns on stuck job killing (default: false)
	Enabled bool `json:"enabled"`

	// AfterDuration kills jobs running longer than this
	AfterDuration metav1.Duration `json:"afterDuration"`

	// DeletePolicy specifies how to handle the job (default: Delete)
	// +kubebuilder:validation:Enum=Delete;Orphan
	// +optional
	DeletePolicy string `json:"deletePolicy,omitempty"`
}

// AutoRetryConfig configures automatic job retry
type AutoRetryConfig struct {
	// Enabled turns on auto-retry (default: false)
	Enabled bool `json:"enabled"`

	// MaxRetries is maximum retry attempts (default: 2)
	// +optional
	MaxRetries *int32 `json:"maxRetries,omitempty"`

	// DelayBetweenRetries is wait time between retries (default: 5m)
	// +optional
	DelayBetweenRetries *metav1.Duration `json:"delayBetweenRetries,omitempty"`

	// Behavior specifies retry strategy (default: CreateNewJob)
	// +kubebuilder:validation:Enum=CreateNewJob;WaitForBackoff
	// +optional
	Behavior string `json:"behavior,omitempty"`

	// OnlyForExitCodes limits retries to specific exit codes (empty = all failures)
	// +optional
	OnlyForExitCodes []int32 `json:"onlyForExitCodes,omitempty"`
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

	// GroupingKey specifies how to group alerts (default: perCronJob)
	// +kubebuilder:validation:Enum=perCronJob;perMonitor;perNamespace
	// +optional
	GroupingKey string `json:"groupingKey,omitempty"`

	// SeverityOverrides customizes severity for alert types
	// +optional
	SeverityOverrides *SeverityOverrides `json:"severityOverrides,omitempty"`
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
type SeverityOverrides struct {
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	MissedSchedule string `json:"missedSchedule,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	JobFailed string `json:"jobFailed,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	SLABreached string `json:"slaBreached,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	DeadManTriggered string `json:"deadManTriggered,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	DurationRegression string `json:"durationRegression,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	StuckJob string `json:"stuckJob,omitempty"`
	// +kubebuilder:validation:Enum=critical;warning;info
	// +optional
	DependencyFailed string `json:"dependencyFailed,omitempty"`
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

	// LastScheduledTime is when the CronJob last created a Job
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

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

	// ActiveAlerts lists current alerts for this CronJob
	// +optional
	ActiveAlerts []ActiveAlert `json:"activeAlerts,omitempty"`

	// LastRemediation describes the most recent remediation action
	// +optional
	LastRemediation *RemediationStatus `json:"lastRemediation,omitempty"`
}

// CronJobMetrics contains SLA metrics for a CronJob
type CronJobMetrics struct {
	SuccessRate    float64 `json:"successRate"`
	WindowDays     int32   `json:"windowDays"`
	TotalRuns      int32   `json:"totalRuns"`
	SuccessfulRuns int32   `json:"successfulRuns"`
	FailedRuns     int32   `json:"failedRuns"`
	MissedRuns     int32   `json:"missedRuns"`
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
}

// RemediationStatus describes a remediation action
type RemediationStatus struct {
	Action  string      `json:"action"`
	Time    metav1.Time `json:"time"`
	Result  string      `json:"result"`
	Message string      `json:"message,omitempty"`
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
