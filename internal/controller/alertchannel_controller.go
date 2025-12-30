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
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
)

// AlertChannelReconciler reconciles an AlertChannel object
type AlertChannelReconciler struct {
	client.Client
	Log             logr.Logger // Required - must be injected
	Scheme          *runtime.Scheme
	AlertDispatcher alerting.Dispatcher
}

// +kubebuilder:rbac:groups=guardian.illenium.net,resources=alertchannels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=alertchannels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=alertchannels/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles AlertChannel reconciliation
func (r *AlertChannelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("channel", req.NamespacedName)
	log.V(1).Info("reconciling AlertChannel")

	// 1. Fetch the AlertChannel
	channel := &guardianv1alpha1.AlertChannel{}
	if err := r.Get(ctx, req.NamespacedName, channel); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.V(1).Info("channel not found, removing from dispatcher")
			// CR deleted, remove from dispatcher
			if r.AlertDispatcher != nil {
				r.AlertDispatcher.RemoveChannel(req.Name)
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get channel")
		return ctrl.Result{}, err
	}
	log.V(1).Info("fetched channel", "type", channel.Spec.Type)

	// 2. Validate configuration
	log.V(1).Info("validating configuration")
	if err := r.validateConfig(ctx, channel); err != nil {
		log.Error(err, "validation failed")
		channel.Status.Ready = false
		channel.Status.LastTestError = err.Error()
		r.setCondition(channel, "Ready", metav1.ConditionFalse, "ValidationFailed", err.Error())
		if err := r.Status().Update(ctx, channel); err != nil {
			log.Error(err, "failed to update status after validation error")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	log.V(1).Info("configuration validated successfully")

	// 3. Test connection if requested
	if channel.Spec.TestOnSave {
		log.V(1).Info("testing channel connection")
		if err := r.testChannel(ctx, channel); err != nil {
			log.Error(err, "channel test failed")
			now := metav1.Now()
			channel.Status.LastTestTime = &now
			channel.Status.LastTestResult = "failed"
			channel.Status.LastTestError = err.Error()
		} else {
			log.V(1).Info("channel test succeeded")
			now := metav1.Now()
			channel.Status.LastTestTime = &now
			channel.Status.LastTestResult = "success"
			channel.Status.LastTestError = ""
		}
	}

	// 4. Register with dispatcher
	if r.AlertDispatcher != nil {
		log.V(1).Info("registering channel with dispatcher")
		if err := r.AlertDispatcher.RegisterChannel(channel); err != nil {
			log.Error(err, "failed to register channel")
			channel.Status.Ready = false
			r.setCondition(channel, "Ready", metav1.ConditionFalse, "RegistrationFailed", err.Error())
			if err := r.Status().Update(ctx, channel); err != nil {
				log.Error(err, "failed to update status after registration error")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.V(1).Info("channel registered successfully")
	}

	// 5. Update status
	channel.Status.Ready = true
	r.setCondition(channel, "Ready", metav1.ConditionTrue, "Validated", "Channel is ready")
	if err := r.Status().Update(ctx, channel); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}
	log.Info("reconciled successfully", "type", channel.Spec.Type, "ready", channel.Status.Ready)

	return ctrl.Result{}, nil
}

func (r *AlertChannelReconciler) validateConfig(ctx context.Context, channel *guardianv1alpha1.AlertChannel) error {
	switch channel.Spec.Type {
	case "slack":
		return r.validateSlack(ctx, channel.Spec.Slack)
	case "pagerduty":
		return r.validatePagerDuty(ctx, channel.Spec.PagerDuty)
	case "webhook":
		return r.validateWebhook(ctx, channel.Spec.Webhook)
	case "email":
		return r.validateEmail(ctx, channel.Spec.Email)
	default:
		return fmt.Errorf("unknown channel type: %s", channel.Spec.Type)
	}
}

func (r *AlertChannelReconciler) validateSlack(ctx context.Context, config *guardianv1alpha1.SlackConfig) error {
	if config == nil {
		return fmt.Errorf("slack config required for slack type")
	}

	// Verify secret exists and has the key
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: config.WebhookSecretRef.Namespace,
		Name:      config.WebhookSecretRef.Name,
	}, secret)
	if err != nil {
		return fmt.Errorf("failed to get webhook secret: %w", err)
	}

	if _, ok := secret.Data[config.WebhookSecretRef.Key]; !ok {
		return fmt.Errorf("key %s not found in secret", config.WebhookSecretRef.Key)
	}

	// Validate template if provided
	if config.MessageTemplate != "" {
		_, err := template.New("msg").Parse(config.MessageTemplate)
		if err != nil {
			return fmt.Errorf("invalid message template: %w", err)
		}
	}

	return nil
}

func (r *AlertChannelReconciler) validatePagerDuty(ctx context.Context, config *guardianv1alpha1.PagerDutyConfig) error {
	if config == nil {
		return fmt.Errorf("pagerduty config required for pagerduty type")
	}

	// Verify secret exists and has the key
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: config.RoutingKeySecretRef.Namespace,
		Name:      config.RoutingKeySecretRef.Name,
	}, secret)
	if err != nil {
		return fmt.Errorf("failed to get routing key secret: %w", err)
	}

	if _, ok := secret.Data[config.RoutingKeySecretRef.Key]; !ok {
		return fmt.Errorf("key %s not found in secret", config.RoutingKeySecretRef.Key)
	}

	return nil
}

