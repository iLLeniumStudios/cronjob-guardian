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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// createTestSecret creates a Secret for testing
//
//nolint:unparam // namespace is always "default" in tests but parameter kept for API clarity
func createTestSecret(namespace, name, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}
}

// createTestAlertChannel creates an AlertChannel CR for testing
func createTestAlertChannel(name, chanType string) *v1alpha1.AlertChannel {
	ac := &v1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.AlertChannelSpec{
			Type: chanType,
		},
	}
	return ac
}

// createTestAlertForChannel creates an Alert for channel testing
func createTestAlertForChannel() Alert {
	return Alert{
		Key:      "test/cronjob/JobFailed",
		Type:     "JobFailed",
		Severity: "critical",
		Title:    "Job Failed",
		Message:  "The job has failed",
		CronJob:  types.NamespacedName{Namespace: "test", Name: "cronjob"},
		MonitorRef: types.NamespacedName{
			Namespace: "test",
			Name:      "monitor",
		},
		Context: AlertContext{
			ExitCode:     137,
			Reason:       "OOMKilled",
			SuggestedFix: "Increase memory limits",
			Logs:         "Error: Out of memory",
		},
		Timestamp: time.Now(),
	}
}

// ==================== Slack Channel Tests ====================

func TestSlackChannel_Send_Success(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Contains(t, receivedBody, "Job Failed")
	assert.Contains(t, receivedBody, "critical")
}

func TestSlackChannel_Send_HTTPError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestSlackChannel_Send_MissingSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "nonexistent-secret",
			Key:       "url",
		},
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
}

func TestSlackChannel_CustomTemplate(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
		MessageTemplate: "Custom Alert: {{ .Title }} in {{ .CronJob.Namespace }}",
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Contains(t, receivedBody, "Custom Alert: Job Failed in test")
}

func TestSlackChannel_DefaultTemplate(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Contains(t, receivedBody, "red_circle")
	assert.Contains(t, receivedBody, "Suggested Fix")
}

func TestSlackChannel_Test(t *testing.T) {
	called := false
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	err = ch.Test(ctx)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestSlackChannel_WithChannel(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &payload)
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
		DefaultChannel: "#alerts",
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Equal(t, "#alerts", payload["channel"])
}

func TestSlackChannel_NameType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("my-slack-channel", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	assert.Equal(t, "my-slack-channel", ch.Name())
	assert.Equal(t, "slack", ch.Type())
}

func TestSlackChannel_MissingConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("slack-test", "slack")

	_, err := NewSlackChannel(fakeClient, ac)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "slack config required")
}

func TestSlackChannel_InvalidTemplate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
		MessageTemplate: "{{ .Invalid",
	}

	_, err := NewSlackChannel(fakeClient, ac)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template")
}

// ==================== Webhook Channel Tests ====================

func TestWebhookChannel_Send_POST(t *testing.T) {
	var receivedMethod string
	var receivedBody string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
		Method: "POST",
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Equal(t, "POST", receivedMethod)
	assert.Contains(t, receivedBody, "JobFailed")
}

func TestWebhookChannel_Send_PUT(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
		Method: "PUT",
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Equal(t, "PUT", receivedMethod)
}

func TestWebhookChannel_Send_DefaultMethod(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Equal(t, "POST", receivedMethod)
}

func TestWebhookChannel_CustomHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				receivedHeaders = r.Header
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer token123",
		},
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "Bearer token123", receivedHeaders.Get("Authorization"))
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
}

func TestWebhookChannel_CustomPayload(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
		PayloadTemplate: `{"custom_alert": "{{ .Title }}", "ns": "{{ .CronJob.Namespace }}"}`,
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	var payload map[string]string
	err = json.Unmarshal([]byte(receivedBody), &payload)
	require.NoError(t, err)

	assert.Equal(t, "Job Failed", payload["custom_alert"])
	assert.Equal(t, "test", payload["ns"])
}

