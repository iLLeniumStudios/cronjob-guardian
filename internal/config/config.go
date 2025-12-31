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

package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration for the operator
type Config struct {
	// configFileUsed is the path to the config file that was loaded (empty if none)
	configFileUsed string

	// LogLevel is the logging level (debug, info, warn, error)
	LogLevel string `mapstructure:"log-level"`

	// Scheduler configuration
	Scheduler SchedulerConfig `mapstructure:"scheduler"`

	// Storage configuration
	Storage StorageConfig `mapstructure:"storage"`

	// HistoryRetention configuration
	HistoryRetention HistoryRetentionConfig `mapstructure:"history-retention"`

	// RateLimits for alerts and remediations
	RateLimits RateLimitsConfig `mapstructure:"rate-limits"`

	// UI server configuration (serves both web UI and REST API)
	UI UIConfig `mapstructure:"ui"`

	// Metrics server configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Probes configuration
	Probes ProbesConfig `mapstructure:"probes"`

	// LeaderElection configuration
	LeaderElection LeaderElectionConfig `mapstructure:"leader-election"`

	// Webhook configuration
	Webhook WebhookConfig `mapstructure:"webhook"`
}

// SchedulerConfig configures background schedulers
type SchedulerConfig struct {
	// DeadManSwitchInterval is how often to check dead-man's switches
	DeadManSwitchInterval time.Duration `mapstructure:"dead-man-switch-interval" json:"deadManSwitchInterval"`

	// SLARecalculationInterval is how often to recalculate SLA metrics
	SLARecalculationInterval time.Duration `mapstructure:"sla-recalculation-interval" json:"slaRecalculationInterval"`

	// PruneInterval is how often to prune old execution history
	PruneInterval time.Duration `mapstructure:"prune-interval" json:"pruneInterval"`
}

// StorageConfig configures the storage backend
type StorageConfig struct {
	// Type is the storage backend type (sqlite, postgres, mysql)
	Type string `mapstructure:"type" json:"type"`

	// SQLite configuration
	SQLite SQLiteConfig `mapstructure:"sqlite" json:"sqlite,omitempty"`

	// PostgreSQL configuration
	PostgreSQL PostgreSQLConfig `mapstructure:"postgres" json:"postgres,omitempty"`

	// MySQL configuration
	MySQL MySQLConfig `mapstructure:"mysql" json:"mysql,omitempty"`

	// LogStorageEnabled is the cluster-wide default for storing logs
	// Per-monitor settings can override this
	LogStorageEnabled bool `mapstructure:"log-storage-enabled" json:"logStorageEnabled"`

	// EventStorageEnabled is the cluster-wide default for storing events
	// Per-monitor settings can override this
	EventStorageEnabled bool `mapstructure:"event-storage-enabled" json:"eventStorageEnabled"`

	// MaxLogSizeKB is the default max log size in KB (default: 100)
	MaxLogSizeKB int `mapstructure:"max-log-size-kb" json:"maxLogSizeKB"`

	// LogRetentionDays is how long to keep logs (default: same as history retention)
	// If 0, uses history-retention.default-days
	LogRetentionDays int `mapstructure:"log-retention-days" json:"logRetentionDays"`
}

// SQLiteConfig configures SQLite storage
type SQLiteConfig struct {
	// Path to database file
	Path string `mapstructure:"path" json:"path"`
}

// PostgreSQLConfig configures PostgreSQL storage
type PostgreSQLConfig struct {
	// Host is the database host
	Host string `mapstructure:"host" json:"host,omitempty"`

	// Port is the database port
	Port int `mapstructure:"port" json:"port,omitempty"`

	// Database name
	Database string `mapstructure:"database" json:"database,omitempty"`

	// Username for authentication
	Username string `mapstructure:"username" json:"username,omitempty"`

	// Password for authentication (omitted from JSON for security)
	Password string `mapstructure:"password" json:"-"`

	// SSLMode for connection
	SSLMode string `mapstructure:"ssl-mode" json:"sslMode,omitempty"`
}

