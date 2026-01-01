# CronJob Guardian Examples

This directory contains example configurations for CronJob Guardian.

## Directory Structure

```
examples/
├── alertchannels/      # AlertChannel configurations
│   ├── slack.yaml      # Slack webhook integration
│   ├── pagerduty.yaml  # PagerDuty integration
│   ├── webhook.yaml    # Generic webhook
│   └── email.yaml      # Email via SMTP
├── monitors/           # CronJobMonitor configurations
│   ├── basic.yaml              # Basic monitor with labels
│   ├── all-in-namespace.yaml   # Monitor all jobs in namespace
│   ├── multi-namespace.yaml    # Monitor across namespaces
│   ├── namespace-selector.yaml # Select namespaces by labels
│   ├── cluster-wide.yaml       # Cluster-wide monitoring
│   ├── full-featured.yaml      # All configuration options
│   ├── database-backups.yaml   # Use case: backups
│   ├── data-pipeline.yaml      # Use case: ETL
│   └── financial-reports.yaml  # Use case: reports
└── cronjobs/           # Example CronJob configurations
    ├── with-timeout.yaml       # activeDeadlineSeconds
    ├── with-retry.yaml         # backoffLimit
    ├── with-timezone.yaml      # timeZone (K8s 1.27+)
    └── with-concurrency.yaml   # concurrencyPolicy
```

## Quick Start

1. **Create an AlertChannel:**

```bash
# Create the Slack webhook secret first
kubectl create secret generic slack-webhook \
  --namespace cronjob-guardian \
  --from-literal=url=https://hooks.slack.com/services/...

# Apply the AlertChannel
kubectl apply -f alertchannels/slack.yaml
```

2. **Create a CronJobMonitor:**

```bash
# Monitor all critical jobs in the production namespace
kubectl apply -f monitors/basic.yaml
```

## AlertChannel Examples

| File | Description |
|------|-------------|
| `slack.yaml` | Slack incoming webhook with rate limiting |
| `pagerduty.yaml` | PagerDuty Events API integration |
| `webhook.yaml` | Generic HTTP webhook with custom headers |
| `email.yaml` | SMTP email notifications |

## CronJobMonitor Examples

### Selector Patterns

| File | Description |
|------|-------------|
| `basic.yaml` | Select by labels in the same namespace |
| `all-in-namespace.yaml` | Monitor all CronJobs in a namespace |
| `multi-namespace.yaml` | Monitor across explicit namespaces |
| `namespace-selector.yaml` | Select namespaces by their labels |
| `cluster-wide.yaml` | Monitor all CronJobs cluster-wide |

### Use Cases

| File | Description |
|------|-------------|
| `database-backups.yaml` | Critical backups with 100% SLA and custom fix patterns |
| `data-pipeline.yaml` | ETL jobs with duration regression detection |
| `financial-reports.yaml` | Reports with maintenance windows |
| `full-featured.yaml` | Comprehensive example with all options |

## CronJob Examples

Native Kubernetes CronJob features that complement CronJob Guardian:

| File | Description |
|------|-------------|
| `with-timeout.yaml` | Kill stuck jobs with `activeDeadlineSeconds` |
| `with-retry.yaml` | Auto-retry with `backoffLimit` |
| `with-timezone.yaml` | Timezone support (Kubernetes 1.27+) |
| `with-concurrency.yaml` | Prevent overlaps with `concurrencyPolicy` |

## Applying Examples

```bash
# Apply a single example
kubectl apply -f monitors/basic.yaml

# Apply all AlertChannels
kubectl apply -f alertchannels/

# Apply all monitors
kubectl apply -f monitors/
```

## Customizing Examples

These examples are starting points. Key things to customize:

1. **Namespaces**: Update `metadata.namespace` to match your setup
2. **Secret references**: Create secrets and update `secretRef` fields
3. **Labels**: Adjust `matchLabels` to match your CronJob labels
4. **Thresholds**: Tune SLA percentages and dead-man switch intervals
5. **Channel refs**: Update `channelRefs` to use your AlertChannels
