---
sidebar_position: 3
title: SLA Configuration
description: Configure SLA monitoring thresholds
---

# SLA Configuration

Configure success rate thresholds and duration monitoring for your CronJobs.

## Success Rate

Track the percentage of successful job executions:

```yaml
spec:
  sla:
    minSuccessRate: 95
    windowDays: 7
```

| Field | Description | Default |
|-------|-------------|---------|
| `minSuccessRate` | Minimum acceptable success percentage (0-100) | 95 |
| `windowDays` | Rolling window for calculation | 7 |

### How It Works

1. Guardian tracks all job executions within the window
2. Calculates `successful / total * 100`
3. Alerts when rate falls below threshold

### Example: 99% SLA

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: critical-jobs
spec:
  selector:
    matchLabels:
      tier: critical
  sla:
    minSuccessRate: 99
    windowDays: 30
  alerting:
    channelRefs:
      - name: pagerduty-critical
```

## Duration Thresholds

Monitor job execution time:

```yaml
spec:
  sla:
    maxDuration: 30m
    durationRegressionThreshold: 50
    durationBaselineWindowDays: 14
```

| Field | Description | Default |
|-------|-------------|---------|
| `maxDuration` | Maximum acceptable duration | - |
| `durationRegressionThreshold` | Percentage increase that triggers alert | 50 |
| `durationBaselineWindowDays` | Window for baseline calculation | 14 |

### Absolute Duration

Alert when any execution exceeds a fixed time:

```yaml
spec:
  sla:
    maxDuration: 1h
```

### Regression Detection

Detect when jobs are getting slower over time:

```yaml
spec:
  sla:
    durationRegressionThreshold: 50
    durationBaselineWindowDays: 14
```

This configuration:
- Calculates baseline from the last 14 days
- Alerts if current duration exceeds baseline by 50%

## Combined Example

```yaml title="full-sla.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: etl-pipelines
spec:
  selector:
    matchLabels:
      type: etl
  sla:
    # Success rate
    minSuccessRate: 98
    windowDays: 7
    # Duration
    maxDuration: 2h
    durationRegressionThreshold: 30
    durationBaselineWindowDays: 14
  alerting:
    channelRefs:
      - name: ops-slack
      - name: pagerduty-critical
        severities: [critical]
```

## SLA Dashboard

View SLA metrics in the dashboard:

1. Go to **SLA** page
2. See compliance percentages per monitor
3. Drill down to individual CronJobs
4. View historical trends

## Alert Types

SLA-related alert types:

| Type | Triggered When |
|------|----------------|
| `SLABreach` | Success rate drops below threshold |
| `DurationExceeded` | Job takes longer than `maxDuration` |
| `DurationRegression` | Duration increases beyond baseline |

## Best Practices

1. **Start lenient**: Begin with lower thresholds, tighten over time
2. **Use appropriate windows**: Longer windows for less frequent jobs
3. **Consider maintenance**: Factor in planned downtime
4. **Set per-criticality**: Critical jobs need tighter SLAs

## Related

- [Selectors](./selectors.md) - CronJob selection patterns
- [Alerting](./alerting.md) - Alert configuration
- [SLA Tracking Feature](/docs/features/sla-tracking) - Feature overview
