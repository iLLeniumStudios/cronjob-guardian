---
sidebar_position: 1
title: CRD API Reference
description: Auto-generated API reference for CronJob Guardian CRDs
---

# API Reference

## Packages
- [guardian.illenium.net/v1alpha1](#guardianilleniumnetv1alpha1)


## guardian.illenium.net/v1alpha1

Package v1alpha1 contains API Schema definitions for the guardian v1alpha1 API group.

### Resource Types
- [AlertChannel](#alertchannel)
- [CronJobMonitor](#cronjobmonitor)



#### ActiveAlert



ActiveAlert represents an active alert



_Appears in:_
- [CronJobStatus](#cronjobstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type of alert |  |  |
| `severity` _string_ | Severity of alert |  |  |
| `message` _string_ | Message describes the alert |  |  |
| `since` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | Since is when the alert became active |  |  |
| `lastNotified` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastNotified is when the alert was last sent |  |  |
| `exitCode` _integer_ | ExitCode from the failed container (for JobFailed alerts) |  |  |
| `reason` _string_ | Reason for the failure (e.g., OOMKilled, Error) |  |  |
| `suggestedFix` _string_ | SuggestedFix provides actionable guidance for resolving the alert |  |  |


#### ActiveJob



ActiveJob represents a currently running job



_Appears in:_
- [CronJobStatus](#cronjobstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Job |  |  |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | StartTime is when the Job started |  |  |
| `runningDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | RunningDuration is how long the job has been running |  |  |
| `podPhase` _string_ | PodPhase is the current phase of the job's pod (Pending, Running, etc.) |  |  |
| `podName` _string_ | PodName is the name of the pod running the job |  |  |
| `ready` _string_ | Ready indicates how many pods are ready vs total |  |  |


#### AlertChannel



AlertChannel is the Schema for the alertchannels API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `guardian.illenium.net/v1alpha1` | | |
| `kind` _string_ | `AlertChannel` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[AlertChannelSpec](#alertchannelspec)_ |  |  |  |
| `status` _[AlertChannelStatus](#alertchannelstatus)_ |  |  |  |


#### AlertChannelSpec



AlertChannelSpec defines the desired state of AlertChannel



_Appears in:_
- [AlertChannel](#alertchannel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type of alert channel |  | Enum: [slack pagerduty webhook email] <br /> |
| `slack` _[SlackConfig](#slackconfig)_ | Slack configuration |  |  |
| `pagerduty` _[PagerDutyConfig](#pagerdutyconfig)_ | PagerDuty configuration |  |  |
| `webhook` _[WebhookConfig](#webhookconfig)_ | Webhook configuration |  |  |
| `email` _[EmailConfig](#emailconfig)_ | Email configuration |  |  |
| `rateLimiting` _[RateLimitConfig](#ratelimitconfig)_ | RateLimiting prevents alert storms |  |  |
| `testOnSave` _boolean_ | TestOnSave sends a test alert when saved (default: false) |  |  |


#### AlertChannelStatus



AlertChannelStatus defines the observed state of AlertChannel



_Appears in:_
- [AlertChannel](#alertchannel)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ready` _boolean_ | Ready indicates the channel is operational |  |  |
| `lastTestTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastTestTime is when the channel was last tested |  |  |
| `lastTestResult` _string_ | LastTestResult is the result of the last test |  | Enum: [success failed] <br /> |
| `lastTestError` _string_ | LastTestError is the error from the last test |  |  |
| `alertsSentTotal` _integer_ | AlertsSentTotal is total alerts successfully sent via this channel |  |  |
| `lastAlertTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastAlertTime is when the last alert was successfully sent |  |  |
| `alertsFailedTotal` _integer_ | AlertsFailedTotal is total alerts that failed to send via this channel |  |  |
| `lastFailedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastFailedTime is when the last alert failed to send |  |  |
| `lastFailedError` _string_ | LastFailedError is the error message from the last failed send |  |  |
| `consecutiveFailures` _integer_ | ConsecutiveFailures is the number of consecutive failed sends<br />Resets to 0 on successful send |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#condition-v1-meta) array_ | Conditions represent latest observations |  |  |


#### AlertContext



AlertContext specifies what context to include in alerts



_Appears in:_
- [AlertingConfig](#alertingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `logs` _boolean_ | Logs includes pod logs (default: true) |  |  |
| `logLines` _integer_ | LogLines is number of log lines to include (default: 50) |  | Maximum: 10000 <br />Minimum: 1 <br /> |
| `logContainerName` _string_ | LogContainerName specifies container for logs (default: first container) |  |  |
| `includeInitContainerLogs` _boolean_ | IncludeInitContainerLogs includes init container logs (default: false) |  |  |
| `events` _boolean_ | Events includes Kubernetes events (default: true) |  |  |
| `podStatus` _boolean_ | PodStatus includes pod status details (default: true) |  |  |
| `suggestedFixes` _boolean_ | SuggestedFixes includes fix suggestions (default: true) |  |  |


#### AlertingConfig



AlertingConfig configures alerting behavior



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled turns on alerting (default: true) |  |  |
| `channelRefs` _[ChannelRef](#channelref) array_ | ChannelRefs references cluster-scoped AlertChannel CRs |  |  |
| `includeContext` _[AlertContext](#alertcontext)_ | IncludeContext specifies what context to include in alerts |  |  |
| `suppressDuplicatesFor` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | SuppressDuplicatesFor prevents re-alerting within this window (default: 1h) |  |  |
| `alertDelay` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | AlertDelay delays alert dispatch to allow transient issues to resolve.<br />If the issue resolves (e.g., next job succeeds) before the delay expires,<br />the alert is cancelled and never sent. Useful for flaky jobs.<br />Example: "5m" waits 5 minutes before sending failure alerts. |  |  |
| `severityOverrides` _[SeverityOverrides](#severityoverrides)_ | SeverityOverrides customizes severity for alert types |  |  |
| `suggestedFixPatterns` _[SuggestedFixPattern](#suggestedfixpattern) array_ | SuggestedFixPatterns defines custom fix patterns for this monitor<br />These are merged with built-in patterns, with custom patterns taking priority |  |  |


#### AutoScheduleConfig



AutoScheduleConfig configures automatic schedule detection



_Appears in:_
- [DeadManSwitchConfig](#deadmanswitchconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled turns on auto-detection (default: false) |  |  |
| `buffer` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | Buffer adds extra time to expected interval (default: 1h) |  |  |
| `missedScheduleThreshold` _integer_ | MissedScheduleThreshold alerts after this many missed schedules (default: 1) |  | Minimum: 1 <br /> |


#### ChannelRef



ChannelRef references an AlertChannel CR



_Appears in:_
- [AlertingConfig](#alertingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the AlertChannel CR |  |  |
| `severities` _string array_ | Severities to send to this channel (empty = all) |  |  |


#### CronJobMetrics



CronJobMetrics contains SLA metrics for a CronJob



_Appears in:_
- [CronJobStatus](#cronjobstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `successRate` _float_ |  |  |  |
| `totalRuns` _integer_ |  |  |  |
| `successfulRuns` _integer_ |  |  |  |
| `failedRuns` _integer_ |  |  |  |
| `avgDurationSeconds` _float_ | Duration in seconds |  |  |
| `p50DurationSeconds` _float_ |  |  |  |
| `p95DurationSeconds` _float_ |  |  |  |
| `p99DurationSeconds` _float_ |  |  |  |


#### CronJobMonitor



CronJobMonitor is the Schema for the cronjobmonitors API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `guardian.illenium.net/v1alpha1` | | |
| `kind` _string_ | `CronJobMonitor` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CronJobMonitorSpec](#cronjobmonitorspec)_ |  |  |  |
| `status` _[CronJobMonitorStatus](#cronjobmonitorstatus)_ |  |  |  |


#### CronJobMonitorSpec



CronJobMonitorSpec defines the desired state of CronJobMonitor



_Appears in:_
- [CronJobMonitor](#cronjobmonitor)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `selector` _[CronJobSelector](#cronjobselector)_ | Selector specifies which CronJobs to monitor |  |  |
| `deadManSwitch` _[DeadManSwitchConfig](#deadmanswitchconfig)_ | DeadManSwitch configures dead-man's switch alerting |  |  |
| `sla` _[SLAConfig](#slaconfig)_ | SLA configures SLA tracking and alerting |  |  |
| `suspendedHandling` _[SuspendedHandlingConfig](#suspendedhandlingconfig)_ | SuspendedHandling configures behavior for suspended CronJobs |  |  |
| `maintenanceWindows` _[MaintenanceWindow](#maintenancewindow) array_ | MaintenanceWindows defines scheduled maintenance periods |  |  |
| `alerting` _[AlertingConfig](#alertingconfig)_ | Alerting configures alert channels and behavior |  |  |
| `dataRetention` _[DataRetentionConfig](#dataretentionconfig)_ | DataRetention configures data lifecycle management |  |  |


#### CronJobMonitorStatus



CronJobMonitorStatus defines the observed state of CronJobMonitor



_Appears in:_
- [CronJobMonitor](#cronjobmonitor)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the generation last processed |  |  |
| `phase` _string_ | Phase indicates the monitor's operational state |  | Enum: [Initializing Active Degraded Error] <br /> |
| `lastReconcileTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastReconcileTime is when the controller last reconciled |  |  |
| `summary` _[MonitorSummary](#monitorsummary)_ | Summary provides aggregate counts |  |  |
| `cronJobs` _[CronJobStatus](#cronjobstatus) array_ | CronJobs contains per-CronJob status |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#condition-v1-meta) array_ | Conditions represent the latest observations |  |  |


#### CronJobSelector



CronJobSelector specifies which CronJobs to monitor.
An empty selector matches all CronJobs in the monitor's namespace.



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchLabels` _object (keys:string, values:string)_ | MatchLabels selects CronJobs by labels |  |  |
| `matchExpressions` _[LabelSelectorRequirement](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#labelselectorrequirement-v1-meta) array_ | MatchExpressions selects CronJobs by label expressions |  |  |
| `matchNames` _string array_ | MatchNames explicitly lists CronJob names to monitor (only valid when watching a single namespace) |  |  |
| `namespaces` _string array_ | Namespaces explicitly lists namespaces to watch for CronJobs.<br />If empty and namespaceSelector is not set, watches only the monitor's namespace. |  |  |
| `namespaceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#labelselector-v1-meta)_ | NamespaceSelector selects namespaces by labels.<br />CronJobs in matching namespaces will be monitored. |  |  |
| `allNamespaces` _boolean_ | AllNamespaces watches CronJobs in all namespaces (except globally ignored ones).<br />Takes precedence over namespaces and namespaceSelector. |  |  |


#### CronJobStatus



CronJobStatus contains status for a single CronJob



_Appears in:_
- [CronJobMonitorStatus](#cronjobmonitorstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the CronJob |  |  |
| `namespace` _string_ | Namespace of the CronJob |  |  |
| `status` _string_ | Status indicates health |  | Enum: [healthy warning critical suspended unknown] <br /> |
| `suspended` _boolean_ | Suspended indicates if the CronJob is suspended |  |  |
| `lastSuccessfulTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastSuccessfulTime is when the last Job succeeded |  |  |
| `lastFailedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | LastFailedTime is when the last Job failed |  |  |
| `lastRunDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | LastRunDuration is the duration of the last completed Job |  |  |
| `nextScheduledTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#time-v1-meta)_ | NextScheduledTime is when the next Job will be created |  |  |
| `metrics` _[CronJobMetrics](#cronjobmetrics)_ | Metrics contains SLA metrics |  |  |
| `activeJobs` _[ActiveJob](#activejob) array_ | ActiveJobs lists currently running jobs for this CronJob |  |  |
| `activeAlerts` _[ActiveAlert](#activealert) array_ | ActiveAlerts lists current alerts for this CronJob |  |  |


#### DataRetentionConfig



DataRetentionConfig configures data lifecycle management for this monitor



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `retentionDays` _integer_ | RetentionDays overrides global retention for this monitor's execution history<br />If not set, uses global history-retention.default-days setting |  | Minimum: 1 <br /> |
| `onCronJobDeletion` _string_ | OnCronJobDeletion defines behavior when a monitored CronJob is deleted |  | Enum: [retain purge purge-after-days] <br /> |
| `purgeAfterDays` _integer_ | PurgeAfterDays specifies how long to wait before purging data<br />Only used when onCronJobDeletion is "purge-after-days" |  | Minimum: 0 <br /> |
| `onRecreation` _string_ | OnRecreation defines behavior when a CronJob is recreated (detected via UID change)<br />"retain" keeps old history, "reset" deletes history from the old UID |  | Enum: [retain reset] <br /> |
| `storeLogs` _boolean_ | StoreLogs enables storing job logs in the database<br />If nil, uses global --storage.log-storage-enabled setting |  |  |
| `logRetentionDays` _integer_ | LogRetentionDays specifies how long to keep stored logs<br />If not set, uses the same value as retentionDays |  | Minimum: 1 <br /> |
| `maxLogSizeKB` _integer_ | MaxLogSizeKB is the maximum log size to store per execution in KB<br />If not set, uses global --storage.max-log-size-kb setting |  | Minimum: 1 <br /> |
| `storeEvents` _boolean_ | StoreEvents enables storing Kubernetes events in the database<br />If nil, uses global --storage.event-storage-enabled setting |  |  |


#### DeadManSwitchConfig



DeadManSwitchConfig configures dead-man's switch behavior



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled turns on dead-man's switch monitoring (default: true) |  |  |
| `maxTimeSinceLastSuccess` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MaxTimeSinceLastSuccess alerts if no success within this duration<br />Example: "25h" for daily jobs with 1h buffer |  |  |
| `autoFromSchedule` _[AutoScheduleConfig](#autoscheduleconfig)_ | AutoFromSchedule auto-calculates expected interval from cron schedule |  |  |


#### EmailConfig



EmailConfig configures email notifications



_Appears in:_
- [AlertChannelSpec](#alertchannelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `smtpSecretRef` _[NamespacedSecretRef](#namespacedsecretref)_ | SMTPSecretRef references Secret with host, port, username, password |  |  |
| `from` _string_ | From is the sender address |  |  |
| `to` _string array_ | To is the list of recipient addresses |  |  |
| `subjectTemplate` _string_ | SubjectTemplate is a Go template for subject |  |  |
| `bodyTemplate` _string_ | BodyTemplate is a Go template for body |  |  |


#### ExitCodeRange



ExitCodeRange defines a range of exit codes [Min, Max] inclusive



_Appears in:_
- [PatternMatch](#patternmatch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `min` _integer_ |  |  |  |
| `max` _integer_ |  |  |  |


#### MaintenanceWindow



MaintenanceWindow defines a scheduled maintenance period



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name identifies this maintenance window |  |  |
| `schedule` _string_ | Schedule is a cron expression for when window starts |  |  |
| `duration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | Duration of the maintenance window |  |  |
| `timezone` _string_ | Timezone for the schedule (default: UTC) |  |  |
| `suppressAlerts` _boolean_ | SuppressAlerts during this window (default: true) |  |  |


#### MonitorSummary



MonitorSummary provides aggregate counts



_Appears in:_
- [CronJobMonitorStatus](#cronjobmonitorstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `totalCronJobs` _integer_ |  |  |  |
| `healthy` _integer_ |  |  |  |
| `warning` _integer_ |  |  |  |
| `critical` _integer_ |  |  |  |
| `suspended` _integer_ |  |  |  |
| `running` _integer_ |  |  |  |
| `activeAlerts` _integer_ |  |  |  |


#### NamespacedSecretKeyRef



NamespacedSecretKeyRef references a key in a namespaced Secret



_Appears in:_
- [PagerDutyConfig](#pagerdutyconfig)
- [SlackConfig](#slackconfig)
- [WebhookConfig](#webhookconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `namespace` _string_ |  |  |  |
| `key` _string_ |  |  |  |


#### NamespacedSecretRef



NamespacedSecretRef references a namespaced Secret



_Appears in:_
- [EmailConfig](#emailconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `namespace` _string_ |  |  |  |


#### PagerDutyConfig



PagerDutyConfig configures PagerDuty notifications



_Appears in:_
- [AlertChannelSpec](#alertchannelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `routingKeySecretRef` _[NamespacedSecretKeyRef](#namespacedsecretkeyref)_ | RoutingKeySecretRef references the Secret containing routing key |  |  |
| `severity` _string_ | Severity is the default PagerDuty severity |  | Enum: [critical error warning info] <br /> |


#### PatternMatch



PatternMatch defines what to match against for suggested fixes



_Appears in:_
- [SuggestedFixPattern](#suggestedfixpattern)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `exitCode` _integer_ | ExitCode matches specific exit codes (e.g., 137 for OOM) |  |  |
| `exitCodeRange` _[ExitCodeRange](#exitcoderange)_ | ExitCodeRange matches a range [min, max] inclusive |  |  |
| `reason` _string_ | Reason matches container termination reason (exact match, case-insensitive) |  |  |
| `reasonPattern` _string_ | ReasonPattern matches reason using regex |  |  |
| `logPattern` _string_ | LogPattern matches log content using regex |  |  |
| `eventPattern` _string_ | EventPattern matches event messages using regex |  |  |


#### RateLimitConfig



RateLimitConfig configures rate limiting



_Appears in:_
- [AlertChannelSpec](#alertchannelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxAlertsPerHour` _integer_ | MaxAlertsPerHour limits alerts per hour (default: 100) |  | Minimum: 1 <br /> |
| `burstLimit` _integer_ | BurstLimit limits alerts per minute (default: 10) |  | Minimum: 1 <br /> |


#### SLAConfig



SLAConfig configures SLA tracking



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled turns on SLA tracking (default: true) |  |  |
| `minSuccessRate` _float_ | MinSuccessRate is minimum acceptable success rate percentage (default: 95) |  | Maximum: 100 <br />Minimum: 0 <br /> |
| `windowDays` _integer_ | WindowDays is the rolling window for success rate calculation (default: 7) |  | Minimum: 1 <br /> |
| `maxDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | MaxDuration alerts if job exceeds this duration |  |  |
| `durationRegressionThreshold` _integer_ | DurationRegressionThreshold alerts if P95 increases by this percentage (default: 50) |  | Maximum: 1000 <br />Minimum: 1 <br /> |
| `durationBaselineWindowDays` _integer_ | DurationBaselineWindowDays for baseline calculation (default: 14) |  | Minimum: 1 <br /> |


#### SeverityOverrides



SeverityOverrides customizes alert severities
Only critical and warning are valid - alerts are actionable notifications



_Appears in:_
- [AlertingConfig](#alertingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `missedSchedule` _string_ |  |  | Enum: [critical warning] <br /> |
| `jobFailed` _string_ |  |  | Enum: [critical warning] <br /> |
| `slaBreached` _string_ |  |  | Enum: [critical warning] <br /> |
| `deadManTriggered` _string_ |  |  | Enum: [critical warning] <br /> |
| `durationRegression` _string_ |  |  | Enum: [critical warning] <br /> |


#### SlackConfig



SlackConfig configures Slack notifications



_Appears in:_
- [AlertChannelSpec](#alertchannelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `webhookSecretRef` _[NamespacedSecretKeyRef](#namespacedsecretkeyref)_ | WebhookSecretRef references the Secret containing webhook URL |  |  |
| `defaultChannel` _string_ | DefaultChannel overrides webhook's default channel |  |  |
| `messageTemplate` _string_ | MessageTemplate is a Go template for message formatting |  |  |


#### SuggestedFixPattern



SuggestedFixPattern defines a pattern for suggesting fixes based on failure context



_Appears in:_
- [AlertingConfig](#alertingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name identifies this pattern (for overriding built-ins like "oom-killed") |  |  |
| `match` _[PatternMatch](#patternmatch)_ | Match criteria - at least one must be specified |  |  |
| `suggestion` _string_ | Suggestion is the fix text (supports Go templates)<br />Available variables: \{\{.Namespace\}\}, \{\{.Name\}\}, \{\{.ExitCode\}\}, \{\{.Reason\}\}, \{\{.JobName\}\} |  |  |
| `priority` _integer_ | Priority determines order (higher = checked first, default: 0)<br />Built-in patterns use priorities 1-100, use >100 to override |  |  |


#### SuspendedHandlingConfig



SuspendedHandlingConfig configures behavior for suspended CronJobs



_Appears in:_
- [CronJobMonitorSpec](#cronjobmonitorspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pauseMonitoring` _boolean_ | PauseMonitoring pauses monitoring when CronJob is suspended (default: true) |  |  |
| `alertIfSuspendedFor` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.29/#duration-v1-meta)_ | AlertIfSuspendedFor alerts if suspended longer than this duration |  |  |


#### WebhookConfig



WebhookConfig configures generic webhook notifications



_Appears in:_
- [AlertChannelSpec](#alertchannelspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `urlSecretRef` _[NamespacedSecretKeyRef](#namespacedsecretkeyref)_ | URLSecretRef references the Secret containing webhook URL |  |  |
| `method` _string_ | Method is the HTTP method (default: POST) |  | Enum: [POST PUT] <br /> |
| `headers` _object (keys:string, values:string)_ | Headers to include in requests |  |  |
| `payloadTemplate` _string_ | PayloadTemplate is a Go template for JSON payload |  |  |


