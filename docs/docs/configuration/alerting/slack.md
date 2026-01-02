---
sidebar_position: 1
title: Slack
description: Configure Slack alerts
---

# Slack Integration

Send alerts to Slack channels via incoming webhooks.

## Prerequisites

1. A Slack workspace with admin access
2. An incoming webhook URL

### Creating a Webhook

1. Go to [Slack Apps](https://api.slack.com/apps)
2. Create a new app or select existing
3. Enable **Incoming Webhooks**
4. Add webhook to your channel
5. Copy the webhook URL

## Configuration

### Create the Secret

Store the webhook URL in a Kubernetes secret:

```bash
kubectl create secret generic slack-webhook \
  --from-literal=url=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### Create the AlertChannel

```yaml title="slack-channel.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: team-slack
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: default
      key: url
```

Apply:

```bash
kubectl apply -f slack-channel.yaml
```

## Customization

### Channel Override

Send to a different channel than the webhook default:

```yaml
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: default
      key: url
    defaultChannel: "#alerts-critical"
```

:::info
Custom username and icon override are not currently supported. The webhook's default settings will be used.
:::

### Message Template

Customize the message format:

```yaml
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: default
      key: url
    messageTemplate: |
      *{{ .Severity | upper }}*: {{ .Title }}

      {{ .Message }}

      *CronJob*: {{ .Namespace }}/{{ .CronJobName }}
      *Time*: {{ .Timestamp }}

      {{ if .SuggestedFix }}
      *Suggested Fix*:
      {{ .SuggestedFix }}
      {{ end }}
```

## Rate Limiting

Prevent alert floods by configuring rate limits at the AlertChannel level:

```yaml
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: default
      key: url
  rateLimiting:
    burstLimit: 10
    maxAlertsPerHour: 100
```

## Complete Example

```yaml title="slack-complete.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: ops-slack
spec:
  type: slack
  slack:
    webhookSecretRef:
      name: slack-webhook
      namespace: default
      key: url
    defaultChannel: "#cronjob-alerts"
  rateLimiting:
    burstLimit: 20
    maxAlertsPerHour: 200
```

## Testing

Test the channel from the dashboard:

1. Go to **Channels**
2. Find your Slack channel
3. Click **Test**

Or via API:

```bash
curl -X POST http://localhost:8080/api/v1/channels/ops-slack/test
```

## Alert Format

Slack alerts include:

- **Color-coded attachment**: Red (critical), yellow (warning), blue (info)
- **Title**: Alert type and CronJob name
- **Fields**: Namespace, status, timestamps
- **Logs section**: Recent pod logs (if enabled)
- **Suggested fix**: Actionable remediation steps

## Troubleshooting

### Webhook Not Working

1. Verify secret exists: `kubectl get secret slack-webhook`
2. Check secret content: `kubectl get secret slack-webhook -o jsonpath='{.data.url}' | base64 -d`
3. Test webhook directly: `curl -X POST -H 'Content-type: application/json' --data '{"text":"test"}' YOUR_WEBHOOK_URL`

### Rate Limited

If you're hitting rate limits:
- Increase `suppressDuplicatesFor` on monitors
- Use `alertDelay` to batch alerts
- Review if too many CronJobs are failing

### Channel Not Found

- Ensure the webhook is configured for the correct channel
- The `channel` override must match an existing Slack channel

## Related

- [Alert Configuration](/docs/configuration/monitors/alerting) - Monitor alerting settings
- [PagerDuty](./pagerduty.md) - Alternative alerting
- [Webhook](./webhook.md) - Custom integrations
