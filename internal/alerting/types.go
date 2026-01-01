package alerting

import (
	"context"
	"time"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"k8s.io/apimachinery/pkg/types"
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
	CancelPendingAlert(alertKey string) bool

	// CancelPendingAlertsForCronJob cancels all pending alerts for a specific CronJob.
	CancelPendingAlertsForCronJob(namespace, name string) int

	// SetGlobalRateLimits updates global rate limits
	SetGlobalRateLimits(limits config.RateLimitsConfig)

	// GetAlertCount24h returns alerts sent in last 24h
	GetAlertCount24h() int32

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
