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

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/testutil"
)

// Test helper to create a fake client with scheme
func newAlertChannelTestClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = guardianv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&guardianv1alpha1.AlertChannel{}).
		Build()
}

// Helper to create a test secret
func createTestSecret(name, namespace, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}
}

// Helper to create a test SMTP secret with required keys
func createTestSMTPSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"host":     []byte("smtp.example.com"),
			"username": []byte("user@example.com"),
			"password": []byte("password123"),
		},
	}
}

// ============================================================================
// Section 1.5.3: AlertChannelReconciler Tests
// ============================================================================

func TestReconcile_NewChannel(t *testing.T) {
	// Create a new webhook channel with required secret
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "test-channel",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.NotZero(t, result.RequeueAfter)

	// Verify channel was registered
	assert.Contains(t, dispatcher.RegisteredChannelsMap, "test-channel")

	// Verify status was updated
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.True(t, updated.Status.Ready)
}

func TestReconcile_UpdateChannel(t *testing.T) {
	// Create channel with existing status
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "existing-channel",
			Namespace:  "default",
			Generation: 2,
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
		Status: guardianv1alpha1.AlertChannelStatus{
			Ready:           true,
			AlertsSentTotal: 10,
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "existing-channel",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.NotZero(t, result.RequeueAfter)

	// Verify channel was re-registered with updated config
	assert.Contains(t, dispatcher.RegisteredChannelsMap, "existing-channel")
}

func TestReconcile_DeleteChannel(t *testing.T) {
	// Channel doesn't exist - simulates deletion
	fakeClient := newAlertChannelTestClient()
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "deleted-channel",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(context.Background(), req)

	require.NoError(t, err)
	assert.Zero(t, result.RequeueAfter)

	// Verify channel was removed from dispatcher
	assert.Contains(t, dispatcher.RemovedChannels, "deleted-channel")
}

func TestValidateConfig_Slack(t *testing.T) {
	secret := createTestSecret("slack-webhook", "default", "webhook-url", "https://hooks.slack.com/xxx")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slack-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "slack",
			Slack: &guardianv1alpha1.SlackConfig{
				WebhookSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "slack-webhook",
					Namespace: "default",
					Key:       "webhook-url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "slack-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify channel was registered (validation passed)
	assert.Contains(t, dispatcher.RegisteredChannelsMap, "slack-channel")

	// Verify status is ready
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.True(t, updated.Status.Ready)
}

func TestValidateConfig_PagerDuty(t *testing.T) {
	secret := createTestSecret("pagerduty-key", "default", "routing-key", "fake-routing-key")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pagerduty-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "pagerduty",
			PagerDuty: &guardianv1alpha1.PagerDutyConfig{
				RoutingKeySecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "pagerduty-key",
					Namespace: "default",
					Key:       "routing-key",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "pagerduty-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify channel was registered (validation passed)
	assert.Contains(t, dispatcher.RegisteredChannelsMap, "pagerduty-channel")
}

func TestValidateConfig_Webhook(t *testing.T) {
	secret := createTestSecret("webhook-url", "default", "url", "https://webhook.example.com/alerts")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-url",
					Namespace: "default",
					Key:       "url",
				},
				Method: "POST",
				Headers: map[string]string{
					"X-Custom-Header": "value",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "webhook-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify channel was registered (validation passed)
	assert.Contains(t, dispatcher.RegisteredChannelsMap, "webhook-channel")
}

func TestValidateConfig_Email(t *testing.T) {
	secret := createTestSMTPSecret("smtp-secret", "default")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "email-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "email",
			Email: &guardianv1alpha1.EmailConfig{
				SMTPSecretRef: guardianv1alpha1.NamespacedSecretRef{
					Name:      "smtp-secret",
					Namespace: "default",
				},
				From: "alerts@example.com",
				To:   []string{"team@example.com"},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "email-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify channel was registered (validation passed)
	assert.Contains(t, dispatcher.RegisteredChannelsMap, "email-channel")
}

func TestValidateConfig_MissingSecret(t *testing.T) {
	// Channel references a secret that doesn't exist
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "slack",
			Slack: &guardianv1alpha1.SlackConfig{
				WebhookSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "missing-secret",
					Namespace: "default",
					Key:       "webhook-url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "invalid-channel",
			Namespace: "default",
		},
	}

	// Reconcile should succeed (no error) but channel should not be ready
	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify channel was NOT registered
	assert.NotContains(t, dispatcher.RegisteredChannelsMap, "invalid-channel")

	// Verify status shows not ready with error
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "failed to get webhook secret")
}

func TestTestChannel_Success(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type:       "webhook",
			TestOnSave: true, // Enable test on save
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()
	dispatcher.SendToChannelError = nil // Test succeeds

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "test-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify test was sent
	assert.Contains(t, dispatcher.SentChannelNames, "test-channel")
	assert.Len(t, dispatcher.SentAlerts, 1)
	assert.Equal(t, "Test", dispatcher.SentAlerts[0].Type)

	// Verify status shows success
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.Equal(t, "success", updated.Status.LastTestResult)
	assert.Empty(t, updated.Status.LastTestError)
	assert.NotNil(t, updated.Status.LastTestTime)
}

func TestTestChannel_Failure(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type:       "webhook",
			TestOnSave: true, // Enable test on save
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()
	dispatcher.SendToChannelError = errors.New("connection refused")

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "failing-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify status shows failure
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status.LastTestResult)
	assert.Contains(t, updated.Status.LastTestError, "connection refused")
	assert.NotNil(t, updated.Status.LastTestTime)
}

func TestUpdateStatus_Ready(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ready-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
		Status: guardianv1alpha1.AlertChannelStatus{
			Ready:               false, // Initially not ready
			ConsecutiveFailures: 0,
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "ready-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify status is now ready
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.True(t, updated.Status.Ready)

	// Verify Ready condition is set
	var readyCondition *metav1.Condition
	for i := range updated.Status.Conditions {
		if updated.Status.Conditions[i].Type == "Ready" {
			readyCondition = &updated.Status.Conditions[i]
			break
		}
	}
	require.NotNil(t, readyCondition)
	assert.Equal(t, metav1.ConditionTrue, readyCondition.Status)
	assert.Equal(t, "Validated", readyCondition.Reason)
}

func TestUpdateStatus_TestResult(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-result-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type:       "webhook",
			TestOnSave: true,
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "test-result-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify test result is recorded in status
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.NotNil(t, updated.Status.LastTestTime)
	assert.Equal(t, "success", updated.Status.LastTestResult)
}

// Additional edge case tests

func TestValidateConfig_MissingKey(t *testing.T) {
	// Secret exists but is missing the required key
	secret := createTestSecret("slack-webhook", "default", "wrong-key", "value")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "missing-key-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "slack",
			Slack: &guardianv1alpha1.SlackConfig{
				WebhookSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "slack-webhook",
					Namespace: "default",
					Key:       "webhook-url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "missing-key-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify status shows error about missing key
	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "not found in secret")
}

func TestValidateConfig_InvalidTemplate(t *testing.T) {
	secret := createTestSecret("slack-webhook", "default", "webhook-url", "https://hooks.slack.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-template-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "slack",
			Slack: &guardianv1alpha1.SlackConfig{
				WebhookSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "slack-webhook",
					Namespace: "default",
					Key:       "webhook-url",
				},
				MessageTemplate: "{{.Invalid Template{{", // Invalid template syntax
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "invalid-template-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "invalid message template")
}

func TestValidateConfig_UnknownType(t *testing.T) {
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unknown-type-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "unknown",
		},
	}

	fakeClient := newAlertChannelTestClient(channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "unknown-type-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "unknown channel type")
}

func TestReconcile_ConsecutiveFailuresNotReady(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()
	// Simulate channel has too many consecutive failures
	dispatcher.ChannelStats["failing-channel"] = &alerting.ChannelStats{
		ConsecutiveFailures: 5,
		LastFailedError:     "connection timeout",
	}

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "failing-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)

	// Should be marked not ready due to consecutive failures
	assert.False(t, updated.Status.Ready)
	assert.Equal(t, int32(5), updated.Status.ConsecutiveFailures)

	// Verify condition shows the reason
	var readyCondition *metav1.Condition
	for i := range updated.Status.Conditions {
		if updated.Status.Conditions[i].Type == "Ready" {
			readyCondition = &updated.Status.Conditions[i]
			break
		}
	}
	require.NotNil(t, readyCondition)
	assert.Equal(t, metav1.ConditionFalse, readyCondition.Status)
	assert.Equal(t, "ConsecutiveFailures", readyCondition.Reason)
}

func TestReconcile_SyncsStatsFromDispatcher(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stats-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()
	// Set up stats in dispatcher
	dispatcher.ChannelStats["stats-channel"] = &alerting.ChannelStats{
		AlertsSentTotal:   100,
		AlertsFailedTotal: 5,
	}

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "stats-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)

	// Verify stats were synced from dispatcher
	assert.Equal(t, int64(100), updated.Status.AlertsSentTotal)
	assert.Equal(t, int64(5), updated.Status.AlertsFailedTotal)
}

func TestValidateEmail_MissingRequiredKeys(t *testing.T) {
	// SMTP secret missing required keys
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "incomplete-smtp",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"host": []byte("smtp.example.com"),
			// Missing username and password
		},
	}
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "incomplete-email-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "email",
			Email: &guardianv1alpha1.EmailConfig{
				SMTPSecretRef: guardianv1alpha1.NamespacedSecretRef{
					Name:      "incomplete-smtp",
					Namespace: "default",
				},
				From: "alerts@example.com",
				To:   []string{"team@example.com"},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "incomplete-email-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "SMTP secret missing")
}

func TestValidateEmail_MissingFromAddress(t *testing.T) {
	secret := createTestSMTPSecret("smtp-secret", "default")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-from-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "email",
			Email: &guardianv1alpha1.EmailConfig{
				SMTPSecretRef: guardianv1alpha1.NamespacedSecretRef{
					Name:      "smtp-secret",
					Namespace: "default",
				},
				From: "", // Missing from address
				To:   []string{"team@example.com"},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "no-from-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "from address is required")
}

func TestValidateEmail_NoRecipients(t *testing.T) {
	secret := createTestSMTPSecret("smtp-secret", "default")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-recipients-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "email",
			Email: &guardianv1alpha1.EmailConfig{
				SMTPSecretRef: guardianv1alpha1.NamespacedSecretRef{
					Name:      "smtp-secret",
					Namespace: "default",
				},
				From: "alerts@example.com",
				To:   []string{}, // Empty recipients
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)
	dispatcher := testutil.NewMockDispatcher()

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: dispatcher,
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "no-recipients-channel",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)

	var updated guardianv1alpha1.AlertChannel
	err = fakeClient.Get(context.Background(), req.NamespacedName, &updated)
	require.NoError(t, err)
	assert.False(t, updated.Status.Ready)
	assert.Contains(t, updated.Status.LastTestError, "at least one recipient is required")
}

func TestReconcile_NoDispatcher(t *testing.T) {
	secret := createTestSecret("webhook-secret", "default", "url", "https://webhook.example.com")
	channel := &guardianv1alpha1.AlertChannel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-dispatcher-channel",
			Namespace: "default",
		},
		Spec: guardianv1alpha1.AlertChannelSpec{
			Type: "webhook",
			Webhook: &guardianv1alpha1.WebhookConfig{
				URLSecretRef: guardianv1alpha1.NamespacedSecretKeyRef{
					Name:      "webhook-secret",
					Namespace: "default",
					Key:       "url",
				},
			},
		},
	}

	fakeClient := newAlertChannelTestClient(secret, channel)

	reconciler := &AlertChannelReconciler{
		Client:          fakeClient,
		Log:             logr.Discard(),
		Scheme:          fakeClient.Scheme(),
		AlertDispatcher: nil, // No dispatcher
	}

	req := ctrl.Request{
		NamespacedName: k8stypes.NamespacedName{
			Name:      "no-dispatcher-channel",
			Namespace: "default",
		},
	}

	// Should not panic with nil dispatcher
	result, err := reconciler.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.NotZero(t, result.RequeueAfter)
}
