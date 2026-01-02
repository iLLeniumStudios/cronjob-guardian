---
sidebar_position: 1
title: Introduction
description: What is CronJob Guardian and why do you need it?
---

# What is CronJob Guardian?

CronJob Guardian is a Kubernetes operator that provides intelligent monitoring, SLA tracking, and alerting for CronJobs.

## The Problem

CronJobs power critical operations in Kubernetes clusters:

- **Database backups** - Nightly dumps, point-in-time recovery
- **ETL pipelines** - Data transformations, syncs, aggregations
- **Reports** - Business metrics, compliance reports, billing
- **Maintenance** - Cache warming, cleanup, health checks

But Kubernetes provides **no built-in monitoring** for CronJobs. When jobs fail silently or stop running entirely, you only find out when it's too late—missing backups, stale data, or compliance violations.

## The Solution

CronJob Guardian watches your CronJobs and alerts you when something goes wrong:

| Issue | Detection |
|-------|-----------|
| **Job failures** | Immediate alerts with logs, events, and suggested fixes |
| **Missed schedules** | Dead-man's switch detects when jobs don't run |
| **Performance regressions** | Duration tracking catches jobs slowing down |
| **SLA breaches** | Success rate monitoring against your thresholds |

## Key Capabilities

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

### Operations

- **Maintenance Windows**: Suppress alerts during scheduled maintenance
- **Built-in Dashboard**: Feature-rich web UI for monitoring and analytics
- **REST API**: Programmatic access to all monitoring data
- **Multiple Storage Backends**: SQLite (default), PostgreSQL, or MySQL

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
│   │   │  • Job         │◀──│  • SLA recalc  │──▶│  • Dedup       │           │   │   │
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

## How It Works

1. **Create `CronJobMonitor` resources** to define what to watch (label selectors, SLA thresholds)
2. **Create `AlertChannel` resources** to configure alert destinations (Slack, PagerDuty, etc.)
3. **The operator watches CronJobs and Jobs**, records executions to the store
4. **Background schedulers check** for missed schedules, SLA breaches, and duration regressions
5. **When issues are detected**, alerts are dispatched with context (logs, events, suggested fixes)

## Next Steps

- [Installation](./installation.md) - Install CronJob Guardian in your cluster
- [Quick Start](./quick-start.md) - Create your first monitor in 5 minutes
