---
sidebar_position: 3
title: Data Retention
description: Configure execution history retention
---

# Data Retention

Configure how long CronJob Guardian retains execution history and related data.

## Default Retention

By default, CronJob Guardian retains:
- Execution records: 90 days
- Logs: 30 days
- Events: 30 days

## Monitor-Level Override

Override retention for specific monitors:

```yaml
spec:
  dataRetention:
    retentionDays: 180          # Keep 180 days of history
```

## Storage Controls

### Log Storage

Control whether logs are stored:

```yaml
spec:
  dataRetention:
    retentionDays: 90
    storeLogs: true             # Store pod logs (default: true)
    logRetentionDays: 14        # Logs retained shorter than executions
```

### Event Storage

Control event storage:

```yaml
spec:
  dataRetention:
    storeEvents: true           # Store Kubernetes events
    eventRetentionDays: 14
```

## CronJob Lifecycle

### On Deletion

Configure behavior when monitored CronJobs are deleted:

```yaml
spec:
  dataRetention:
    onCronJobDeletion: retain   # Options: retain, delete
```

| Value | Behavior |
|-------|----------|
| `retain` | Keep history after CronJob is deleted |
| `delete` | Delete history when CronJob is deleted |

### On Recreation

When a CronJob with the same name is recreated:

```yaml
spec:
  dataRetention:
    onRecreation: merge         # Options: merge, reset
```

| Value | Behavior |
|-------|----------|
| `merge` | Combine with previous history |
| `reset` | Start fresh, archive old data |

## Examples

### Long-Term Compliance

```yaml title="compliance-retention.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: audit-jobs
  namespace: compliance
spec:
  selector:
    matchLabels:
      compliance: required

  dataRetention:
    retentionDays: 365          # 1 year for compliance
    storeLogs: true
    logRetentionDays: 90
    storeEvents: true
    eventRetentionDays: 90
    onCronJobDeletion: retain   # Never delete compliance data

  alerting:
    channelRefs:
      - name: compliance-slack
```

### Short-Term Development

```yaml title="dev-retention.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: dev-jobs
  namespace: development
spec:
  selector: {}

  dataRetention:
    retentionDays: 7            # 1 week only
    storeLogs: true
    logRetentionDays: 3
    storeEvents: false          # Don't store events
    onCronJobDeletion: delete   # Clean up with CronJob

  alerting:
    channelRefs:
      - name: dev-slack
```

### Standard Production

```yaml title="prod-retention.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: production-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      monitored: "true"

  dataRetention:
    retentionDays: 90
    storeLogs: true
    logRetentionDays: 30
    storeEvents: true
    eventRetentionDays: 30
    onCronJobDeletion: retain
    onRecreation: merge

  alerting:
    channelRefs:
      - name: ops-slack
```

## Global Configuration

Set default retention via Helm values:

```yaml
config:
  dataRetention:
    defaultRetentionDays: 90
    defaultLogRetentionDays: 30
    pruneInterval: 1h           # How often to run cleanup
```

## Manual Pruning

Trigger manual data pruning via the dashboard:

1. Go to **Settings**
2. Click **Prune Old Data**
3. Confirm the action

Or via API:

```bash
curl -X POST http://localhost:8080/api/v1/admin/prune
```

## Storage Considerations

### SQLite

- Data stored in a single file
- Regular pruning keeps file size manageable
- Consider vacuuming periodically

### PostgreSQL/MySQL

- More efficient for large datasets
- Pruning runs as DELETE queries
- Consider partitioning for very large deployments

## Configuration Reference

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `retentionDays` | int | Days to retain execution records | `90` |
| `storeLogs` | bool | Store pod logs | `true` |
| `logRetentionDays` | int | Days to retain logs | `30` |
| `storeEvents` | bool | Store Kubernetes events | `true` |
| `eventRetentionDays` | int | Days to retain events | `30` |
| `onCronJobDeletion` | string | Behavior on CronJob deletion | `retain` |
| `onRecreation` | string | Behavior on CronJob recreation | `merge` |

## Related

- [Storage Backends](/docs/configuration/storage/sqlite) - Storage configuration
- [Dashboard](/docs/features/dashboard) - Manual pruning
- [Helm Values](/docs/reference/helm-values) - Global configuration
