---
sidebar_position: 1
title: CronJob Selectors
description: Define which CronJobs to monitor
---

# CronJob Selectors

CronJobMonitor resources use selectors to define which CronJobs to monitor. This page covers all selection patterns.

## Overview

Selectors can match CronJobs by:
- **Labels**: Standard Kubernetes label matching
- **Names**: Explicit name lists
- **Namespaces**: Single, multiple, or all namespaces

## Label Selectors

### matchLabels

Match CronJobs with specific labels:

```yaml
spec:
  selector:
    matchLabels:
      tier: critical
      app: backup
```

This matches CronJobs that have **both** labels.

### matchExpressions

Use operators for complex matching:

```yaml
spec:
  selector:
    matchExpressions:
      - key: tier
        operator: In
        values:
          - critical
          - high
      - key: team
        operator: Exists
```

**Operators:**
| Operator | Description |
|----------|-------------|
| `In` | Value is in the list |
| `NotIn` | Value is not in the list |
| `Exists` | Label key exists (any value) |
| `DoesNotExist` | Label key does not exist |

### Combined Labels and Expressions

```yaml
spec:
  selector:
    matchLabels:
      monitored: "true"
    matchExpressions:
      - key: tier
        operator: In
        values:
          - critical
          - high
      - key: experimental
        operator: DoesNotExist
```

All conditions must match.

## Name-based Selection

### matchNames

Explicitly list CronJob names:

```yaml
spec:
  selector:
    matchNames:
      - daily-backup
      - weekly-report
      - hourly-sync
```

### Combining with Labels

```yaml
spec:
  selector:
    matchLabels:
      type: report
    matchNames:
      - quarterly-audit  # Also include this even without label
```

## Namespace Selection

### Single Namespace (Default)

By default, monitors watch their own namespace:

```yaml
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: my-monitor
  namespace: production     # Only watches CronJobs in 'production'
spec:
  selector: {}
```

### Explicit Namespaces

Watch multiple namespaces:

```yaml
spec:
  namespaces:
    - production
    - staging
    - batch-jobs
  selector:
    matchLabels:
      monitored: "true"
```

### Namespace Selector

Select namespaces by label:

```yaml
spec:
  namespaceSelector:
    matchLabels:
      environment: production
  selector:
    matchLabels:
      tier: critical
```

This watches CronJobs in namespaces labeled `environment: production`.

### All Namespaces

Watch cluster-wide:

```yaml
spec:
  selector:
    allNamespaces: true
    matchLabels:
      global-monitoring: "true"
```

:::caution
Cluster-wide monitoring requires appropriate RBAC permissions.
:::

## Empty Selector

An empty selector matches all CronJobs in scope:

```yaml
spec:
  selector: {}   # Matches ALL CronJobs in the namespace
```

## Complete Examples

### All in Namespace

```yaml title="all-in-namespace.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: all-jobs
  namespace: production
spec:
  selector: {}

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true

  alerting:
    channelRefs:
      - name: team-slack
```

### Critical Tier Only

```yaml title="critical-only.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: critical-jobs
  namespace: production
spec:
  selector:
    matchLabels:
      tier: critical

  sla:
    minSuccessRate: 99.9
    windowDays: 30

  alerting:
    channelRefs:
      - name: pagerduty-critical
    severityOverrides:
      jobFailed: critical
```

### Multi-Namespace by Team

```yaml title="team-jobs.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: data-team-jobs
  namespace: cronjob-guardian
spec:
  namespaces:
    - data-pipelines
    - analytics
    - ml-training

  selector:
    matchLabels:
      team: data

  sla:
    minSuccessRate: 95
    windowDays: 14

  alerting:
    channelRefs:
      - name: data-team-slack
```

### Cluster-Wide Critical

```yaml title="cluster-critical.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: cluster-critical
  namespace: cronjob-guardian
spec:
  selector:
    allNamespaces: true
    matchLabels:
      tier: critical
    matchExpressions:
      - key: skip-monitoring
        operator: DoesNotExist

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true
      missedScheduleThreshold: 1

  sla:
    minSuccessRate: 99.9
    windowDays: 30

  alerting:
    channelRefs:
      - name: pagerduty-critical
      - name: ops-slack
```

### Production Namespaces by Label

```yaml title="prod-namespaces.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: production-jobs
  namespace: cronjob-guardian
spec:
  namespaceSelector:
    matchLabels:
      environment: production

  selector:
    matchLabels:
      monitored: "true"

  deadManSwitch:
    enabled: true
    autoFromSchedule:
      enabled: true

  alerting:
    channelRefs:
      - name: ops-slack
```

## Selector Priority

When multiple conditions are specified:

1. All namespace conditions must match (namespaces, namespaceSelector, allNamespaces)
2. Within matched namespaces, all selector conditions must match
3. matchLabels AND matchExpressions AND matchNames all apply

## Best Practices

1. **Start specific, expand later**: Begin with explicit names, then generalize to labels
2. **Use consistent labeling**: Establish a labeling convention for monitoring
3. **Test selectors**: Verify which CronJobs match before enabling strict SLAs
4. **Document selection logic**: Comment your monitors explaining what they target
5. **Avoid over-broad selectors**: Don't monitor everything cluster-wide without good reason

## Related

- [Dead-Man's Switch](/docs/features/dead-man-switch) - Configure timing detection
- [SLA Configuration](./sla) - Set success rate thresholds
- [Alerting Configuration](./alerting.md) - Configure alert behavior
