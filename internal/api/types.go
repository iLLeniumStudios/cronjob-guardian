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
	"time"

	"k8s.io/apimachinery/pkg/types"
)

// HealthResponse is the response for GET /api/v1/health
type HealthResponse struct {
	Status  string `json:"status"`
	Storage string `json:"storage"`
	Leader  bool   `json:"leader"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

// StatsResponse is the response for GET /api/v1/stats
type StatsResponse struct {
	TotalMonitors         int32          `json:"totalMonitors"`
	TotalCronJobs         int32          `json:"totalCronJobs"`
	Summary               SummaryStats   `json:"summary"`
	ActiveAlerts          int32          `json:"activeAlerts"`
	AlertsSent24h         int32          `json:"alertsSent24h"`
	Remediations24h       int32          `json:"remediations24h"`
	ExecutionsRecorded24h int64          `json:"executionsRecorded24h"`
}

// SummaryStats contains aggregate status counts
type SummaryStats struct {
	Healthy   int32 `json:"healthy"`
	Warning   int32 `json:"warning"`
	Critical  int32 `json:"critical"`
	Suspended int32 `json:"suspended"`
}

// MonitorListResponse is the response for GET /api/v1/monitors
type MonitorListResponse struct {
	Items []MonitorListItem `json:"items"`
}

// MonitorListItem is a single monitor in the list
type MonitorListItem struct {
	Name          string       `json:"name"`
	Namespace     string       `json:"namespace"`
	CronJobCount  int32        `json:"cronJobCount"`
	Summary       SummaryStats `json:"summary"`
	ActiveAlerts  int32        `json:"activeAlerts"`
	LastReconcile *time.Time   `json:"lastReconcile,omitempty"`
	Phase         string       `json:"phase"`
}

// CronJobListResponse is the response for GET /api/v1/cronjobs
type CronJobListResponse struct {
	Items   []CronJobListItem `json:"items"`
	Summary SummaryStats      `json:"summary"`
}

// CronJobListItem is a single CronJob in the list
type CronJobListItem struct {
	Name            string              `json:"name"`
	Namespace       string              `json:"namespace"`
	Status          string              `json:"status"`
	Schedule        string              `json:"schedule"`
	Timezone        string              `json:"timezone,omitempty"`
	Suspended       bool                `json:"suspended"`
	SuccessRate     float64             `json:"successRate"`
	LastSuccess     *time.Time          `json:"lastSuccess,omitempty"`
	LastRunDuration string              `json:"lastRunDuration,omitempty"`
	NextRun         *time.Time          `json:"nextRun,omitempty"`
	ActiveAlerts    int                 `json:"activeAlerts"`
	MonitorRef      *types.NamespacedName `json:"monitorRef,omitempty"`
}

// CronJobDetailResponse is the response for GET /api/v1/cronjobs/:namespace/:name
type CronJobDetailResponse struct {
	Name            string                `json:"name"`
	Namespace       string                `json:"namespace"`
	Status          string                `json:"status"`
	Schedule        string                `json:"schedule"`
	Timezone        string                `json:"timezone,omitempty"`
	Suspended       bool                  `json:"suspended"`
	MonitorRef      *types.NamespacedName `json:"monitorRef,omitempty"`
	Metrics         *CronJobMetrics       `json:"metrics,omitempty"`
	LastExecution   *ExecutionSummary     `json:"lastExecution,omitempty"`
	NextRun         *time.Time            `json:"nextRun,omitempty"`
	ActiveAlerts    []AlertItem           `json:"activeAlerts"`
	LastRemediation *RemediationItem      `json:"lastRemediation,omitempty"`
}

// CronJobMetrics contains SLA metrics
type CronJobMetrics struct {
	SuccessRate7d      float64 `json:"successRate7d"`
	SuccessRate30d     float64 `json:"successRate30d"`
	TotalRuns7d        int32   `json:"totalRuns7d"`
	SuccessfulRuns7d   int32   `json:"successfulRuns7d"`
	FailedRuns7d       int32   `json:"failedRuns7d"`
	AvgDurationSeconds float64 `json:"avgDurationSeconds"`
	P50DurationSeconds float64 `json:"p50DurationSeconds"`
	P95DurationSeconds float64 `json:"p95DurationSeconds"`
	P99DurationSeconds float64 `json:"p99DurationSeconds"`
}

// ExecutionSummary contains execution details
type ExecutionSummary struct {
	JobName        string     `json:"jobName"`
	Status         string     `json:"status"`
	StartTime      time.Time  `json:"startTime"`
	CompletionTime *time.Time `json:"completionTime,omitempty"`
	Duration       string     `json:"duration"`
	ExitCode       int32      `json:"exitCode"`
}

// ExecutionListResponse is the response for GET /api/v1/cronjobs/:namespace/:name/executions
type ExecutionListResponse struct {
	Items      []ExecutionItem `json:"items"`
	Pagination Pagination      `json:"pagination"`
}

// ExecutionItem is a single execution in the list
type ExecutionItem struct {
	ID             int64      `json:"id"`
	JobName        string     `json:"jobName"`
	Status         string     `json:"status"`
	StartTime      time.Time  `json:"startTime"`
	CompletionTime *time.Time `json:"completionTime,omitempty"`
	Duration       string     `json:"duration"`
	ExitCode       int32      `json:"exitCode"`
	Reason         string     `json:"reason,omitempty"`
	IsRetry        bool       `json:"isRetry"`
}

// Pagination contains pagination info
type Pagination struct {
	Total   int64 `json:"total"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	HasMore bool  `json:"hasMore"`
}

