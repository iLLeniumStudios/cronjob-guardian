# CronJob Guardian

[![Go Version](https://img.shields.io/github/go-mod/go-version/iLLeniumStudios/cronjob-guardian)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/iLLeniumStudios/cronjob-guardian)](https://goreportcard.com/report/github.com/iLLeniumStudios/cronjob-guardian)

A Kubernetes operator that provides intelligent monitoring, SLA tracking, and alerting for CronJobs. Stop firefighting failed batch jobs and let CronJob Guardian watch over your scheduled workloads.

## Why CronJob Guardian?

Kubernetes CronJobs are critical for many operations: database backups, report generation, data pipelines, cache warming, and more. When they fail silently or stop running, the consequences can be severe.

CronJob Guardian solves common pain points:

- **Silent failures**: Get alerted immediately when jobs fail, with logs and suggested fixes
- **Missed schedules**: Dead-man's switch alerts when jobs don't run on time
- **Performance degradation**: Track SLAs and detect when jobs slow down or regress
- **Alert fatigue**: Smart deduplication, maintenance windows, and severity routing
- **Visibility**: Rich web dashboard with SLA compliance tracking, health heatmaps, and trend analysis

## Features

### Monitoring

- **Dead-Man's Switch**: Alert when CronJobs don't run within expected windows. Auto-detects expected intervals from cron schedules.
- **SLA Tracking**: Monitor success rates, duration percentiles (P50/P95/P99), and detect performance regressions.
- **Execution History**: Store and query job execution records with logs and events.
- **Prometheus Metrics**: Export metrics for integration with existing monitoring infrastructure.

### Alerting

- **Multiple Channels**: Slack, PagerDuty, generic webhooks, and email
- **Smart Context**: Alerts include pod logs, Kubernetes events, and suggested fixes
- **Deduplication**: Prevent alert storms with configurable suppression windows
- **Severity Routing**: Send critical alerts to PagerDuty, warnings to Slack

### Operations

- **Maintenance Windows**: Suppress alerts during scheduled maintenance
- **Built-in Dashboard**: Feature-rich web UI for monitoring and analytics
- **REST API**: Programmatic access to all monitoring data
- **Multiple Storage Backends**: SQLite (default), PostgreSQL, or MySQL

### Prometheus Metrics

CronJob Guardian exports the following metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `cronjob_guardian_success_rate` | Gauge | Success rate percentage (0-100) per CronJob |
| `cronjob_guardian_duration_seconds` | Histogram | Execution duration with P50/P95/P99 buckets |
| `cronjob_guardian_alerts_total` | Counter | Total alerts sent by type, severity, and channel |
| `cronjob_guardian_executions_total` | Counter | Total executions by status (success/failed) |
| `cronjob_guardian_active_alerts` | Gauge | Currently active alerts per CronJob |

Metrics are available at `/metrics` endpoint on port 8080.

## Quick Start

### Prerequisites

- Kubernetes 1.27+ (for CronJob timezone support)
- kubectl configured with cluster access
- Helm 3.x (optional)

### Installation

```bash
# Install CRDs and operator
kubectl apply -f https://raw.githubusercontent.com/iLLeniumStudios/cronjob-guardian/main/dist/install.yaml
```

Or build from source:

```bash
make docker-build docker-push IMG=your-registry/cronjob-guardian:latest
make deploy IMG=your-registry/cronjob-guardian:latest
```

### Basic Setup

1. **Create an AlertChannel** for notifications:

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: slack-ops
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: guardian-system
      key: url
```

2. **Create a CronJobMonitor** to watch your jobs:

```yaml
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
    maxTimeSinceLastSuccess: 25h
  sla:
    enabled: true
    minSuccessRate: 99
    windowDays: 7
  alerting:
    channelRefs:
      - name: slack-ops
```

## Configuration

### CronJobMonitor

The main resource for configuring what to monitor.

#### Selector

Select CronJobs by labels, expressions, or explicit names:

```yaml
spec:
  selector:
    # Match by labels
    matchLabels:
      app: backup
      environment: production

    # Or by label expressions
    matchExpressions:
      - key: tier
        operator: In
        values: [critical, high]

    # Or by explicit names
    matchNames:
      - daily-backup
      - weekly-report
```

#### Dead-Man's Switch

Alert when jobs don't run on schedule:

```yaml
spec:
  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 25h  # Alert if no success within 25 hours
```

#### SLA Tracking

Monitor success rates and performance:

```yaml
spec:
  sla:
    enabled: true
    minSuccessRate: 95        # Minimum 95% success rate
    windowDays: 7             # Over rolling 7-day window
```

#### Maintenance Windows

Suppress alerts during planned maintenance:

```yaml
spec:
  maintenanceWindows:
    - name: weekly-maintenance
      schedule: "0 2 * * 0"    # Every Sunday at 2 AM
      duration: 4h
      suppressAlerts: true
```

#### Alerting Configuration

Route alerts to channels with severity filtering:

```yaml
spec:
  alerting:
    enabled: true
    channelRefs:
      - name: pagerduty-oncall
        severities: [critical]
      - name: slack-ops
        severities: [critical, warning]
      - name: slack-info
        severities: [info]

    severityOverrides:
      jobFailed: critical
      slaBreached: warning
      missedSchedule: warning

    suppressDuplicatesFor: 1h

    context:
      includeLogs: true
      logLines: 100
      includeEvents: true
      includeSuggestedFix: true
```

### AlertChannel

Define where to send alerts. AlertChannel resources are cluster-scoped and can be referenced by any CronJobMonitor.

#### Slack

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: slack-alerts
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: guardian-system
      key: url
    defaultChannel: "#alerts"
  rateLimiting:
    maxPerHour: 100
    burstSize: 10
```

#### PagerDuty

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: pagerduty-critical
spec:
  type: pagerduty
  pagerduty:
    routingKeySecretRef:
      name: pagerduty-key
      namespace: guardian-system
      key: routing-key
    severity: critical
```

#### Webhook

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: custom-webhook
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: webhook-url
      namespace: guardian-system
      key: url
    method: POST
    headers:
      Content-Type: application/json
      X-Custom-Header: guardian
```

#### Email

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: email-team
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: guardian-system
    from: guardian@example.com
    to:
      - ops-team@example.com
      - oncall@example.com
```

## Native Kubernetes Features

CronJob Guardian focuses on monitoring and alerting, leaving job execution control to native Kubernetes features. Use these built-in CronJob/Job spec options:

### Handling Stuck Jobs

Use `activeDeadlineSeconds` on the Job template to automatically terminate long-running jobs:

```yaml
apiVersion: batch/v1
kind: CronJob
spec:
  jobTemplate:
    spec:
      activeDeadlineSeconds: 3600  # Kill after 1 hour
```

### Auto-Retry Failed Jobs

Use `backoffLimit` on the Job template to configure retry behavior:

```yaml
apiVersion: batch/v1
kind: CronJob
spec:
  jobTemplate:
    spec:
      backoffLimit: 3  # Retry up to 3 times
```

### Timezone Configuration

Use `timeZone` on the CronJob spec (Kubernetes 1.27+):

```yaml
apiVersion: batch/v1
kind: CronJob
spec:
  timeZone: "America/New_York"
  schedule: "0 9 * * *"  # 9 AM Eastern
```

### Concurrency Control

Use `concurrencyPolicy` to control overlapping executions:

```yaml
apiVersion: batch/v1
kind: CronJob
spec:
  concurrencyPolicy: Forbid  # Skip if previous still running
```

## Use Cases

### Database Backups

Monitor critical backup jobs with strict SLA:

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: database-backups
  namespace: databases
spec:
  selector:
    matchLabels:
      type: backup
  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 25h
  sla:
    enabled: true
    minSuccessRate: 100
  alerting:
    channelRefs:
      - name: pagerduty-dba
        severities: [critical]
```

### Data Pipeline Jobs

Track ETL performance and catch regressions:

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: etl-pipeline
  namespace: data-eng
spec:
  selector:
    matchLabels:
      pipeline: etl
  sla:
    enabled: true
    minSuccessRate: 95
    windowDays: 7
  alerting:
    channelRefs:
      - name: slack-data-eng
```

### Report Generation

Monitor business-critical reports:

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: financial-reports
  namespace: finance
spec:
  selector:
    matchNames:
      - daily-revenue-report
      - weekly-summary
  deadManSwitch:
    enabled: true
    maxTimeSinceLastSuccess: 25h
  maintenanceWindows:
    - name: quarter-end
      schedule: "0 0 1 1,4,7,10 *"
      duration: 24h
      suppressAlerts: true
```

## Web Dashboard

CronJob Guardian includes a feature-rich web dashboard accessible on port 8080.

### Dashboard Features

- **Overview**: Summary cards showing total CronJobs, health status, and active alerts
- **CronJob Details**: Per-job metrics, execution history, and performance charts
- **SLA Compliance**: Dedicated page showing which jobs are meeting/breaching SLA targets
- **Monitors**: View and manage CronJobMonitor resources with aggregate metrics
- **Alert Channels**: Manage and test notification channels
- **Alert History**: Browse past alerts with filtering

### Visualization Features

- **Success Rate Charts**: Bar charts with 14/30/90 day range selection and week-over-week comparison
- **Duration Trend Charts**: Line charts showing P50/P95 with regression detection and baseline indicators
- **Health Heatmap**: GitHub-style calendar view showing daily success rates (30/60/90 days)
- **Monitor Aggregate Charts**: Cross-CronJob comparison charts, health distribution pie charts
- **SLA Dashboard**: Summary cards and compliance table with status indicators and trend arrows

### Export Features

- **CSV Export**: Download execution history or SLA reports as CSV files
- **PDF Reports**: Generate printable reports with metrics, charts summary, and alert history

### Accessing the Dashboard

```bash
kubectl port-forward -n guardian-system svc/cronjob-guardian 8080:8080
```

Then open http://localhost:8080 in your browser.

## REST API

The operator exposes a REST API for programmatic access.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Operator health status |
| GET | `/api/v1/stats` | Summary statistics |
| GET | `/api/v1/monitors` | List all monitors |
| GET | `/api/v1/monitors/{ns}/{name}` | Get monitor details |
| GET | `/api/v1/cronjobs` | List monitored CronJobs |
| GET | `/api/v1/cronjobs/{ns}/{name}` | Get CronJob details with metrics |
| GET | `/api/v1/cronjobs/{ns}/{name}/executions` | Execution history |
| POST | `/api/v1/cronjobs/{ns}/{name}/trigger` | Manually trigger a job |
| POST | `/api/v1/cronjobs/{ns}/{name}/suspend` | Suspend a CronJob |
| POST | `/api/v1/cronjobs/{ns}/{name}/resume` | Resume a CronJob |
| DELETE | `/api/v1/cronjobs/{ns}/{name}/history` | Delete execution history |
| GET | `/api/v1/alerts` | Active alerts |
| GET | `/api/v1/alerts/history` | Alert history |
| GET | `/api/v1/channels` | List alert channels |
| GET | `/api/v1/channels/{name}` | Get channel details |
| POST | `/api/v1/channels/{name}/test` | Send test alert |
| GET | `/api/v1/config` | Get operator configuration |
| GET | `/api/v1/admin/storage-stats` | Storage statistics |
| POST | `/api/v1/admin/prune` | Prune old execution records |
| GET | `/metrics` | Prometheus metrics |

### Example

```bash
# Get all monitored CronJobs
curl http://localhost:8080/api/v1/cronjobs

# Get execution history for a specific job
curl http://localhost:8080/api/v1/cronjobs/production/daily-backup/executions

# Get Prometheus metrics
curl http://localhost:8080/metrics
```

## Storage Backends

CronJob Guardian supports multiple storage backends for execution history.

### SQLite (Default)

Lightweight, requires a PVC. Good for single-replica deployments.

```yaml
# In GuardianConfig
spec:
  storage:
    type: sqlite
    sqlite:
      path: /data/guardian.db
```

### PostgreSQL

For high-availability deployments with multiple replicas.

```yaml
spec:
  storage:
    type: postgresql
    postgresql:
      host: postgres.database.svc
      port: 5432
      database: guardian
      secretRef:
        name: postgres-credentials
        namespace: guardian-system
```

### MySQL/MariaDB

Alternative enterprise database option.

```yaml
spec:
  storage:
    type: mysql
    mysql:
      host: mysql.database.svc
      port: 3306
      database: guardian
      secretRef:
        name: mysql-credentials
        namespace: guardian-system
```

## GuardianConfig

Global operator settings are configured via the `GuardianConfig` resource (singleton named "default").

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: GuardianConfig
metadata:
  name: default
spec:
  # Background task intervals
  deadManSwitchInterval: 1m
  slaRecalculationInterval: 5m

  # History retention
  historyRetention:
    defaultDays: 30
    maxDays: 90

  # Storage configuration
  storage:
    type: sqlite
    sqlite:
      path: /data/guardian.db

  # Namespaces to ignore
  ignoredNamespaces:
    - kube-system
    - kube-public
```

## Development

### Prerequisites

- Go 1.23+
- Docker
- Kind (for local testing)
- Node.js 20+ or Bun (for UI development)

### Building

```bash
# Build the operator binary
make build

# Build Docker image
make docker-build IMG=cronjob-guardian:dev

# Build UI
cd ui && pnpm build

# Generate CRDs and code
make manifests generate

# Run linters
make lint

# Run tests
make test
```

### Running Locally

```bash
# Install CRDs
make install

# Run the operator locally
make run

# Or run in a local Kind cluster
make test-e2e
```

### Project Structure

```
├── api/v1alpha1/          # CRD type definitions
├── cmd/                   # Main entrypoint
├── config/                # Kubernetes manifests
│   ├── crd/              # Generated CRD YAML
│   ├── manager/          # Operator deployment
│   ├── rbac/             # RBAC rules
│   └── samples/          # Example CRs
├── internal/
│   ├── controller/       # Kubernetes reconcilers
│   ├── alerting/         # Alert dispatcher and channels
│   ├── analyzer/         # SLA calculation
│   ├── scheduler/        # Background tasks (dead-man switch, SLA recalc)
│   ├── store/            # Database abstraction
│   ├── api/              # REST API server
│   ├── metrics/          # Prometheus metrics
│   └── config/           # Configuration handling
└── ui/                   # Next.js dashboard
    └── src/
        ├── app/          # Pages (dashboard, monitors, sla, alerts, etc.)
        ├── components/   # React components
        └── lib/          # API client and utilities
```

## Uninstalling

```bash
# Remove all CronJobMonitor and AlertChannel resources
kubectl delete cronjobmonitors --all-namespaces --all
kubectl delete alertchannels --all
kubectl delete guardianconfigs --all

# Remove the operator
make undeploy

# Remove CRDs
make uninstall
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