func TestWebhookChannel_HTTPError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestWebhookChannel_AcceptsMultipleSuccessCodes(t *testing.T) {
	for _, code := range []int{200, 201, 202, 204} {
		t.Run(
			string(rune(code)), func(t *testing.T) {
				server := httptest.NewServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(code)
						},
					),
				)
				defer server.Close()

				scheme := runtime.NewScheme()
				_ = corev1.AddToScheme(scheme)
				secret := createTestSecret("default", "webhook-url", "url", server.URL)
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

				ac := createTestAlertChannel("webhook-test", "webhook")
				ac.Spec.Webhook = &v1alpha1.WebhookConfig{
					URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
						Namespace: "default",
						Name:      "webhook-url",
						Key:       "url",
					},
				}

				ch, err := NewWebhookChannel(fakeClient, ac)
				require.NoError(t, err)

				ctx := context.Background()
				alert := createTestAlertForChannel()

				err = ch.Send(ctx, alert)
				assert.NoError(t, err)
			},
		)
	}
}

func TestWebhookChannel_NameType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("my-webhook", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	assert.Equal(t, "my-webhook", ch.Name())
	assert.Equal(t, "webhook", ch.Type())
}

func TestWebhookChannel_MissingConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")

	_, err := NewWebhookChannel(fakeClient, ac)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook config required")
}

func TestWebhookChannel_Test(t *testing.T) {
	called := false
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "webhook-url", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("webhook-test", "webhook")
	ac.Spec.Webhook = &v1alpha1.WebhookConfig{
		URLSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "webhook-url",
			Key:       "url",
		},
	}

	ch, err := NewWebhookChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	err = ch.Test(ctx)
	require.NoError(t, err)
	assert.True(t, called)
}

// ==================== PagerDuty Channel Tests ====================

func TestPagerDutyChannel_MissingSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("pagerduty-test", "pagerduty")
	ac.Spec.PagerDuty = &v1alpha1.PagerDutyConfig{
		RoutingKeySecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "nonexistent",
			Key:       "key",
		},
	}

	ch, err := NewPagerDutyChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret")
}

func TestPagerDutyChannel_NameType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("my-pagerduty", "pagerduty")
	ac.Spec.PagerDuty = &v1alpha1.PagerDutyConfig{
		RoutingKeySecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "pd-routing-key",
			Key:       "key",
		},
	}

	ch, err := NewPagerDutyChannel(fakeClient, ac)
	require.NoError(t, err)

	assert.Equal(t, "my-pagerduty", ch.Name())
	assert.Equal(t, "pagerduty", ch.Type())
}

func TestPagerDutyChannel_MissingConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ac := createTestAlertChannel("pagerduty-test", "pagerduty")

	_, err := NewPagerDutyChannel(fakeClient, ac)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pagerduty config required")
}

// ==================== Rate Limiter Tests for Channels ====================

func TestChannelRateLimiting(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := createTestSecret("default", "slack-webhook", "url", server.URL)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	ac := createTestAlertChannel("slack-test", "slack")
	ac.Spec.Slack = &v1alpha1.SlackConfig{
		WebhookSecretRef: v1alpha1.NamespacedSecretKeyRef{
			Namespace: "default",
			Name:      "slack-webhook",
			Key:       "url",
		},
	}
	maxAlertsPerHour := int32(1)
	burstLimit := int32(1)
	ac.Spec.RateLimiting = &v1alpha1.RateLimitConfig{
		MaxAlertsPerHour: &maxAlertsPerHour,
		BurstLimit:       &burstLimit,
	}

	ch, err := NewSlackChannel(fakeClient, ac)
	require.NoError(t, err)

	ctx := context.Background()
	alert := createTestAlertForChannel()

	err = ch.Send(ctx, alert)
	require.NoError(t, err)

	err = ch.Send(ctx, alert)
	assert.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "rate limit")

	assert.Equal(t, 1, requestCount)
}

// ==================== Helper Function Tests ====================

func TestNewRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(nil)
	assert.NotNil(t, limiter)
	assert.True(t, limiter.Allow())

	maxAlertsPerHour := int32(100)
	burstLimit := int32(5)
	config := &v1alpha1.RateLimitConfig{
		MaxAlertsPerHour: &maxAlertsPerHour,
		BurstLimit:       &burstLimit,
	}
	limiter = NewRateLimiter(config)
	assert.NotNil(t, limiter)
}
