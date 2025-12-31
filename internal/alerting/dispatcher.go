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
	"github.com/iLLeniumStudios/cronjob-guardian/internal/metrics"
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

	// Type returns the channel type (slack, pagerduty, webhook, email)
	Type() string

	// Send delivers an alert
	Send(ctx context.Context, alert Alert) error

	// Test sends a test alert
	Test(ctx context.Context) error
}

// ChannelStats tracks success/failure statistics for a channel
type ChannelStats struct {
	AlertsSentTotal     int64
	AlertsFailedTotal   int64
	LastAlertTime       time.Time
	LastFailedTime      time.Time
	LastFailedError     string
	ConsecutiveFailures int32
}

// Dispatcher handles alert routing and delivery
type Dispatcher interface {
	// Dispatch sends an alert through configured channels
	// If alertCfg.AlertDelay is set, the alert is queued and sent after the delay
	// unless cancelled by CancelPendingAlert
	Dispatch(ctx context.Context, alert Alert, alertCfg *v1alpha1.AlertingConfig) error

	// RegisterChannel adds or updates an alert channel
	RegisterChannel(channel *v1alpha1.AlertChannel) error

	// RemoveChannel removes an alert channel
	RemoveChannel(name string)

	// SendToChannel sends to a specific channel (for testing)
	SendToChannel(ctx context.Context, channelName string, alert Alert) error

	// IsSuppressed checks if an alert should be suppressed
	IsSuppressed(alert Alert, alertCfg *v1alpha1.AlertingConfig) (bool, string)

	// ClearAlert clears an active alert (e.g., when resolved)
	ClearAlert(ctx context.Context, alertKey string) error

	// ClearAlertsForMonitor clears all alerts for a monitor
	ClearAlertsForMonitor(namespace, name string)

	// CancelPendingAlert cancels a pending (delayed) alert before it's sent.
	// Call this when an issue resolves (e.g., next job succeeds) to prevent
	// the delayed alert from being sent.
	CancelPendingAlert(alertKey string) bool

	// CancelPendingAlertsForCronJob cancels all pending alerts for a specific CronJob.
	// Useful when a CronJob succeeds after a failure to cancel any pending failure alerts.
	CancelPendingAlertsForCronJob(namespace, name string) int

	// SetGlobalRateLimits updates global rate limits
	SetGlobalRateLimits(limits config.RateLimitsConfig)

	// SetStartupGracePeriod sets the grace period during which alerts are suppressed
	// to prevent alert floods on operator restart. Must be called before alerts start dispatching.
	SetStartupGracePeriod(d time.Duration)

	// GetAlertCount24h returns alerts sent in last 24h
	GetAlertCount24h() int32

	// SetStore sets the store for persisting alerts
	SetStore(s store.Store)

	// GetChannelStats returns statistics for a specific channel
	GetChannelStats(channelName string) *ChannelStats
}

// PendingAlert represents an alert that is delayed before sending
type PendingAlert struct {
	Alert    Alert
	AlertCfg *v1alpha1.AlertingConfig
	SendAt   time.Time
	Cancel   chan struct{}
}

type dispatcher struct {
	channels           map[string]Channel       // name -> channel
	channelStats       map[string]*ChannelStats // name -> stats
	sentAlerts         map[string]time.Time     // alertKey -> lastSent
	activeAlerts       map[string]Alert         // alertKey -> alert
	pendingAlerts      map[string]*PendingAlert // alertKey -> pending alert (delayed)
	globalLimiter      *rate.Limiter
	channelMu          sync.RWMutex
	alertMu            sync.RWMutex
	statsMu            sync.RWMutex
	pendingMu          sync.RWMutex
	alertCount24h      int32
	client             client.Client // K8s client for secret lookups
	store              store.Store   // Store for persisting alerts
	cleanupDone        chan struct{} // Signal channel for cleanup goroutine shutdown
	startupGracePeriod time.Duration // Grace period after startup to suppress alerts
	readyAt            time.Time     // Time when dispatcher becomes ready (after grace period)
}