// MySQLConfig configures MySQL/MariaDB storage
type MySQLConfig struct {
	// Host is the database host
	Host string `mapstructure:"host" json:"host,omitempty"`

	// Port is the database port
	Port int `mapstructure:"port" json:"port,omitempty"`

	// Database name
	Database string `mapstructure:"database" json:"database,omitempty"`

	// Username for authentication
	Username string `mapstructure:"username" json:"username,omitempty"`

	// Password for authentication (omitted from JSON for security)
	Password string `mapstructure:"password" json:"-"`
}

// HistoryRetentionConfig configures retention
type HistoryRetentionConfig struct {
	// DefaultDays is default retention period
	DefaultDays int `mapstructure:"default-days" json:"defaultDays"`

	// MaxDays is maximum allowed retention
	MaxDays int `mapstructure:"max-days" json:"maxDays"`
}

// RateLimitsConfig configures global rate limits
type RateLimitsConfig struct {
	// MaxAlertsPerMinute across all channels
	MaxAlertsPerMinute int `mapstructure:"max-alerts-per-minute" json:"maxAlertsPerMinute"`
}

// UIConfig configures the web UI and REST API server
type UIConfig struct {
	// Enabled turns on the UI server (serves both web UI and REST API)
	Enabled bool `mapstructure:"enabled" json:"enabled"`

	// Port for UI server
	Port int `mapstructure:"port" json:"port"`
}

// MetricsConfig configures the metrics server
type MetricsConfig struct {
	// BindAddress is the address to bind to (use 0 to disable)
	BindAddress string `mapstructure:"bind-address"`

	// Secure enables HTTPS for metrics
	Secure bool `mapstructure:"secure"`

	// CertPath is the directory containing TLS certificates
	CertPath string `mapstructure:"cert-path"`

	// CertName is the certificate file name
	CertName string `mapstructure:"cert-name"`

	// CertKey is the key file name
	CertKey string `mapstructure:"cert-key"`
}

// ProbesConfig configures health probes
type ProbesConfig struct {
	// BindAddress is the address for health probes
	BindAddress string `mapstructure:"bind-address"`
}

// LeaderElectionConfig configures leader election
type LeaderElectionConfig struct {
	// Enabled turns on leader election
	Enabled bool `mapstructure:"enabled"`

	// LeaseDuration is the leader lease duration
	LeaseDuration time.Duration `mapstructure:"lease-duration"`

	// RenewDeadline is the leader renew deadline
	RenewDeadline time.Duration `mapstructure:"renew-deadline"`

	// RetryPeriod is the leader retry period
	RetryPeriod time.Duration `mapstructure:"retry-period"`
}

