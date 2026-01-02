---
sidebar_position: 2
title: PagerDuty
description: Configure PagerDuty alerts
---

# PagerDuty Integration

Send alerts to PagerDuty for incident management and on-call routing.

## Prerequisites

1. A PagerDuty account
2. A service with Events API v2 integration
3. A routing key (integration key)

### Getting a Routing Key

1. Go to PagerDuty → Services
2. Select or create a service
3. Go to Integrations → Add Integration
4. Select "Events API v2"
5. Copy the Integration Key (routing key)

## Configuration

### Create the Secret

```bash
kubectl create secret generic pagerduty-key \
  --from-literal=routingKey=YOUR_ROUTING_KEY
```

### Create the AlertChannel

```yaml title="pagerduty-channel.yaml"
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

## Severity Configuration

Set the default PagerDuty severity for alerts:

```yaml
spec:
  type: pagerduty
  pagerduty:
    routingKeySecretRef:
      name: pagerduty-key
      namespace: default
      key: routingKey
    severity: critical    # critical, error, warning, or info
```

PagerDuty severities: `critical`, `error`, `warning`, `info`

## Rate Limiting

Configure rate limits at the AlertChannel level:

```yaml
spec:
  type: pagerduty
  pagerduty:
    routingKeySecretRef:
      name: pagerduty-key
      namespace: default
      key: routingKey
  rateLimiting:
    burstLimit: 30
    maxAlertsPerHour: 300
```

## Complete Example

```yaml title="pagerduty-complete.yaml"
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

## Usage in Monitor

Route critical alerts to PagerDuty:

```yaml
spec:
  alerting:
    channelRefs:
      - name: critical-pagerduty
        severities:
          - critical
      - name: team-slack
        severities:
          - critical
          - warning
          - info
```

## Incident Lifecycle

### Triggering

CronJob Guardian creates incidents via `trigger` events:

- **Summary**: Alert title and CronJob details
- **Severity**: Mapped from Guardian severity
- **Source**: CronJob namespace and name
- **Custom Details**: Logs, events, suggested fixes

### Resolving

Incidents are automatically resolved when:
- SLA violation clears (success rate recovers)
- Dead-man's switch clears (job runs successfully)

Manual resolution in PagerDuty also works—Guardian tracks state.

## Testing

Test via dashboard or API:

```bash
curl -X POST http://localhost:8080/api/v1/channels/ops-pagerduty/test
```

This sends a test trigger followed by immediate resolve.

## Troubleshooting

### Events Not Arriving

1. Verify routing key: Check secret exists and is correct
2. Check PagerDuty service: Ensure Events API v2 integration is active
3. Check operator logs: `kubectl logs -n cronjob-guardian deploy/cronjob-guardian`

### Duplicate Incidents

- Review `dedupKeyTemplate`—ensure it groups appropriately
- Check `suppressDuplicatesFor` on monitors

### Wrong Severity

- Verify `severityMapping` in AlertChannel
- Check `severityOverrides` in CronJobMonitor

## Related

- [Slack](./slack.md) - Slack integration
- [Alert Configuration](/docs/configuration/monitors/alerting) - Monitor alerting
- [High Availability](/docs/guides/high-availability) - Production setup
