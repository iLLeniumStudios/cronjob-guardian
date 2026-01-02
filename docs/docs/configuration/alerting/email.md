---
sidebar_position: 4
title: Email
description: Configure email alerts
---

# Email Integration

Send alerts via email using SMTP.

## Prerequisites

- SMTP server access
- Credentials for authentication

## Configuration

### Create the Secret

```bash
kubectl create secret generic smtp-credentials \
  --from-literal=host=smtp.example.com \
  --from-literal=port=587 \
  --from-literal=username=alerts@example.com \
  --from-literal=password=your-password
```

### Create the AlertChannel

```yaml title="email-channel.yaml"
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
      - team-leads@example.com
```

## Email Content

### Subject Template

```yaml
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - team@example.com
    subjectTemplate: "[{{ .Severity | upper }}] CronJob {{ .CronJobName }}: {{ .AlertType }}"
```

### Body Template

```yaml
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - team@example.com
    bodyTemplate: |
      CronJob Guardian Alert

      Severity: {{ .Severity | upper }}
      Type: {{ .AlertType }}
      CronJob: {{ .Namespace }}/{{ .CronJobName }}
      Time: {{ .Timestamp }}

      {{ .Message }}

      {{ if .SuggestedFix }}
      Suggested Fix:
      {{ .SuggestedFix }}
      {{ end }}

      {{ if .Logs }}
      Recent Logs:
      {{ .Logs }}
      {{ end }}

      ---
      This alert was sent by CronJob Guardian
```

### HTML Body

```yaml
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - team@example.com
    bodyTemplate: |
      <html>
      <body style="font-family: Arial, sans-serif;">
        <h2 style="color: {{ if eq .Severity "critical" }}#dc3545{{ else if eq .Severity "warning" }}#ffc107{{ else }}#17a2b8{{ end }};">
          {{ .Title }}
        </h2>
        <table>
          <tr><td><strong>Severity:</strong></td><td>{{ .Severity | upper }}</td></tr>
          <tr><td><strong>CronJob:</strong></td><td>{{ .Namespace }}/{{ .CronJobName }}</td></tr>
          <tr><td><strong>Time:</strong></td><td>{{ .Timestamp }}</td></tr>
        </table>
        <p>{{ .Message }}</p>
        {{ if .SuggestedFix }}
        <h3>Suggested Fix</h3>
        <pre style="background: #f5f5f5; padding: 10px;">{{ .SuggestedFix }}</pre>
        {{ end }}
      </body>
      </html>
```

## Recipients

### Severity-Based Routing

Use multiple channels with different recipients:

```yaml title="critical-email.yaml"
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: critical-email
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - oncall@example.com
      - leadership@example.com
    subjectTemplate: "[CRITICAL] CronJob {{ .CronJobName }} requires immediate attention"
---
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: warning-email
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - team@example.com
    subjectTemplate: "[Warning] CronJob {{ .CronJobName }}: {{ .AlertType }}"
```

## Rate Limiting

Configure rate limits at the AlertChannel level:

```yaml
spec:
  type: email
  email:
    smtpSecretRef:
      name: smtp-credentials
      namespace: default
    from: alerts@example.com
    to:
      - team@example.com
  rateLimiting:
    burstLimit: 10
    maxAlertsPerHour: 50
```

## Complete Example

```yaml title="email-complete.yaml"
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
    from: cronjob-guardian@example.com
    to:
      - platform-team@example.com
    subjectTemplate: "[{{ .Severity | upper }}] {{ .CronJobName }} - {{ .AlertType }}"
    bodyTemplate: |
      CronJob Guardian Alert

      Title: {{ .Title }}
      CronJob: {{ .Namespace }}/{{ .CronJobName }}
      Severity: {{ .Severity | upper }}

      {{ .Message }}

      {{ if .SuggestedFix }}
      Suggested Fix:
      {{ .SuggestedFix }}
      {{ end }}
  rateLimiting:
    burstLimit: 10
    maxAlertsPerHour: 50
```

## Testing

```bash
curl -X POST http://localhost:8080/api/v1/channels/ops-email/test
```

## Troubleshooting

### Connection Failed

- Verify SMTP host and port in secret
- Check network connectivity from cluster
- Verify TLS settings match server requirements

### Authentication Failed

- Check username and password in secret
- Verify auth type matches server (plain, login, crammd5)

### Emails Not Received

- Check spam folders
- Verify recipient addresses
- Check SMTP server logs
- Ensure from address is allowed by server

### Rate Limiting

Emails being dropped due to rate limits:
- Increase rate limits if needed
- Use `suppressDuplicatesFor` on monitors
- Consider routing only critical alerts to email

## Related

- [Slack](./slack.md) - Slack integration
- [Webhook](./webhook.md) - Custom integrations
- [Alert Configuration](/docs/configuration/monitors/alerting) - Monitor alerting
