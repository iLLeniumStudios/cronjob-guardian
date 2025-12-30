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
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// Alert represents an alert to be dispatched
type Alert struct {
	Key        string // Deduplication key
	Type       string // JobFailed, MissedSchedule, DeadManTriggered, etc.
	Severity   string // critical, warning, info
	Title      string
	Message    string
	CronJob    types.NamespacedName
	MonitorRef types.NamespacedName
	Context    AlertContext
	Timestamp  time.Time
}

// AlertContext contains additional context for alerts
type AlertContext struct {
	Logs         string
	Events       []string
	PodStatus    string
	SuggestedFix string
	SuccessRate  float64
	LastDuration time.Duration
	ExitCode     int32
	Reason       string
}

// Channel represents an alert delivery channel
type Channel interface {
	// Name returns the channel name
	Name() string

	// Send delivers an alert
	Send(ctx context.Context, alert Alert) error

	// Test sends a test alert
	Test(ctx context.Context) error
}

// Dispatcher handles alert routing and delivery
type Dispatcher interface {
	// Dispatch sends an alert through configured channels
	Dispatch(ctx context.Context, alert Alert, config *v1alpha1.AlertingConfig) error

	// RegisterChannel adds or updates an alert channel
	RegisterChannel(channel *v1alpha1.AlertChannel) error

	// RemoveChannel removes an alert channel
	RemoveChannel(name string)

	// SendToChannel sends to a specific channel (for testing)
	SendToChannel(ctx context.Context, channelName string, alert Alert) error

	// IsSuppressed checks if an alert should be suppressed
	IsSuppressed(alert Alert, config *v1alpha1.AlertingConfig) (bool, string)

	// ClearAlert clears an active alert (e.g., when resolved)
	ClearAlert(ctx context.Context, alertKey string) error

	// ClearAlertsForMonitor clears all alerts for a monitor
	ClearAlertsForMonitor(namespace, name string)

	// SetGlobalRateLimits updates global rate limits
	SetGlobalRateLimits(limits config.RateLimitsConfig)

	// GetAlertCount24h returns alerts sent in last 24h
	GetAlertCount24h() int32

	// SetStore sets the store for persisting alerts
	SetStore(s store.Store)
}

type dispatcher struct {
	channels      map[string]Channel   // name -> channel
	sentAlerts    map[string]time.Time // alertKey -> lastSent
	activeAlerts  map[string]Alert     // alertKey -> alert
	globalLimiter *rate.Limiter
	channelMu     sync.RWMutex
	alertMu       sync.RWMutex
	alertCount24h int32
	client        client.Client // K8s client for secret lookups
	store         store.Store   // Store for persisting alerts
}

// NewDispatcher creates a new alert dispatcher
func NewDispatcher(c client.Client) Dispatcher {
	return &dispatcher{
		channels:      make(map[string]Channel),
		sentAlerts:    make(map[string]time.Time),
		activeAlerts:  make(map[string]Alert),
		globalLimiter: rate.NewLimiter(rate.Limit(50.0/60.0), 10), // 50/min, burst 10
		client:        c,
	}
}

