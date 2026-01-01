# CronJob Guardian Helm Chart

This directory contains the Helm chart for deploying CronJob Guardian to Kubernetes.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+ (for OCI registry support)

## Installation

### From OCI Registry (Recommended)

CronJob Guardian is distributed as an OCI Helm chart:

```bash
# Install with default configuration (SQLite storage)
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace

# Install with custom values
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace \
  --values values.yaml
```

### From Local Chart

```bash
# Clone the repository
git clone https://github.com/iLLeniumStudios/cronjob-guardian.git
cd cronjob-guardian

# Install using local chart
helm install cronjob-guardian ./deploy/helm/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace
```

## Quick Start Examples

### Default Installation (SQLite)

```bash
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace
```

### With PostgreSQL

```bash
# Create a secret for database credentials
kubectl create namespace cronjob-guardian
kubectl create secret generic postgres-credentials \
  --namespace cronjob-guardian \
  --from-literal=password=your-secure-password

# Install with PostgreSQL storage
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --set config.storage.type=postgres \
  --set config.storage.postgres.host=postgres.database.svc \
  --set config.storage.postgres.database=guardian \
  --set config.storage.postgres.username=guardian \
  --set config.storage.postgres.existingSecret=postgres-credentials
```

### High Availability Setup

```bash
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace \
  --set replicaCount=2 \
  --set leaderElection.enabled=true \
  --set config.storage.type=postgres \
  --set config.storage.postgres.host=postgres.database.svc \
  --set config.storage.postgres.database=guardian \
  --set config.storage.postgres.username=guardian \
  --set config.storage.postgres.existingSecret=postgres-credentials
```

## Configuration

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.registry` | Image registry (optional prefix) | `""` |
| `image.repository` | Image repository | `ghcr.io/illeniumstudios/cronjob-guardian` |
| `image.tag` | Image tag | `appVersion` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |

### Storage Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.storage.type` | Storage backend (`sqlite`, `postgres`, `mysql`) | `sqlite` |
| `config.storage.sqlite.path` | SQLite database path | `/data/guardian.db` |
| `config.storage.logStorageEnabled` | Store job logs in database | `false` |
| `config.storage.eventStorageEnabled` | Store K8s events in database | `false` |
| `config.storage.maxLogSizeKB` | Maximum log size per execution (KB) | `100` |
| `config.storage.logRetentionDays` | Log retention days (0 = use default) | `0` |

#### PostgreSQL

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.storage.postgres.host` | PostgreSQL host | `""` |
| `config.storage.postgres.port` | PostgreSQL port | `5432` |
| `config.storage.postgres.database` | PostgreSQL database name | `""` |
| `config.storage.postgres.username` | PostgreSQL username | `""` |
| `config.storage.postgres.password` | PostgreSQL password (ignored if existingSecret is set) | `""` |
| `config.storage.postgres.existingSecret` | Secret containing password | `""` |
| `config.storage.postgres.existingSecretKey` | Key in secret containing password | `password` |
| `config.storage.postgres.sslMode` | PostgreSQL SSL mode | `require` |

#### MySQL

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.storage.mysql.host` | MySQL host | `""` |
| `config.storage.mysql.port` | MySQL port | `3306` |
| `config.storage.mysql.database` | MySQL database name | `""` |
| `config.storage.mysql.username` | MySQL username | `""` |
| `config.storage.mysql.password` | MySQL password (ignored if existingSecret is set) | `""` |
| `config.storage.mysql.existingSecret` | Secret containing password | `""` |
| `config.storage.mysql.existingSecretKey` | Key in secret containing password | `password` |

### Persistence (SQLite)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `persistence.enabled` | Enable PVC for SQLite | `true` |
| `persistence.storageClass` | Storage class (`""` for default, `"-"` for no class) | `""` |
| `persistence.size` | PVC size | `1Gi` |
| `persistence.accessModes` | PVC access modes | `[ReadWriteOnce]` |
| `persistence.annotations` | PVC annotations | `{}` |
| `persistence.selector` | PVC selector | `{}` |

