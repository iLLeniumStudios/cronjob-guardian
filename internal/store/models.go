package store

import (
	"strings"
	"time"
)

// Execution represents a CronJob execution record (GORM model)
type Execution struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	CronJobNamespace string     `gorm:"column:cronjob_ns;size:253;not null;index:idx_cronjob_time,priority:1;index:idx_cronjob_uid,priority:1;index:idx_cronjob_duration,priority:1"`
	CronJobName      string     `gorm:"column:cronjob_name;size:253;not null;index:idx_cronjob_time,priority:2;index:idx_cronjob_uid,priority:2;index:idx_cronjob_duration,priority:2"`
	CronJobUID       string     `gorm:"column:cronjob_uid;size:36;index:idx_cronjob_uid,priority:3"`
	JobName          string     `gorm:"column:job_name;size:253;not null;index"`
	ScheduledTime    *time.Time `gorm:"column:scheduled_time"`
	StartTime        time.Time  `gorm:"column:start_time;not null;index:idx_cronjob_time,priority:3,sort:desc;index:idx_start_time;index:idx_cronjob_duration,priority:3"`
	CompletionTime   time.Time  `gorm:"column:completion_time"`
	DurationSecs     *float64   `gorm:"column:duration_secs;index:idx_cronjob_duration,priority:4"`
	Succeeded        bool       `gorm:"column:succeeded;not null"`
	ExitCode         int32      `gorm:"column:exit_code"`
	Reason           string     `gorm:"column:reason;size:255"`
	IsRetry          bool       `gorm:"column:is_retry;default:false"`
	RetryOf          string     `gorm:"column:retry_of;size:253"`
	Logs             *string    `gorm:"column:logs;type:text"`
	Events           *string    `gorm:"column:events;type:text"`
	SuggestedFix     string     `gorm:"column:suggested_fix;type:text"` // Generated fix suggestion for failures
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime"`
}

// TableName specifies the table name for Execution
func (*Execution) TableName() string {
	return "executions"
}

// Duration returns the duration as time.Duration
func (e *Execution) Duration() time.Duration {
	if e.DurationSecs == nil {
		return 0
	}
	return time.Duration(*e.DurationSecs * float64(time.Second))
}

// SetDuration sets the duration from time.Duration
func (e *Execution) SetDuration(d time.Duration) {
	secs := d.Seconds()
	e.DurationSecs = &secs
}

// AlertHistory represents an alert event record (GORM model)
type AlertHistory struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	Type             string     `gorm:"column:alert_type;size:100;not null;index:idx_alert_resolve,priority:1"`
	Severity         string     `gorm:"column:severity;size:20;not null;index:idx_alert_severity"`
	Title            string     `gorm:"column:title;size:500;not null"`
	Message          string     `gorm:"column:message;type:text"`
	CronJobNamespace string     `gorm:"column:cronjob_ns;size:253;index:idx_alert_cronjob,priority:1;index:idx_alert_cronjob_time,priority:1;index:idx_alert_resolve,priority:2"`
	CronJobName      string     `gorm:"column:cronjob_name;size:253;index:idx_alert_cronjob,priority:2;index:idx_alert_cronjob_time,priority:2;index:idx_alert_resolve,priority:3"`
	MonitorNamespace string     `gorm:"column:monitor_ns;size:253"`
	MonitorName      string     `gorm:"column:monitor_name;size:253"`
	ChannelsNotified string     `gorm:"column:channels_notified;type:text"` // Comma-separated
	OccurredAt       time.Time  `gorm:"column:occurred_at;not null;index:idx_alert_occurred,sort:desc;index:idx_alert_cronjob_time,priority:3,sort:desc"`
	ResolvedAt       *time.Time `gorm:"column:resolved_at;index:idx_alert_unresolved;index:idx_alert_resolve,priority:4"`
	// Context fields for failure alerts
	ExitCode     int32  `gorm:"column:exit_code"`
	Reason       string `gorm:"column:reason;size:255"`
	SuggestedFix string `gorm:"column:suggested_fix;type:text"`
}

// TableName specifies the table name for AlertHistory
func (*AlertHistory) TableName() string {
	return "alert_history"
}

// GetChannelsNotified returns the channels as a slice
func (a *AlertHistory) GetChannelsNotified() []string {
	if a.ChannelsNotified == "" {
		return nil
	}
	return strings.Split(a.ChannelsNotified, ",")
}

// SetChannelsNotified sets the channels from a slice
func (a *AlertHistory) SetChannelsNotified(channels []string) {
	a.ChannelsNotified = strings.Join(channels, ",")
}

// Metrics contains aggregated SLA metrics (query result, not a GORM model)
type Metrics struct {
	SuccessRate        float64
	WindowDays         int32
	TotalRuns          int32
	SuccessfulRuns     int32
	FailedRuns         int32
	AvgDurationSeconds float64
	P50DurationSeconds float64
	P95DurationSeconds float64
	P99DurationSeconds float64
}

// AlertHistoryQuery contains parameters for querying alert history
type AlertHistoryQuery struct {
	Limit    int
	Offset   int
	Since    *time.Time
	Severity string
	Type     string // Filter by alert type (e.g., "JobFailed", "SLABreached")
}

// ChannelAlertStats contains alert statistics for a channel (query result)
type ChannelAlertStats struct {
	ChannelName     string
	AlertsSentTotal int64
}

// ChannelStatsRecord persists channel alert statistics (GORM model)
type ChannelStatsRecord struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement"`
	ChannelName         string     `gorm:"column:channel_name;size:253;not null;uniqueIndex"`
	AlertsSentTotal     int64      `gorm:"column:alerts_sent_total;default:0"`
	AlertsFailedTotal   int64      `gorm:"column:alerts_failed_total;default:0"`
	LastAlertTime       *time.Time `gorm:"column:last_alert_time"`
	LastFailedTime      *time.Time `gorm:"column:last_failed_time"`
	LastFailedError     string     `gorm:"column:last_failed_error;type:text"`
	ConsecutiveFailures int32      `gorm:"column:consecutive_failures;default:0"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName specifies the table name for ChannelStatsRecord
func (*ChannelStatsRecord) TableName() string {
	return "channel_stats"
}
