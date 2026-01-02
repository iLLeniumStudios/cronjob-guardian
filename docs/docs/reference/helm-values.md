---
sidebar_position: 2
title: Helm Values
description: Helm chart configuration reference
---

# Helm Values Reference

Complete reference for CronJob Guardian Helm chart values.

:::note
This page provides an overview. For the most up-to-date reference, see the chart's [values.yaml](https://github.com/iLLeniumStudios/cronjob-guardian/blob/main/deploy/helm/cronjob-guardian/values.yaml).
:::

## Image

```yaml
image:
  repository: ghcr.io/illeniumstudios/cronjob-guardian
  tag: ""                    # Defaults to chart appVersion
  pullPolicy: IfNotPresent

imagePullSecrets: []
```

## Replicas & HA

```yaml
replicaCount: 1

leaderElection:
  enabled: false
  leaseDuration: 15s
  renewDeadline: 10s
  retryPeriod: 2s
```

## Storage

```yaml
config:
  storage:
    type: sqlite           # sqlite, postgres, mysql

    sqlite:
      path: /data/guardian.db
      walMode: true

    postgres:
      host: ""
      port: 5432
      database: guardian
      username: guardian
      existingSecret: ""   # Secret with 'password' key
      sslMode: disable

    mysql:
      host: ""
      port: 3306
      database: guardian
      username: guardian
      existingSecret: ""
      tls: false

persistence:
  enabled: true
  size: 10Gi
  storageClass: ""
  accessModes:
    - ReadWriteOnce
  existingClaim: ""
```

## Resources

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## Security

```yaml
serviceAccount:
  create: true
  name: ""
  annotations: {}

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

## Networking

```yaml
service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts:
    - host: guardian.example.com
      paths:
        - path: /
          pathType: Prefix
  tls: []

# OpenShift Route (alternative to Ingress)
route:
  enabled: false
  host: ""
  tls:
    termination: edge
```

## Monitoring

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: false
    namespace: ""
    interval: 30s
    scrapeTimeout: 10s
    labels: {}
```

## Scheduling

```yaml
nodeSelector: {}

tolerations: []

affinity: {}

topologySpreadConstraints: []

podDisruptionBudget:
  enabled: false
  minAvailable: 1
  # maxUnavailable: 1
```

## Probes

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 15
  periodSeconds: 20

readinessProbe:
  httpGet:
    path: /readyz
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Data Retention

```yaml
config:
  dataRetention:
    defaultRetentionDays: 90
    defaultLogRetentionDays: 30
    pruneInterval: 1h
```

## Logging

```yaml
config:
  logging:
    level: info          # debug, info, warn, error
    format: json         # json, text
```

## Complete Example

```yaml title="values-complete.yaml"
replicaCount: 2

leaderElection:
  enabled: true

image:
  repository: ghcr.io/illeniumstudios/cronjob-guardian
  pullPolicy: IfNotPresent

config:
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc
      port: 5432
      database: cronjob_guardian
      username: guardian
      existingSecret: postgres-credentials
      sslMode: require

  dataRetention:
    defaultRetentionDays: 90
    defaultLogRetentionDays: 30

  logging:
    level: info
    format: json

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

securityContext:
  runAsNonRoot: true
  runAsUser: 65534

service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
  hosts:
    - host: guardian.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: guardian-tls
      hosts:
        - guardian.example.com

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring
    labels:
      release: prometheus

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
```

## Related

- [Installation](/docs/getting-started/installation) - Installation guide
- [High Availability](/docs/guides/high-availability) - HA configuration
- [Storage Configuration](/docs/configuration/storage/sqlite) - Storage backends
