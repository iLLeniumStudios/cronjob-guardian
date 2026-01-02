---
sidebar_position: 5
title: Maintenance Windows
description: Suppress alerts during planned maintenance
---

# Maintenance Windows

Configure maintenance windows to suppress alerts during planned downtime. This prevents false alarms when jobs are expected to fail or not run.

## Use Cases

- **Planned deployments**: Suppress during release windows
- **Database maintenance**: Quiet during backup windows
- **Infrastructure updates**: No alerts during node upgrades
- **Regular maintenance**: Weekly maintenance periods

## Configuration

### Basic Maintenance Window

```yaml
spec:
  maintenanceWindows:
    - schedule: "0 2 * * 0"      # Every Sunday at 2 AM
      duration: 4h               # For 4 hours
      timezone: America/New_York
```

### Multiple Windows

```yaml
spec:
  maintenanceWindows:
    # Weekly maintenance
    - schedule: "0 2 * * 0"
      duration: 4h
      timezone: America/New_York
      suppressAlerts: true

    # Monthly extended maintenance
    - schedule: "0 1 1 * *"      # First of each month at 1 AM
      duration: 8h
      timezone: America/New_York
      suppressAlerts: true

    # Daily backup window
    - schedule: "0 3 * * *"
      duration: 1h
      timezone: UTC
      suppressAlerts: true
```

## Configuration Reference

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `schedule` | string | Cron expression for window start | Required |
| `duration` | duration | How long the window lasts | Required |
| `timezone` | string | IANA timezone name | `UTC` |
| `suppressAlerts` | bool | Whether to suppress alerts | `true` |

## Examples

### Release Window

```yaml title="release-window.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: production-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      monitored: "true"

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true

  maintenanceWindows:
    # Tuesday deployment window
    - schedule: "0 14 * * 2"     # Tuesday 2 PM
      duration: 2h
      timezone: America/Los_Angeles
      suppressAlerts: true

    # Thursday deployment window
    - schedule: "0 14 * * 4"     # Thursday 2 PM
      duration: 2h
      timezone: America/Los_Angeles
      suppressAlerts: true

  alerting:
    channelRefs:
      - name: team-slack
```

### Database Maintenance

```yaml title="db-maintenance.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: db-dependent-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      database: postgres

  sla:
    minSuccessRate: 95
    windowDays: 7

  maintenanceWindows:
    # Database backup window (daily 3-4 AM UTC)
    - schedule: "0 3 * * *"
      duration: 1h
      timezone: UTC
      suppressAlerts: true

    # Weekly vacuum/analyze
    - schedule: "0 4 * * 0"      # Sunday 4 AM
      duration: 3h
      timezone: UTC
      suppressAlerts: true

  alerting:
    channelRefs:
      - name: dba-slack
```

### Multi-Region Maintenance

```yaml title="multi-region.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: global-jobs
  namespace: production
spec:
  selector: {}

  maintenanceWindows:
    # US maintenance
    - schedule: "0 3 * * 0"
      duration: 4h
      timezone: America/New_York
      suppressAlerts: true

    # EU maintenance
    - schedule: "0 3 * * 0"
      duration: 4h
      timezone: Europe/London
      suppressAlerts: true

    # APAC maintenance
    - schedule: "0 3 * * 0"
      duration: 4h
      timezone: Asia/Tokyo
      suppressAlerts: true

  alerting:
    channelRefs:
      - name: global-ops
```

## Behavior During Maintenance

When a maintenance window is active:

1. **Dead-man's switch**: Paused, won't alert for missed runs
2. **SLA tracking**: Continues calculating, but doesn't alert
3. **Failure alerts**: Suppressed for jobs within scope
4. **Execution recording**: Continues normally

After the window ends:
- Alerting resumes immediately
- Metrics reflect the full period (including maintenance)
- Dashboard shows maintenance periods

## Timezone Handling

Use IANA timezone names:
- `America/New_York`
- `Europe/London`
- `Asia/Tokyo`
- `UTC`

The schedule is evaluated in the specified timezone, accounting for DST.

## Combining with Alert Delay

For extra protection against false alarms:

```yaml
spec:
  maintenanceWindows:
    - schedule: "0 2 * * 0"
      duration: 4h
      timezone: UTC

  alerting:
    alertDelay: 5m              # Wait 5 min after window before alerting
    channelRefs:
      - name: team-slack
```

## Dashboard Indication

The dashboard shows:
- Active maintenance windows with countdown
- Historical maintenance periods on timeline
- Suppressed alert count during windows

## Best Practices

1. **Schedule conservatively**: Add buffer before and after actual maintenance
2. **Use specific timezones**: Be explicit about timezone to avoid confusion
3. **Document windows**: Keep a record of why each window exists
4. **Review periodically**: Remove obsolete windows
5. **Test alerting after windows**: Verify alerts resume correctly

## Related

- [Dead-Man's Switch](./dead-man-switch.md) - Affected by maintenance windows
- [SLA Tracking](./sla-tracking.md) - Continues during maintenance
- [Alerting Configuration](/docs/configuration/monitors/alerting) - Alert suppression options
