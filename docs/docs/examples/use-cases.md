---
sidebar_position: 3
title: Use Cases
description: Real-world monitoring scenarios
---

# Real-World Use Cases

Practical examples of CronJob Guardian configurations for common scenarios.

## Database Backups

Monitor critical database backup jobs that must never be missed.

```yaml title="database-backup-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: database-backups
spec:
  selector:
    matchLabels:
      app: backup
      type: database
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      buffer: 15m
      missedScheduleThreshold: 1
  sla:
    minSuccessRate: 100
    windowDays: 7
  alerting:
    includeContext:
      logs: true
      events: true
    channelRefs:
      - name: pagerduty-critical
      - name: ops-slack
```

### Why This Configuration

- **100% success rate**: Backups are critical; any failure is unacceptable
- **Auto-detect schedule**: Works with any backup frequency
- **Single miss threshold**: Alert immediately if backup doesn't run
- **Full context**: Include logs to diagnose backup failures quickly

## ETL Pipelines

Monitor data processing jobs that feed analytics and reporting.

```yaml title="etl-pipeline-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: etl-pipelines
spec:
  selector:
    matchLabels:
      type: etl
  sla:
    minSuccessRate: 95
    windowDays: 7
    maxDuration: 2h
    durationRegressionThreshold: 30
    durationBaselineWindowDays: 14
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
  alerting:
    suppressDuplicatesFor: 1h
    severityOverrides:
      durationRegression: warning
    channelRefs:
      - name: data-team-slack
```

### Why This Configuration

- **Duration monitoring**: ETL jobs getting slower indicates data growth issues
- **30% regression threshold**: Catch performance degradation early
- **Duplicate suppression**: Prevent alert fatigue during incident response
- **Warning severity for regression**: Duration issues are important but not critical

## Report Generation

Monitor scheduled report generation for business users.

```yaml title="report-generation-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: business-reports
spec:
  selector:
    matchLabels:
      type: report
  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 25h
  sla:
    minSuccessRate: 90
    windowDays: 30
  maintenanceWindows:
    - name: monthly-maintenance
      schedule: "0 2 1 * *"
      duration: 4h
      timezone: America/New_York
      suppressAlerts: true
  alerting:
    channelRefs:
      - name: ops-slack
```

### Why This Configuration

- **25-hour window**: Daily reports with buffer for occasional delays
- **90% success rate**: Some failures acceptable for less critical reports
- **Maintenance window**: Suppress alerts during monthly maintenance
- **30-day window**: Longer evaluation period for occasional jobs

## Multi-Tenant Batch Jobs

Monitor jobs across multiple namespaces with different SLAs.

```yaml title="multi-tenant-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: tenant-jobs-premium
spec:
  selector:
    namespaceSelector:
      matchLabels:
        tier: premium
    matchLabels:
      managed-by: platform
  sla:
    minSuccessRate: 99
    windowDays: 7
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1
  alerting:
    channelRefs:
      - name: pagerduty-premium
      - name: customer-slack
---
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: tenant-jobs-standard
spec:
  selector:
    namespaceSelector:
      matchLabels:
        tier: standard
    matchLabels:
      managed-by: platform
  sla:
    minSuccessRate: 95
    windowDays: 7
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 3
  alerting:
    channelRefs:
      - name: ops-slack
```

### Why This Configuration

- **Separate monitors by tier**: Different SLAs for different service levels
- **Namespace selector**: Automatically covers all tenant namespaces
- **Stricter thresholds for premium**: Higher success rate, faster alerts
- **More tolerance for standard**: 3 consecutive misses before alerting

## Infrastructure Maintenance Jobs

Monitor cluster maintenance tasks like certificate rotation and cleanup.

```yaml title="infrastructure-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: infra-maintenance
spec:
  selector:
    matchLabels:
      category: infrastructure
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      buffer: 1h
  sla:
    minSuccessRate: 100
  alerting:
    includeContext:
      logs: true
      events: true
      podStatus: true
    suggestedFixPatterns:
      - name: cert-rotation-failed
        match:
          logPattern: "certificate.*expired|x509.*certificate"
        suggestion: |
          Certificate rotation failed. Check:
          1. kubectl get secrets -n cert-manager
          2. kubectl describe certificate <name>
          3. Manually trigger: kubectl cert-manager renew <name>
        priority: 150
    channelRefs:
      - name: platform-pagerduty
```

### Why This Configuration

- **100% success required**: Infrastructure jobs are critical
- **Custom fix patterns**: Domain-specific troubleshooting guidance
- **Full context**: All diagnostic information included
- **Hour buffer**: Allow for scheduling variance in cluster

## Compliance Audit Jobs

Monitor jobs required for regulatory compliance with audit trail.

```yaml title="compliance-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: compliance-audits
spec:
  selector:
    matchLabels:
      compliance: required
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1
  sla:
    minSuccessRate: 100
    windowDays: 90
  dataRetention:
    retentionDays: 365
    storeLogs: true
  alerting:
    channelRefs:
      - name: compliance-email
      - name: security-slack
      - name: pagerduty-critical
```

### Why This Configuration

- **365-day retention**: Keep audit trail for compliance requirements
- **90-day SLA window**: Quarterly compliance reporting
- **Multiple channels**: Ensure alerts reach compliance team
- **Store logs**: Full execution history for audits

## High-Frequency Jobs

Monitor jobs that run every few minutes.

```yaml title="high-frequency-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: health-checks
spec:
  selector:
    matchLabels:
      type: health-check
  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 10m
  sla:
    minSuccessRate: 99
    windowDays: 1
  alerting:
    alertDelay: 5m
    suppressDuplicatesFor: 15m
    channelRefs:
      - name: ops-slack
```

### Why This Configuration

- **Short dead-man window**: 10 minutes for jobs running every 5 minutes
- **1-day SLA window**: Appropriate for high-frequency jobs
- **Alert delay**: Wait 5 minutes to avoid transient failures
- **Duplicate suppression**: Prevent alert storms

## Related

- [Monitor Examples](./monitors.md) - More CronJobMonitor configurations
- [Channel Examples](./channels.md) - AlertChannel configurations
- [Production Setup](/docs/guides/production-setup) - Production best practices
