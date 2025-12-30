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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

type pagerdutyChannel struct {
	name        string
	client      client.Client
	secretRef   v1alpha1.NamespacedSecretKeyRef
	severity    string
	rateLimiter *rate.Limiter
}

// NewPagerDutyChannel creates a new PagerDuty channel
func NewPagerDutyChannel(c client.Client, ac *v1alpha1.AlertChannel) (*pagerdutyChannel, error) {
	if ac.Spec.PagerDuty == nil {
		return nil, fmt.Errorf("pagerduty config required for pagerduty channel")
	}

	pc := &pagerdutyChannel{
		name:      ac.Name,
		client:    c,
		secretRef: ac.Spec.PagerDuty.RoutingKeySecretRef,
		severity:  ac.Spec.PagerDuty.Severity,
	}

	// Setup rate limiter
	maxPerHour := int32(100)
	burst := int32(10)
	if ac.Spec.RateLimiting != nil {
		if ac.Spec.RateLimiting.MaxAlertsPerHour != nil {
			maxPerHour = *ac.Spec.RateLimiting.MaxAlertsPerHour
		}
		if ac.Spec.RateLimiting.BurstLimit != nil {
			burst = *ac.Spec.RateLimiting.BurstLimit
		}
	}
	pc.rateLimiter = rate.NewLimiter(rate.Limit(float64(maxPerHour)/3600), int(burst))

	return pc, nil
}

// Name returns the channel name
func (p *pagerdutyChannel) Name() string {
	return p.name
}

// Send delivers an alert to PagerDuty
func (p *pagerdutyChannel) Send(ctx context.Context, alert Alert) error {
	if !p.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", p.name)
	}

	routingKey, err := p.getRoutingKey(ctx)
	if err != nil {
		return err
	}

	// Map severity
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

	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(pagerDutyEventsURL, "application/json", bytes.NewReader(jsonPayload))
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
func (p *pagerdutyChannel) Test(ctx context.Context) error {
	return p.Send(ctx, Alert{
		Key:       "test-alert",
		Type:      "Test",
		Severity:  "info",
		Title:     "CronJob Guardian Test Alert",
		Message:   "This is a test alert from CronJob Guardian.",
		CronJob:   types.NamespacedName{Namespace: "test", Name: "test"},
		Timestamp: timeNow(),
	})
}

func (p *pagerdutyChannel) getRoutingKey(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	err := p.client.Get(ctx, types.NamespacedName{
		Namespace: p.secretRef.Namespace,
		Name:      p.secretRef.Name,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	key, ok := secret.Data[p.secretRef.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret", p.secretRef.Key)
	}

	return string(key), nil
}

// timeNow is a variable for testing
var timeNow = time.Now
