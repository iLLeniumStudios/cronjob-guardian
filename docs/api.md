# REST API Reference

CronJob Guardian exposes a REST API for programmatic access to monitoring data, CronJob management, and alerting configuration.

## Base URL

The API is served on port 8080 by default (configurable via `ui.port`).

```
http://localhost:8080/api/v1
```

## Authentication

The API does not currently require authentication. Access control should be managed via Kubernetes RBAC and network policies.

## Endpoints

### Health & Status

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Operator health status |
| GET | `/api/v1/stats` | Summary statistics |
| GET | `/api/v1/config` | Operator configuration |

### CronJobs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/cronjobs` | List all monitored CronJobs |
| GET | `/api/v1/cronjobs/{namespace}/{name}` | Get CronJob details with metrics |
| GET | `/api/v1/cronjobs/{namespace}/{name}/executions` | Get execution history |
| GET | `/api/v1/cronjobs/{namespace}/{name}/logs/{jobName}` | Get logs for a specific job |
| POST | `/api/v1/cronjobs/{namespace}/{name}/trigger` | Manually trigger a CronJob |
| POST | `/api/v1/cronjobs/{namespace}/{name}/suspend` | Suspend a CronJob |
| POST | `/api/v1/cronjobs/{namespace}/{name}/resume` | Resume a suspended CronJob |
| DELETE | `/api/v1/cronjobs/{namespace}/{name}/history` | Delete execution history |

### Monitors

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/monitors` | List all CronJobMonitors |
| GET | `/api/v1/monitors/{namespace}/{name}` | Get monitor details |

### Alerts

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/alerts` | List active alerts |
| GET | `/api/v1/alerts/history` | Get alert history |

### Alert Channels

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/channels` | List all alert channels |
| GET | `/api/v1/channels/{name}` | Get channel details |
| POST | `/api/v1/channels/{name}/test` | Send a test alert |

### Administration

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/admin/storage-stats` | Get storage statistics |
| POST | `/api/v1/admin/prune` | Prune old execution records |

### Metrics

| Method | Path | Description |
|--------|------|-------------|
| GET | `/metrics` | Prometheus metrics endpoint |

---

## Endpoint Details

### GET /api/v1/health

Returns the operator's health status.

**Response:**

```json
{
  "status": "healthy",
  "storage": "connected",
  "leader": true,
  "version": "v0.1.0",
  "uptime": "2h30m15s"
}
```

### GET /api/v1/stats

Returns summary statistics.

**Response:**

```json
{
  "totalMonitors": 5,
  "totalCronJobs": 25,
  "summary": {
    "total": 25,
    "healthy": 20,
    "warning": 3,
    "critical": 2
  },
  "activeAlerts": 5,
  "executionsRecorded24h": 150
}
```

### GET /api/v1/cronjobs

Lists all monitored CronJobs.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `namespace` | string | Filter by namespace |
| `status` | string | Filter by status (healthy, warning, critical, suspended) |
| `monitor` | string | Filter by monitor name |

**Response:**

```json
{
  "items": [
    {
      "name": "daily-backup",
      "namespace": "production",
      "status": "healthy",
      "schedule": "0 2 * * *",
      "timezone": "UTC",
      "suspended": false,
      "successRate": 98.5,
      "lastSuccess": "2025-01-01T02:00:00Z",
      "lastRunDuration": "5m30s",
      "nextRun": "2025-01-02T02:00:00Z",
      "activeAlerts": 0,
      "monitorRef": {
        "name": "critical-jobs",
        "namespace": "production"
      }
    }
  ],
  "summary": {
    "total": 25,
    "healthy": 20,
    "warning": 3,
    "critical": 2
  }
}
```

### GET /api/v1/cronjobs/{namespace}/{name}

Gets detailed information about a specific CronJob.

**Response:**

```json
{
  "name": "daily-backup",
  "namespace": "production",
  "status": "healthy",
  "schedule": "0 2 * * *",
  "suspended": false,
  "successRate": 98.5,
  "lastSuccess": "2025-01-01T02:00:00Z",
  "lastRunDuration": "5m30s",
  "nextRun": "2025-01-02T02:00:00Z",
  "metrics": {
    "successRate7d": 98.5,
    "successRate30d": 97.2,
    "totalRuns7d": 7,
    "successfulRuns7d": 7,
    "failedRuns7d": 0,
    "avgDurationSeconds": 330,
    "p50DurationSeconds": 320,
    "p95DurationSeconds": 450,
    "p99DurationSeconds": 520
  },
  "lastExecution": {
    "jobName": "daily-backup-28012345",
    "status": "success",
    "startTime": "2025-01-01T02:00:00Z",
    "completionTime": "2025-01-01T02:05:30Z",
    "duration": "5m30s",
    "exitCode": 0,
    "reason": ""
  },
  "activeAlerts": []
}
```

### GET /api/v1/cronjobs/{namespace}/{name}/executions