### Operator Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.logLevel` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `config.scheduler.deadManSwitchInterval` | Dead-man's switch check interval | `1m` |
| `config.scheduler.slaRecalculationInterval` | SLA recalculation interval | `5m` |
| `config.scheduler.pruneInterval` | History prune interval | `1h` |
| `config.scheduler.startupGracePeriod` | Grace period after startup before sending alerts | `30s` |
| `config.historyRetention.defaultDays` | Default history retention | `30` |
| `config.historyRetention.maxDays` | Maximum history retention | `90` |
| `config.rateLimits.maxAlertsPerMinute` | Maximum alerts per minute | `50` |

### UI Server (Web UI & REST API)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.enabled` | Enable UI server (serves both web UI and REST API) | `true` |
| `ui.port` | UI server port | `8080` |
| `ui.service.type` | UI service type | `ClusterIP` |
| `ui.service.port` | UI service port | `8080` |
| `ui.service.nodePort` | NodePort (only if type=NodePort) | `null` |
| `ui.service.annotations` | UI service annotations | `{}` |

### Ingress

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.ingress.enabled` | Enable Ingress | `false` |
| `ui.ingress.className` | Ingress class name | `""` |
| `ui.ingress.annotations` | Ingress annotations | `{}` |
| `ui.ingress.hosts` | Ingress hosts configuration | See values.yaml |
| `ui.ingress.tls` | Ingress TLS configuration | `[]` |

### OpenShift Route

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ui.route.enabled` | Enable OpenShift Route | `false` |
| `ui.route.host` | Route host (empty for auto-generated) | `""` |
| `ui.route.path` | Route path | `""` |
| `ui.route.annotations` | Route annotations | `{}` |
| `ui.route.tls.enabled` | Enable TLS termination | `true` |
| `ui.route.tls.termination` | TLS termination type (edge, passthrough, reencrypt) | `edge` |
| `ui.route.tls.insecureEdgeTerminationPolicy` | Insecure edge termination policy | `Redirect` |

### Metrics

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.bindAddress` | Metrics bind address | `:8443` |
| `metrics.secure` | Enable HTTPS for metrics | `true` |
| `metrics.certPath` | Path to TLS certificate directory | `""` |
| `metrics.certName` | TLS certificate file name | `tls.crt` |
| `metrics.certKey` | TLS key file name | `tls.key` |

### Health Probes

| Parameter | Description | Default |
|-----------|-------------|---------|
| `probes.bindAddress` | Probes bind address | `:8081` |
| `livenessProbe.initialDelaySeconds` | Liveness probe initial delay | `15` |
| `livenessProbe.periodSeconds` | Liveness probe period | `20` |
| `livenessProbe.timeoutSeconds` | Liveness probe timeout | `1` |
| `livenessProbe.failureThreshold` | Liveness probe failure threshold | `3` |
| `readinessProbe.initialDelaySeconds` | Readiness probe initial delay | `5` |
| `readinessProbe.periodSeconds` | Readiness probe period | `10` |
| `readinessProbe.timeoutSeconds` | Readiness probe timeout | `1` |
| `readinessProbe.failureThreshold` | Readiness probe failure threshold | `3` |

### High Availability

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `leaderElection.enabled` | Enable leader election | `false` |
| `leaderElection.leaseDuration` | Leader lease duration | `15s` |
| `leaderElection.renewDeadline` | Leader renew deadline | `10s` |
| `leaderElection.retryPeriod` | Leader retry period | `2s` |

### Prometheus Integration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceMonitor.enabled` | Create ServiceMonitor for Prometheus Operator | `false` |
| `serviceMonitor.labels` | Additional labels for ServiceMonitor | `{}` |
| `serviceMonitor.interval` | Scrape interval | `30s` |
| `serviceMonitor.scrapeTimeout` | Scrape timeout | `10s` |
| `serviceMonitor.metricRelabelings` | Metric relabelings | `[]` |
| `serviceMonitor.relabelings` | Relabelings | `[]` |

### Webhook Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhook.certPath` | Path to TLS certificate directory | `""` |
| `webhook.certName` | TLS certificate file name | `tls.crt` |
| `webhook.certKey` | TLS key file name | `tls.key` |
| `webhook.enableHTTP2` | Enable HTTP/2 for webhook server | `false` |

