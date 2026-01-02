---
sidebar_position: 4
title: Suggested Fixes
description: Intelligent fix suggestions in alerts
---

# Suggested Fixes

CronJob Guardian analyzes failure context and provides actionable fix suggestions in alerts. This helps on-call engineers quickly understand what went wrong and how to fix it.

## Overview

When a job fails, CronJob Guardian:
1. Analyzes exit codes, termination reasons, logs, and events
2. Matches against built-in and custom patterns
3. Includes relevant fix suggestions in the alert

## Built-in Patterns

| Pattern | Trigger | Suggestion |
|---------|---------|------------|
| **OOMKilled** | Reason: `OOMKilled` | Increase `resources.limits.memory` |
| **SIGKILL (137)** | Exit code 137 | Check for OOM, inspect pod state |
| **SIGTERM (143)** | Exit code 143 | Check `activeDeadlineSeconds` or eviction |
| **ImagePullBackOff** | Reason match | Verify image name and `imagePullSecrets` |
| **CrashLoopBackOff** | Reason match | Check application startup logs |
| **ConfigError** | Reason: `CreateContainerConfigError` | Verify Secret/ConfigMap references |
| **DeadlineExceeded** | Reason match | Increase deadline or optimize job |
| **BackoffLimitExceeded** | Reason match | Check logs from failed attempts |
| **Evicted** | Reason match | Check node pressure, set pod priority |
| **FailedScheduling** | Event pattern | Check resources, taints, affinity |

## Custom Patterns

Define custom patterns to match application-specific failures:

```yaml
spec:
  alerting:
    suggestedFixPatterns:
      - name: db-connection-failed
        match:
          logPattern: "connection refused.*:5432|ECONNREFUSED"
        suggestion: |
          PostgreSQL connection failed. Check:
          kubectl get pods -n {{.Namespace}} -l app=postgres
        priority: 150

      - name: s3-access-denied
        match:
          logPattern: "AccessDenied|NoCredentialProviders"
        suggestion: |
          S3 access denied. Verify:
          1. IAM role attached to service account
          2. Bucket policy allows access
        priority: 140

      - name: redis-timeout
        match:
          logPattern: "redis.*timeout|ETIMEDOUT.*:6379"
        suggestion: |
          Redis connection timeout. Check Redis health:
          kubectl exec -n {{.Namespace}} deploy/redis -- redis-cli ping
        priority: 130
```

## Match Conditions

Patterns can match on multiple conditions:

### Exit Code

```yaml
match:
  exitCode: 1
```

### Termination Reason

```yaml
match:
  reason: "OOMKilled"
```

### Log Pattern (Regex)

```yaml
match:
  logPattern: "FATAL.*database connection failed"
```

### Event Pattern (Regex)

```yaml
match:
  eventPattern: "FailedScheduling.*Insufficient memory"
```

### Combined Conditions

All specified conditions must match:

```yaml
match:
  exitCode: 1
  logPattern: "connection refused"
```

## Priority System

Patterns are matched in priority order:
- Built-in patterns: priority 1-100
- Custom patterns: use priority 101+ to override built-ins

Higher priority patterns are checked first.

## Template Variables

Suggestions support Go template variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Namespace}}` | CronJob namespace | `production` |
| `{{.Name}}` | CronJob name | `daily-backup` |
| `{{.JobName}}` | Job name (with timestamp) | `daily-backup-28374658` |
| `{{.ExitCode}}` | Container exit code | `137` |
| `{{.Reason}}` | Termination reason | `OOMKilled` |

## Pattern Tester

Test patterns before deploying via the UI:

1. Go to **Settings > Pattern Tester**
2. Enter match criteria (exit code, reason, log sample)
3. Define your pattern
4. Click **Test** to see if it matches

This helps validate patterns without waiting for real failures.

## Example: Complete Pattern Configuration

```yaml title="full-pattern-example.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: production-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      tier: critical

  alerting:
    channelRefs:
      - name: team-slack

    suggestedFixPatterns:
      # Database patterns
      - name: postgres-connection
        match:
          logPattern: "could not connect to server|connection refused.*:5432"
        suggestion: |
          PostgreSQL unreachable. Debug steps:
          1. Check postgres pod: kubectl get pods -n {{.Namespace}} -l app=postgres
          2. Check pg_isready: kubectl exec deploy/postgres -- pg_isready
          3. Check secrets: kubectl get secret postgres-credentials -o yaml
        priority: 150

      - name: postgres-auth-failed
        match:
          logPattern: "password authentication failed|FATAL.*authentication"
        suggestion: |
          PostgreSQL authentication failed. Verify credentials match:
          kubectl get secret postgres-credentials -n {{.Namespace}}
        priority: 149

      # API patterns
      - name: api-rate-limited
        match:
          logPattern: "429|rate limit|too many requests"
        suggestion: |
          External API rate limit hit. Consider:
          1. Reduce batch size in job config
          2. Add delays between API calls
          3. Contact API provider for limit increase
        priority: 140

      # Infrastructure patterns
      - name: disk-full
        match:
          logPattern: "no space left on device|ENOSPC"
        suggestion: |
          Disk space exhausted. Check PVC usage:
          kubectl exec -n {{.Namespace}} job/{{.JobName}} -- df -h
        priority: 160
```

## Alert Example

When a job fails with matching pattern:

```
🚨 Job Failed: daily-backup

Exit Code: 137 (SIGKILL)
Reason: OOMKilled

💡 Suggested Fix:
Container was killed due to OOM. Increase memory limit:

kubectl patch cronjob daily-backup -p '
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            resources:
              limits:
                memory: 2Gi
'

Current limit: 512Mi
Recommended: 1-2Gi based on recent usage

Pod Logs (last 10 lines):
...
```

## Best Practices

1. **Start with built-ins**: Built-in patterns cover most common failures
2. **Add app-specific patterns**: Create patterns for your known failure modes
3. **Use specific matches**: More specific patterns prevent false matches
4. **Include actionable commands**: Give exact kubectl commands when possible
5. **Test before deploying**: Use the Pattern Tester to validate
6. **Review periodically**: Add patterns for recurring issues

## Related

- [Alerting Configuration](/docs/configuration/monitors/alerting) - Configure alert channels
- [Dashboard](./dashboard.md) - View failure details
- [Examples](/docs/examples/monitors) - Complete monitor examples