// NewDispatcher creates a new alert dispatcher
func NewDispatcher(c client.Client) Dispatcher {
	d := &dispatcher{
		channels:           make(map[string]Channel),
		channelStats:       make(map[string]*ChannelStats),
		sentAlerts:         make(map[string]time.Time),
		activeAlerts:       make(map[string]Alert),
		pendingAlerts:      make(map[string]*PendingAlert),
		globalLimiter:      rate.NewLimiter(rate.Limit(50.0/60.0), 10), // 50/min, burst 10
		client:             c,
		cleanupDone:        make(chan struct{}),
		startupGracePeriod: 0,          // Set via SetStartupGracePeriod from config
		readyAt:            time.Now(), // Ready immediately until grace period is set
	}
	d.startCleanup()
	return d
}

// Dispatch sends an alert through configured channels.
// If alertCfg.AlertDelay is set, the alert is queued and sent after the delay
// unless cancelled by CancelPendingAlert.
func (d *dispatcher) Dispatch(ctx context.Context, alert Alert, alertCfg *v1alpha1.AlertingConfig) error {
	logger := log.FromContext(ctx)

	if alertCfg == nil || !isEnabled(alertCfg.Enabled) {
		return nil
	}

	// Generate dedup key if not set
	if alert.Key == "" {
		alert.Key = fmt.Sprintf("%s/%s/%s",
			alert.CronJob.Namespace,
			alert.CronJob.Name,
			alert.Type)
	}

	// Check startup grace period - suppress alerts during initial reconciliation
	// to prevent flood of alerts on operator restart
	if time.Now().Before(d.readyAt) {
		remaining := time.Until(d.readyAt).Round(time.Second)
		logger.V(1).Info("alert suppressed during startup grace period",
			"key", alert.Key,
			"remainingGracePeriod", remaining)
		// Track that we've seen this alert to prevent duplicate after grace period
		d.alertMu.Lock()
		d.sentAlerts[alert.Key] = time.Now()
		d.activeAlerts[alert.Key] = alert
		d.alertMu.Unlock()
		return nil
	}

	// Check suppression
	if suppressed, reason := d.IsSuppressed(alert, alertCfg); suppressed {
		logger.V(1).Info("alert suppressed", "key", alert.Key, "reason", reason)
		return nil
	}

	// Check if alert delay is configured
	if alertCfg.AlertDelay != nil && alertCfg.AlertDelay.Duration > 0 {
		return d.queueDelayedAlert(alert, alertCfg)
	}

	// No delay configured, dispatch immediately
	return d.dispatchImmediate(ctx, alert, alertCfg)
}