### Resources and Scheduling

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |
| `terminationGracePeriodSeconds` | Termination grace period | `10` |

### RBAC and Service Account

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create a ServiceAccount | `true` |
| `serviceAccount.automount` | Automatically mount ServiceAccount token | `true` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |
| `serviceAccount.name` | ServiceAccount name (auto-generated if not set) | `""` |
| `rbac.create` | Create ClusterRole and ClusterRoleBinding | `true` |

### Additional Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Override the name of the chart | `""` |
| `fullnameOverride` | Override the full name of the chart | `""` |
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Pod labels | `{}` |
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext` | Container security context | `{}` |
| `extraEnv` | Additional environment variables | `[]` |
| `extraVolumeMounts` | Additional volume mounts | `[]` |
| `extraVolumes` | Additional volumes | `[]` |

## Example values.yaml

### High-availability Setup with PostgreSQL

```yaml
replicaCount: 2

leaderElection:
  enabled: true

config:
  logLevel: info
  storage:
    type: postgres
    postgres:
      host: postgres.database.svc
      port: 5432
      database: guardian
      username: guardian
      existingSecret: postgres-credentials
      sslMode: require
  historyRetention:
    defaultDays: 30
    maxDays: 90

ui:
  enabled: true
  service:
    type: ClusterIP

metrics:
  enabled: true
  secure: true

serviceMonitor:
  enabled: true
  interval: 30s

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

### Development Setup (SQLite with exposed UI)

```yaml
replicaCount: 1

config:
  logLevel: debug
  storage:
    type: sqlite

persistence:
  enabled: true
  size: 1Gi

ui:
  enabled: true
  service:
    type: NodePort
    nodePort: 30080

metrics:
  enabled: true
  secure: false

resources:
  limits:
    cpu: 200m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi
```

### With Ingress

```yaml
ui:
  enabled: true
  service:
    type: ClusterIP
  ingress:
    enabled: true
    className: nginx
    annotations:
      nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    hosts:
      - host: cronjob-guardian.example.com
        paths:
          - path: /
            pathType: Prefix
    tls:
      - secretName: cronjob-guardian-tls
        hosts:
          - cronjob-guardian.example.com
```

### With OpenShift Route

```yaml
ui:
  enabled: true
  service:
    type: ClusterIP
  route:
    enabled: true
    host: cronjob-guardian.apps.example.com
    annotations:
      haproxy.router.openshift.io/timeout: 60s
    tls:
      enabled: true
      termination: edge
      insecureEdgeTerminationPolicy: Redirect
```

## Upgrading

```bash
# Upgrade to latest version
helm upgrade cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --reuse-values

# Upgrade with new values
helm upgrade cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --values values.yaml
```

## Uninstalling

```bash
# Uninstall the release
helm uninstall cronjob-guardian --namespace cronjob-guardian

# Delete CRDs (optional - this removes all CronJobMonitor and AlertChannel data)
kubectl delete crd cronjobmonitors.guardian.illenium.net
kubectl delete crd alertchannels.guardian.illenium.net

# Delete the namespace
kubectl delete namespace cronjob-guardian
```

## Chart Structure

```
deploy/helm/cronjob-guardian/
├── Chart.yaml           # Chart metadata
├── values.yaml          # Default configuration values
├── crds/                # Custom Resource Definitions
│   ├── guardian.illenium.net_alertchannels.yaml
│   └── guardian.illenium.net_cronjobmonitors.yaml
└── templates/
    ├── _helpers.tpl     # Template helpers
    ├── configmap.yaml   # Generates config.yaml from values
    ├── deployment.yaml  # Deployment with all options
    ├── ingress.yaml     # Ingress for UI (conditional)
    ├── NOTES.txt        # Installation notes
    ├── pvc.yaml         # PVC for SQLite (conditional)
    ├── rbac.yaml        # ClusterRole, ClusterRoleBinding, leader election Role
    ├── route.yaml       # OpenShift Route for UI (conditional)
    ├── service.yaml     # Metrics and UI services
    ├── serviceaccount.yaml
    └── servicemonitor.yaml  # For Prometheus Operator (conditional)
```
