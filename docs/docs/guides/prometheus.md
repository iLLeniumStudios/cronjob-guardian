---
sidebar_position: 2
title: Prometheus Integration
description: Monitor CronJob Guardian with Prometheus
---

# Prometheus & Grafana Integration

CronJob Guardian exports Prometheus metrics for integration with your monitoring stack.

## Metrics Endpoint

Metrics are exposed at `/metrics` on port 8080 by default.

```bash
# Port-forward and view metrics
kubectl port-forward -n cronjob-guardian svc/cronjob-guardian 8080:8080
curl http://localhost:8080/metrics
```

## Available Metrics

### CronJob Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cronjob_guardian_success_rate` | Gauge | namespace, cronjob, monitor | Success rate percentage (0-100) |
| `cronjob_guardian_duration_seconds` | Histogram | namespace, cronjob | Execution duration |
| `cronjob_guardian_executions_total` | Counter | namespace, cronjob, status | Total executions |
| `cronjob_guardian_active_alerts` | Gauge | namespace, cronjob, severity | Active alert count |

### Alert Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cronjob_guardian_alerts_total` | Counter | type, severity, channel | Total alerts sent |
| `cronjob_guardian_alert_dispatch_duration_seconds` | Histogram | channel | Alert dispatch duration |

### Operator Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cronjob_guardian_leader_status` | Gauge | - | 1 if leader, 0 if standby |
| `cronjob_guardian_db_connections_open` | Gauge | - | Open database connections |
| `cronjob_guardian_db_connections_idle` | Gauge | - | Idle database connections |

## ServiceMonitor

Enable ServiceMonitor for automatic Prometheus discovery:

### Helm Configuration

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring      # Prometheus namespace
    interval: 30s
    scrapeTimeout: 10s
    labels:
      release: prometheus      # Match your Prometheus selector
```

### Manual ServiceMonitor

```yaml title="servicemonitor.yaml"
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cronjob-guardian
  namespace: monitoring
  labels:
    release: prometheus
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: cronjob-guardian
  namespaceSelector:
    matchNames:
      - cronjob-guardian
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

## Prometheus Rules

### Recording Rules

```yaml title="recording-rules.yaml"
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: cronjob-guardian-recording
  namespace: monitoring
spec:
  groups:
    - name: cronjob-guardian.recording
      rules:
        # Average success rate across all CronJobs
        - record: cronjob_guardian:success_rate:avg
          expr: avg(cronjob_guardian_success_rate)

        # CronJobs with low success rate
        - record: cronjob_guardian:low_success_rate:count
          expr: count(cronjob_guardian_success_rate < 95)

        # Alert rate per minute
        - record: cronjob_guardian:alerts:rate1m
          expr: sum(rate(cronjob_guardian_alerts_total[1m]))
```

### Alert Rules

```yaml title="alert-rules.yaml"
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: cronjob-guardian-alerts
  namespace: monitoring
spec:
  groups:
    - name: cronjob-guardian.alerts
      rules:
        # CronJob success rate below threshold
        - alert: CronJobLowSuccessRate
          expr: cronjob_guardian_success_rate < 90
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "CronJob {{ $labels.namespace }}/{{ $labels.cronjob }} has low success rate"
            description: "Success rate is {{ $value }}% (threshold: 90%)"

        # CronJob Guardian not running
        - alert: CronJobGuardianDown
          expr: up{job="cronjob-guardian"} == 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "CronJob Guardian is down"

        # No leader elected (HA)
        - alert: CronJobGuardianNoLeader
          expr: sum(cronjob_guardian_leader_status) == 0
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "No CronJob Guardian leader elected"

        # High alert rate
        - alert: CronJobGuardianHighAlertRate
          expr: sum(rate(cronjob_guardian_alerts_total[5m])) > 1
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "High rate of CronJob alerts"
            description: "Sending more than 1 alert per second for 10+ minutes"

        # Database connection issues
        - alert: CronJobGuardianDBConnections
          expr: cronjob_guardian_db_connections_open > 20
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "High number of database connections"
```

## Grafana Dashboard

### Import Dashboard

1. Go to Grafana → Dashboards → Import
2. Use dashboard ID or paste JSON
3. Select your Prometheus data source

### Dashboard JSON

```json
{
  "title": "CronJob Guardian",
  "panels": [
    {
      "title": "Overall Success Rate",
      "type": "gauge",
      "targets": [
        {
          "expr": "avg(cronjob_guardian_success_rate)"
        }
      ]
    },
    {
      "title": "Success Rate by CronJob",
      "type": "table",
      "targets": [
        {
          "expr": "cronjob_guardian_success_rate",
          "legendFormat": "{{ namespace }}/{{ cronjob }}"
        }
      ]
    },
    {
      "title": "Alerts Over Time",
      "type": "timeseries",
      "targets": [
        {
          "expr": "sum(rate(cronjob_guardian_alerts_total[5m])) by (severity)"
        }
      ]
    },
    {
      "title": "Execution Duration (P95)",
      "type": "timeseries",
      "targets": [
        {
          "expr": "histogram_quantile(0.95, sum(rate(cronjob_guardian_duration_seconds_bucket[5m])) by (le, cronjob))"
        }
      ]
    }
  ]
}
```

## Example Queries

### CronJob Health

```promql
# Success rate below 95%
cronjob_guardian_success_rate < 95

# CronJobs with active critical alerts
cronjob_guardian_active_alerts{severity="critical"} > 0

# Slowest CronJobs (P95 duration)
topk(10, histogram_quantile(0.95, sum(rate(cronjob_guardian_duration_seconds_bucket[1h])) by (le, namespace, cronjob)))
```

### Alert Analysis

```promql
# Alert rate by channel
sum(rate(cronjob_guardian_alerts_total[1h])) by (channel)

# Alert rate by type
sum(rate(cronjob_guardian_alerts_total[1h])) by (type)

# Most alerting CronJobs
topk(10, sum(increase(cronjob_guardian_alerts_total[24h])) by (namespace, cronjob))
```

### Operator Health

```promql
# Leader status
cronjob_guardian_leader_status

# Database connections
cronjob_guardian_db_connections_open

# Replica count
sum(up{job="cronjob-guardian"})
```

## Integration with Existing Monitoring

### Using Prometheus for All Alerting

If you prefer Prometheus/Alertmanager over CronJob Guardian's built-in alerting:

1. Disable built-in alerting in monitors
2. Create Prometheus alert rules
3. Route via Alertmanager

```yaml
# In CronJobMonitor - minimal alerting
spec:
  alerting:
    channelRefs: []    # No direct channels
```

```yaml
# Prometheus alert rules instead
- alert: CronJobFailure
  expr: increase(cronjob_guardian_executions_total{status="failed"}[5m]) > 0
  labels:
    severity: warning
```

### Dual Alerting

Run both for redundancy:
- CronJob Guardian alerts for immediate, contextual notifications
- Prometheus alerts as backup and for metrics-based alerting

## Related

- [High Availability](./high-availability.md) - HA deployment
- [Production Setup](./production-setup.md) - Production configuration
- [Dashboard](/docs/features/dashboard) - Built-in UI
