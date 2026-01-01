# cronjob-guardian

[![Helm Chart Version](https://img.shields.io/github/v/release/iLLeniumStudios/cronjob-guardian?filter=cronjob-guardian-*&logo=helm&label=chart)](https://github.com/iLLeniumStudios/cronjob-guardian/releases)
[![App Version](https://img.shields.io/github/v/release/iLLeniumStudios/cronjob-guardian?filter=v*&logo=github&label=app)](https://github.com/iLLeniumStudios/cronjob-guardian/releases/latest)
[![License](https://img.shields.io/github/license/iLLeniumStudios/cronjob-guardian)](https://github.com/iLLeniumStudios/cronjob-guardian/blob/main/LICENSE)

A Kubernetes operator for intelligent CronJob monitoring, SLA tracking, and auto-remediation.

**Homepage:** <https://github.com/iLLeniumStudios/cronjob-guardian>

## Prerequisites

- Kubernetes 1.26+
- Helm 3.8+ (for OCI registry support)

## Installation

### From OCI Registry (Recommended)

```bash
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace
```

### From Local Chart

```bash
git clone https://github.com/iLLeniumStudios/cronjob-guardian.git
cd cronjob-guardian

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

## Values



### General


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>replicaCount</td>
<td>

Number of replicas. Use 1 for leader election, or increase with leaderElection.enabled=true

</td>
<td>number</td>
<td>

```yaml
1
```

</td>
</tr>
<tr>

<td>nameOverride</td>
<td>

Override the name of the chart

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>fullnameOverride</td>
<td>

Override the full name of the chart

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
</table>

### Image


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>image.registry</td>
<td>

Image registry (optional, prepended to repository if set)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>image.repository</td>
<td>

Image repository

</td>
<td>string</td>
<td>

```yaml
ghcr.io/illeniumstudios/cronjob-guardian
```

</td>
</tr>
<tr>

<td>image.pullPolicy</td>
<td>

Image pull policy

</td>
<td>string</td>
<td>

```yaml
IfNotPresent
```

</td>
</tr>
<tr>

<td>image.tag</td>
<td>

Image tag (defaults to appVersion)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>imagePullSecrets</td>
<td>

Image pull secrets for private registries

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
</table>

### ServiceAccount & RBAC


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>serviceAccount.create</td>
<td>

Create a ServiceAccount

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>serviceAccount.automount</td>
<td>

Automatically mount ServiceAccount token

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>serviceAccount.annotations</td>
<td>

ServiceAccount annotations

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>serviceAccount.name</td>
<td>

ServiceAccount name (auto-generated if not set)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>rbac.create</td>
<td>

Create ClusterRole and ClusterRoleBinding

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
</table>

### Pod Configuration


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>podAnnotations</td>
<td>

Pod annotations

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>podLabels</td>
<td>

Pod labels

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>podSecurityContext</td>
<td>

Pod security context

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>securityContext</td>
<td>

Container security context

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>terminationGracePeriodSeconds</td>
<td>

Termination grace period in seconds

</td>
<td>number</td>
<td>

```yaml
10
```

</td>
</tr>
<tr>

<td>resources.limits.cpu</td>
<td>

</td>
<td>string</td>
<td>

```yaml
500m
```

</td>
</tr>
<tr>

<td>resources.limits.memory</td>
<td>

</td>
<td>string</td>
<td>

```yaml
256Mi
```

</td>
</tr>
<tr>

<td>resources.requests.cpu</td>
<td>

</td>
<td>string</td>
<td>

```yaml
10m
```

</td>
</tr>
<tr>

<td>resources.requests.memory</td>
<td>

</td>
<td>string</td>
<td>

```yaml
64Mi
```

</td>
</tr>
<tr>

<td>livenessProbe.initialDelaySeconds</td>
<td>

</td>
<td>number</td>
<td>

```yaml
15
```

</td>
</tr>
<tr>

<td>livenessProbe.periodSeconds</td>
<td>

</td>
<td>number</td>
<td>

```yaml
20
```

</td>
</tr>
<tr>

<td>livenessProbe.timeoutSeconds</td>
<td>

</td>
<td>number</td>
<td>

```yaml
1
```

</td>
</tr>
<tr>

<td>livenessProbe.failureThreshold</td>
<td>

</td>
<td>number</td>
<td>

```yaml
3
```

</td>
</tr>
<tr>

<td>readinessProbe.initialDelaySeconds</td>
<td>

</td>
<td>number</td>
<td>

```yaml
5
```

</td>
</tr>
<tr>

<td>readinessProbe.periodSeconds</td>
<td>

</td>
<td>number</td>
<td>

```yaml
10
```

</td>
</tr>
<tr>

<td>readinessProbe.timeoutSeconds</td>
<td>

</td>
<td>number</td>
<td>

```yaml
1
```

</td>
</tr>
<tr>

<td>readinessProbe.failureThreshold</td>
<td>

</td>
<td>number</td>
<td>

```yaml
3
```

</td>
</tr>
<tr>

<td>extraEnv</td>
<td>

Additional environment variables

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
<tr>

<td>extraVolumeMounts</td>
<td>

Additional volume mounts

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
<tr>

<td>extraVolumes</td>
<td>

Additional volumes

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
</table>

### Scheduling


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>nodeSelector</td>
<td>

Node selector

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>tolerations</td>
<td>

Tolerations

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
<tr>

<td>affinity</td>
<td>

Affinity rules

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
</table>

### Operator Configuration


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>config.logLevel</td>
<td>

Log level (debug, info, warn, error)

</td>
<td>string</td>
<td>

```yaml
info
```

</td>
</tr>
<tr>

<td>config.scheduler.deadManSwitchInterval</td>
<td>

Dead-man's switch check interval

</td>
<td>string</td>
<td>

```yaml
1m
```

</td>
</tr>
<tr>

<td>config.scheduler.slaRecalculationInterval</td>
<td>

SLA recalculation interval

</td>
<td>string</td>
<td>

```yaml
5m
```

</td>
</tr>
<tr>

<td>config.scheduler.pruneInterval</td>
<td>

History prune interval

</td>
<td>string</td>
<td>

```yaml
1h
```

</td>
</tr>
<tr>

<td>config.scheduler.startupGracePeriod</td>
<td>

Grace period after startup before sending alerts (prevents alert floods on restart)

</td>
<td>string</td>
<td>

```yaml
30s
```

</td>
</tr>
<tr>

<td>config.historyRetention.defaultDays</td>
<td>

Default retention period in days

</td>
<td>number</td>
<td>

```yaml
30
```

</td>
</tr>
<tr>

<td>config.historyRetention.maxDays</td>
<td>

Maximum retention period in days

</td>
<td>number</td>
<td>

```yaml
90
```

</td>
</tr>
<tr>

<td>config.rateLimits.maxAlertsPerMinute</td>
<td>

Maximum alerts per minute across all channels

</td>
<td>number</td>
<td>

```yaml
50
```

</td>
</tr>
</table>

### Storage


Configuration for the storage backend. Supports SQLite (default), PostgreSQL, and MySQL.

<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>config.storage.type</td>
<td>

Storage type: sqlite, postgres, or mysql

</td>
<td>string</td>
<td>

```yaml
sqlite
```

</td>
</tr>
<tr>

<td>config.storage.sqlite.path</td>
<td>

Path to SQLite database file

</td>
<td>string</td>
<td>

```yaml
/data/guardian.db
```

</td>
</tr>
<tr>

<td>config.storage.postgres.host</td>
<td>

PostgreSQL host

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.postgres.port</td>
<td>

PostgreSQL port

</td>
<td>number</td>
<td>

```yaml
5432
```

</td>
</tr>
<tr>

<td>config.storage.postgres.database</td>
<td>

PostgreSQL database name

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.postgres.username</td>
<td>

PostgreSQL username

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.postgres.password</td>
<td>

PostgreSQL password (ignored if existingSecret is set)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.postgres.existingSecret</td>
<td>

Use existing secret for password

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.postgres.existingSecretKey</td>
<td>

Key in existing secret containing password

</td>
<td>string</td>
<td>

```yaml
password
```

</td>
</tr>
<tr>

<td>config.storage.postgres.sslMode</td>
<td>

PostgreSQL SSL mode

</td>
<td>string</td>
<td>

```yaml
require
```

</td>
</tr>
<tr>

<td>config.storage.mysql.host</td>
<td>

MySQL host

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.mysql.port</td>
<td>

MySQL port

</td>
<td>number</td>
<td>

```yaml
3306
```

</td>
</tr>
<tr>

<td>config.storage.mysql.database</td>
<td>

MySQL database name

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.mysql.username</td>
<td>

MySQL username

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.mysql.password</td>
<td>

MySQL password (ignored if existingSecret is set)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.mysql.existingSecret</td>
<td>

Use existing secret for password

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>config.storage.mysql.existingSecretKey</td>
<td>

Key in existing secret containing password

</td>
<td>string</td>
<td>

```yaml
password
```

</td>
</tr>
<tr>

<td>config.storage.logStorageEnabled</td>
<td>

Enable storing job logs in database

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
<tr>

<td>config.storage.eventStorageEnabled</td>
<td>

Enable storing K8s events in database

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
<tr>

<td>config.storage.maxLogSizeKB</td>
<td>

Maximum log size to store per execution (KB)

</td>
<td>number</td>
<td>

```yaml
100
```

</td>
</tr>
<tr>

<td>config.storage.logRetentionDays</td>
<td>

Log retention days (0 = use history-retention.default-days)

</td>
<td>number</td>
<td>

```yaml
0
```

</td>
</tr>
</table>

### Persistence


Persistence configuration for SQLite storage backend.

<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>persistence.enabled</td>
<td>

Enable persistence (required for SQLite)

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>persistence.storageClass</td>
<td>

Storage class (use "-" for default, or specify a class name)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>persistence.accessModes[0]</td>
<td>

</td>
<td>string</td>
<td>

```yaml
ReadWriteOnce
```

</td>
</tr>
<tr>

<td>persistence.size</td>
<td>

Storage size

</td>
<td>string</td>
<td>

```yaml
1Gi
```

</td>
</tr>
<tr>

<td>persistence.annotations</td>
<td>

PVC annotations

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>persistence.selector</td>
<td>

PVC selector

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
</table>

### UI & Ingress


Configuration for the web UI and REST API server.

<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>ui.enabled</td>
<td>

Enable UI server

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>ui.port</td>
<td>

UI server port

</td>
<td>number</td>
<td>

```yaml
8080
```

</td>
</tr>
<tr>

<td>ui.service.type</td>
<td>

Service type (ClusterIP, NodePort, LoadBalancer)

</td>
<td>string</td>
<td>

```yaml
ClusterIP
```

</td>
</tr>
<tr>

<td>ui.service.port</td>
<td>

Service port

</td>
<td>number</td>
<td>

```yaml
8080
```

</td>
</tr>
<tr>

<td>ui.service.nodePort</td>
<td>

NodePort (only used if type=NodePort)

</td>
<td>unknown</td>
<td>

```yaml
null
```

</td>
</tr>
<tr>

<td>ui.service.annotations</td>
<td>

Service annotations

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>ui.ingress.enabled</td>
<td>

Enable ingress

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
<tr>

<td>ui.ingress.className</td>
<td>

Ingress class name

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>ui.ingress.annotations</td>
<td>

Ingress annotations

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>ui.ingress.hosts[0].host</td>
<td>

</td>
<td>string</td>
<td>

```yaml
cronjob-guardian.local
```

</td>
</tr>
<tr>

<td>ui.ingress.hosts[0].paths[0].path</td>
<td>

</td>
<td>string</td>
<td>

```yaml
/
```

</td>
</tr>
<tr>

<td>ui.ingress.hosts[0].paths[0].pathType</td>
<td>

</td>
<td>string</td>
<td>

```yaml
Prefix
```

</td>
</tr>
<tr>

<td>ui.ingress.tls</td>
<td>

Ingress TLS configuration

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
<tr>

<td>ui.route.enabled</td>
<td>

Enable OpenShift Route

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
<tr>

<td>ui.route.host</td>
<td>

Route host (leave empty for auto-generated)

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>ui.route.path</td>
<td>

Route path

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>ui.route.annotations</td>
<td>

Route annotations

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>ui.route.tls.enabled</td>
<td>

Enable TLS termination

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>ui.route.tls.termination</td>
<td>

TLS termination type (edge, passthrough, reencrypt)

</td>
<td>string</td>
<td>

```yaml
edge
```

</td>
</tr>
<tr>

<td>ui.route.tls.insecureEdgeTerminationPolicy</td>
<td>

Insecure edge termination policy (Allow, Redirect, None)

</td>
<td>string</td>
<td>

```yaml
Redirect
```

</td>
</tr>
</table>

### Metrics & Monitoring


Prometheus metrics and ServiceMonitor configuration.

<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>metrics.enabled</td>
<td>

Enable metrics endpoint

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>metrics.bindAddress</td>
<td>

Metrics bind address (port only, e.g., ":8443")

</td>
<td>string</td>
<td>

```yaml
:8443
```

</td>
</tr>
<tr>

<td>metrics.secure</td>
<td>

Enable HTTPS for metrics

</td>
<td>bool</td>
<td>

```yaml
true
```

</td>
</tr>
<tr>

<td>metrics.certPath</td>
<td>

Path to TLS certificate directory

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>metrics.certName</td>
<td>

TLS certificate file name

</td>
<td>string</td>
<td>

```yaml
tls.crt
```

</td>
</tr>
<tr>

<td>metrics.certKey</td>
<td>

TLS key file name

</td>
<td>string</td>
<td>

```yaml
tls.key
```

</td>
</tr>
<tr>

<td>probes.bindAddress</td>
<td>

Probes bind address

</td>
<td>string</td>
<td>

```yaml
:8081
```

</td>
</tr>
<tr>

<td>serviceMonitor.enabled</td>
<td>

Enable ServiceMonitor

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
<tr>

<td>serviceMonitor.labels</td>
<td>

ServiceMonitor labels

</td>
<td>object</td>
<td>

```yaml
{}
```

</td>
</tr>
<tr>

<td>serviceMonitor.interval</td>
<td>

Scrape interval

</td>
<td>string</td>
<td>

```yaml
30s
```

</td>
</tr>
<tr>

<td>serviceMonitor.scrapeTimeout</td>
<td>

Scrape timeout

</td>
<td>string</td>
<td>

```yaml
10s
```

</td>
</tr>
<tr>

<td>serviceMonitor.metricRelabelings</td>
<td>

Metric relabelings

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
<tr>

<td>serviceMonitor.relabelings</td>
<td>

Relabelings

</td>
<td>array</td>
<td>

```yaml
[]
```

</td>
</tr>
</table>

### High Availability


Leader election configuration for running multiple replicas.

<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>leaderElection.enabled</td>
<td>

Enable leader election (required for multiple replicas)

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
<tr>

<td>leaderElection.leaseDuration</td>
<td>

Leader lease duration

</td>
<td>string</td>
<td>

```yaml
15s
```

</td>
</tr>
<tr>

<td>leaderElection.renewDeadline</td>
<td>

Leader renew deadline

</td>
<td>string</td>
<td>

```yaml
10s
```

</td>
</tr>
<tr>

<td>leaderElection.retryPeriod</td>
<td>

Leader retry period

</td>
<td>string</td>
<td>

```yaml
2s
```

</td>
</tr>
</table>

### Webhook


<table>
<tr>
<th>Property</th>
<th>Description</th>
<th>Type</th>
<th>Default</th>
</tr>
<tr>

<td>webhook.certPath</td>
<td>

Path to TLS certificate directory

</td>
<td>string</td>
<td>

```yaml
""
```

</td>
</tr>
<tr>

<td>webhook.certName</td>
<td>

TLS certificate file name

</td>
<td>string</td>
<td>

```yaml
tls.crt
```

</td>
</tr>
<tr>

<td>webhook.certKey</td>
<td>

TLS key file name

</td>
<td>string</td>
<td>

```yaml
tls.key
```

</td>
</tr>
<tr>

<td>webhook.enableHTTP2</td>
<td>

Enable HTTP/2 for webhook server

</td>
<td>bool</td>
<td>

```yaml
false
```

</td>
</tr>
</table>

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
