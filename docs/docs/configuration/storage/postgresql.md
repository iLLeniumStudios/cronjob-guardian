---
sidebar_position: 2
title: PostgreSQL
description: Production-ready database backend
---

# PostgreSQL Storage

PostgreSQL is recommended for production deployments, supporting high availability and large datasets.

## When to Use

PostgreSQL is ideal for:
- Production deployments
- High-availability with multiple replicas
- Large clusters (100+ CronJobs)
- Long retention periods
- Teams already running PostgreSQL

## Prerequisites

- PostgreSQL 12 or later
- Database created for CronJob Guardian
- User with appropriate permissions

## Configuration

### Create Credentials Secret

```bash
kubectl create namespace cronjob-guardian
kubectl create secret generic postgres-credentials \
  --namespace cronjob-guardian \
  --from-literal=password=your-secure-password
```

### Helm Values

```yaml
config:
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc
      port: 5432
      database: guardian
      username: guardian
      existingSecret: postgres-credentials
      sslMode: require

persistence:
  enabled: false    # Not needed with external database
```

## SSL Configuration

### Require SSL

```yaml
config:
  storage:
    postgres:
      sslMode: require
```

### Verify CA

```yaml
config:
  storage:
    postgres:
      sslMode: verify-ca
      sslRootCert:
        secretKeyRef:
          name: postgres-ca
          key: ca.crt
```

### Full Verification

```yaml
config:
  storage:
    postgres:
      sslMode: verify-full
      sslRootCert:
        secretKeyRef:
          name: postgres-ca
          key: ca.crt
```

## Connection Pool

Tune connection pool for performance:

```yaml
config:
  storage:
    postgres:
      maxOpenConns: 25
      maxIdleConns: 10
      connMaxLifetime: 5m
      connMaxIdleTime: 1m
```

## High Availability

With PostgreSQL, run multiple operator replicas:

```yaml
replicaCount: 2

leaderElection:
  enabled: true
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s

config:
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc
      database: guardian
      username: guardian
      existingSecret: postgres-credentials
```

## Complete Example

```yaml title="values-postgres.yaml"
replicaCount: 2

leaderElection:
  enabled: true

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
      connMaxLifetime: 5m

persistence:
  enabled: false

resources:
  limits:
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## Database Setup

### Create Database

```sql
CREATE DATABASE cronjob_guardian;
CREATE USER guardian WITH ENCRYPTED PASSWORD 'secure-password';
GRANT ALL PRIVILEGES ON DATABASE cronjob_guardian TO guardian;
```

### Schema Permissions

CronJob Guardian creates tables automatically. Ensure the user has:

```sql
GRANT CREATE ON SCHEMA public TO guardian;
GRANT ALL ON ALL TABLES IN SCHEMA public TO guardian;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO guardian;
```

## Managed PostgreSQL

### AWS RDS

```yaml
config:
  storage:
    postgres:
      host: guardian.xxx.us-east-1.rds.amazonaws.com
      port: 5432
      database: guardian
      username: guardian
      existingSecret: rds-credentials
      sslMode: require
```

### Google Cloud SQL

```yaml
config:
  storage:
    postgres:
      host: 10.0.0.3    # Private IP
      port: 5432
      database: guardian
      username: guardian
      existingSecret: cloudsql-credentials
      sslMode: disable  # If using Cloud SQL Proxy
```

### Azure Database

```yaml
config:
  storage:
    postgres:
      host: guardian.postgres.database.azure.com
      port: 5432
      database: guardian
      username: guardian@guardian
      existingSecret: azure-pg-credentials
      sslMode: require
```

## Monitoring

### Metrics

Enable PostgreSQL connection metrics:

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
```

Exposed metrics include:
- `cronjob_guardian_db_connections_open`
- `cronjob_guardian_db_connections_idle`
- `cronjob_guardian_db_query_duration_seconds`

### Health Checks

CronJob Guardian performs health checks on the database connection. Configure via:

```yaml
config:
  storage:
    postgres:
      healthCheckInterval: 30s
```

## Backup

Use standard PostgreSQL backup tools:

### pg_dump

```bash
pg_dump -h postgres.database.svc -U guardian cronjob_guardian > backup.sql
```

### Restore

```bash
psql -h postgres.database.svc -U guardian cronjob_guardian < backup.sql
```

### Managed Service Backups

Use your cloud provider's backup features for production.

## Troubleshooting

### Connection Refused

- Verify host and port
- Check network policies
- Verify PostgreSQL is running

### Authentication Failed

- Check secret contains correct password
- Verify username exists in database
- Check pg_hba.conf allows connections

### SSL Errors

- Verify sslMode matches server configuration
- Check CA certificate if using verify-ca/verify-full

### Connection Pool Exhausted

- Increase `maxOpenConns`
- Check for connection leaks
- Review slow queries

## Related

- [SQLite](./sqlite.md) - For simple deployments
- [MySQL](./mysql.md) - Alternative production backend
- [High Availability](/docs/guides/high-availability) - Multi-replica setup