// LogsResponse is the response for GET /api/v1/cronjobs/:namespace/:name/executions/:jobName/logs
type LogsResponse struct {
	JobName   string `json:"jobName"`
	Container string `json:"container"`
	Logs      string `json:"logs"`
	Truncated bool   `json:"truncated"`
}

// AlertListResponse is the response for GET /api/v1/alerts
type AlertListResponse struct {
	Items      []AlertItem     `json:"items"`
	Total      int             `json:"total"`
	BySeverity map[string]int  `json:"bySeverity"`
}

// AlertItem is a single alert
type AlertItem struct {
	ID           string                `json:"id"`
	Type         string                `json:"type"`
	Severity     string                `json:"severity"`
	Title        string                `json:"title"`
	Message      string                `json:"message"`
	CronJob      *types.NamespacedName `json:"cronjob,omitempty"`
	Monitor      *types.NamespacedName `json:"monitor,omitempty"`
	Since        time.Time             `json:"since"`
	LastNotified *time.Time            `json:"lastNotified,omitempty"`
}

// AlertHistoryResponse is the response for GET /api/v1/alerts/history
type AlertHistoryResponse struct {
	Items      []AlertHistoryItem `json:"items"`
	Pagination Pagination         `json:"pagination"`
}

// AlertHistoryItem is a single historical alert
type AlertHistoryItem struct {
	ID               string                `json:"id"`
	Type             string                `json:"type"`
	Severity         string                `json:"severity"`
	Title            string                `json:"title"`
	Message          string                `json:"message"`
	CronJob          *types.NamespacedName `json:"cronjob,omitempty"`
	OccurredAt       time.Time             `json:"occurredAt"`
	ResolvedAt       *time.Time            `json:"resolvedAt,omitempty"`
	ChannelsNotified []string              `json:"channelsNotified"`
}

// ChannelListResponse is the response for GET /api/v1/channels
type ChannelListResponse struct {
	Items   []ChannelListItem `json:"items"`
	Summary ChannelSummary    `json:"summary"`
}

// ChannelListItem is a single channel in the list
type ChannelListItem struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Ready    bool           `json:"ready"`
	Config   map[string]any `json:"config,omitempty"`
	Stats    ChannelStats   `json:"stats"`
	LastTest *TestResult    `json:"lastTest,omitempty"`
}

// ChannelStats contains channel statistics
type ChannelStats struct {
	AlertsSent24h   int32 `json:"alertsSent24h"`
	AlertsSentTotal int64 `json:"alertsSentTotal"`
}

// TestResult contains test results
type TestResult struct {
	Time   time.Time `json:"time"`
	Result string    `json:"result"`
}

// ChannelSummary contains channel summary
type ChannelSummary struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	NotReady int `json:"notReady"`
}

// RemediationItem describes a remediation action
type RemediationItem struct {
	Action  string    `json:"action"`
	Time    time.Time `json:"time"`
	Result  string    `json:"result"`
	Message string    `json:"message,omitempty"`
}

// TriggerResponse is the response for POST /api/v1/cronjobs/:namespace/:name/trigger
type TriggerResponse struct {
	Success bool   `json:"success"`
	JobName string `json:"jobName,omitempty"`
	Message string `json:"message"`
}

// SimpleResponse is a simple success/error response
type SimpleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}