// Dispatch sends an alert through configured channels
func (d *dispatcher) Dispatch(ctx context.Context, alert Alert, config *v1alpha1.AlertingConfig) error {
	logger := log.FromContext(ctx)

	if config == nil || !isEnabled(config.Enabled) {
		return nil
	}

	// Generate dedup key if not set
	if alert.Key == "" {
		alert.Key = fmt.Sprintf("%s/%s/%s",
			alert.CronJob.Namespace,
			alert.CronJob.Name,
			alert.Type)
	}

	// Check suppression
	if suppressed, reason := d.IsSuppressed(alert, config); suppressed {
		logger.V(1).Info("alert suppressed", "key", alert.Key, "reason", reason)
		return nil
	}

	// Check global rate limit
	if !d.globalLimiter.Allow() {
		logger.Info("alert rate limited", "key", alert.Key)
		return fmt.Errorf("global rate limit exceeded")
	}

	// Collect target channels
	targetChannels := d.resolveChannels(config, alert.Severity)

	// Send to each channel
	var errs []error
	var channelNames []string
	for _, ch := range targetChannels {
		if err := ch.Send(ctx, alert); err != nil {
			logger.Error(err, "failed to send alert", "channel", ch.Name())
			errs = append(errs, err)
		} else {
			channelNames = append(channelNames, ch.Name())
		}
	}

	// Record sent time for dedup
	d.alertMu.Lock()
	d.sentAlerts[alert.Key] = time.Now()
	d.activeAlerts[alert.Key] = alert
	d.alertCount24h++
	d.alertMu.Unlock()

	// Store alert in history if store is available
	if d.store != nil && len(channelNames) > 0 {
		alertHistory := store.AlertHistory{
			Type:             alert.Type,
			Severity:         alert.Severity,
			Title:            alert.Title,
			Message:          alert.Message,
			CronJobNamespace: alert.CronJob.Namespace,
			CronJobName:      alert.CronJob.Name,
			MonitorNamespace: alert.MonitorRef.Namespace,
			MonitorName:      alert.MonitorRef.Name,
			ChannelsNotified: channelNames,
			OccurredAt:       alert.Timestamp,
		}
		if err := d.store.StoreAlert(ctx, alertHistory); err != nil {
			logger.Error(err, "failed to store alert in history")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to send to %d channels", len(errs))
	}
	return nil
}

// RegisterChannel adds or updates an alert channel
func (d *dispatcher) RegisterChannel(ac *v1alpha1.AlertChannel) error {
	ch, err := d.createChannel(ac)
	if err != nil {
		return err
	}

	d.channelMu.Lock()
	d.channels[ac.Name] = ch
	d.channelMu.Unlock()

	return nil
}

// RemoveChannel removes an alert channel
func (d *dispatcher) RemoveChannel(name string) {
	d.channelMu.Lock()
	delete(d.channels, name)
	d.channelMu.Unlock()
}

// SendToChannel sends to a specific channel (for testing)
func (d *dispatcher) SendToChannel(ctx context.Context, channelName string, alert Alert) error {
	d.channelMu.RLock()
	ch, ok := d.channels[channelName]
	d.channelMu.RUnlock()

	if !ok {
		return fmt.Errorf("channel %s not found", channelName)
	}

	return ch.Send(ctx, alert)
}

// IsSuppressed checks if an alert should be suppressed
func (d *dispatcher) IsSuppressed(alert Alert, config *v1alpha1.AlertingConfig) (bool, string) {
	d.alertMu.RLock()
	defer d.alertMu.RUnlock()

	// Check duplicate suppression
	if lastSent, ok := d.sentAlerts[alert.Key]; ok {
		suppressDuration := 1 * time.Hour
		if config.SuppressDuplicatesFor != nil {
			suppressDuration = config.SuppressDuplicatesFor.Duration
		}
		if time.Since(lastSent) < suppressDuration {
			return true, "duplicate within suppression window"
		}
	}

	// Maintenance window check would be done by caller (controller)
	// Dependency suppression would be done by caller (controller)

	return false, ""
}

// ClearAlert clears an active alert
func (d *dispatcher) ClearAlert(ctx context.Context, alertKey string) error {
	d.alertMu.Lock()
	delete(d.activeAlerts, alertKey)
	delete(d.sentAlerts, alertKey)
	d.alertMu.Unlock()
	return nil
}

// ClearAlertsForMonitor clears all alerts for a monitor
func (d *dispatcher) ClearAlertsForMonitor(namespace, name string) {
	prefix := fmt.Sprintf("%s/%s/", namespace, name)

	d.alertMu.Lock()
	defer d.alertMu.Unlock()

	for key := range d.activeAlerts {
		if strings.HasPrefix(key, prefix) {
			delete(d.activeAlerts, key)
			delete(d.sentAlerts, key)
		}
	}
}

// SetGlobalRateLimits updates global rate limits
func (d *dispatcher) SetGlobalRateLimits(limits config.RateLimitsConfig) {
	maxPerMinute := limits.MaxAlertsPerMinute
	if maxPerMinute <= 0 {
		maxPerMinute = 50
	}

	d.globalLimiter = rate.NewLimiter(rate.Limit(float64(maxPerMinute)/60.0), 10)
}

// GetAlertCount24h returns alerts sent in last 24h
func (d *dispatcher) GetAlertCount24h() int32 {
	d.alertMu.RLock()
	defer d.alertMu.RUnlock()
	return d.alertCount24h
}

// SetStore sets the store for persisting alerts
func (d *dispatcher) SetStore(s store.Store) {
	d.store = s
}

// resolveChannels resolves channel refs to actual channels
func (d *dispatcher) resolveChannels(config *v1alpha1.AlertingConfig, severity string) []Channel {
	var channels []Channel

	d.channelMu.RLock()
	defer d.channelMu.RUnlock()

	// Resolve channel refs
	for _, ref := range config.ChannelRefs {
		if ch, ok := d.channels[ref.Name]; ok {
			if len(ref.Severities) == 0 || contains(ref.Severities, severity) {
				channels = append(channels, ch)
			}
		}
	}

	return channels
}

// createChannel creates a Channel from an AlertChannel CR
func (d *dispatcher) createChannel(ac *v1alpha1.AlertChannel) (Channel, error) {
	switch ac.Spec.Type {
	case "slack":
		return NewSlackChannel(d.client, ac)
	case "pagerduty":
		return NewPagerDutyChannel(d.client, ac)
	case "webhook":
		return NewWebhookChannel(d.client, ac)
	case "email":
		return NewEmailChannel(d.client, ac)
	default:
		return nil, fmt.Errorf("unknown channel type: %s", ac.Spec.Type)
	}
}

// Helper functions

func isEnabled(b *bool) bool {
	return b == nil || *b // Default to true if not set
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Template functions for alert message formatting
var templateFuncs = template.FuncMap{
	"formatTime": func(t time.Time, layout string) string {
		if layout == "RFC3339" {
			return t.Format(time.RFC3339)
		}
		return t.Format(layout)
	},
	"humanizeDuration": func(d time.Duration) string {
		if d < time.Minute {
			return fmt.Sprintf("%ds", int(d.Seconds()))
		}
		if d < time.Hour {
			return fmt.Sprintf("%dm", int(d.Minutes()))
		}
		if d < 24*time.Hour {
			return fmt.Sprintf("%dh", int(d.Hours()))
		}
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	},
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "..."
	},
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"jsonEscape": func(s string) string {
		// Marshal to JSON string (includes quotes and escaping)
		b, err := json.Marshal(s)
		if err != nil {
			return `""`
		}
		return string(b)
	},
}
