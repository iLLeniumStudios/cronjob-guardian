# CronJob Guardian Operator

A Kubernetes operator that provides intelligent monitoring, SLA tracking, and auto-remediation for CronJobs.

## Project Info

- **Repository:** github.com/iLLeniumStudios/cronjob-guardian
- **API Group:** guardian.illenium.net
- **API Version:** v1alpha1
- **Go Module:** github.com/iLLeniumStudios/cronjob-guardian

## Project Initialization

Use operator-sdk to initialize the project:

```bash
# Initialize the project
operator-sdk init --domain illenium.net --repo github.com/iLLeniumStudios/cronjob-guardian

# Create APIs (CRDs)
operator-sdk create api --group guardian --version v1alpha1 --kind CronJobMonitor --resource --controller
operator-sdk create api --group guardian --version v1alpha1 --kind AlertChannel --resource --controller
operator-sdk create api --group guardian --version v1alpha1 --kind GuardianConfig --resource --controller
```

## Directory Structure

```
cronjob-guardian/
├── CLAUDE.md                     # This file
├── api/
│   └── v1alpha1/
│       ├── cronjobmonitor_types.go
│       ├── alertchannel_types.go
│       ├── guardianconfig_types.go
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
├── cmd/
│   └── main.go
├── config/
│   ├── crd/
│   ├── manager/
│   ├── rbac/
│   └── samples/
├── internal/
│   ├── controller/
│   │   ├── cronjobmonitor_controller.go
│   │   ├── alertchannel_controller.go
│   │   ├── guardianconfig_controller.go
│   │   └── job_handler.go
│   ├── store/
│   │   ├── interface.go
│   │   ├── sqlite.go
│   │   ├── postgres.go
│   │   └── mysql.go
│   ├── alerting/
│   │   ├── dispatcher.go
│   │   ├── slack.go
│   │   ├── pagerduty.go
│   │   ├── webhook.go
│   │   └── email.go
│   ├── analyzer/
│   │   └── sla.go
│   ├── remediation/
│   │   └── engine.go
│   ├── scheduler/
│   │   ├── deadman.go
│   │   └── sla_recalc.go
│   └── api/
│       ├── server.go
│       └── handlers.go
├── ui/                           # React + Vite frontend
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   ├── architecture.md
│   ├── api-reference.md
│   ├── ui-spec.md
│   └── components/
│       ├── controllers.md
│       ├── store.md
│       ├── alerting.md
│       ├── analyzer.md
│       ├── remediation.md
│       └── scheduler.md
└── examples/
    └── *.yaml
```

## CRDs Overview

| CRD | Scope | Purpose |
|-----|-------|---------|
| CronJobMonitor | Namespaced | Main config: what to monitor, SLA thresholds, alerting, remediation |
| AlertChannel | Cluster | Reusable alert destinations (Slack, PagerDuty, webhook, email) |
| GuardianConfig | Cluster | Global operator settings (singleton named "default") |

## Key Features

1. **Dead-man's switch** - Alert if CronJob doesn't run within expected window
2. **SLA tracking** - Success rate, duration percentiles, regression detection
3. **Smart alerting** - Logs, events, suggested fixes included in alerts
4. **Dependency awareness** - Track upstream failures, suppress cascading alerts
5. **Auto-remediation** - Kill stuck jobs, auto-retry failed jobs
6. **Multiple alert channels** - Slack, PagerDuty, webhooks, email
7. **Pluggable storage** - SQLite (default), PostgreSQL, MySQL/MariaDB
8. **Embedded UI** - Lightweight web dashboard

## Storage Backend

The operator uses a pluggable storage interface for execution history. SQLite is the default (requires PVC), with optional PostgreSQL or MySQL support.

See `docs/components/store.md` for implementation details.

## Development Workflow

1. Read the component docs in `docs/components/` before implementing
2. Start with the CRD types in `api/v1alpha1/`
3. Implement the store interface first (SQLite backend)
4. Implement controllers one at a time
5. Add alerting integrations
6. Implement background schedulers
7. Build the UI last

## Testing

```bash
# Run unit tests
make test

# Run e2e tests (requires kind cluster)
make test-e2e

# Run locally against a cluster
make run
```

## Building

```bash
# Build the operator binary
make build

# Build and push Docker image
make docker-build docker-push IMG=<registry>/cronjob-guardian:tag

# Build the UI
cd ui && npm run build

# Generate manifests
make manifests
```

## File References

When implementing, refer to these documentation files:

- **CRD type definitions:** `docs/crds.md`
- **Example CRs:** `docs/examples.md`  
- **Controller logic:** `docs/components/controllers.md`
- **Store implementation:** `docs/components/store.md`
- **Alerting system:** `docs/components/alerting.md`
- **SLA analyzer:** `docs/components/analyzer.md`
- **Remediation engine:** `docs/components/remediation.md`
- **Background schedulers:** `docs/components/scheduler.md`
- **REST API:** `docs/api-reference.md`
- **UI specification:** `docs/ui-spec.md`
