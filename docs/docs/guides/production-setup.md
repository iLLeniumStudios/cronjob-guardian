---
sidebar_position: 3
title: Production Setup
description: Best practices for production deployments
---

# Production Setup Guide

This guide covers best practices for running CronJob Guardian in production.

## Checklist

Before going to production:

- [ ] External database (PostgreSQL/MySQL) configured
- [ ] Multiple replicas with leader election
- [ ] Resource limits set
- [ ] Monitoring and alerting configured
- [ ] Backup strategy in place
- [ ] Security reviewed
- [ ] Network policies configured

## Recommended Configuration

```yaml title="values-production.yaml"
# High availability
replicaCount: 2

leaderElection:
  enabled: true

# External database
config:
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc
      database: cronjob_guardian
      username: guardian
      existingSecret: postgres-credentials
      sslMode: require
      maxOpenConns: 25

# Resource management
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

# Availability
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

# Monitoring
metrics:
  enabled: true
  serviceMonitor:
    enabled: true

# Security
securityContext:
  runAsNonRoot: true
  runAsUser: 65534

serviceAccount:
  create: true
```

## Storage Backend

### PostgreSQL (Recommended)

```yaml
config:
  storage:
    type: postgres
    postgres:
      host: postgres-primary.database.svc
      port: 5432
      database: cronjob_guardian
      username: guardian
      existingSecret: postgres-credentials
      sslMode: require
      maxOpenConns: 25
      maxIdleConns: 10
      connMaxLifetime: 5m
```

### Database Requirements

- PostgreSQL 12+ or MySQL 8.0+
- Dedicated database for CronJob Guardian
- User with CREATE and full table access
- SSL/TLS enabled for connections

## Resource Sizing

### Small Cluster (fewer than 50 CronJobs)

```yaml
resources:
  limits:
    cpu: 200m
    memory: 128Mi
  requests:
    cpu: 50m
    memory: 64Mi
```

### Medium Cluster (50-200 CronJobs)

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### Large Cluster (over 200 CronJobs)

```yaml
resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

## Security

### RBAC

CronJob Guardian requires specific RBAC permissions. The Helm chart creates these automatically.

Review the generated ClusterRole:

```bash
kubectl get clusterrole cronjob-guardian -o yaml
```

### Pod Security

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  fsGroup: 65534

containerSecurityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

### Network Policies

Restrict network access:

```yaml title="network-policy.yaml"
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: cronjob-guardian
  namespace: cronjob-guardian
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: cronjob-guardian
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Prometheus scraping
    - from:
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - port: 8080
    # Dashboard access
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress
      ports:
        - port: 8080
  egress:
    # Kubernetes API
    - to:
        - namespaceSelector: {}
          podSelector:
            matchLabels:
              component: kube-apiserver
    # Database
    - to:
        - namespaceSelector:
            matchLabels:
              name: database
      ports:
        - port: 5432
    # Alert channels (external)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - port: 443
```

### Secret Management

Use external secret management for sensitive data:

```yaml
# With External Secrets Operator
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: postgres-credentials
  namespace: cronjob-guardian
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: postgres-credentials
  data:
    - secretKey: password
      remoteRef:
        key: secret/data/cronjob-guardian/postgres
        property: password
```

## Monitoring

### Prometheus Integration

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring
    labels:
      release: prometheus
```

### Key Alerts

Configure alerts for:

1. **Operator health**: Down, no leader, high error rate
2. **Database**: Connection issues, high latency
3. **Alert delivery**: Failed dispatches, high rate

See [Prometheus Integration](./prometheus.md) for alert rule examples.

### Logging

Configure structured logging:

```yaml
config:
  logging:
    level: info
    format: json
```

Aggregate logs with your preferred solution (Loki, ELK, etc.).

## Backup Strategy

### Database Backups

- Daily automated backups of the database
- Test restore procedures regularly
- Consider point-in-time recovery for PostgreSQL

### Configuration Backups

```bash
# Export all CronJobMonitors
kubectl get cronjobmonitors -A -o yaml > monitors-backup.yaml

# Export all AlertChannels
kubectl get alertchannels -o yaml > channels-backup.yaml
```

## Disaster Recovery

### Recovery Procedure

1. Restore database from backup
2. Apply CronJobMonitor and AlertChannel manifests
3. Deploy CronJob Guardian
4. Verify connectivity and functionality

### RPO/RTO Considerations

| Component | RPO | RTO |
|-----------|-----|-----|
| Configuration (CRDs) | Near-zero (GitOps) | Minutes |
| Execution history | Last backup | Depends on backup |
| Active alerts | Lost on failure | Reconstructed |

## Performance Tuning

### Database Connections

```yaml
config:
  storage:
    postgres:
      maxOpenConns: 25
      maxIdleConns: 10
      connMaxLifetime: 5m
```

### Scheduler Intervals

```yaml
config:
  schedulers:
    deadManSwitch:
      interval: 1m
    slaRecalc:
      interval: 5m
    prune:
      interval: 1h
```

### Data Retention

Balance storage costs with data needs:

```yaml
config:
  dataRetention:
    defaultRetentionDays: 90
    defaultLogRetentionDays: 30
```

## Upgrade Strategy

### Rolling Updates

Helm upgrades perform rolling updates by default:

```bash
helm upgrade cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --values values-production.yaml
```

### Pre-Upgrade Checklist

1. Review changelog for breaking changes
2. Backup database
3. Test in staging environment
4. Schedule maintenance window if needed

### Rollback

```bash
helm rollback cronjob-guardian -n cronjob-guardian
```

## Related

- [High Availability](./high-availability.md) - HA configuration details
- [Prometheus Integration](./prometheus.md) - Monitoring setup
- [PostgreSQL Storage](/docs/configuration/storage/postgresql) - Database configuration
