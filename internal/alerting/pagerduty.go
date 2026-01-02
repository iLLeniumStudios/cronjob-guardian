/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

type pagerDutyChannel struct {
	name        string
	client      client.Client
	secretRef   v1alpha1.NamespacedSecretKeyRef
	severity    string
	rateLimiter *rate.Limiter
}

// NewPagerDutyChannel creates a new PagerDuty channel
func NewPagerDutyChannel(c client.Client, ac *v1alpha1.AlertChannel) (Channel, error) {
	if ac.Spec.PagerDuty == nil {
		return nil, fmt.Errorf("pagerduty config required for pagerduty channel")
	}

	pc := &pagerDutyChannel{
		name:        ac.Name,
		client:      c,
		secretRef:   ac.Spec.PagerDuty.RoutingKeySecretRef,
		severity:    ac.Spec.PagerDuty.Severity,
		rateLimiter: NewRateLimiter(ac.Spec.RateLimiting),
	}

	return pc, nil
}

// Name returns the channel name
func (p *pagerDutyChannel) Name() string {
	return p.name
}

// Type returns the channel type
func (p *pagerDutyChannel) Type() string {
	return "pagerduty"
}

// Send delivers an alert to PagerDuty
func (p *pagerDutyChannel) Send(ctx context.Context, alert Alert) error {
	if !p.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", p.name)
	}

	routingKey, err := getValueFromSecret(ctx, p.client, p.secretRef)
	if err != nil {
		return err
	}

	pdSeverity := p.severity
	if pdSeverity == "" {
		switch alert.Severity {
		case "critical":
			pdSeverity = "critical"
		case "warning":
			pdSeverity = "warning"
		default:
			pdSeverity = "info"
		}
	}

	payload := map[string]interface{}{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"dedup_key":    alert.Key,
		"payload": map[string]interface{}{
			"summary":   alert.Title,
			"source":    fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name),
			"severity":  pdSeverity,
			"timestamp": alert.Timestamp.Format(time.RFC3339),
			"custom_details": map[string]interface{}{
				"type":          alert.Type,
				"message":       alert.Message,
				"suggested_fix": alert.Context.SuggestedFix,
				"success_rate":  alert.Context.SuccessRate,
				"exit_code":     alert.Context.ExitCode,
				"reason":        alert.Context.Reason,
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal PagerDuty payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", pagerDutyEventsURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := AlertHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pagerduty event: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pagerduty returned status %d", resp.StatusCode)
	}

	return nil
}

// Test sends a test alert
func (p *pagerDutyChannel) Test(ctx context.Context) error {
	return p.Send(
		ctx, Alert{
			Key:       "test-alert",
			Type:      "Test",
			Severity:  "info",
			Title:     "CronJob Guardian Test Alert",
			Message:   "This is a test alert from CronJob Guardian.",
			CronJob:   types.NamespacedName{Namespace: "test", Name: "test"},
			Timestamp: time.Now(),
		},
	)
}
