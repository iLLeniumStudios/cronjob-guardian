---
sidebar_position: 3
title: MySQL
description: MySQL/MariaDB database backend
---

# MySQL/MariaDB Storage

MySQL and MariaDB are supported for organizations with existing MySQL infrastructure.

## When to Use

MySQL is ideal for:
- Organizations standardized on MySQL/MariaDB
- High-availability deployments
- Large clusters
- Integration with existing MySQL operations

## Prerequisites

- MySQL 8.0+ or MariaDB 10.5+
- Database created for CronJob Guardian
- User with appropriate permissions

## Configuration

### Create Credentials Secret

```bash
kubectl create namespace cronjob-guardian
kubectl create secret generic mysql-credentials \
  --namespace cronjob-guardian \
  --from-literal=password=your-secure-password
```

### Helm Values

```yaml
config:
  storage:
    type: mysql
    mysql:
      host: mysql.database.svc
      port: 3306
      database: guardian
      username: guardian
      existingSecret: mysql-credentials

persistence:
  enabled: false
```

## TLS Configuration

### Require TLS

```yaml
config:
  storage:
    mysql:
      tls: true
      tlsSkipVerify: false
```

### Custom CA

```yaml
config:
  storage:
    mysql:
      tls: true
      tlsCA:
        secretKeyRef:
          name: mysql-ca
          key: ca.crt
```

## Connection Pool

```yaml
config:
  storage:
    mysql:
      maxOpenConns: 25
      maxIdleConns: 10
      connMaxLifetime: 5m
      connMaxIdleTime: 1m
```

## Complete Example

```yaml title="values-mysql.yaml"
replicaCount: 2

leaderElection:
  enabled: true

config:
  storage:
    type: mysql
    mysql:
      host: mysql.database.svc.cluster.local
      port: 3306
      database: cronjob_guardian
      username: guardian
      existingSecret: mysql-credentials
      tls: true
      maxOpenConns: 25
      maxIdleConns: 10
      connMaxLifetime: 5m
      parseTime: true
      charset: utf8mb4

persistence:
  enabled: false
```

## Database Setup

### Create Database

```sql
CREATE DATABASE cronjob_guardian CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'guardian'@'%' IDENTIFIED BY 'secure-password';
GRANT ALL PRIVILEGES ON cronjob_guardian.* TO 'guardian'@'%';
FLUSH PRIVILEGES;
```

### MariaDB

Same syntax works for MariaDB:

```sql
CREATE DATABASE cronjob_guardian CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'guardian'@'%' IDENTIFIED BY 'secure-password';
GRANT ALL PRIVILEGES ON cronjob_guardian.* TO 'guardian'@'%';
FLUSH PRIVILEGES;
```

## Managed MySQL

### AWS RDS MySQL

```yaml
config:
  storage:
    mysql:
      host: guardian.xxx.us-east-1.rds.amazonaws.com
      port: 3306
      database: guardian
      username: guardian
      existingSecret: rds-credentials
      tls: true
```

### Google Cloud SQL

```yaml
config:
  storage:
    mysql:
      host: 10.0.0.4
      port: 3306
      database: guardian
      username: guardian
      existingSecret: cloudsql-credentials
      tls: false    # If using Cloud SQL Proxy
```

### Azure Database for MySQL

```yaml
config:
  storage:
    mysql:
      host: guardian.mysql.database.azure.com
      port: 3306
      database: guardian
      username: guardian@guardian
      existingSecret: azure-mysql-credentials
      tls: true
```

### PlanetScale

```yaml
config:
  storage:
    mysql:
      host: xxx.us-east-1.psdb.cloud
      port: 3306
      database: guardian
      username: xxx
      existingSecret: planetscale-credentials
      tls: true
```

## High Availability

Run multiple replicas with leader election:

```yaml
replicaCount: 2

leaderElection:
  enabled: true
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s

config:
  storage:
    type: mysql
    mysql:
      host: mysql.database.svc
      database: guardian
      existingSecret: mysql-credentials
```

## Character Set

Use utf8mb4 for full Unicode support:

```yaml
config:
  storage:
    mysql:
      charset: utf8mb4
      collation: utf8mb4_unicode_ci
```

## Backup

### mysqldump

```bash
mysqldump -h mysql.database.svc -u guardian -p cronjob_guardian > backup.sql
```

### Restore

```bash
mysql -h mysql.database.svc -u guardian -p cronjob_guardian < backup.sql
```

## Troubleshooting

### Connection Refused

- Verify host and port
- Check MySQL is running
- Verify network policies allow connection

### Access Denied

- Check username and password
- Verify user has access from pod IP
- Check GRANT statements

### TLS Errors

- Verify TLS settings match server
- Check CA certificate
- Try `tlsSkipVerify: true` for testing (not production)

### Character Set Issues

- Ensure database uses utf8mb4
- Verify `charset: utf8mb4` in configuration

### Connection Pool Exhausted

- Increase `maxOpenConns`
- Check MySQL `max_connections` setting
- Review slow queries

## Performance Tuning

### MySQL Server Settings

Recommended MySQL settings for CronJob Guardian:

```ini
[mysqld]
max_connections = 200
innodb_buffer_pool_size = 256M
innodb_log_file_size = 64M
```

### Index Optimization

CronJob Guardian creates necessary indexes. For very large deployments, monitor query performance.

## Related

- [SQLite](./sqlite.md) - For simple deployments
- [PostgreSQL](./postgresql.md) - Alternative production backend
- [High Availability](/docs/guides/high-availability) - Multi-replica setup