func (r *AlertChannelReconciler) validateWebhook(ctx context.Context, config *guardianv1alpha1.WebhookConfig) error {
	if config == nil {
		return fmt.Errorf("webhook config required for webhook type")
	}

	// Verify secret exists and has the key
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: config.URLSecretRef.Namespace,
		Name:      config.URLSecretRef.Name,
	}, secret)
	if err != nil {
		return fmt.Errorf("failed to get URL secret: %w", err)
	}

	if _, ok := secret.Data[config.URLSecretRef.Key]; !ok {
		return fmt.Errorf("key %s not found in secret", config.URLSecretRef.Key)
	}

	// Validate template if provided
	if config.PayloadTemplate != "" {
		_, err := template.New("payload").Parse(config.PayloadTemplate)
		if err != nil {
			return fmt.Errorf("invalid payload template: %w", err)
		}
	}

	return nil
}

func (r *AlertChannelReconciler) validateEmail(ctx context.Context, config *guardianv1alpha1.EmailConfig) error {
	if config == nil {
		return fmt.Errorf("email config required for email type")
	}

	if config.From == "" {
		return fmt.Errorf("from address is required")
	}

	if len(config.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	// Verify SMTP secret exists
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: config.SMTPSecretRef.Namespace,
		Name:      config.SMTPSecretRef.Name,
	}, secret)
	if err != nil {
		return fmt.Errorf("failed to get SMTP secret: %w", err)
	}

	// Check required keys
	requiredKeys := []string{"host", "username", "password"}
	for _, key := range requiredKeys {
		if _, ok := secret.Data[key]; !ok {
			return fmt.Errorf("SMTP secret missing '%s' key", key)
		}
	}

	// Validate templates if provided
	if config.SubjectTemplate != "" {
		_, err := template.New("subject").Parse(config.SubjectTemplate)
		if err != nil {
			return fmt.Errorf("invalid subject template: %w", err)
		}
	}
	if config.BodyTemplate != "" {
		_, err := template.New("body").Parse(config.BodyTemplate)
		if err != nil {
			return fmt.Errorf("invalid body template: %w", err)
		}
	}

	return nil
}

func (r *AlertChannelReconciler) testChannel(ctx context.Context, channel *guardianv1alpha1.AlertChannel) error {
	if r.AlertDispatcher == nil {
		return fmt.Errorf("dispatcher not available")
	}

	testAlert := alerting.Alert{
		Key:       "test-alert",
		Type:      "Test",
		Severity:  "info",
		Title:     "CronJob Guardian Test Alert",
		Message:   "This is a test alert to verify channel configuration.",
		CronJob:   types.NamespacedName{Namespace: "test", Name: "test-cronjob"},
		Timestamp: metav1.Now().Time,
	}

	return r.AlertDispatcher.SendToChannel(ctx, channel.Name, testAlert)
}

func (r *AlertChannelReconciler) setCondition(channel *guardianv1alpha1.AlertChannel, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	found := false
	for i, c := range channel.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				channel.Status.Conditions[i] = condition
			}
			found = true
			break
		}
	}
	if !found {
		channel.Status.Conditions = append(channel.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AlertChannelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("setting up AlertChannel controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&guardianv1alpha1.AlertChannel{}).
		Named("alertchannel").
		Complete(r)
}