// dispatchImmediate sends an alert immediately without delay
func (d *dispatcher) dispatchImmediate(ctx context.Context, alert Alert, alertCfg *v1alpha1.AlertingConfig) error {
	logger := log.FromContext(ctx)

	// Check global rate limit
	if !d.globalLimiter.Allow() {
		logger.Info("alert rate limited", "key", alert.Key)
		return fmt.Errorf("global rate limit exceeded")
	}

	// Collect target channels
	targetChannels := d.resolveChannels(alertCfg, alert.Severity)

	if len(targetChannels) == 0 {
		logger.V(1).Info("no channels configured for alert",
			"alertKey", alert.Key,
			"severity", alert.Severity,
			"cronjob", fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name))
		return nil
	}

	// Log which channels will receive the alert
	channelInfo := make([]string, 0, len(targetChannels))
	for _, ch := range targetChannels {
		channelInfo = append(channelInfo, fmt.Sprintf("%s(%s)", ch.Name(), ch.Type()))
	}
	logger.Info("dispatching alert to channels",
		"alertKey", alert.Key,
		"alertType", alert.Type,
		"severity", alert.Severity,
		"cronjob", fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name),
		"channels", strings.Join(channelInfo, ", "))

	// Send to each channel
	var errs []error
	var channelNames []string
	for _, ch := range targetChannels {
		logger.V(1).Info("sending alert to channel",
			"channel", ch.Name(),
			"provider", ch.Type(),
			"alertKey", alert.Key)

		if err := ch.Send(ctx, alert); err != nil {
			logger.Error(err, "failed to send alert to channel",
				"channel", ch.Name(),
				"provider", ch.Type(),
				"alertKey", alert.Key)
			errs = append(errs, err)

			// Record failure stats
			d.recordChannelFailure(ch.Name(), err)

			// Record Prometheus metrics for failed alert delivery
			metrics.RecordAlertFailed(
				alert.CronJob.Namespace,
				alert.CronJob.Name,
				alert.Type,
				alert.Severity,
				ch.Name(),
			)
		} else {
			logger.Info("alert sent successfully",
				"channel", ch.Name(),
				"provider", ch.Type(),
				"alertKey", alert.Key,
				"cronjob", fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name))
			channelNames = append(channelNames, ch.Name())

			// Record success stats
			d.recordChannelSuccess(ch.Name())

			// Record Prometheus metrics for successful alert delivery
			metrics.RecordAlert(
				alert.CronJob.Namespace,
				alert.CronJob.Name,
				alert.Type,
				alert.Severity,
				ch.Name(),
			)
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
			OccurredAt:       alert.Timestamp,
			// Include context for failure alerts
			ExitCode:     alert.Context.ExitCode,
			Reason:       alert.Context.Reason,
			SuggestedFix: alert.Context.SuggestedFix,
		}
		alertHistory.SetChannelsNotified(channelNames)
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
func (d *dispatcher) IsSuppressed(alert Alert, alertCfg *v1alpha1.AlertingConfig) (bool, string) {
	d.alertMu.RLock()
	defer d.alertMu.RUnlock()

	// Check duplicate suppression
	if lastSent, ok := d.sentAlerts[alert.Key]; ok {
		suppressDuration := 1 * time.Hour
		if alertCfg.SuppressDuplicatesFor != nil {
			suppressDuration = alertCfg.SuppressDuplicatesFor.Duration
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

// queueDelayedAlert queues an alert to be sent after the configured delay.
// If the alert is cancelled before the delay expires, it won't be sent.
func (d *dispatcher) queueDelayedAlert(alert Alert, alertCfg *v1alpha1.AlertingConfig) error {
	delay := alertCfg.AlertDelay.Duration

	d.pendingMu.Lock()

	// Check if there's already a pending alert for this key
	if existing, ok := d.pendingAlerts[alert.Key]; ok {
		d.pendingMu.Unlock()
		// Already pending, don't queue another
		log.Log.V(1).Info("alert already pending",
			"key", alert.Key,
			"sendAt", existing.SendAt)
		return nil
	}

	// Create pending alert
	pending := &PendingAlert{
		Alert:    alert,
		AlertCfg: alertCfg,
		SendAt:   time.Now().Add(delay),
		Cancel:   make(chan struct{}),
	}
	d.pendingAlerts[alert.Key] = pending
	d.pendingMu.Unlock()

	log.Log.Info("alert queued with delay",
		"key", alert.Key,
		"delay", delay,
		"sendAt", pending.SendAt,
		"cronjob", fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name))

	// Start goroutine to send after delay
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Delay expired, check if still pending and send
			d.pendingMu.Lock()
			_, stillPending := d.pendingAlerts[alert.Key]
			if stillPending {
				delete(d.pendingAlerts, alert.Key)
			}
			d.pendingMu.Unlock()

			if stillPending {
				log.Log.Info("alert delay expired, dispatching",
					"key", alert.Key,
					"cronjob", fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name))

				ctx := context.Background()
				if err := d.dispatchImmediate(ctx, alert, alertCfg); err != nil {
					log.Log.Error(err, "failed to dispatch delayed alert", "key", alert.Key)
				}
			}

		case <-pending.Cancel:
			// Alert was cancelled
			d.pendingMu.Lock()
			delete(d.pendingAlerts, alert.Key)
			d.pendingMu.Unlock()

			log.Log.Info("pending alert cancelled",
				"key", alert.Key,
				"cronjob", fmt.Sprintf("%s/%s", alert.CronJob.Namespace, alert.CronJob.Name))
		}
	}()

	return nil
}

// CancelPendingAlert cancels a pending (delayed) alert before it's sent.
// Returns true if an alert was cancelled, false if no pending alert was found.
func (d *dispatcher) CancelPendingAlert(alertKey string) bool {
	d.pendingMu.Lock()
	defer d.pendingMu.Unlock()

	if pending, ok := d.pendingAlerts[alertKey]; ok {
		// Signal the goroutine to cancel
		close(pending.Cancel)
		delete(d.pendingAlerts, alertKey)
		return true
	}
	return false
}

