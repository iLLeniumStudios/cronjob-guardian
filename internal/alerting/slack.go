package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"golang.org/x/time/rate"
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
func NewSlackChannel(c client.Client, ac *v1alpha1.AlertChannel) (Channel, error) {
	if ac.Spec.Slack == nil {
		return nil, fmt.Errorf("slack config required for slack channel")
	}

	sc := &slackChannel{
		name:      ac.Name,
		client:    c,
		secretRef: ac.Spec.Slack.WebhookSecretRef,
		channel:   ac.Spec.Slack.DefaultChannel,
	}

	tmplStr := defaultSlackTemplate
	if ac.Spec.Slack.MessageTemplate != "" {
		tmplStr = ac.Spec.Slack.MessageTemplate
	}
	tmpl, err := template.New("slack").Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	sc.template = tmpl
	sc.rateLimiter = NewRateLimiter(ac.Spec.RateLimiting)

	return sc, nil
}

// Name returns the channel name
func (s *slackChannel) Name() string {
	return s.name
}

// Type returns the channel type
func (s *slackChannel) Type() string {
	return "slack"
}

// Send delivers an alert to Slack
func (s *slackChannel) Send(ctx context.Context, alert Alert) error {
	if !s.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", s.name)
	}

	webhookURL, err := getValueFromSecret(ctx, s.client, s.secretRef)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := s.template.Execute(&buf, alert); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	payload := map[string]interface{}{
		"text": buf.String(),
	}
	if s.channel != "" {
		payload["channel"] = s.channel
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := AlertHTTPClient.Do(req)
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
			Timestamp: time.Now(),
		},
	)
}

var defaultSlackTemplate = `:{{ if eq .Severity "critical" }}red_circle{{ else if eq .Severity "warning" }}warning{{ else }}large_blue_circle{{ end }}: *{{ .Title }}*

*CronJob:* ` + "`{{ .CronJob.Namespace }}/{{ .CronJob.Name }}`" + `
*Type:* {{ .Type }}
*Severity:* {{ .Severity }}

{{ .Message }}

{{ if .Context.ExitCode }}*Exit Code:* {{ .Context.ExitCode }}{{ end }}
{{ if .Context.Reason }}*Reason:* {{ .Context.Reason }}{{ end }}
{{ if .Context.SuggestedFix }}:bulb: *Suggested Fix:* {{ .Context.SuggestedFix }}{{ end }}
{{ if .Context.Logs }}
*Recent Logs:*
` + "```" + `{{ truncate .Context.Logs 1500 }}` + "```" + `
{{ end }}
`
