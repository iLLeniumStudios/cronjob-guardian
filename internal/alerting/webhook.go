package alerting

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"golang.org/x/time/rate"
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
func NewWebhookChannel(c client.Client, ac *v1alpha1.AlertChannel) (Channel, error) {
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

	tmplStr := defaultWebhookTemplate
	if ac.Spec.Webhook.PayloadTemplate != "" {
		tmplStr = ac.Spec.Webhook.PayloadTemplate
	}
	tmpl, err := template.New("webhook").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	wc.template = tmpl
	wc.rateLimiter = NewRateLimiter(ac.Spec.RateLimiting)

	return wc, nil
}

// Name returns the channel name
func (w *webhookChannel) Name() string {
	return w.name
}

// Type returns the channel type
func (w *webhookChannel) Type() string {
	return "webhook"
}

// Send delivers an alert via webhook
func (w *webhookChannel) Send(ctx context.Context, alert Alert) error {
	if !w.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", w.name)
	}

	url, err := getValueFromSecret(ctx, w.client, w.secretRef)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := w.template.Execute(&buf, alert); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, w.method, url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := AlertHTTPClient.Do(req)
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
			Timestamp: time.Now(),
		},
	)
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
    "reason": "{{ .Context.Reason }}",
    "logs": {{ jsonEscape .Context.Logs }}
  }
}`
