---
sidebar_position: 1
title: High Availability
description: Deploy CronJob Guardian with high availability
---

# High Availability Setup

This guide covers deploying CronJob Guardian in a highly available configuration.

## Requirements

For HA deployment:
- **External database**: PostgreSQL or MySQL (SQLite doesn't support HA)
- **Multiple replicas**: 2+ operator pods
- **Leader election**: Enabled for safe coordination

## Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   Replica 1     │    │   Replica 2     │
│   (Leader)      │    │   (Standby)     │
│                 │    │                 │
│ • Controllers   │    │ • Watches       │
│ • Schedulers    │    │ • Health checks │
│ • API server    │    │ • API server    │
└────────┬────────┘    └────────┬────────┘
         │                      │
         └──────────┬───────────┘
                    │
         ┌──────────▼──────────┐
         │     PostgreSQL      │
         │   (shared state)    │
         └─────────────────────┘
```

## Configuration

### Minimal HA Setup

```yaml title="values-ha.yaml"
replicaCount: 2

leaderElection:
  enabled: true

config:
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc
      database: guardian
      username: guardian
      existingSecret: postgres-credentials
```

### Full Production HA

```yaml title="values-ha-production.yaml"
replicaCount: 3

leaderElection:
  enabled: true
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s

config:
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc.cluster.local
      port: 5432
      database: cronjob_guardian
      username: guardian
      existingSecret: postgres-credentials
      sslMode: require
      maxOpenConns: 25
      maxIdleConns: 10

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

podDisruptionBudget:
  enabled: true
  minAvailable: 1

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: cronjob-guardian
          topologyKey: kubernetes.io/hostname

topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: ScheduleAnyway
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: cronjob-guardian
```

## Leader Election

### How It Works

1. All replicas compete for a lease lock
2. One replica becomes the leader
3. Leader runs controllers and schedulers
4. Standby replicas monitor and serve API
5. If leader fails, another replica takes over

### Configuration

```yaml
leaderElection:
  enabled: true
  leaseDuration: 15s      # How long lease is valid
  renewDeadline: 10s      # Deadline to renew
  retryPeriod: 2s         # Retry interval
```

### Failover Timing

With default settings:
- Leader renewal every 10s
- Lease expires after 15s
- New leader election within ~5s
- **Total failover: ~20-30 seconds**

### Aggressive Settings (Faster Failover)

```yaml
leaderElection:
  enabled: true
  leaseDuration: 10s
  renewDeadline: 8s
  retryPeriod: 1s
```

**Faster failover (~15s) but more resource usage.**

## Pod Disruption Budget

Prevent all replicas from being evicted:

```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 1    # At least 1 pod always running
```

Or with percentage:

```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 50%
```

## Anti-Affinity

Spread replicas across nodes:

```yaml
affinity:
  podAntiAffinity:
    # Hard requirement: different nodes
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app.kubernetes.io/name: cronjob-guardian
        topologyKey: kubernetes.io/hostname
```

Or soft preference:

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: cronjob-guardian
          topologyKey: kubernetes.io/hostname
```

## Zone Spreading

Spread across availability zones:

```yaml
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: DoNotSchedule
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: cronjob-guardian
```

## Database HA

Ensure your database is also highly available:

### PostgreSQL HA Options

- **Managed services**: RDS, Cloud SQL, Azure Database
- **Patroni**: Self-managed PostgreSQL HA
- **Zalando PostgreSQL Operator**: Kubernetes-native

### Connection Handling

During database failover:

```yaml
config:
  storage:
    postgres:
      maxOpenConns: 25
      connMaxLifetime: 5m    # Reconnect periodically
      healthCheckInterval: 30s
```

## Monitoring HA

### Metrics

Key metrics for HA monitoring:

```promql
# Leader status
cronjob_guardian_leader_status

# Replica count
sum(up{job="cronjob-guardian"})

# Database connections
cronjob_guardian_db_connections_open
```

### Alerts

```yaml
groups:
  - name: cronjob-guardian-ha
    rules:
      - alert: CronJobGuardianNoLeader
        expr: sum(cronjob_guardian_leader_status) == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: No CronJob Guardian leader elected

      - alert: CronJobGuardianReplicasLow
        expr: sum(up{job="cronjob-guardian"}) < 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: CronJob Guardian has fewer than 2 replicas
```

## Health Checks

### Liveness Probe

Checks if the process is healthy:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 15
  periodSeconds: 20
```

### Readiness Probe

Checks if ready to serve:

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Graceful Shutdown

Ensure graceful termination:

```yaml
terminationGracePeriodSeconds: 30
```

During shutdown:
1. Stop accepting new work
2. Complete in-flight operations
3. Release leader lock
4. Close database connections

## Testing HA

### Simulate Leader Failure

```bash
# Find the leader
kubectl get lease -n cronjob-guardian cronjob-guardian-leader -o yaml

# Delete the leader pod
kubectl delete pod -n cronjob-guardian <leader-pod>

# Watch failover
kubectl logs -n cronjob-guardian -l app.kubernetes.io/name=cronjob-guardian -f
```

### Rolling Update

```bash
# Trigger rolling update
kubectl rollout restart -n cronjob-guardian deploy/cronjob-guardian

# Watch status
kubectl rollout status -n cronjob-guardian deploy/cronjob-guardian
```

## Troubleshooting

### No Leader Elected

- Check all pods are running
- Verify network connectivity between pods
- Check lease resource exists
- Review pod logs for election errors

### Split Brain

Prevented by:
- Single database as source of truth
- Lease-based leader election
- Fencing (standby replicas don't run controllers)

### Slow Failover

- Reduce lease duration (increases load)
- Check pod readiness probes
- Verify database connectivity

## Related

- [PostgreSQL Storage](/docs/configuration/storage/postgresql) - Database setup
- [Production Setup](./production-setup.md) - Full production guide
- [Prometheus Integration](./prometheus.md) - Monitoring