// CancelPendingAlertsForCronJob cancels all pending alerts for a specific CronJob.
// Returns the number of alerts cancelled.
func (d *dispatcher) CancelPendingAlertsForCronJob(namespace, name string) int {
	prefix := fmt.Sprintf("%s/%s/", namespace, name)
	cancelled := 0

	d.pendingMu.Lock()
	defer d.pendingMu.Unlock()

	for key, pending := range d.pendingAlerts {
		if strings.HasPrefix(key, prefix) {
			close(pending.Cancel)
			delete(d.pendingAlerts, key)
			cancelled++
		}
	}

	if cancelled > 0 {
		log.Log.Info("cancelled pending alerts for cronjob",
			"namespace", namespace,
			"name", name,
			"count", cancelled)
	}

	return cancelled
}

// SetGlobalRateLimits updates global rate limits
func (d *dispatcher) SetGlobalRateLimits(limits config.RateLimitsConfig) {
	maxPerMinute := limits.MaxAlertsPerMinute
	if maxPerMinute <= 0 {
		maxPerMinute = 50
	}

	d.globalLimiter = rate.NewLimiter(rate.Limit(float64(maxPerMinute)/60.0), 10)
}

// SetStartupGracePeriod sets the grace period during which alerts are suppressed
func (d *dispatcher) SetStartupGracePeriod(gracePeriod time.Duration) {
	d.startupGracePeriod = gracePeriod
	d.readyAt = time.Now().Add(gracePeriod)
	log.Log.Info("dispatcher startup grace period set",
		"gracePeriod", gracePeriod,
		"readyAt", d.readyAt)
}

// GetAlertCount24h returns alerts sent in last 24h
func (d *dispatcher) GetAlertCount24h() int32 {
	d.alertMu.RLock()
	defer d.alertMu.RUnlock()
	return d.alertCount24h
}

// SetStore sets the store for persisting alerts and loads existing channel stats
func (d *dispatcher) SetStore(s store.Store) {
	d.store = s
	// Load persisted channel stats
	d.loadChannelStats()
	// Load recent alerts to restore sentAlerts state for duplicate suppression
	d.loadRecentAlerts()
}

// loadChannelStats loads channel statistics from the store
func (d *dispatcher) loadChannelStats() {
	if d.store == nil {
		return
	}

	ctx := context.Background()
	allStats, err := d.store.GetAllChannelStats(ctx)
	if err != nil {
		// Log error but don't fail - we'll start fresh
		return
	}

	d.statsMu.Lock()
	defer d.statsMu.Unlock()

	for name, record := range allStats {
		d.channelStats[name] = &ChannelStats{
			AlertsSentTotal:     record.AlertsSentTotal,
			AlertsFailedTotal:   record.AlertsFailedTotal,
			ConsecutiveFailures: record.ConsecutiveFailures,
			LastFailedError:     record.LastFailedError,
		}
		if record.LastAlertTime != nil {
			d.channelStats[name].LastAlertTime = *record.LastAlertTime
		}
		if record.LastFailedTime != nil {
			d.channelStats[name].LastFailedTime = *record.LastFailedTime
		}
	}
}

// loadRecentAlerts loads recent unresolved alerts from the store to restore
// the sentAlerts map. This ensures duplicate suppression works across operator restarts.
func (d *dispatcher) loadRecentAlerts() {
	if d.store == nil {
		return
	}

	ctx := context.Background()
	// Load alerts from the last hour (typical suppression window)
	since := time.Now().Add(-1 * time.Hour)
	query := store.AlertHistoryQuery{
		Limit: 1000,
		Since: &since,
	}

	alerts, _, err := d.store.ListAlertHistory(ctx, query)
	if err != nil {
		log.Log.Error(err, "failed to load recent alerts on startup")
		return
	}

	d.alertMu.Lock()
	defer d.alertMu.Unlock()

	loaded := 0
	for _, alert := range alerts {
		// Only load unresolved alerts
		if alert.ResolvedAt != nil {
			continue
		}

		// Generate the same key format used in Dispatch
		alertKey := fmt.Sprintf("%s/%s/%s",
			alert.CronJobNamespace,
			alert.CronJobName,
			alert.Type)

		// Record as sent at the time it occurred
		d.sentAlerts[alertKey] = alert.OccurredAt
		loaded++
	}

	if loaded > 0 {
		log.Log.Info("loaded recent alerts for duplicate suppression",
			"count", loaded,
			"since", since)
	}
}