// WebhookConfig configures webhook server TLS
type WebhookConfig struct {
	// CertPath is the directory containing webhook TLS certificates
	CertPath string `mapstructure:"cert-path"`

	// CertName is the certificate file name
	CertName string `mapstructure:"cert-name"`

	// CertKey is the key file name
	CertKey string `mapstructure:"cert-key"`

	// EnableHTTP2 enables HTTP/2 for the webhook server
	EnableHTTP2 bool `mapstructure:"enable-http2"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		LogLevel: "info",
		Scheduler: SchedulerConfig{
			DeadManSwitchInterval:    1 * time.Minute,
			SLARecalculationInterval: 5 * time.Minute,
			PruneInterval:            1 * time.Hour,
		},
		Storage: StorageConfig{
			Type: "sqlite",
			SQLite: SQLiteConfig{
				Path: "/data/guardian.db",
			},
			PostgreSQL: PostgreSQLConfig{
				Port:    5432,
				SSLMode: "require",
			},
			MySQL: MySQLConfig{
				Port: 3306,
			},
			LogStorageEnabled:   false, // Opt-in by default
			EventStorageEnabled: false, // Opt-in by default
			MaxLogSizeKB:        100,   // 100KB default max log size
			LogRetentionDays:    0,     // 0 means use history-retention.default-days
		},
		HistoryRetention: HistoryRetentionConfig{
			DefaultDays: 30,
			MaxDays:     90,
		},
		RateLimits: RateLimitsConfig{
			MaxAlertsPerMinute: 50,
		},
		UI: UIConfig{
			Enabled: true,
			Port:    8080,
		},
		Metrics: MetricsConfig{
			BindAddress: "0",
			Secure:      true,
			CertName:    "tls.crt",
			CertKey:     "tls.key",
		},
		Probes: ProbesConfig{
			BindAddress: ":8081",
		},
		LeaderElection: LeaderElectionConfig{
			Enabled:       false,
			LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second,
			RetryPeriod:   2 * time.Second,
		},
		Webhook: WebhookConfig{
			CertName:    "tls.crt",
			CertKey:     "tls.key",
			EnableHTTP2: false,
		},
	}
}

// BindFlags binds configuration flags to pflags
func BindFlags(flags *pflag.FlagSet) {
	// Top-level
	flags.String("config", "", "Path to config file")
	flags.String("log-level", "info", "Log level (debug, info, warn, error)")

	// Scheduler
	flags.Duration("scheduler.dead-man-switch-interval", 1*time.Minute, "How often to check dead-man's switches")
	flags.Duration("scheduler.sla-recalculation-interval", 5*time.Minute, "How often to recalculate SLA metrics")
	flags.Duration("scheduler.prune-interval", 1*time.Hour, "How often to prune old execution history")

	// Storage
	flags.String("storage.type", "sqlite", "Storage backend type (sqlite, postgres, mysql)")
	flags.String("storage.sqlite.path", "/data/guardian.db", "Path to SQLite database file")
	flags.String("storage.postgres.host", "", "PostgreSQL host")
	flags.Int("storage.postgres.port", 5432, "PostgreSQL port")
	flags.String("storage.postgres.database", "", "PostgreSQL database name")
	flags.String("storage.postgres.username", "", "PostgreSQL username")
	flags.String("storage.postgres.password", "", "PostgreSQL password")
	flags.String("storage.postgres.ssl-mode", "require", "PostgreSQL SSL mode")
	flags.String("storage.mysql.host", "", "MySQL host")
	flags.Int("storage.mysql.port", 3306, "MySQL port")
	flags.String("storage.mysql.database", "", "MySQL database name")
	flags.String("storage.mysql.username", "", "MySQL username")
	flags.String("storage.mysql.password", "", "MySQL password")
	flags.Bool("storage.log-storage-enabled", false, "Enable storing job logs in database (default: false, opt-in)")
	flags.Bool("storage.event-storage-enabled", false, "Enable storing K8s events in database (default: false, opt-in)")
	flags.Int("storage.max-log-size-kb", 100, "Maximum log size to store per execution in KB")
	flags.Int("storage.log-retention-days", 0, "How long to keep logs (0 = use history-retention.default-days)")

	// History retention
	flags.Int("history-retention.default-days", 30, "Default retention period in days")
	flags.Int("history-retention.max-days", 90, "Maximum retention period in days")

	// Rate limits
	flags.Int("rate-limits.max-alerts-per-minute", 50, "Maximum alerts per minute across all channels")

	// UI server (serves both web UI and REST API)
	flags.Bool("ui.enabled", true, "Enable the UI server (serves both web UI and REST API)")
	flags.Int("ui.port", 8080, "UI server port")

	// Metrics
	flags.String("metrics.bind-address", "0", "Metrics endpoint bind address (0 to disable)")
	flags.Bool("metrics.secure", true, "Enable HTTPS for metrics")
	flags.String("metrics.cert-path", "", "Path to metrics TLS certificate directory")
	flags.String("metrics.cert-name", "tls.crt", "Metrics TLS certificate file name")
	flags.String("metrics.cert-key", "tls.key", "Metrics TLS key file name")

	// Probes
	flags.String("probes.bind-address", ":8081", "Health probes bind address")

	// Leader election
	flags.Bool("leader-election.enabled", false, "Enable leader election")
	flags.Duration("leader-election.lease-duration", 15*time.Second, "Leader lease duration")
	flags.Duration("leader-election.renew-deadline", 10*time.Second, "Leader renew deadline")
	flags.Duration("leader-election.retry-period", 2*time.Second, "Leader retry period")

	// Webhook
	flags.String("webhook.cert-path", "", "Path to webhook TLS certificate directory")
	flags.String("webhook.cert-name", "tls.crt", "Webhook TLS certificate file name")
	flags.String("webhook.cert-key", "tls.key", "Webhook TLS key file name")
	flags.Bool("webhook.enable-http2", false, "Enable HTTP/2 for webhook server")
}

// Load loads configuration from flags, environment, and config file
func Load(flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	// Set defaults from DefaultConfig
	defaults := DefaultConfig()
	v.SetDefault("log-level", defaults.LogLevel)
	v.SetDefault("scheduler.dead-man-switch-interval", defaults.Scheduler.DeadManSwitchInterval)
	v.SetDefault("scheduler.sla-recalculation-interval", defaults.Scheduler.SLARecalculationInterval)
	v.SetDefault("scheduler.prune-interval", defaults.Scheduler.PruneInterval)
	v.SetDefault("storage.type", defaults.Storage.Type)
	v.SetDefault("storage.sqlite.path", defaults.Storage.SQLite.Path)
	v.SetDefault("storage.postgres.port", defaults.Storage.PostgreSQL.Port)
	v.SetDefault("storage.postgres.ssl-mode", defaults.Storage.PostgreSQL.SSLMode)
	v.SetDefault("storage.mysql.port", defaults.Storage.MySQL.Port)
	v.SetDefault("storage.log-storage-enabled", defaults.Storage.LogStorageEnabled)
	v.SetDefault("storage.event-storage-enabled", defaults.Storage.EventStorageEnabled)
	v.SetDefault("storage.max-log-size-kb", defaults.Storage.MaxLogSizeKB)
	v.SetDefault("storage.log-retention-days", defaults.Storage.LogRetentionDays)
	v.SetDefault("history-retention.default-days", defaults.HistoryRetention.DefaultDays)
	v.SetDefault("history-retention.max-days", defaults.HistoryRetention.MaxDays)
	v.SetDefault("rate-limits.max-alerts-per-minute", defaults.RateLimits.MaxAlertsPerMinute)
	v.SetDefault("ui.enabled", defaults.UI.Enabled)
	v.SetDefault("ui.port", defaults.UI.Port)
	v.SetDefault("metrics.bind-address", defaults.Metrics.BindAddress)
	v.SetDefault("metrics.secure", defaults.Metrics.Secure)
	v.SetDefault("metrics.cert-name", defaults.Metrics.CertName)
	v.SetDefault("metrics.cert-key", defaults.Metrics.CertKey)
	v.SetDefault("probes.bind-address", defaults.Probes.BindAddress)
	v.SetDefault("leader-election.enabled", defaults.LeaderElection.Enabled)
	v.SetDefault("leader-election.lease-duration", defaults.LeaderElection.LeaseDuration)
	v.SetDefault("leader-election.renew-deadline", defaults.LeaderElection.RenewDeadline)
	v.SetDefault("leader-election.retry-period", defaults.LeaderElection.RetryPeriod)
	v.SetDefault("webhook.cert-name", defaults.Webhook.CertName)
	v.SetDefault("webhook.cert-key", defaults.Webhook.CertKey)
	v.SetDefault("webhook.enable-http2", defaults.Webhook.EnableHTTP2)

	// Bind flags
	if err := v.BindPFlags(flags); err != nil {
		return nil, fmt.Errorf("binding flags: %w", err)
	}

	// Environment variables
	v.SetEnvPrefix("GUARDIAN")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	// Config file
	var configFileUsed string
	if configFile, _ := flags.GetString("config"); configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		configFileUsed = v.ConfigFileUsed()
	} else {
		// Try default locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("/etc/cronjob-guardian")
		v.AddConfigPath(".")
		if err := v.ReadInConfig(); err == nil {
			configFileUsed = v.ConfigFileUsed()
		}
		// Ignore error if no config file found - will use defaults
	}

	// Unmarshal into Config struct
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Store which config file was used (empty string if none)
	cfg.configFileUsed = configFileUsed

	return cfg, nil
}

// ConfigFileUsed returns the path to the config file that was loaded (empty if none)
func (c *Config) ConfigFileUsed() string {
	return c.configFileUsed
}
