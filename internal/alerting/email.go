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
	"net/smtp"
	"strings"
	"text/template"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// SMTPConfig holds SMTP connection details
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

type emailChannel struct {
	name            string
	client          client.Client
	smtpSecretRef   v1alpha1.NamespacedSecretRef
	from            string
	to              []string
	subjectTemplate *template.Template
	bodyTemplate    *template.Template
	rateLimiter     *rate.Limiter
}

// NewEmailChannel creates a new email channel
func NewEmailChannel(c client.Client, ac *v1alpha1.AlertChannel) (*emailChannel, error) {
	if ac.Spec.Email == nil {
		return nil, fmt.Errorf("email config required for email channel")
	}

	ec := &emailChannel{
		name:          ac.Name,
		client:        c,
		smtpSecretRef: ac.Spec.Email.SMTPSecretRef,
		from:          ac.Spec.Email.From,
		to:            ac.Spec.Email.To,
	}

	// Parse subject template
	subjectTmplStr := defaultEmailSubjectTemplate
	if ac.Spec.Email.SubjectTemplate != "" {
		subjectTmplStr = ac.Spec.Email.SubjectTemplate
	}
	subjectTmpl, err := template.New("subject").Funcs(templateFuncs).Parse(subjectTmplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid subject template: %w", err)
	}
	ec.subjectTemplate = subjectTmpl

	// Parse body template
	bodyTmplStr := defaultEmailBodyTemplate
	if ac.Spec.Email.BodyTemplate != "" {
		bodyTmplStr = ac.Spec.Email.BodyTemplate
	}
	bodyTmpl, err := template.New("body").Funcs(templateFuncs).Parse(bodyTmplStr)
	if err != nil {
		return nil, fmt.Errorf("invalid body template: %w", err)
	}
	ec.bodyTemplate = bodyTmpl

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
	ec.rateLimiter = rate.NewLimiter(rate.Limit(float64(maxPerHour)/3600), int(burst))

	return ec, nil
}

// Name returns the channel name
func (e *emailChannel) Name() string {
	return e.name
}

// Send delivers an alert via email
func (e *emailChannel) Send(ctx context.Context, alert Alert) error {
	if !e.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded for channel %s", e.name)
	}

	// Get SMTP credentials
	smtpConfig, err := e.getSMTPConfig(ctx)
	if err != nil {
		return err
	}

	// Render subject and body
	var subjectBuf, bodyBuf bytes.Buffer
	if err := e.subjectTemplate.Execute(&subjectBuf, alert); err != nil {
		return fmt.Errorf("failed to render subject: %w", err)
	}
	if err := e.bodyTemplate.Execute(&bodyBuf, alert); err != nil {
		return fmt.Errorf("failed to render body: %w", err)
	}

	// Build message
	msg := fmt.Sprintf("From: %s\r\n", e.from)
	msg += fmt.Sprintf("To: %s\r\n", strings.Join(e.to, ", "))
	msg += fmt.Sprintf("Subject: %s\r\n", subjectBuf.String())
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: text/plain; charset=utf-8\r\n"
	msg += "\r\n"
	msg += bodyBuf.String()

	// Send
	auth := smtp.PlainAuth("", smtpConfig.Username, smtpConfig.Password, smtpConfig.Host)
	addr := fmt.Sprintf("%s:%s", smtpConfig.Host, smtpConfig.Port)

	return smtp.SendMail(addr, auth, e.from, e.to, []byte(msg))
}

// Test sends a test alert
func (e *emailChannel) Test(ctx context.Context) error {
	return e.Send(ctx, Alert{
		Key:       "test-alert",
		Type:      "Test",
		Severity:  "info",
		Title:     "CronJob Guardian Test Alert",
		Message:   "This is a test alert from CronJob Guardian.",
		CronJob:   types.NamespacedName{Namespace: "test", Name: "test"},
		Timestamp: timeNow(),
	})
}

func (e *emailChannel) getSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	secret := &corev1.Secret{}
	err := e.client.Get(ctx, types.NamespacedName{
		Namespace: e.smtpSecretRef.Namespace,
		Name:      e.smtpSecretRef.Name,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMTP secret: %w", err)
	}

	config := &SMTPConfig{}

	if host, ok := secret.Data["host"]; ok {
		config.Host = string(host)
	} else {
		return nil, fmt.Errorf("SMTP secret missing 'host' key")
	}

	if port, ok := secret.Data["port"]; ok {
		config.Port = string(port)
	} else {
		config.Port = "587" // Default to submission port
	}

	if username, ok := secret.Data["username"]; ok {
		config.Username = string(username)
	} else {
		return nil, fmt.Errorf("SMTP secret missing 'username' key")
	}

	if password, ok := secret.Data["password"]; ok {
		config.Password = string(password)
	} else {
		return nil, fmt.Errorf("SMTP secret missing 'password' key")
	}

	return config, nil
}

var defaultEmailSubjectTemplate = `[{{ upper .Severity }}] {{ .Title }}`

var defaultEmailBodyTemplate = `CronJob Guardian Alert

Type: {{ .Type }}
Severity: {{ .Severity }}
CronJob: {{ .CronJob.Namespace }}/{{ .CronJob.Name }}
Time: {{ formatTime .Timestamp "RFC3339" }}

{{ .Message }}

{{ if .Context.SuggestedFix }}
Suggested Fix:
{{ .Context.SuggestedFix }}
{{ end }}

{{ if .Context.Logs }}
Logs:
{{ .Context.Logs }}
{{ end }}

--
CronJob Guardian
`
