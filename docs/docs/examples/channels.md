---
sidebar_position: 2
title: Channel Examples
description: AlertChannel configuration examples
---

# AlertChannel Examples

Collection of AlertChannel configurations for different integrations.

## Slack

### Basic Slack Channel

```yaml title="slack-basic.yaml"
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

### Customized Slack Channel

```yaml title="slack-custom.yaml"
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
    messageTemplate: "*{{ .Severity | upper }}*: {{ .Title }}\n\n*CronJob*: {{ .Namespace }}/{{ .CronJobName }}\n*Time*: {{ .Timestamp }}\n\n{{ .Message }}"
  rateLimiting:
    burstLimit: 20
    maxAlertsPerHour: 200
```

## PagerDuty

### Basic PagerDuty Channel

```yaml title="pagerduty-basic.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: ops-pagerduty
spec:
  type: pagerduty
  pagerduty:
    routingKeySecretRef:
      name: pagerduty-key
      namespace: default
      key: routingKey
```

### PagerDuty with Severity Setting

```yaml title="pagerduty-critical.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: critical-pagerduty
spec:
  type: pagerduty
  pagerduty:
    routingKeySecretRef:
      name: pagerduty-key
      namespace: default
      key: routingKey
    severity: critical
  rateLimiting:
    burstLimit: 30
    maxAlertsPerHour: 300
```

## Webhook

### Generic Webhook

```yaml title="webhook-generic.yaml"
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
    method: POST
    headers:
      Content-Type: application/json
      X-Source: cronjob-guardian
```

### Webhook with Custom Payload

```yaml title="webhook-custom.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: authed-webhook
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
    payloadTemplate: |
      {
        "source": "cronjob-guardian",
        "severity": "{{ .Severity }}",
        "cronjob": "{{ .Namespace }}/{{ .CronJobName }}",
        "type": "{{ .AlertType }}",
        "message": "{{ .Message }}",
        "timestamp": "{{ .Timestamp }}"
      }
```

### Microsoft Teams Webhook

```yaml title="webhook-teams.yaml"
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
        }],
        "potentialAction": [{
          "@type": "OpenUri",
          "name": "View Dashboard",
          "targets": [{"os": "default", "uri": "https://guardian.example.com"}]
        }]
      }
```

### Discord Webhook

```yaml title="webhook-discord.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: discord-alerts
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: discord-webhook
      namespace: default
      key: url
    payloadTemplate: |
      {
        "embeds": [{
          "title": "{{ .Title }}",
          "description": "{{ .Message }}",
          "color": {{ if eq .Severity "critical" }}15158332{{ else if eq .Severity "warning" }}15105570{{ else }}3447003{{ end }},
          "fields": [
            {"name": "CronJob", "value": "{{ .Namespace }}/{{ .CronJobName }}", "inline": true},
            {"name": "Severity", "value": "{{ .Severity | upper }}", "inline": true}
          ],
          "timestamp": "{{ .Timestamp }}"
        }]
      }
```

## Email

### Basic Email Channel

```yaml title="email-basic.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: ops-email
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - oncall@example.com
```

### Email with Custom Template

```yaml title="email-custom.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: team-email
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: cronjob-guardian@example.com
    to:
      - platform-team@example.com
    subjectTemplate: "[{{ .Severity | upper }}] {{ .CronJobName }} - {{ .AlertType }}"
    bodyTemplate: |
      CronJob Guardian Alert

      Title: {{ .Title }}

      CronJob: {{ .Namespace }}/{{ .CronJobName }}
      Severity: {{ .Severity | upper }}
      Time: {{ .Timestamp }}

      {{ .Message }}

      {{ if .SuggestedFix }}
      Suggested Fix:
      {{ .SuggestedFix }}
      {{ end }}

      ---
      Sent by CronJob Guardian
  rateLimiting:
    burstLimit: 10
    maxAlertsPerHour: 50
```

## Multi-Channel Setup

Typical production setup with multiple channels:

```yaml title="multi-channel-setup.yaml"
---
# Critical alerts to PagerDuty
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: pagerduty-critical
spec:
  type: pagerduty
  pagerduty:
    routingKeySecretRef:
      name: pagerduty-key
      namespace: default
      key: routingKey
    severity: critical
---
# All alerts to Slack
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
---
# Critical email for audit trail
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: audit-email
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - audit-log@example.com
    subjectTemplate: "[AUDIT] {{ .CronJobName }} - {{ .AlertType }}"
```

## Related

- [Monitor Examples](./monitors.md) - CronJobMonitor configurations
- [Slack Integration](/docs/configuration/alerting/slack) - Slack details
- [PagerDuty Integration](/docs/configuration/alerting/pagerduty) - PagerDuty details
