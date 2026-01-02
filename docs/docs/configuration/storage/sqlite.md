---
sidebar_position: 1
title: SQLite
description: Default embedded storage backend
---

# SQLite Storage

SQLite is the default storage backend, providing simple setup with no external dependencies.

## When to Use

SQLite is ideal for:
- Single-replica deployments
- Development and testing
- Small to medium clusters (< 100 CronJobs)
- Simple deployments without external database

## When Not to Use

Consider PostgreSQL or MySQL for:
- High-availability deployments (multiple replicas)
- Large clusters with many CronJobs
- Long retention periods with large data volumes
- Production deployments requiring database-level backup

## Configuration

### Helm Values

```yaml
config:
  storage:
    type: sqlite
    sqlite:
      path: /data/guardian.db
      walMode: true

persistence:
  enabled: true
  size: 10Gi
  storageClass: ""    # Uses default storage class
```

### Minimal Configuration

SQLite works out of the box with just persistence:

```yaml
persistence:
  enabled: true
  size: 5Gi
```

## WAL Mode

Write-Ahead Logging (WAL) mode improves performance:

```yaml
config:
  storage:
    sqlite:
      walMode: true       # Default: true
```

Benefits:
- Better concurrent read performance
- Improved write performance
- More resilient to crashes

## Persistence

### PersistentVolumeClaim

CronJob Guardian creates a PVC for SQLite data:

```yaml
persistence:
  enabled: true
  size: 10Gi
  storageClass: standard
  accessModes:
    - ReadWriteOnce
```

### Storage Sizing

Estimate storage needs:

| Metric | Approximate Size |
|--------|------------------|
| Execution record | ~1 KB |
| Logs per execution | ~10-50 KB |
| Events per execution | ~1-5 KB |

Example: 100 CronJobs × 10 runs/day × 90 days = ~5 GB with logs

### Existing PVC

Use an existing PVC:

```yaml
persistence:
  enabled: true
  existingClaim: guardian-data
```

## Performance Tuning

### Connection Pool

```yaml
config:
  storage:
    sqlite:
      maxOpenConns: 1       # SQLite only supports 1 writer
      maxIdleConns: 1
```

### Busy Timeout

```yaml
config:
  storage:
    sqlite:
      busyTimeout: 5s       # Wait for locks
```

## Complete Example

```yaml title="values-sqlite.yaml"
config:
  storage:
    type: sqlite
    sqlite:
      path: /data/guardian.db
      walMode: true
      busyTimeout: 5s

persistence:
  enabled: true
  size: 10Gi
  storageClass: standard
  accessModes:
    - ReadWriteOnce

# Single replica only with SQLite
replicaCount: 1
leaderElection:
  enabled: false
```

## Backup and Restore

### Manual Backup

```bash
# Create backup
kubectl exec -n cronjob-guardian deploy/cronjob-guardian -- \
  sqlite3 /data/guardian.db ".backup /data/backup.db"

# Copy to local
kubectl cp cronjob-guardian/cronjob-guardian-xxx:/data/backup.db ./backup.db
```

### Restore

```bash
# Copy backup to pod
kubectl cp ./backup.db cronjob-guardian/cronjob-guardian-xxx:/data/restore.db

# Stop operator, replace database, restart
kubectl scale -n cronjob-guardian deploy/cronjob-guardian --replicas=0
kubectl exec -n cronjob-guardian deploy/cronjob-guardian -- \
  mv /data/restore.db /data/guardian.db
kubectl scale -n cronjob-guardian deploy/cronjob-guardian --replicas=1
```

### Volume Snapshots

For production, use Kubernetes VolumeSnapshots:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: guardian-backup
  namespace: cronjob-guardian
spec:
  volumeSnapshotClassName: csi-hostpath-snapclass
  source:
    persistentVolumeClaimName: cronjob-guardian-data
```

## Maintenance

### VACUUM

Reclaim space after large deletions:

```bash
kubectl exec -n cronjob-guardian deploy/cronjob-guardian -- \
  sqlite3 /data/guardian.db "VACUUM"
```

### Integrity Check

```bash
kubectl exec -n cronjob-guardian deploy/cronjob-guardian -- \
  sqlite3 /data/guardian.db "PRAGMA integrity_check"
```

## Limitations

1. **Single writer**: Only one process can write at a time
2. **No HA**: Cannot run multiple replicas with shared SQLite
3. **Network storage issues**: SQLite doesn't work well on NFS or other network filesystems
4. **Large datasets**: Performance degrades with very large datasets

## Migrating Away from SQLite

To migrate to PostgreSQL or MySQL:

1. Export data: Use the REST API to export execution history
2. Deploy with new backend: Configure PostgreSQL/MySQL
3. Import data: Restore from export

Or simply start fresh—historical data will rebuild from new executions.

## Related

- [PostgreSQL](./postgresql.md) - For production/HA
- [MySQL](./mysql.md) - Alternative production backend
- [High Availability](/docs/guides/high-availability) - Multi-replica setup
