---
sidebar_position: 1
title: Monitor Examples
description: CronJobMonitor configuration examples
---

# CronJobMonitor Examples

Collection of CronJobMonitor configurations for common scenarios.

## Basic Monitor

Monitor all CronJobs in a namespace:

```yaml title="basic.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: all-jobs
  namespace: production
spec:
  selector: {}

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 2

  alerting:
    channelRefs:
      - name: team-slack
```

## Critical Tier Monitor

Strict monitoring for critical jobs:

```yaml title="critical-tier.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: critical-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      tier: critical

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1
      buffer: 10m

  sla:
    minSuccessRate: 99.9
    windowDays: 30
    maxDuration: 30m
    durationRegressionThreshold: 25
    durationBaselineWindowDays: 14

  alerting:
    channelRefs:
      - name: pagerduty-critical
        severities:
          - critical
      - name: ops-slack

    alertDelay: 0s
    suppressDuplicatesFor: 15m

    severityOverrides:
      jobFailed: critical
      deadManTriggered: critical
      slaBreached: critical

    includeContext:
      logs: true
      events: true
      podStatus: true
      logLines: 100
```

## Database Backup Monitor

Monitor database backup jobs with strict SLA:

```yaml title="database-backups.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: database-backups
  namespace: production
spec:
  selector:
    matchLabels:
      app: postgres-backup

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1
      buffer: 30m

  sla:
    minSuccessRate: 100
    windowDays: 7
    maxDuration: 2h

  maintenanceWindows:
    - name: sunday-maintenance
      schedule: "0 10 * * 0"     # Sunday 10 AM
      duration: 2h
      timezone: America/New_York

  alerting:
    channelRefs:
      - name: dba-pagerduty
        severities:
          - critical
      - name: dba-slack

    severityOverrides:
      jobFailed: critical
      deadManTriggered: critical

    suggestedFixPatterns:
      - name: disk-space
        match:
          logPattern: "no space left on device"
        suggestion: |
          Backup volume is full. Check PVC usage:
          kubectl exec -n {{.Namespace}} deploy/postgres -- df -h /backups
        priority: 150

  dataRetention:
    retentionDays: 365
    storeLogs: true
    logRetentionDays: 90
```

## ETL Pipeline Monitor

Monitor data pipelines with duration tracking:

```yaml title="data-pipeline.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: etl-pipeline
  namespace: data
spec:
  selector:
    matchLabels:
      pipeline: etl

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 2

  sla:
    minSuccessRate: 98
    windowDays: 14
    maxDuration: 45m
    durationRegressionThreshold: 30
    durationBaselineWindowDays: 7

  alerting:
    channelRefs:
      - name: data-team-slack
      - name: ops-pagerduty
        severities:
          - critical

    alertDelay: 5m
    suppressDuplicatesFor: 1h

    severityOverrides:
      jobFailed: warning
      deadManTriggered: critical
      slaBreached: warning
      durationRegression: warning

    suggestedFixPatterns:
      - name: source-unavailable
        match:
          logPattern: "connection refused.*source-db|ECONNREFUSED.*5432"
        suggestion: "Source database unavailable. Check source DB health."
        priority: 150
      - name: destination-full
        match:
          logPattern: "disk quota exceeded|ENOSPC"
        suggestion: "Destination storage full. Increase PVC or clean up old data."
        priority: 145
```

## Multi-Namespace Monitor

Watch CronJobs across multiple namespaces:

```yaml title="multi-namespace.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: all-production
  namespace: cronjob-guardian
spec:
  namespaces:
    - production
    - staging
    - batch

  selector:
    matchLabels:
      monitored: "true"

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true

  sla:
    minSuccessRate: 95
    windowDays: 7

  alerting:
    channelRefs:
      - name: ops-slack
```

## Cluster-Wide Monitor

Watch critical jobs across all namespaces:

```yaml title="cluster-wide.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: cluster-critical
  namespace: cronjob-guardian
spec:
  selector:
    allNamespaces: true
    matchLabels:
      tier: critical
    matchExpressions:
      - key: skip-monitoring
        operator: DoesNotExist

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1

  sla:
    minSuccessRate: 99
    windowDays: 30

  alerting:
    channelRefs:
      - name: pagerduty-critical
      - name: ops-slack
```

## Low-Priority Batch Monitor

Relaxed monitoring for non-critical batch jobs:

```yaml title="batch-jobs.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: batch-jobs
  namespace: batch
spec:
  selector:
    matchLabels:
      tier: low

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 5

  sla:
    minSuccessRate: 80
    windowDays: 7

  alerting:
    channelRefs:
      - name: batch-slack

    alertDelay: 30m
    suppressDuplicatesFor: 4h

    severityOverrides:
      jobFailed: warning
      deadManTriggered: warning
      slaBreached: warning

  dataRetention:
    retentionDays: 30
    storeLogs: false
```

## Financial Reports Monitor

Business-critical reports with maintenance windows:

```yaml title="financial-reports.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: financial-reports
  namespace: finance
spec:
  selector:
    matchLabels:
      type: financial-report

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1

  sla:
    minSuccessRate: 100
    windowDays: 30
    maxDuration: 1h

  maintenanceWindows:
    # Month-end processing
    - name: month-end
      schedule: "0 0 1 * *"
      duration: 6h
      timezone: America/New_York
    # Quarter-end processing
    - name: quarter-end
      schedule: "0 0 1 1,4,7,10 *"
      duration: 12h
      timezone: America/New_York

  alerting:
    channelRefs:
      - name: finance-pagerduty
        severities:
          - critical
      - name: finance-slack

    severityOverrides:
      jobFailed: critical
      deadManTriggered: critical

  dataRetention:
    retentionDays: 365
    onCronJobDeletion: retain
```

## Related

- [Alert Channel Examples](./channels.md) - Channel configurations
- [Use Cases](./use-cases) - Real-world scenarios
- [CronJob Selectors](/docs/configuration/monitors/selectors) - Selection patterns
