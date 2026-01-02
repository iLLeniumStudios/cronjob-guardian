---
sidebar_position: 4
title: Prometheus Metrics
description: Prometheus metrics reference
---

# Prometheus Metrics Reference

CronJob Guardian exports Prometheus metrics on port 8080 at `/metrics`.

## CronJob Metrics

### cronjob_guardian_success_rate

Current success rate percentage for each monitored CronJob.

| Label | Description |
|-------|-------------|
| `namespace` | CronJob namespace |
| `cronjob` | CronJob name |
| `monitor` | Monitor name |

**Type**: Gauge

**Example**:
```promql
cronjob_guardian_success_rate{namespace="production", cronjob="daily-backup"}
```

### cronjob_guardian_duration_seconds

Execution duration histogram.

| Label | Description |
|-------|-------------|
| `namespace` | CronJob namespace |
| `cronjob` | CronJob name |

**Type**: Histogram

**Buckets**: 1s, 5s, 10s, 30s, 60s, 300s, 600s, 1800s, 3600s

**Example**:
```promql
# P95 duration
histogram_quantile(0.95, sum(rate(cronjob_guardian_duration_seconds_bucket[5m])) by (le, cronjob))
```

### cronjob_guardian_executions_total

Total number of job executions.

| Label | Description |
|-------|-------------|
| `namespace` | CronJob namespace |
| `cronjob` | CronJob name |
| `status` | Execution status (success, failed) |

**Type**: Counter

**Example**:
```promql
# Failure rate
rate(cronjob_guardian_executions_total{status="failed"}[1h])
```

### cronjob_guardian_active_alerts

Number of currently active alerts.

| Label | Description |
|-------|-------------|
| `namespace` | CronJob namespace |
| `cronjob` | CronJob name |
| `severity` | Alert severity |

**Type**: Gauge

**Example**:
```promql
cronjob_guardian_active_alerts{severity="critical"} > 0
```

## Alert Metrics

### cronjob_guardian_alerts_total

Total alerts sent.

| Label | Description |
|-------|-------------|
| `type` | Alert type (failure, deadManSwitch, slaViolation, durationRegression) |
| `severity` | Alert severity (critical, warning, info) |
| `channel` | Channel name |

**Type**: Counter

**Example**:
```promql
# Alert rate by channel
sum(rate(cronjob_guardian_alerts_total[1h])) by (channel)
```

### cronjob_guardian_alert_dispatch_duration_seconds

Time to dispatch alerts to channels.

| Label | Description |
|-------|-------------|
| `channel` | Channel name |
| `success` | Whether dispatch succeeded |

**Type**: Histogram

## Operator Metrics

### cronjob_guardian_leader_status

Whether this instance is the leader (1) or standby (0).

**Type**: Gauge

**Example**:
```promql
# Ensure a leader exists
sum(cronjob_guardian_leader_status) == 1
```

### cronjob_guardian_db_connections_open

Number of open database connections.

**Type**: Gauge

### cronjob_guardian_db_connections_idle

Number of idle database connections.

**Type**: Gauge

### cronjob_guardian_reconcile_total

Total reconciliation operations.

| Label | Description |
|-------|-------------|
| `controller` | Controller name |
| `result` | Result (success, error, requeue) |

**Type**: Counter

### cronjob_guardian_reconcile_duration_seconds

Reconciliation duration.

| Label | Description |
|-------|-------------|
| `controller` | Controller name |

**Type**: Histogram

## Standard Metrics

CronJob Guardian also exports standard controller-runtime metrics:

- `controller_runtime_reconcile_total`
- `controller_runtime_reconcile_errors_total`
- `controller_runtime_reconcile_time_seconds`
- `workqueue_*` metrics

And standard Go runtime metrics:

- `go_goroutines`
- `go_memstats_*`
- `process_*`

## Example Queries

### CronJob Health

```promql
# CronJobs with success rate below 95%
cronjob_guardian_success_rate < 95

# Slowest CronJobs by P95 duration
topk(10, histogram_quantile(0.95,
  sum(rate(cronjob_guardian_duration_seconds_bucket[1h])) by (le, namespace, cronjob)
))

# CronJobs with active critical alerts
cronjob_guardian_active_alerts{severity="critical"} > 0
```

### Alert Analysis

```promql
# Total alerts in last 24 hours
sum(increase(cronjob_guardian_alerts_total[24h]))

# Most alerting CronJobs
topk(10, sum(increase(cronjob_guardian_alerts_total[24h])) by (namespace, cronjob))

# Channel alert rate
sum(rate(cronjob_guardian_alerts_total[1h])) by (channel)
```

### Operator Health

```promql
# Operator up
up{job="cronjob-guardian"}

# Leader elected
cronjob_guardian_leader_status == 1

# Reconciliation error rate
sum(rate(cronjob_guardian_reconcile_total{result="error"}[5m]))
```

## Grafana Dashboard

See [Prometheus Integration](/docs/guides/prometheus) for a complete Grafana dashboard configuration.

## Related

- [Prometheus Integration](/docs/guides/prometheus) - Setup guide
- [REST API](./rest-api.md) - API reference
