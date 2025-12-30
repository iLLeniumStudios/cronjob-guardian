# CronJob Guardian

[![Go Version](https://img.shields.io/github/go-mod/go-version/iLLeniumStudios/cronjob-guardian)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/iLLeniumStudios/cronjob-guardian)](https://goreportcard.com/report/github.com/iLLeniumStudios/cronjob-guardian)

A Kubernetes operator that provides intelligent monitoring, SLA tracking, and auto-remediation for CronJobs. Stop firefighting failed batch jobs and let CronJob Guardian watch over your scheduled workloads.

## Why CronJob Guardian?

Kubernetes CronJobs are critical for many operations: database backups, report generation, data pipelines, cache warming, and more. When they fail silently or stop running, the consequences can be severe.

CronJob Guardian solves common pain points:

- **Silent failures**: Get alerted immediately when jobs fail, with logs and suggested fixes
- **Missed schedules**: Dead-man's switch alerts when jobs don't run on time
- **Performance degradation**: Track SLAs and detect when jobs slow down
- **Stuck jobs**: Automatically kill jobs that run too long
- **Alert fatigue**: Smart deduplication, maintenance windows, and severity routing

## Features

### Monitoring

- **Dead-Man's Switch**: Alert when CronJobs don't run within expected windows. Auto-detects expected intervals from cron schedules.
- **SLA Tracking**: Monitor success rates, duration percentiles (P50/P95/P99), and detect performance regressions.
- **Execution History**: Store and query job execution records with logs and events.

### Alerting

- **Multiple Channels**: Slack, PagerDuty, generic webhooks, and email
- **Smart Context**: Alerts include pod logs, Kubernetes events, and suggested fixes
- **Deduplication**: Prevent alert storms with configurable suppression windows
- **Severity Routing**: Send critical alerts to PagerDuty, warnings to Slack

### Auto-Remediation

- **Kill Stuck Jobs**: Automatically terminate jobs running beyond thresholds
- **Auto-Retry**: Retry failed jobs with configurable limits and delays
- **Dry-Run Mode**: Test remediation policies without taking action

### Operations

- **Maintenance Windows**: Suppress alerts during scheduled maintenance
- **Built-in Dashboard**: Web UI for monitoring status and browsing history
- **REST API**: Programmatic access to all monitoring data
- **Multiple Storage Backends**: SQLite (default), PostgreSQL, or MySQL

## Quick Start

### Prerequisites

- Kubernetes 1.25+
- kubectl configured with cluster access
- Helm 3.x (optional)

### Installation

```bash
# Install CRDs
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
    autoFromSchedule:
      enabled: true
      buffer: 1h
  sla:
    minSuccessRate: 99
    maxDuration: 30m
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
    # Option 1: Auto-detect from cron schedule
    autoFromSchedule:
      enabled: true
      buffer: 1h  # Extra buffer time
      missedScheduleThreshold: 2  # Alert after 2 missed runs

    # Option 2: Explicit threshold
    maxTimeSinceLastSuccess: 25h  # For daily jobs
```

#### SLA Tracking

Monitor success rates and performance:

```yaml
spec:
  sla:
    minSuccessRate: 95        # Minimum 95% success rate
    windowDays: 7             # Over rolling 7-day window
    maxDuration: 1h           # Alert if job exceeds 1 hour
    durationPercentiles:      # Track these percentiles
      - 50
      - 95
      - 99
    durationRegressionThreshold: 50  # Alert if P95 increases 50%
```

#### Auto-Remediation

Automatically fix common issues:

```yaml
spec:
  remediation:
    enabled: true
    dryRun: false  # Set true to test without action

    # Kill jobs stuck for too long
    killStuckJobs:
      enabled: true
      afterDuration: 2h
      deletePolicy: Foreground  # or Orphan

    # Retry failed jobs
    autoRetry:
      enabled: true
      maxRetries: 3
      delay: 5m
      retryOn:  # Only retry specific exit codes
        - 1     # Generic error
        - 137   # OOMKilled
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
      suppressRemediation: true
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
    defaultChannel: "#alerts"  # Override webhook default
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
    maxTimeSinceLastSuccess: 25h  # Daily backups with buffer
  sla:
    minSuccessRate: 100  # Backups must never fail
  remediation:
    autoRetry:
      enabled: true
      maxRetries: 3
      delay: 10m
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
    minSuccessRate: 95
    maxDuration: 2h
    durationRegressionThreshold: 30  # Alert if 30% slower
  remediation:
    killStuckJobs:
      enabled: true
      afterDuration: 3h
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
    autoFromSchedule:
      enabled: true
      buffer: 30m
  maintenanceWindows:
    - name: quarter-end
      schedule: "0 0 1 1,4,7,10 *"  # First day of quarter
      duration: 24h
      suppressAlerts: true
```

## Web Dashboard

CronJob Guardian includes a built-in web dashboard accessible on port 8080.

**Features:**
- Overview of all monitors and their health status
- Real-time alert feed
- Execution history with logs and events
- CronJob success rate trends
- Alert channel management and testing

Access the dashboard:

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
| GET | `/api/v1/monitors` | List all monitors |
| GET | `/api/v1/monitors/{ns}/{name}` | Get monitor details |
| GET | `/api/v1/cronjobs` | List monitored CronJobs |
| GET | `/api/v1/cronjobs/{ns}/{name}/executions` | Execution history |
| GET | `/api/v1/alerts` | Active alerts |
| GET | `/api/v1/alerts/history` | Alert history |
| POST | `/api/v1/channels/{name}/test` | Send test alert |

### Example

```bash
# Get all monitored CronJobs
curl http://localhost:8080/api/v1/cronjobs

# Get execution history for a specific job
curl http://localhost:8080/api/v1/cronjobs/production/daily-backup/executions
```

## Storage Backends

CronJob Guardian supports multiple storage backends for execution history.

### SQLite (Default)

Lightweight, requires a PVC. Good for single-replica deployments.

```yaml
# In GuardianConfig or operator flags
storage:
  type: sqlite
  sqlite:
    path: /data/guardian.db
```

### PostgreSQL

For high-availability deployments with multiple replicas.

```yaml
storage:
  type: postgres
  postgres:
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

## Development

### Prerequisites

- Go 1.23+
- Docker
- Kind (for local testing)
- Bun (for UI development)

### Building

```bash
# Build the operator binary
make build

# Build Docker image
make docker-build IMG=cronjob-guardian:dev

# Build UI
make ui-build

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
├── cmd/                   # Main entrypoint and embedded UI
├── config/                # Kubernetes manifests
│   ├── crd/              # Generated CRD YAML
│   ├── manager/          # Operator deployment
│   └── rbac/             # RBAC rules
├── internal/
│   ├── controller/       # Kubernetes reconcilers
│   ├── alerting/         # Alert dispatcher and channels
│   ├── analyzer/         # SLA calculation
│   ├── remediation/      # Auto-remediation engine
│   ├── scheduler/        # Background tasks
│   ├── store/            # Database abstraction
│   └── api/              # REST API server
└── ui/                   # Next.js dashboard
```

## Uninstalling

```bash
# Remove all CronJobMonitor and AlertChannel resources
kubectl delete cronjobmonitors --all-namespaces --all
kubectl delete alertchannels --all

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