// resolveChannels resolves channel refs to actual channels
func (d *dispatcher) resolveChannels(alertCfg *v1alpha1.AlertingConfig, severity string) []Channel {
	var channels []Channel

	d.channelMu.RLock()
	defer d.channelMu.RUnlock()

	// Resolve channel refs
	for _, ref := range alertCfg.ChannelRefs {
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

// recordChannelSuccess records a successful alert send for a channel
func (d *dispatcher) recordChannelSuccess(channelName string) {
	d.statsMu.Lock()
	stats, ok := d.channelStats[channelName]
	if !ok {
		stats = &ChannelStats{}
		d.channelStats[channelName] = stats
	}

	stats.AlertsSentTotal++
	stats.LastAlertTime = time.Now()
	stats.ConsecutiveFailures = 0 // Reset on success

	// Copy stats for persistence (avoid holding lock during DB call)
	statsCopy := *stats
	d.statsMu.Unlock()

	// Persist to store asynchronously
	d.persistChannelStats(channelName, statsCopy)
}

// recordChannelFailure records a failed alert send for a channel
func (d *dispatcher) recordChannelFailure(channelName string, err error) {
	d.statsMu.Lock()
	stats, ok := d.channelStats[channelName]
	if !ok {
		stats = &ChannelStats{}
		d.channelStats[channelName] = stats
	}

	stats.AlertsFailedTotal++
	stats.LastFailedTime = time.Now()
	stats.LastFailedError = err.Error()
	stats.ConsecutiveFailures++

	// Copy stats for persistence (avoid holding lock during DB call)
	statsCopy := *stats
	d.statsMu.Unlock()

	// Persist to store asynchronously
	d.persistChannelStats(channelName, statsCopy)
}

// persistChannelStats saves channel stats to the store asynchronously
func (d *dispatcher) persistChannelStats(channelName string, stats ChannelStats) {
	if d.store == nil {
		return
	}

	// Run in goroutine to avoid blocking alert dispatch
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		record := store.ChannelStatsRecord{
			ChannelName:         channelName,
			AlertsSentTotal:     stats.AlertsSentTotal,
			AlertsFailedTotal:   stats.AlertsFailedTotal,
			ConsecutiveFailures: stats.ConsecutiveFailures,
			LastFailedError:     stats.LastFailedError,
		}
		if !stats.LastAlertTime.IsZero() {
			record.LastAlertTime = &stats.LastAlertTime
		}
		if !stats.LastFailedTime.IsZero() {
			record.LastFailedTime = &stats.LastFailedTime
		}

		_ = d.store.SaveChannelStats(ctx, record)
	}()
}

// GetChannelStats returns statistics for a specific channel
func (d *dispatcher) GetChannelStats(channelName string) *ChannelStats {
	d.statsMu.RLock()
	defer d.statsMu.RUnlock()

	if stats, ok := d.channelStats[channelName]; ok {
		// Return a copy to avoid race conditions
		return &ChannelStats{
			AlertsSentTotal:     stats.AlertsSentTotal,
			AlertsFailedTotal:   stats.AlertsFailedTotal,
			LastAlertTime:       stats.LastAlertTime,
			LastFailedTime:      stats.LastFailedTime,
			LastFailedError:     stats.LastFailedError,
			ConsecutiveFailures: stats.ConsecutiveFailures,
		}
	}
	return nil
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

// startCleanup starts a background goroutine that periodically cleans up old alerts
// to prevent unbounded memory growth in sentAlerts and activeAlerts maps
func (d *dispatcher) startCleanup() {
	ticker := time.NewTicker(1 * time.Hour)

	go func() {
		for {
			select {
			case <-ticker.C:
				d.cleanupOldAlerts()
			case <-d.cleanupDone:
				ticker.Stop()
				return
			}
		}
	}()
}

// cleanupOldAlerts removes alerts older than 24 hours from in-memory maps
func (d *dispatcher) cleanupOldAlerts() {
	cutoff := time.Now().Add(-24 * time.Hour)

	d.alertMu.Lock()
	defer d.alertMu.Unlock()

	for key, sentTime := range d.sentAlerts {
		if sentTime.Before(cutoff) {
			delete(d.sentAlerts, key)
			delete(d.activeAlerts, key)
		}
	}

	// Reset 24h counter (will be recalculated from remaining alerts)
	d.alertCount24h = int32(len(d.sentAlerts))
}
