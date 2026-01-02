---
sidebar_position: 2
title: Installation
description: Install CronJob Guardian in your Kubernetes cluster
---

# Installation

This guide covers installing CronJob Guardian in your Kubernetes cluster.

## Prerequisites

- Kubernetes 1.26+
- kubectl configured with cluster access
- Helm 3.8+ (for OCI registry support)

## Helm Installation (Recommended)

CronJob Guardian is distributed as an OCI Helm chart.

### Basic Installation

Install with default configuration (SQLite storage):

```bash
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace
```

### Custom Values

Install with custom configuration:

```bash
helm install cronjob-guardian oci://ghcr.io/illeniumstudios/charts/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace \
  --values values.yaml
```

### PostgreSQL Storage

For production deployments, use PostgreSQL for better durability and HA support:

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

### High Availability

For HA deployments with multiple replicas:

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

## kubectl Installation

If you prefer not to use Helm, you can install using kubectl.

### From URL

```bash
kubectl apply -f https://raw.githubusercontent.com/iLLeniumStudios/cronjob-guardian/main/dist/install.yaml
```

### From Source

Clone the repository and build:

```bash
# Clone the repository
git clone https://github.com/iLLeniumStudios/cronjob-guardian.git
cd cronjob-guardian

# Build and push your own image
make docker-build docker-push IMG=your-registry/cronjob-guardian:latest

# Deploy
make deploy IMG=your-registry/cronjob-guardian:latest
```

## Local Chart Installation

For development or customization, install from the local chart:

```bash
git clone https://github.com/iLLeniumStudios/cronjob-guardian.git
cd cronjob-guardian

helm install cronjob-guardian ./deploy/helm/cronjob-guardian \
  --namespace cronjob-guardian \
  --create-namespace
```

## Verification

After installation, verify the operator is running:

```bash
# Check pods
kubectl get pods -n cronjob-guardian

# Expected output:
# NAME                                 READY   STATUS    RESTARTS   AGE
# cronjob-guardian-xxxxxxxxxx-xxxxx   1/1     Running   0          1m

# Check CRDs are installed
kubectl get crd | grep guardian

# Expected output:
# alertchannels.guardian.illenium.net    2024-01-01T00:00:00Z
# cronjobmonitors.guardian.illenium.net  2024-01-01T00:00:00Z
```

## Accessing the Dashboard

The dashboard is available on port 8080. Use port-forward for quick access:

```bash
kubectl port-forward -n cronjob-guardian svc/cronjob-guardian 8080:8080
```

Then open http://localhost:8080 in your browser.

For production, configure an Ingress or OpenShift Route. See [Helm Values Reference](/docs/reference/helm-values) for details.

## Next Steps

- [Quick Start](./quick-start.md) - Create your first monitor
- [Features](/docs/features/dead-man-switch) - Explore all features
- [Configuration](/docs/configuration/monitors/selectors) - Configure monitors and alerts

## Uninstalling

### Helm

```bash
# Uninstall the release
helm uninstall cronjob-guardian --namespace cronjob-guardian

# Delete CRDs (optional - removes all CronJobMonitor and AlertChannel data)
kubectl delete crd cronjobmonitors.guardian.illenium.net
kubectl delete crd alertchannels.guardian.illenium.net

# Delete the namespace
kubectl delete namespace cronjob-guardian
```

### kubectl

```bash
# Remove all CronJobMonitor and AlertChannel resources
kubectl delete cronjobmonitors --all-namespaces --all
kubectl delete alertchannels --all-namespaces --all

# Remove the operator
make undeploy

# Remove CRDs
make uninstall
```
