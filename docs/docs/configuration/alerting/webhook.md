---
sidebar_position: 3
title: Webhook
description: Configure generic webhook alerts
---

# Webhook Integration

Send alerts to any HTTP endpoint with customizable payloads.

## Basic Configuration

### Create the Secret

```bash
kubectl create secret generic webhook-url \
  --from-literal=url=https://api.example.com/alerts
```

### Create the AlertChannel

```yaml title="webhook-channel.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: custom-webhook
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: webhook-url
      namespace: default
      key: url
```

## HTTP Method

Default is POST, but can be configured:

```yaml
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: webhook-url
      namespace: default
      key: url
    method: POST    # POST or PUT
```

## Custom Headers

Add authentication or custom headers:

```yaml
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: webhook-url
      namespace: default
      key: url
    headers:
      Authorization: "Bearer your-token"
      Content-Type: application/json
      X-Source: cronjob-guardian
```

## Payload Template

Customize the JSON payload:

```yaml
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: webhook-url
      namespace: default
      key: url
    payloadTemplate: |
      {
        "alert": {
          "type": "{{ .AlertType }}",
          "severity": "{{ .Severity }}",
          "title": "{{ .Title }}",
          "message": "{{ .Message }}"
        },
        "cronjob": {
          "namespace": "{{ .Namespace }}",
          "name": "{{ .CronJobName }}",
          "jobName": "{{ .JobName }}"
        },
        "timestamp": "{{ .Timestamp }}",
        "suggestedFix": {{ .SuggestedFix | toJson }}
      }
```

### Template Variables

| Variable | Description |
|----------|-------------|
| `{{ .AlertType }}` | Type: failure, deadManSwitch, slaViolation, etc. |
| `{{ .Severity }}` | critical, warning, info |
| `{{ .Title }}` | Alert title |
| `{{ .Message }}` | Full alert message |
| `{{ .Namespace }}` | CronJob namespace |
| `{{ .CronJobName }}` | CronJob name |
| `{{ .JobName }}` | Job name (with timestamp) |
| `{{ .Timestamp }}` | Alert timestamp (ISO 8601) |
| `{{ .SuggestedFix }}` | Suggested fix text |
| `{{ .Logs }}` | Pod logs (if enabled) |
| `{{ .Events }}` | Kubernetes events (if enabled) |

### Template Functions

| Function | Description | Example |
|----------|-------------|---------|
| `toJson` | JSON encode | `{{ .Logs \| toJson }}` |
| `upper` | Uppercase | `{{ .Severity \| upper }}` |
| `lower` | Lowercase | `{{ .AlertType \| lower }}` |
| `default` | Default value | `{{ .Value \| default "N/A" }}` |

## Complete Examples

### Custom Monitoring System

```yaml title="monitoring-webhook.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: monitoring-system
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: monitoring-webhook
      namespace: default
      key: url
    method: POST
    headers:
      Content-Type: application/json
    payloadTemplate: |
      {
        "source": "cronjob-guardian",
        "event_type": "cronjob_{{ .AlertType }}",
        "severity": "{{ .Severity }}",
        "resource": {
          "type": "kubernetes_cronjob",
          "namespace": "{{ .Namespace }}",
          "name": "{{ .CronJobName }}"
        },
        "details": {
          "message": "{{ .Message }}",
          "suggested_action": {{ .SuggestedFix | toJson }}
        },
        "occurred_at": "{{ .Timestamp }}"
      }
```

### Incident Management System

```yaml title="incident-webhook.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: incident-system
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: incident-webhook
      namespace: default
      key: url
    headers:
      Content-Type: application/json
    payloadTemplate: |
      {
        "title": "[{{ .Severity | upper }}] {{ .CronJobName }} - {{ .AlertType }}",
        "description": "{{ .Message }}",
        "priority": {{ if eq .Severity "critical" }}"P1"{{ else if eq .Severity "warning" }}"P2"{{ else }}"P3"{{ end }},
        "labels": ["cronjob", "{{ .Namespace }}", "{{ .AlertType }}"],
        "custom_fields": {
          "namespace": "{{ .Namespace }}",
          "cronjob": "{{ .CronJobName }}",
          "runbook": "https://wiki.example.com/cronjobs/{{ .CronJobName }}"
        }
      }
```

### Microsoft Teams

```yaml title="teams-webhook.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: teams-alerts
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: teams-webhook
      namespace: default
      key: url
    payloadTemplate: |
      {
        "@type": "MessageCard",
        "@context": "http://schema.org/extensions",
        "themeColor": {{ if eq .Severity "critical" }}"FF0000"{{ else if eq .Severity "warning" }}"FFA500"{{ else }}"0076D7"{{ end }},
        "summary": "{{ .Title }}",
        "sections": [{
          "activityTitle": "{{ .Title }}",
          "facts": [
            {"name": "CronJob", "value": "{{ .Namespace }}/{{ .CronJobName }}"},
            {"name": "Severity", "value": "{{ .Severity | upper }}"},
            {"name": "Type", "value": "{{ .AlertType }}"}
          ],
          "text": "{{ .Message }}"
        }]
      }
```

## Rate Limiting

Configure rate limits at the AlertChannel level:

```yaml
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: webhook-url
      namespace: default
      key: url
  rateLimiting:
    burstLimit: 60
    maxAlertsPerHour: 500
```

## Testing

```bash
curl -X POST http://localhost:8080/api/v1/channels/custom-webhook/test
```

## Troubleshooting

### Connection Refused

- Verify URL is correct and accessible from the cluster
- Check network policies

### Authentication Errors

- Verify secrets exist and contain correct values
- Check header format

### Payload Rejected

- Test payload template with sample data
- Validate JSON syntax
- Check API documentation for required fields

## Related

- [Slack](./slack.md) - Slack integration
- [PagerDuty](./pagerduty.md) - PagerDuty integration
- [Email](./email.md) - Email alerts
