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

// GuardianConfigSpec defines the desired state of GuardianConfig
type GuardianConfigSpec struct {
	// DeadManSwitchInterval is how often to check dead-man's switches (default: 1m)
	// +optional
	DeadManSwitchInterval *metav1.Duration `json:"deadManSwitchInterval,omitempty"`

	// SLARecalculationInterval is how often to recalculate SLA metrics (default: 5m)
	// +optional
	SLARecalculationInterval *metav1.Duration `json:"slaRecalculationInterval,omitempty"`

	// HistoryRetention configures execution history retention
	// +optional
	HistoryRetention *HistoryRetentionConfig `json:"historyRetention,omitempty"`

	// Storage configures the metrics storage backend
	// +optional
	Storage *StorageConfig `json:"storage,omitempty"`

	// MetricsExport configures Prometheus metrics
	// +optional
	MetricsExport *MetricsExportConfig `json:"metricsExport,omitempty"`

	// GlobalRateLimits prevents alert/remediation storms
	// +optional
	GlobalRateLimits *GlobalRateLimitsConfig `json:"globalRateLimits,omitempty"`

	// IgnoredNamespaces lists namespaces to skip
	// +optional
	IgnoredNamespaces []string `json:"ignoredNamespaces,omitempty"`

	// LeaderElection configures leader election for HA
	// +optional
	LeaderElection *LeaderElectionConfig `json:"leaderElection,omitempty"`

	// UI configures the embedded web UI
	// +optional
	UI *UIConfig `json:"ui,omitempty"`
}

// HistoryRetentionConfig configures retention
type HistoryRetentionConfig struct {
	// DefaultDays is default retention period (default: 30)
	// +optional
	DefaultDays *int32 `json:"defaultDays,omitempty"`

	// MaxDays is maximum allowed retention (default: 90)
	// +optional
	MaxDays *int32 `json:"maxDays,omitempty"`
}

// StorageConfig configures the storage backend
type StorageConfig struct {
	// Type is the storage backend type (default: sqlite)
	// +kubebuilder:validation:Enum=sqlite;postgres;mysql
	// +optional
	Type string `json:"type,omitempty"`

	// SQLite configuration (used when type=sqlite)
	// +optional
	SQLite *SQLiteConfig `json:"sqlite,omitempty"`

	// PostgreSQL configuration (used when type=postgres)
	// +optional
	PostgreSQL *PostgreSQLConfig `json:"postgres,omitempty"`

	// MySQL configuration (used when type=mysql)
	// +optional
	MySQL *MySQLConfig `json:"mysql,omitempty"`
}

// SQLiteConfig configures SQLite storage
type SQLiteConfig struct {
	// Path to database file (default: /data/guardian.db)
	// +optional
	Path string `json:"path,omitempty"`
}

// PostgreSQLConfig configures PostgreSQL storage
type PostgreSQLConfig struct {
	// Host is the database host
	Host string `json:"host"`

	// Port is the database port (default: 5432)
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Database name
	Database string `json:"database"`

	// CredentialsSecretRef references Secret with username and password keys
	CredentialsSecretRef NamespacedSecretRef `json:"credentialsSecretRef"`

	// SSLMode for connection (default: require)
	// +optional
	SSLMode string `json:"sslMode,omitempty"`
}

// MySQLConfig configures MySQL/MariaDB storage
type MySQLConfig struct {
	// Host is the database host
	Host string `json:"host"`

	// Port is the database port (default: 3306)
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Database name
	Database string `json:"database"`

	// CredentialsSecretRef references Secret with username and password keys
	CredentialsSecretRef NamespacedSecretRef `json:"credentialsSecretRef"`
}

// MetricsExportConfig configures Prometheus metrics
type MetricsExportConfig struct {
	// Enabled turns on metrics export (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port for metrics endpoint (default: 8080)
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Path for metrics endpoint (default: /metrics)
	// +optional
	Path string `json:"path,omitempty"`
}

// GlobalRateLimitsConfig configures global rate limits
type GlobalRateLimitsConfig struct {
	// MaxAlertsPerMinute across all channels (default: 50)
	// +optional
	MaxAlertsPerMinute *int32 `json:"maxAlertsPerMinute,omitempty"`

	// MaxRemediationsPerHour across all monitors (default: 100)
	// +optional
	MaxRemediationsPerHour *int32 `json:"maxRemediationsPerHour,omitempty"`
}

// LeaderElectionConfig configures leader election
type LeaderElectionConfig struct {
	// Enabled turns on leader election (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// LeaseDuration is the leader lease duration (default: 15s)
	// +optional
	LeaseDuration *metav1.Duration `json:"leaseDuration,omitempty"`

	// RenewDeadline is the leader renew deadline (default: 10s)
	// +optional
	RenewDeadline *metav1.Duration `json:"renewDeadline,omitempty"`

	// RetryPeriod is the leader retry period (default: 2s)
	// +optional
	RetryPeriod *metav1.Duration `json:"retryPeriod,omitempty"`
}

// UIConfig configures the embedded web UI
type UIConfig struct {
	// Enabled turns on the web UI (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Port for UI server (default: 8081)
	// +optional
	Port *int32 `json:"port,omitempty"`
}

// GuardianConfigStatus defines the observed state of GuardianConfig
type GuardianConfigStatus struct {
	// ActiveLeader is the current leader pod
	// +optional
	ActiveLeader string `json:"activeLeader,omitempty"`

	// TotalMonitors is count of CronJobMonitor CRs
	TotalMonitors int32 `json:"totalMonitors"`

	// TotalCronJobsWatched is count of CronJobs being monitored
	TotalCronJobsWatched int32 `json:"totalCronJobsWatched"`

	// TotalAlertsSent24h is alerts sent in last 24 hours
	TotalAlertsSent24h int32 `json:"totalAlertsSent24h"`

	// TotalRemediations24h is remediations in last 24 hours
	TotalRemediations24h int32 `json:"totalRemediations24h"`

	// StorageStatus indicates storage backend health
	// +optional
	StorageStatus string `json:"storageStatus,omitempty"`

	// LastReconcileTime is when the controller last reconciled
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// Conditions represent latest observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Monitors",type=integer,JSONPath=`.status.totalMonitors`
// +kubebuilder:printcolumn:name="CronJobs",type=integer,JSONPath=`.status.totalCronJobsWatched`
// +kubebuilder:printcolumn:name="Storage",type=string,JSONPath=`.status.storageStatus`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GuardianConfig is the Schema for the guardianconfigs API.
type GuardianConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GuardianConfigSpec   `json:"spec,omitempty"`
	Status GuardianConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GuardianConfigList contains a list of GuardianConfig.
type GuardianConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GuardianConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GuardianConfig{}, &GuardianConfigList{})
}
