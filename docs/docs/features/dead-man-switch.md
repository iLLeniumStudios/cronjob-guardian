---
sidebar_position: 1
title: Dead-Man's Switch
description: Detect when CronJobs stop running entirely
---

# Dead-Man's Switch Detection

The dead-man's switch feature alerts you when CronJobs don't run within their expected time window. Unlike failure alerts that trigger when jobs fail, this catches jobs that **stop running entirely**.

## Why It Matters

CronJobs can silently stop running for many reasons:
- The CronJob was accidentally deleted
- The schedule was misconfigured
- Node issues prevented scheduling
- Resource quotas blocked pod creation

Without dead-man's switch monitoring, you'd only discover these issues when the consequences become apparent—missing backups, stale data, or compliance violations.

## Configuration

### Auto-Detection from Schedule

The simplest configuration auto-detects the expected interval from the cron schedule:

```yaml
spec:
  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 2   # Alert after 2 consecutive missed runs
      buffer: 15m                  # Grace period after expected run time
```

**How it works:**
1. CronJob Guardian parses the cron schedule (e.g., `0 2 * * *` = daily at 2 AM)
2. It calculates the expected interval between runs
3. Adds `buffer` as grace period
4. Alerts if no successful run within the expected window

### Fixed Time Window

For more control, specify a fixed time window:

```yaml
spec:
  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 26h   # Alert if no success in 26 hours
```

This is useful when:
- The cron schedule is complex
- You want stricter or looser thresholds than the schedule implies
- The job might run via external triggers too

## Examples

### Daily Backup Job

```yaml title="daily-backup-monitor.yaml"
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
      missedScheduleThreshold: 1   # Alert immediately on first miss
      buffer: 30m                  # 30 min grace period

  alerting:
    channelRefs:
      - name: ops-pagerduty
    severityOverrides:
      deadManTriggered: critical   # Missed backups are critical
```

### Hourly ETL Job

```yaml title="hourly-etl-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: etl-pipeline
  namespace: data
spec:
  selector:
    matchNames:
      - hourly-sync

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 3   # Allow 2 misses before alerting
      buffer: 10m

  alerting:
    channelRefs:
      - name: team-slack
```

### Weekly Report

```yaml title="weekly-report-monitor.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: weekly-reports
  namespace: analytics
spec:
  selector:
    matchLabels:
      schedule: weekly

  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 8d    # Alert if no run in 8 days

  alerting:
    channelRefs:
      - name: team-slack
```

## Configuration Reference

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `enabled` | bool | Enable dead-man's switch detection | `true` |
| `maxTimeSinceLastSuccess` | duration | Fixed maximum time since last successful run | - |
| `autoFromSchedule.enabled` | bool | Auto-detect interval from cron schedule | `false` |
| `autoFromSchedule.missedScheduleThreshold` | int | Number of consecutive missed runs before alerting | `1` |
| `autoFromSchedule.buffer` | duration | Grace period added to expected run time | `1h` |

## How It Works Internally

1. **Scheduler Loop**: A background scheduler runs every minute checking all monitored CronJobs
2. **Expected Run Calculation**: For auto-detect, it uses the cron schedule to calculate when the job should have run
3. **Consecutive Miss Tracking**: Tracks how many expected runs have been missed in a row
4. **Alert Dispatch**: When threshold is exceeded, dispatches an alert with context

## Alert Content

Dead-man's switch alerts include:
- Last successful execution time
- Expected run time
- Number of consecutive misses
- CronJob schedule expression
- Link to dashboard for more details

## Best Practices

1. **Set appropriate thresholds**: Critical jobs should alert on first miss; non-critical can tolerate a few misses
2. **Use buffer**: Add buffer for jobs that might run slightly late
3. **Consider timezone**: CronJobs use cluster timezone; account for this in monitoring
4. **Combine with SLA tracking**: Dead-man catches missing runs; SLA catches degraded success rates

## Related

- [SLA Tracking](./sla-tracking.md) - Monitor success rates
- [Duration Regression](./duration-regression.md) - Catch jobs slowing down
- [Alerting Configuration](/docs/configuration/monitors/alerting) - Configure alert behavior
