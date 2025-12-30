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
	"text/template"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

type slackChannel struct {
	name        string
	client      client.Client
	secretRef   v1alpha1.NamespacedSecretKeyRef
	channel     string
	template    *template.Template
	rateLimiter *rate.Limiter
}

// NewSlackChannel creates a new Slack channel
func NewSlackChannel(c client.Client, ac *v1alpha1.AlertChannel) (*slackChannel, error) {
	if ac.Spec.Slack == nil {
		return nil, fmt.Errorf("slack config required for slack channel")
	}

	sc := &slackChannel{
		name:      ac.Name,
		client:    c,
		secretRef: ac.Spec.Slack.WebhookSecretRef,
		channel:   ac.Spec.Slack.DefaultChannel,
	}

	// Parse template
	tmplStr := defaultSlackTemplate
	if ac.Spec.Slack.MessageTemplate != "" {
		tmplStr = ac.Spec.Slack.MessageTemplate
	}
	tmpl, err := template.New("slack").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	sc.template = tmpl

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
	sc.rateLimiter = rate.NewLimiter(rate.Limit(float64(maxPerHour)/3600), int(burst))

	return sc, nil
}

// Name returns the channel name
func (s *slackChannel) Name() string {
	return s.name
}

// Send delivers an alert to Slack
func (s *slackChannel) Send(ctx context.Context, alert Alert) error {
	// Check rate limit
	if !s.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", s.name)
	}

	// Get webhook URL from secret
	webhookURL, err := s.getWebhookURL(ctx)
	if err != nil {
		return err
	}

	// Render message
	var buf bytes.Buffer
	if err := s.template.Execute(&buf, alert); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Build payload
	payload := map[string]interface{}{
		"text": buf.String(),
	}
	if s.channel != "" {
		payload["channel"] = s.channel
	}

	// Send
	jsonPayload, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}

// Test sends a test alert
func (s *slackChannel) Test(ctx context.Context) error {
	return s.Send(
		ctx, Alert{
			Type:      "Test",
			Severity:  "info",
			Title:     "CronJob Guardian Test Alert",
			Message:   "This is a test alert from CronJob Guardian.",
			CronJob:   types.NamespacedName{Namespace: "test", Name: "test"},
			Timestamp: timeNow(),
		},
	)
}

func (s *slackChannel) getWebhookURL(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	err := s.client.Get(
		ctx, types.NamespacedName{
			Namespace: s.secretRef.Namespace,
			Name:      s.secretRef.Name,
		}, secret,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	url, ok := secret.Data[s.secretRef.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret", s.secretRef.Key)
	}

	return string(url), nil
}

var defaultSlackTemplate = `:{{ if eq .Severity "critical" }}red_circle{{ else if eq .Severity "warning" }}warning{{ else }}large_blue_circle{{ end }}: *{{ .Title }}*

*CronJob:* ` + "`{{ .CronJob.Namespace }}/{{ .CronJob.Name }}`" + `
*Type:* {{ .Type }}
*Severity:* {{ .Severity }}

{{ .Message }}

{{ if .Context.SuggestedFix }}:bulb: *Suggested Fix:* {{ .Context.SuggestedFix }}{{ end }}
`
