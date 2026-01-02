---
sidebar_position: 2
title: Alert Configuration
description: Configure how alerts are sent
---

# Alert Configuration

Configure how CronJob Guardian sends alerts for monitored CronJobs.

## Channel References

Link monitors to alert channels:

```yaml
spec:
  alerting:
    channelRefs:
      - name: team-slack
      - name: ops-pagerduty
```

Alerts are sent to all referenced channels based on their configuration.

## Severity Levels

CronJob Guardian uses three severity levels:

| Severity | Description | Default Use |
|----------|-------------|-------------|
| `critical` | Immediate attention required | Configurable |
| `warning` | Attention needed soon | Configurable |
| `info` | Informational only | Status updates |

### Severity Overrides

Customize severity per alert type:

```yaml
spec:
  alerting:
    severityOverrides:
      jobFailed: warning           # Job failures
      deadManTriggered: critical   # Missed schedules
      slaBreached: warning         # SLA breaches
      durationRegression: info     # Performance degradation
```

### Routing by Severity

Send different severities to different channels:

```yaml
spec:
  alerting:
    channelRefs:
      - name: pagerduty-critical
        severities:
          - critical
      - name: team-slack
        severities:
          - critical
          - warning
          - info
```

## Alert Suppression

### Duplicate Suppression

Prevent alert storms for recurring failures:

```yaml
spec:
  alerting:
    suppressDuplicatesFor: 1h     # Suppress same alert for 1 hour
```

### Alert Delay

Wait before alerting for transient issues:

```yaml
spec:
  alerting:
    alertDelay: 5m                # Wait 5 min before sending
```

Useful for flaky jobs that often recover on retry.

### Combined Example

```yaml
spec:
  alerting:
    alertDelay: 5m
    suppressDuplicatesFor: 1h
    channelRefs:
      - name: team-slack
```

## Alert Context

### Logs and Events

Control what context is included in alerts:

```yaml
spec:
  alerting:
    includeContext:
      logs: true                  # Include pod logs
      events: true                # Include Kubernetes events
      podStatus: true             # Include pod status details
      logLines: 50                # Number of log lines
```

### Suggested Fixes

Enable intelligent fix suggestions:

```yaml
spec:
  alerting:
    includeSuggestedFixes: true
    suggestedFixPatterns:
      - name: custom-pattern
        match:
          logPattern: "connection refused"
        suggestion: "Check database connectivity"
        priority: 150
```

See [Suggested Fixes](/docs/features/suggested-fixes) for details.

## Complete Examples

### Standard Team Monitor

```yaml title="team-alerting.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: team-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      team: platform

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true

  sla:
    minSuccessRate: 95
    windowDays: 7

  alerting:
    channelRefs:
      - name: platform-slack

    alertDelay: 2m
    suppressDuplicatesFor: 30m

    severityOverrides:
      jobFailed: warning
      deadManTriggered: critical
      slaBreached: warning

    includeContext:
      logs: true
      events: true
      logLines: 30

    includeSuggestedFixes: true
```

### Critical with Escalation

```yaml title="critical-escalation.yaml"
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

  sla:
    minSuccessRate: 99.9
    windowDays: 30

  alerting:
    channelRefs:
      # Critical goes to PagerDuty
      - name: pagerduty-critical
        severities:
          - critical
      # All severities to Slack
      - name: ops-slack
        severities:
          - critical
          - warning
          - info

    # No delay for critical jobs
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

### Low-Priority with Aggregation

```yaml title="low-priority.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: batch-jobs
  namespace: batch
spec:
  selector:
    matchLabels:
      tier: low

  sla:
    minSuccessRate: 80
    windowDays: 7

  alerting:
    channelRefs:
      - name: batch-slack
        severities:
          - critical
          - warning

    # Generous delays for low-priority
    alertDelay: 15m
    suppressDuplicatesFor: 4h

    severityOverrides:
      jobFailed: info            # Failures are just info
      deadManTriggered: warning  # Missing is warning
      slaBreached: warning

    includeContext:
      logs: true
      logLines: 20
```

## Configuration Reference

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `channelRefs` | []ChannelRef | Alert channels to notify | Required |
| `channelRefs[].name` | string | AlertChannel resource name | Required |
| `channelRefs[].severities` | []string | Severities to send to this channel | All |
| `alertDelay` | duration | Wait before sending alert | `0s` |
| `suppressDuplicatesFor` | duration | Suppress duplicate alerts | `0s` |
| `severityOverrides` | map | Override default severities | - |
| `includeContext` | object | What to include in alerts | - |
| `includeSuggestedFixes` | bool | Include fix suggestions | `true` |
| `suggestedFixPatterns` | []Pattern | Custom fix patterns | - |

## Related

- [Slack Integration](/docs/configuration/alerting/slack) - Configure Slack
- [PagerDuty Integration](/docs/configuration/alerting/pagerduty) - Configure PagerDuty
- [Suggested Fixes](/docs/features/suggested-fixes) - Fix suggestion patterns
- [Maintenance Windows](/docs/features/maintenance-windows) - Suppress during maintenance
