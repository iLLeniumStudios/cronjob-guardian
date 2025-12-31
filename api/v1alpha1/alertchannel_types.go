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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertChannelSpec defines the desired state of AlertChannel
type AlertChannelSpec struct {
	// Type of alert channel
	// +kubebuilder:validation:Enum=slack;pagerduty;webhook;email
	Type string `json:"type"`

	// Slack configuration
	// +optional
	Slack *SlackConfig `json:"slack,omitempty"`

	// PagerDuty configuration
	// +optional
	PagerDuty *PagerDutyConfig `json:"pagerduty,omitempty"`

	// Webhook configuration
	// +optional
	Webhook *WebhookConfig `json:"webhook,omitempty"`

	// Email configuration
	// +optional
	Email *EmailConfig `json:"email,omitempty"`

	// RateLimiting prevents alert storms
	// +optional
	RateLimiting *RateLimitConfig `json:"rateLimiting,omitempty"`

	// TestOnSave sends a test alert when saved (default: false)
	// +optional
	TestOnSave bool `json:"testOnSave,omitempty"`
}

// SlackConfig configures Slack notifications
type SlackConfig struct {
	// WebhookSecretRef references the Secret containing webhook URL
	WebhookSecretRef NamespacedSecretKeyRef `json:"webhookSecretRef"`

	// DefaultChannel overrides webhook's default channel
	// +optional
	DefaultChannel string `json:"defaultChannel,omitempty"`

	// MessageTemplate is a Go template for message formatting
	// +optional
	MessageTemplate string `json:"messageTemplate,omitempty"`
}

// PagerDutyConfig configures PagerDuty notifications
type PagerDutyConfig struct {
	// RoutingKeySecretRef references the Secret containing routing key
	RoutingKeySecretRef NamespacedSecretKeyRef `json:"routingKeySecretRef"`

	// Severity is the default PagerDuty severity
	// +kubebuilder:validation:Enum=critical;error;warning;info
	// +optional
	Severity string `json:"severity,omitempty"`
}

// WebhookConfig configures generic webhook notifications
type WebhookConfig struct {
	// URLSecretRef references the Secret containing webhook URL
	URLSecretRef NamespacedSecretKeyRef `json:"urlSecretRef"`

	// Method is the HTTP method (default: POST)
	// +kubebuilder:validation:Enum=POST;PUT
	// +optional
	Method string `json:"method,omitempty"`

	// Headers to include in requests
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// PayloadTemplate is a Go template for JSON payload
	// +optional
	PayloadTemplate string `json:"payloadTemplate,omitempty"`
}

// EmailConfig configures email notifications
type EmailConfig struct {
	// SMTPSecretRef references Secret with host, port, username, password
	SMTPSecretRef NamespacedSecretRef `json:"smtpSecretRef"`

	// From is the sender address
	From string `json:"from"`

	// To is the list of recipient addresses
	To []string `json:"to"`

	// SubjectTemplate is a Go template for subject
	// +optional
	SubjectTemplate string `json:"subjectTemplate,omitempty"`

	// BodyTemplate is a Go template for body
	// +optional
	BodyTemplate string `json:"bodyTemplate,omitempty"`
}

// NamespacedSecretKeyRef references a key in a namespaced Secret
type NamespacedSecretKeyRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

// NamespacedSecretRef references a namespaced Secret
type NamespacedSecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	// MaxAlertsPerHour limits alerts per hour (default: 100)
	// +optional
	MaxAlertsPerHour *int32 `json:"maxAlertsPerHour,omitempty"`

	// BurstLimit limits alerts per minute (default: 10)
	// +optional
	BurstLimit *int32 `json:"burstLimit,omitempty"`
}

// AlertChannelStatus defines the observed state of AlertChannel
type AlertChannelStatus struct {
	// Ready indicates the channel is operational
	Ready bool `json:"ready"`

	// LastTestTime is when the channel was last tested
	// +optional
	LastTestTime *metav1.Time `json:"lastTestTime,omitempty"`

	// LastTestResult is the result of the last test
	// +kubebuilder:validation:Enum=success;failed
	// +optional
	LastTestResult string `json:"lastTestResult,omitempty"`

	// LastTestError is the error from the last test
	// +optional
	LastTestError string `json:"lastTestError,omitempty"`

	// AlertsSentTotal is total alerts successfully sent via this channel
	AlertsSentTotal int64 `json:"alertsSentTotal"`

	// LastAlertTime is when the last alert was successfully sent
	// +optional
	LastAlertTime *metav1.Time `json:"lastAlertTime,omitempty"`

	// AlertsFailedTotal is total alerts that failed to send via this channel
	AlertsFailedTotal int64 `json:"alertsFailedTotal"`

	// LastFailedTime is when the last alert failed to send
	// +optional
	LastFailedTime *metav1.Time `json:"lastFailedTime,omitempty"`

	// LastFailedError is the error message from the last failed send
	// +optional
	LastFailedError string `json:"lastFailedError,omitempty"`

	// ConsecutiveFailures is the number of consecutive failed sends
	// Resets to 0 on successful send
	ConsecutiveFailures int32 `json:"consecutiveFailures"`

	// Conditions represent latest observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Last Alert",type=date,JSONPath=`.status.lastAlertTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AlertChannel is the Schema for the alertchannels API.
type AlertChannel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertChannelSpec   `json:"spec,omitempty"`
	Status AlertChannelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AlertChannelList contains a list of AlertChannel.
type AlertChannelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertChannel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertChannel{}, &AlertChannelList{})
}
