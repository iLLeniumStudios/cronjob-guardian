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
	"fmt"
	"net/http"
	"text/template"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

type webhookChannel struct {
	name        string
	client      client.Client
	secretRef   v1alpha1.NamespacedSecretKeyRef
	method      string
	headers     map[string]string
	template    *template.Template
	rateLimiter *rate.Limiter
}

// NewWebhookChannel creates a new webhook channel
func NewWebhookChannel(c client.Client, ac *v1alpha1.AlertChannel) (*webhookChannel, error) {
	if ac.Spec.Webhook == nil {
		return nil, fmt.Errorf("webhook config required for webhook channel")
	}

	wc := &webhookChannel{
		name:      ac.Name,
		client:    c,
		secretRef: ac.Spec.Webhook.URLSecretRef,
		method:    ac.Spec.Webhook.Method,
		headers:   ac.Spec.Webhook.Headers,
	}

	if wc.method == "" {
		wc.method = "POST"
	}

	// Parse template
	tmplStr := defaultWebhookTemplate
	if ac.Spec.Webhook.PayloadTemplate != "" {
		tmplStr = ac.Spec.Webhook.PayloadTemplate
	}
	tmpl, err := template.New("webhook").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	wc.template = tmpl

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
	wc.rateLimiter = rate.NewLimiter(rate.Limit(float64(maxPerHour)/3600), int(burst))

	return wc, nil
}

// Name returns the channel name
func (w *webhookChannel) Name() string {
	return w.name
}

// Send delivers an alert via webhook
func (w *webhookChannel) Send(ctx context.Context, alert Alert) error {
	if !w.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", w.name)
	}

	url, err := w.getURL(ctx)
	if err != nil {
		return err
	}

	// Render payload
	var buf bytes.Buffer
	if err := w.template.Execute(&buf, alert); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, w.method, url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	// Send
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// Test sends a test alert
func (w *webhookChannel) Test(ctx context.Context) error {
	return w.Send(
		ctx, Alert{
			Key:       "test-alert",
			Type:      "Test",
			Severity:  "info",
			Title:     "CronJob Guardian Test Alert",
			Message:   "This is a test alert from CronJob Guardian.",
			CronJob:   types.NamespacedName{Namespace: "test", Name: "test"},
			Timestamp: timeNow(),
		},
	)
}

func (w *webhookChannel) getURL(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	err := w.client.Get(
		ctx, types.NamespacedName{
			Namespace: w.secretRef.Namespace,
			Name:      w.secretRef.Name,
		}, secret,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	url, ok := secret.Data[w.secretRef.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret", w.secretRef.Key)
	}

	return string(url), nil
}

var defaultWebhookTemplate = `{
  "key": "{{ .Key }}",
  "type": "{{ .Type }}",
  "severity": "{{ .Severity }}",
  "title": "{{ .Title }}",
  "message": "{{ .Message }}",
  "cronjob": {
    "namespace": "{{ .CronJob.Namespace }}",
    "name": "{{ .CronJob.Name }}"
  },
  "timestamp": "{{ formatTime .Timestamp "RFC3339" }}",
  "context": {
    "suggested_fix": "{{ .Context.SuggestedFix }}",
    "success_rate": {{ .Context.SuccessRate }},
    "exit_code": {{ .Context.ExitCode }},
    "reason": "{{ .Context.Reason }}"
  }
}`