Gets execution history for a CronJob.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Maximum records to return |
| `offset` | int | 0 | Offset for pagination |
| `status` | string | | Filter by status (success, failed) |
| `since` | string | | ISO 8601 timestamp to filter from |

**Response:**

```json
{
  "items": [
    {
      "jobName": "daily-backup-28012345",
      "status": "success",
      "startTime": "2025-01-01T02:00:00Z",
      "completionTime": "2025-01-01T02:05:30Z",
      "duration": "5m30s",
      "exitCode": 0,
      "reason": ""
    }
  ],
  "pagination": {
    "total": 150,
    "limit": 50,
    "offset": 0,
    "hasMore": true
  }
}
```

### POST /api/v1/cronjobs/{namespace}/{name}/trigger

Manually triggers a CronJob execution.

**Response:**

```json
{
  "success": true,
  "message": "Job triggered successfully",
  "jobName": "daily-backup-manual-28012345"
}
```

### POST /api/v1/cronjobs/{namespace}/{name}/suspend

Suspends a CronJob.

**Response:**

```json
{
  "success": true,
  "message": "CronJob suspended"
}
```

### POST /api/v1/cronjobs/{namespace}/{name}/resume

Resumes a suspended CronJob.

**Response:**

```json
{
  "success": true,
  "message": "CronJob resumed"
}
```

### DELETE /api/v1/cronjobs/{namespace}/{name}/history

Deletes execution history for a CronJob.

**Response:**

```json
{
  "success": true,
  "recordsDeleted": 150,
  "message": "Execution history deleted"
}
```

### GET /api/v1/alerts

Lists active alerts.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `severity` | string | Filter by severity (critical, warning, info) |
| `namespace` | string | Filter by CronJob namespace |

**Response:**

```json
{
  "items": [
    {
      "id": "abc123",
      "type": "JobFailed",
      "severity": "critical",
      "title": "CronJob Failed",
      "message": "Job daily-backup failed with exit code 1",
      "cronjob": {
        "namespace": "production",
        "name": "daily-backup"
      },
      "monitor": {
        "namespace": "production",
        "name": "critical-jobs"
      },
      "since": "2025-01-01T02:05:30Z",
      "lastNotified": "2025-01-01T02:06:00Z"
    }
  ],
  "total": 5,
  "bySeverity": {
    "critical": 2,
    "warning": 3,
    "info": 0
  }
}
```

### GET /api/v1/alerts/history

Gets alert history.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Maximum records to return |
| `offset` | int | 0 | Offset for pagination |
| `severity` | string | | Filter by severity |
| `since` | string | | ISO 8601 timestamp to filter from |

**Response:**

```json
{
  "items": [
    {
      "id": "abc123",
      "type": "JobFailed",
      "severity": "critical",
      "title": "CronJob Failed",
      "message": "Job daily-backup failed",
      "cronjob": {
        "namespace": "production",
        "name": "daily-backup"
      },
      "occurredAt": "2025-01-01T02:05:30Z",
      "resolvedAt": "2025-01-01T03:00:00Z",
      "channelsNotified": ["slack-ops", "pagerduty-critical"]
    }
  ],
  "pagination": {
    "total": 100,
    "limit": 50,
    "offset": 0
  }
}
```

### POST /api/v1/channels/{name}/test

Sends a test alert to verify channel configuration.

**Response:**

```json
{
  "success": true,
  "message": "Test alert sent successfully"
}
```

### GET /api/v1/admin/storage-stats

Gets storage statistics.

**Response:**

```json
{
  "executionCount": 15000,
  "storageType": "sqlite",
  "healthy": true,
  "retentionDays": 30,
  "logStorageEnabled": false
}
```

### POST /api/v1/admin/prune

Prunes old execution records.

**Request Body:**

```json
{
  "olderThanDays": 30,
  "dryRun": false,
  "pruneLogsOnly": false
}
```

**Response:**

```json
{
  "success": true,
  "recordsPruned": 5000,
  "dryRun": false,
  "cutoff": "2024-12-01T00:00:00Z",
  "olderThanDays": 30,
  "message": "Pruned 5000 records older than 30 days"
}
```

---

## Error Responses

All endpoints return errors in a consistent format:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "CronJob not found"
  }
}
```

**Common Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `NOT_FOUND` | 404 | Resource not found |
| `BAD_REQUEST` | 400 | Invalid request parameters |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `CONFLICT` | 409 | Resource conflict |

---

## Examples

### List all CronJobs

```bash
curl http://localhost:8080/api/v1/cronjobs
```

### Get execution history

```bash
curl "http://localhost:8080/api/v1/cronjobs/production/daily-backup/executions?limit=10"
```

### Trigger a job manually

```bash
curl -X POST http://localhost:8080/api/v1/cronjobs/production/daily-backup/trigger
```

### Prune old records (dry run)

```bash
curl -X POST http://localhost:8080/api/v1/admin/prune \
  -H "Content-Type: application/json" \
  -d '{"olderThanDays": 30, "dryRun": true}'
```

### Get Prometheus metrics

```bash
curl http://localhost:8080/metrics
```
