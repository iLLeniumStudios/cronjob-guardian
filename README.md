# CronJob Guardian

[![GitHub Release](https://img.shields.io/github/v/release/iLLeniumStudios/cronjob-guardian?logo=github&sort=semver)](https://github.com/iLLeniumStudios/cronjob-guardian/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/iLLeniumStudios/cronjob-guardian/ci.yaml?logo=githubactions&logoColor=white&label=CI)](https://github.com/iLLeniumStudios/cronjob-guardian/actions/workflows/ci.yaml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/iLLeniumStudios/cronjob-guardian?logo=go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/iLLeniumStudios/cronjob-guardian)](https://goreportcard.com/report/github.com/iLLeniumStudios/cronjob-guardian)
[![License](https://img.shields.io/github/license/iLLeniumStudios/cronjob-guardian)](https://github.com/iLLeniumStudios/cronjob-guardian/blob/main/LICENSE)

A Kubernetes operator for monitoring CronJobs with SLA tracking, intelligent alerting, and a built-in dashboard.

![CronJob Guardian Dashboard](docs/images/dashboard.png)

## Why CronJob Guardian?

CronJobs power critical operations—backups, ETL pipelines, reports, cache warming—but Kubernetes provides no built-in monitoring for them. When jobs fail silently or stop running, you only find out when it's too late.

CronJob Guardian watches your CronJobs and alerts you when something goes wrong:

- **Job failures** with logs, events, and suggested fixes
- **Missed schedules** via dead-man's switch detection
- **Performance regressions** when jobs slow down over time
- **SLA breaches** when success rates drop below thresholds

## Architecture

```
                                    Kubernetes Cluster
┌──────────────────────────────────────────────────────────────────────────────────┐
│                                                                                  │
│   ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐                │
│   │ CronJobMonitor  │   │  AlertChannel   │   │    CronJobs     │                │
│   │     (CRD)       │   │     (CRD)       │   │    & Jobs       │                │
│   └────────┬────────┘   └────────┬────────┘   └────────┬────────┘                │
│            │                     │                     │                         │
│            └─────────────────────┼─────────────────────┘                         │
│                                  ▼                                               │
│   ┌──────────────────────────────────────────────────────────────────────────┐   │
│   │                      CronJob Guardian Operator                           │   │
│   │                                                                          │   │
│   │   ┌────────────────┐   ┌────────────────┐   ┌────────────────┐           │   │
│   │   │  Controllers   │   │   Schedulers   │   │    Alerting    │           │   │
│   │   │                │   │                │   │   Dispatcher   │───────────────────┐
│   │   │  • Monitor     │   │  • Dead-man    │   │                │           │   │   │
│   │   │  • Job         │◀─│  • SLA recalc  │─▶│  • Dedup       │           │   │   │
│   │   │  • Channel     │   │  • Prune       │   │  • Rate limit  │           │   │   │
│   │   └───────┬────────┘   └────────────────┘   └────────────────┘           │   │   │
│   │           │                                                              │   │   │
│   │           ▼                                                              │   │   │
│   │   ┌─────────────────────────────────────┐   ┌────────────────┐           │   │   │
│   │   │              Store                  │   │   Prometheus   │           │   │   │
│   │   │    SQLite / PostgreSQL / MySQL      │   │    Metrics     │───────────────────┐
│   │   │                                     │   │   :8443        │           │   │   │
│   │   │  • Executions  • Logs  • Alerts     │   └────────────────┘           │   │   │
│   │   └──────────────────┬──────────────────┘                                │   │   │
│   │                      │                                                   │   │   │
│   │   ┌──────────────────┴──────────────────┐                                │   │   │
│   │   │        Web UI & REST API            │                                │   │   │
│   │   │             :8080                   │────────────────────────────────────────┐
│   │   └─────────────────────────────────────┘                                │   │   │
│   └──────────────────────────────────────────────────────────────────────────┘   │   │
│                                                                                  │   │
└──────────────────────────────────────────────────────────────────────────────────┘   │
                                                                                       │
     ┌─────────────────────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              External Services                                  │
│                                                                                 │
│   ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐     │
│   │   Slack   │  │ PagerDuty │  │  Webhook  │  │   Email   │  │Prometheus │     │
│   └───────────┘  └───────────┘  └───────────┘  └───────────┘  └───────────┘     │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

**How it works:**
1. Create `CronJobMonitor` resources to define what to watch (label selectors, SLA thresholds)
2. Create `AlertChannel` resources to configure alert destinations (Slack, PagerDuty, etc.)
3. The operator watches CronJobs and Jobs, records executions to the store
4. Background schedulers check for missed schedules, SLA breaches, and duration regressions
5. When issues are detected, alerts are dispatched with context (logs, events, suggested fixes)

## Features

### Monitoring

- **Dead-Man's Switch**: Alert when CronJobs don't run within expected windows. Auto-detects expected intervals from cron schedules.
- **SLA Tracking**: Monitor success rates, duration percentiles (P50/P95/P99), and detect performance regressions.
- **Execution History**: Store and query job execution records with logs and events.
- **Prometheus Metrics**: Export metrics for integration with existing monitoring infrastructure.

### Alerting

- **Multiple Channels**: Slack, PagerDuty, generic webhooks, and email
- **Rich Context**: Alerts include pod logs, Kubernetes events, and suggested fixes
- **Deduplication**: Configurable suppression windows and alert delays for flaky jobs
- **Severity Routing**: Route critical and warning alerts to different channels

<!-- TODO: Add Slack alert example screenshot -->
<!-- ![Slack Alert Example](docs/images/slack-alert.png) -->

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

- Kubernetes 1.26+
- kubectl configured with cluster access
- Helm 3.8+ (for OCI registry support)

### Installation

#### Helm (Recommended)

CronJob Guardian is distributed as an OCI Helm chart:

```bash
# Install with default configuration (SQLite storage)
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace

# Install with custom values
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace \
  --values values.yaml
```

#### Quick Start with PostgreSQL

```bash
# Create a secret for database credentials
kubectl create namespace cronjob-guardian
kubectl create secret generic postgres-credentials \
  --namespace cronjob-guardian \
  --from-literal=password=your-secure-password

# Install with PostgreSQL storage
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --set config.storage.type=postgres \
  --set config.storage.postgres.host=postgres.database.svc \
  --set config.storage.postgres.database=guardian \
  --set config.storage.postgres.username=guardian \
  --set config.storage.postgres.existingSecret=postgres-credentials
```

#### High Availability Setup

```bash
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace \
  --set replicaCount=2 \
  --set leaderElection.enabled=true \
  --set config.storage.type=postgres \
  --set config.storage.postgres.host=postgres.database.svc \
  --set config.storage.postgres.database=guardian \
  --set config.storage.postgres.username=guardian \
  --set config.storage.postgres.existingSecret=postgres-credentials
```

#### Install from Source

```bash
# Clone the repository
git clone https://github.com/iLLeniumStudios/cronjob-guardian.git
cd cronjob-guardian

# Install using local chart
helm install cronjob-guardian ./deploy/helm/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace
```

#### kubectl (Alternative)

```bash
# Install CRDs and operator
kubectl apply -f https://raw.githubusercontent.com/iLLeniumStudios/cronjob-guardian/main/dist/install.yaml
```

Or build from source:

```bash
make docker-build docker-push IMG=your-registry/cronjob-guardian:latest
make deploy IMG=your-registry/cronjob-guardian:latest
```

### Helm Configuration

The Helm chart supports extensive configuration for storage backends, high availability, metrics, and more.

See the **[Helm Chart Documentation](deploy/helm/README.md)** for complete configuration reference including:

- Storage backends (SQLite, PostgreSQL, MySQL)
- High availability with leader election
- Ingress and OpenShift Route support for UI access
- Prometheus ServiceMonitor integration
- Resource limits and scheduling
- All available values and their defaults

### Basic Setup

1. **Create an AlertChannel** for notifications:

```bash
kubectl apply -f examples/alertchannels/slack.yaml
```

2. **Create a CronJobMonitor** to watch your jobs:

```bash
kubectl apply -f examples/monitors/basic.yaml
```

See the **[examples/](examples/)** directory for complete configuration examples.

## Configuration

### CronJobMonitor

The main resource for configuring what to monitor. Select CronJobs by labels, expressions, names, or namespaces.

| Selector Pattern | Example |
|------------------|---------|
| All in namespace | `selector: {}` |
| By labels | `matchLabels: {tier: critical}` |
| By expressions | `matchExpressions: [{key: tier, operator: In, values: [critical]}]` |
| By names | `matchNames: [daily-backup, weekly-report]` |
| Multiple namespaces | `namespaces: [prod, staging]` |
| Namespace labels | `namespaceSelector: {matchLabels: {env: prod}}` |
| Cluster-wide | `allNamespaces: true` |

See [examples/monitors/](examples/monitors/) for complete examples of each pattern.

#### Key Features

| Feature | Description |
|---------|-------------|
| **Dead-Man's Switch** | Alert when jobs don't run within expected window |
| **SLA Tracking** | Monitor success rates and duration percentiles |
| **Maintenance Windows** | Suppress alerts during planned maintenance |
| **Severity Routing** | Route critical/warning alerts to different channels |

### AlertChannel

Define where to send alerts. AlertChannel resources are cluster-scoped.

| Type | Description | Example |
|------|-------------|---------|
| **Slack** | Incoming webhook | [slack.yaml](examples/alertchannels/slack.yaml) |
| **PagerDuty** | Events API | [pagerduty.yaml](examples/alertchannels/pagerduty.yaml) |
| **Webhook** | Generic HTTP | [webhook.yaml](examples/alertchannels/webhook.yaml) |
| **Email** | SMTP | [email.yaml](examples/alertchannels/email.yaml) |

## Native Kubernetes Features

CronJob Guardian focuses on monitoring and alerting, leaving job execution control to native Kubernetes features:

| Feature | Spec Field | Description | Example |
|---------|------------|-------------|---------|
| **Timeout** | `activeDeadlineSeconds` | Kill stuck jobs | [with-timeout.yaml](examples/cronjobs/with-timeout.yaml) |
| **Retry** | `backoffLimit` | Auto-retry failed jobs | [with-retry.yaml](examples/cronjobs/with-retry.yaml) |
| **Timezone** | `timeZone` | Schedule in specific timezone | [with-timezone.yaml](examples/cronjobs/with-timezone.yaml) |
| **Concurrency** | `concurrencyPolicy` | Prevent overlapping runs | [with-concurrency.yaml](examples/cronjobs/with-concurrency.yaml) |

## Use Cases

Example monitors for common scenarios:

| Use Case | Description | Example |
|----------|-------------|---------|
| **Database Backups** | Critical backups with 100% SLA | [database-backups.yaml](examples/monitors/database-backups.yaml) |
| **Data Pipelines** | ETL with performance tracking | [data-pipeline.yaml](examples/monitors/data-pipeline.yaml) |
| **Reports** | Business reports with maintenance windows | [financial-reports.yaml](examples/monitors/financial-reports.yaml) |
| **Full Featured** | All configuration options | [full-featured.yaml](examples/monitors/full-featured.yaml) |

## Web Dashboard

CronJob Guardian includes a feature-rich web UI that serves both an interactive dashboard and REST API on port 8080.

<!-- TODO: Add dashboard screenshots -->
<!--
<p align="center">
  <img src="docs/images/dashboard-overview.png" alt="Dashboard Overview" width="800">
</p>

<p align="center">
  <img src="docs/images/cronjob-detail.png" alt="CronJob Detail View" width="400">
  <img src="docs/images/sla-dashboard.png" alt="SLA Dashboard" width="400">
</p>
-->

### Dashboard Pages

| Page | Description |
|------|-------------|
| **Overview** | Summary cards, CronJob table with health status, active alerts panel |
| **CronJob Details** | Per-job metrics, execution history, duration/success charts, health heatmap |
| **Monitors** | CronJobMonitor list with aggregate metrics and cronjob counts |
| **Channels** | AlertChannel management with test functionality |
| **Alerts** | Alert history with filtering by type, severity, and time range |
| **SLA** | SLA compliance dashboard with breach tracking |
| **Settings** | System config, storage stats, data pruning, and **Pattern Tester** |

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
kubectl port-forward -n cronjob-guardian svc/cronjob-guardian-ui 8080:8080
```

Then open http://localhost:8080 in your browser.

For production deployments, you can expose the UI via Ingress or OpenShift Route. See the [Helm Chart Documentation](deploy/helm/README.md) for configuration details.

## REST API

The operator exposes a REST API for programmatic access to monitoring data, CronJob management, and alerting.

```bash
# Get all monitored CronJobs
curl http://localhost:8080/api/v1/cronjobs

# Get execution history
curl http://localhost:8080/api/v1/cronjobs/production/daily-backup/executions

# Trigger a job manually
curl -X POST http://localhost:8080/api/v1/cronjobs/production/daily-backup/trigger
```

See the **[API Reference](docs/api.md)** for complete endpoint documentation.

## Suggested Fixes

CronJob Guardian includes intelligent fix suggestions that analyze failure context (exit codes, reasons, logs, events) and provide actionable guidance in alerts.

### Built-in Patterns

| Pattern | Trigger | Suggestion |
|---------|---------|------------|
| OOMKilled | Reason: `OOMKilled` | Increase `resources.limits.memory` |
| SIGKILL (137) | Exit code 137 | Check for OOM, inspect pod state |
| SIGTERM (143) | Exit code 143 | Check `activeDeadlineSeconds` or eviction |
| ImagePullBackOff | Reason match | Verify image name and `imagePullSecrets` |
| CrashLoopBackOff | Reason match | Check application startup logs |
| ConfigError | Reason: `CreateContainerConfigError` | Verify Secret/ConfigMap references |
| DeadlineExceeded | Reason match | Increase deadline or optimize job |
| BackoffLimitExceeded | Reason match | Check logs from failed attempts |
| Evicted | Reason match | Check node pressure, set pod priority |
| FailedScheduling | Event pattern | Check resources, taints, affinity |

### Custom Patterns

Define custom patterns in your CronJobMonitor to match application-specific failures:

```yaml
alerting:
  suggestedFixPatterns:
    - name: db-connection-failed
      match:
        logPattern: "connection refused.*:5432|ECONNREFUSED"
      suggestion: "PostgreSQL connection failed. Check: kubectl get pods -n {{.Namespace}} -l app=postgres"
      priority: 150  # Higher than built-ins (1-100)
    - name: s3-access-denied
      match:
        logPattern: "AccessDenied|NoCredentialProviders"
      suggestion: "S3 access denied. Verify IAM role and bucket policy."
      priority: 140
```

### Pattern Tester

Test patterns before deploying via the **Settings > Pattern Tester** page in the UI. Enter match criteria and sample failure data to verify your pattern works correctly.

### Template Variables

Suggestions support Go template variables:
- `{{.Namespace}}` - CronJob namespace
- `{{.Name}}` - CronJob name
- `{{.JobName}}` - Job name (includes timestamp suffix)
- `{{.ExitCode}}` - Container exit code
- `{{.Reason}}` - Termination reason

## Storage Backends

CronJob Guardian supports multiple storage backends for execution history:

| Backend | Use Case | HA Support |
|---------|----------|------------|
| **SQLite** (default) | Single-replica, lightweight | No |
| **PostgreSQL** | Production, high-availability | Yes |
| **MySQL/MariaDB** | Enterprise environments | Yes |

Configure via Helm values or the `GuardianConfig` resource. See [Helm Chart Documentation](deploy/helm/README.md) for details.

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

### Updating Helm Documentation

Before releasing, regenerate the Helm chart documentation:

```bash
# Generate both values.schema.json and update README.md Values section
make helm-docs

# Or run individually:
make helm-schema  # Generate values.schema.json only
make helm-readme  # Update README.md Values section only

# Sync CRDs if API types changed
make helm-sync-crds
```

This uses [helm-tool](https://github.com/cert-manager/helm-tool) to:
- Generate `values.schema.json` from `values.yaml` comments (enables IDE autocompletion)
- Update the `## Values` section in the chart README with HTML tables organized by section

**Documenting values.yaml:**
- Use `# Description` comments above properties to add descriptions
- Use `# +docs:section=SectionName` to organize values into sections
- Section comments can include additional description text on following lines

Example:
```yaml
# +docs:section=Storage
# Configuration for the storage backend.

config:
  storage:
    # Storage type: sqlite, postgres, or mysql
    type: sqlite
```

### Project Structure

```
├── api/v1alpha1/          # CRD type definitions
├── cmd/                   # Main entrypoint
├── config/                # Kubernetes manifests (CRDs, RBAC, etc.)
├── deploy/helm/           # Helm chart
├── docs/                  # Documentation
│   └── api.md            # REST API reference
├── examples/              # Example configurations
│   ├── alertchannels/    # AlertChannel examples
│   ├── monitors/         # CronJobMonitor examples
│   └── cronjobs/         # CronJob examples
├── internal/
│   ├── controller/       # Kubernetes reconcilers
│   ├── alerting/         # Alert dispatcher and channels
│   ├── analyzer/         # SLA calculation
│   ├── scheduler/        # Background tasks
│   ├── store/            # Database abstraction (GORM)
│   ├── api/              # REST API server
│   └── config/           # Configuration handling
└── ui/                   # React dashboard
```

## Uninstalling

### Helm

```bash
# Uninstall the release
helm uninstall cronjob-guardian --namespace cronjob-guardian

# Delete CRDs (optional - this removes all CronJobMonitor and AlertChannel data)
kubectl delete crd cronjobmonitors.guardian.illenium.net
kubectl delete crd alertchannels.guardian.illenium.net

# Delete the namespace
kubectl delete namespace cronjob-guardian
```

### kubectl

```bash
# Remove all CronJobMonitor and AlertChannel resources
kubectl delete cronjobmonitors --all-namespaces --all
kubectl delete alertchannels --all-namespaces --all

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
