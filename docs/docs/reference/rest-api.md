---
sidebar_position: 3
title: REST API
description: REST API reference
---

# REST API Reference

CronJob Guardian exposes a REST API on port 8080 for programmatic access.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

The API does not require authentication by default. For production, use network policies or an ingress with authentication.

## Endpoints

### CronJobs

#### List CronJobs

```http
GET /api/v1/cronjobs
```

Query parameters:
- `namespace` - Filter by namespace
- `status` - Filter by status (healthy, warning, critical)
- `monitor` - Filter by monitor name

Response:
```json
{
  "items": [
    {
      "namespace": "production",
      "name": "daily-backup",
      "status": "healthy",
      "successRate": 98.5,
      "lastRun": "2024-01-15T02:00:00Z",
      "nextRun": "2024-01-16T02:00:00Z"
    }
  ],
  "total": 1
}
```

#### Get CronJob

```http
GET /api/v1/cronjobs/{namespace}/{name}
```

Response:
```json
{
  "namespace": "production",
  "name": "daily-backup",
  "status": "healthy",
  "schedule": "0 2 * * *",
  "metrics": {
    "successRate": 98.5,
    "avgDuration": 245.5,
    "p50Duration": 230.0,
    "p95Duration": 310.0,
    "totalExecutions": 100
  },
  "lastRun": "2024-01-15T02:00:00Z",
  "nextRun": "2024-01-16T02:00:00Z"
}
```

#### Get Executions

```http
GET /api/v1/cronjobs/{namespace}/{name}/executions
```

Query parameters:
- `limit` - Number of results (default: 50)
- `offset` - Pagination offset
- `status` - Filter by status (success, failed)
- `from` - Start time (RFC3339)
- `to` - End time (RFC3339)

Response:
```json
{
  "items": [
    {
      "id": "exec-123",
      "jobName": "daily-backup-28374658",
      "status": "success",
      "startTime": "2024-01-15T02:00:00Z",
      "completionTime": "2024-01-15T02:04:05Z",
      "duration": "4m5s",
      "exitCode": 0
    }
  ],
  "total": 100
}
```

#### Trigger Job

```http
POST /api/v1/cronjobs/{namespace}/{name}/trigger
```

Response:
```json
{
  "jobName": "daily-backup-28374700",
  "message": "Job triggered successfully"
}
```

### Monitors

#### List Monitors

```http
GET /api/v1/monitors
```

Response:
```json
{
  "items": [
    {
      "namespace": "production",
      "name": "critical-jobs",
      "cronJobCount": 5,
      "healthySummary": {
        "healthy": 4,
        "warning": 1,
        "critical": 0
      }
    }
  ]
}
```

#### Get Monitor

```http
GET /api/v1/monitors/{namespace}/{name}
```

### Channels

#### List Channels

```http
GET /api/v1/channels
```

Response:
```json
{
  "items": [
    {
      "name": "team-slack",
      "type": "slack",
      "ready": true,
      "alertsSent": 150,
      "lastAlertTime": "2024-01-15T10:30:00Z"
    }
  ]
}
```

#### Test Channel

```http
POST /api/v1/channels/{name}/test
```

Response:
```json
{
  "success": true,
  "message": "Test alert sent successfully"
}
```

### Alerts

#### List Alerts

```http
GET /api/v1/alerts
```

Query parameters:
- `namespace` - Filter by namespace
- `cronjob` - Filter by CronJob name
- `type` - Filter by type (failure, deadManSwitch, slaViolation)
- `severity` - Filter by severity
- `active` - Show only active alerts (true/false)
- `from` - Start time
- `to` - End time

Response:
```json
{
  "items": [
    {
      "id": "alert-456",
      "type": "failure",
      "severity": "warning",
      "namespace": "production",
      "cronjobName": "daily-backup",
      "message": "Job failed with exit code 1",
      "createdAt": "2024-01-15T02:05:00Z",
      "resolvedAt": null,
      "active": true
    }
  ]
}
```

#### Acknowledge Alert

```http
POST /api/v1/alerts/{id}/acknowledge
```

### SLA

#### Get SLA Report

```http
GET /api/v1/sla
```

Query parameters:
- `namespace` - Filter by namespace
- `monitor` - Filter by monitor

Response:
```json
{
  "items": [
    {
      "namespace": "production",
      "cronjobName": "daily-backup",
      "successRate": 98.5,
      "slaTarget": 95.0,
      "compliant": true,
      "windowDays": 7
    }
  ],
  "summary": {
    "totalCronJobs": 10,
    "compliant": 9,
    "nonCompliant": 1,
    "overallRate": 97.2
  }
}
```

### Admin

#### Prune Data

```http
POST /api/v1/admin/prune
```

Request:
```json
{
  "olderThanDays": 90,
  "dryRun": false
}
```

Response:
```json
{
  "executionsDeleted": 1500,
  "logsDeleted": 3000,
  "eventsDeleted": 500
}
```

#### Get Stats

```http
GET /api/v1/admin/stats
```

Response:
```json
{
  "database": {
    "type": "postgres",
    "size": "256MB",
    "executions": 50000,
    "alerts": 1200
  },
  "monitors": 5,
  "channels": 3,
  "uptime": "72h30m"
}
```

## Export Endpoints

### Export Executions CSV

```http
GET /api/v1/export/executions
```

Query parameters:
- `namespace` - Filter by namespace
- `cronjob` - Filter by CronJob
- `from` - Start time
- `to` - End time

Returns: CSV file

### Export SLA Report CSV

```http
GET /api/v1/export/sla
```

Returns: CSV file

## Error Responses

```json
{
  "error": "not_found",
  "message": "CronJob not found",
  "details": {
    "namespace": "production",
    "name": "unknown-job"
  }
}
```

Common error codes:
- `400` - Bad request
- `404` - Not found
- `500` - Internal server error

## Related

- [Dashboard](/docs/features/dashboard) - Web UI
- [Prometheus Metrics](./metrics.md) - Metrics reference
