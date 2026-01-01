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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Default Values Tests
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Log level
	assert.Equal(t, "info", cfg.LogLevel)

	// Scheduler defaults
	assert.Equal(t, 1*time.Minute, cfg.Scheduler.DeadManSwitchInterval)
	assert.Equal(t, 5*time.Minute, cfg.Scheduler.SLARecalculationInterval)
	assert.Equal(t, 1*time.Hour, cfg.Scheduler.PruneInterval)
	assert.Equal(t, 30*time.Second, cfg.Scheduler.StartupGracePeriod)

	// Storage defaults
	assert.Equal(t, "sqlite", cfg.Storage.Type)
	assert.Equal(t, "/data/guardian.db", cfg.Storage.SQLite.Path)
	assert.Equal(t, 5432, cfg.Storage.PostgreSQL.Port)
	assert.Equal(t, "require", cfg.Storage.PostgreSQL.SSLMode)
	assert.Equal(t, 3306, cfg.Storage.MySQL.Port)
	assert.False(t, cfg.Storage.LogStorageEnabled)
	assert.False(t, cfg.Storage.EventStorageEnabled)
	assert.Equal(t, 100, cfg.Storage.MaxLogSizeKB)
	assert.Equal(t, 0, cfg.Storage.LogRetentionDays)

	// History retention defaults
	assert.Equal(t, 30, cfg.HistoryRetention.DefaultDays)
	assert.Equal(t, 90, cfg.HistoryRetention.MaxDays)

	// Rate limits defaults
	assert.Equal(t, 50, cfg.RateLimits.MaxAlertsPerMinute)

	// UI defaults
	assert.True(t, cfg.UI.Enabled)
	assert.Equal(t, 8080, cfg.UI.Port)

	// Metrics defaults
	assert.Equal(t, "0", cfg.Metrics.BindAddress)
	assert.True(t, cfg.Metrics.Secure)
	assert.Equal(t, "tls.crt", cfg.Metrics.CertName)
	assert.Equal(t, "tls.key", cfg.Metrics.CertKey)

	// Probes defaults
	assert.Equal(t, ":8081", cfg.Probes.BindAddress)

	// Leader election defaults
	assert.False(t, cfg.LeaderElection.Enabled)
	assert.Equal(t, 15*time.Second, cfg.LeaderElection.LeaseDuration)
	assert.Equal(t, 10*time.Second, cfg.LeaderElection.RenewDeadline)
	assert.Equal(t, 2*time.Second, cfg.LeaderElection.RetryPeriod)

	// Webhook defaults
	assert.Equal(t, "tls.crt", cfg.Webhook.CertName)
	assert.Equal(t, "tls.key", cfg.Webhook.CertKey)
	assert.False(t, cfg.Webhook.EnableHTTP2)
}

func TestLoad_DefaultValues(t *testing.T) {
	// Create empty flags
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	// Load config without any overrides
	cfg, err := Load(flags)
	require.NoError(t, err)

	// Should have defaults
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "sqlite", cfg.Storage.Type)
	assert.Equal(t, 30, cfg.HistoryRetention.DefaultDays)
	assert.True(t, cfg.UI.Enabled)
	assert.Equal(t, "", cfg.ConfigFileUsed())
}

// ============================================================================
// YAML File Loading Tests
// ============================================================================

func TestLoad_YAMLFile(t *testing.T) {
	// Create a temp YAML config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
log-level: debug
scheduler:
  dead-man-switch-interval: 2m
  sla-recalculation-interval: 10m
  prune-interval: 2h
  startup-grace-period: 60s
storage:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    database: guardian
    username: user
    password: secret
    ssl-mode: disable
  log-storage-enabled: true
  event-storage-enabled: true
  max-log-size-kb: 200
history-retention:
  default-days: 60
  max-days: 180
rate-limits:
  max-alerts-per-minute: 100
ui:
  enabled: true
  port: 9090
leader-election:
  enabled: true
  lease-duration: 30s
  renew-deadline: 20s
  retry-period: 5s
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	// Create flags and set config path
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)
	err = flags.Set("config", configPath)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(flags)
	require.NoError(t, err)

	// Verify YAML values are loaded
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 2*time.Minute, cfg.Scheduler.DeadManSwitchInterval)
	assert.Equal(t, 10*time.Minute, cfg.Scheduler.SLARecalculationInterval)
	assert.Equal(t, 2*time.Hour, cfg.Scheduler.PruneInterval)
	assert.Equal(t, 60*time.Second, cfg.Scheduler.StartupGracePeriod)

	assert.Equal(t, "postgres", cfg.Storage.Type)
	assert.Equal(t, "localhost", cfg.Storage.PostgreSQL.Host)
	assert.Equal(t, 5432, cfg.Storage.PostgreSQL.Port)
	assert.Equal(t, "guardian", cfg.Storage.PostgreSQL.Database)
	assert.Equal(t, "user", cfg.Storage.PostgreSQL.Username)
	assert.Equal(t, "secret", cfg.Storage.PostgreSQL.Password)
	assert.Equal(t, "disable", cfg.Storage.PostgreSQL.SSLMode)
	assert.True(t, cfg.Storage.LogStorageEnabled)
	assert.True(t, cfg.Storage.EventStorageEnabled)
	assert.Equal(t, 200, cfg.Storage.MaxLogSizeKB)

	assert.Equal(t, 60, cfg.HistoryRetention.DefaultDays)
	assert.Equal(t, 180, cfg.HistoryRetention.MaxDays)

	assert.Equal(t, 100, cfg.RateLimits.MaxAlertsPerMinute)

	assert.True(t, cfg.UI.Enabled)
	assert.Equal(t, 9090, cfg.UI.Port)

	assert.True(t, cfg.LeaderElection.Enabled)
	assert.Equal(t, 30*time.Second, cfg.LeaderElection.LeaseDuration)
	assert.Equal(t, 20*time.Second, cfg.LeaderElection.RenewDeadline)
	assert.Equal(t, 5*time.Second, cfg.LeaderElection.RetryPeriod)

	// Verify config file path is reported
	assert.Equal(t, configPath, cfg.ConfigFileUsed())
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Create a temp invalid YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidYAML := `
log-level: debug
storage:
  type: [invalid yaml
    - missing bracket
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0600)
	require.NoError(t, err)

	// Create flags and set config path
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)
	err = flags.Set("config", configPath)
	require.NoError(t, err)

	// Load should fail
	_, err = Load(flags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

func TestLoad_NonExistentConfigFile(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)
	err := flags.Set("config", "/nonexistent/path/config.yaml")
	require.NoError(t, err)

	// Load should fail
	_, err = Load(flags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

// ============================================================================
// CLI Flags Override Tests
// ============================================================================

func TestLoad_Flags(t *testing.T) {
	// Create a YAML with some values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
log-level: info
storage:
  type: sqlite
ui:
  port: 8080
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	// Create flags
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	// Set config file and override some values via flags
	err = flags.Set("config", configPath)
	require.NoError(t, err)
	err = flags.Set("log-level", "debug")
	require.NoError(t, err)
	err = flags.Set("ui.port", "9999")
	require.NoError(t, err)
	err = flags.Set("storage.type", "postgres")
	require.NoError(t, err)

	// Load config
	cfg, err := Load(flags)
	require.NoError(t, err)

	// Flags should override YAML values
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 9999, cfg.UI.Port)
	assert.Equal(t, "postgres", cfg.Storage.Type)
}

func TestLoad_Flags_AllSchedulerOptions(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	err := flags.Set("scheduler.dead-man-switch-interval", "3m")
	require.NoError(t, err)
	err = flags.Set("scheduler.sla-recalculation-interval", "15m")
	require.NoError(t, err)
	err = flags.Set("scheduler.prune-interval", "4h")
	require.NoError(t, err)
	err = flags.Set("scheduler.startup-grace-period", "2m")
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	assert.Equal(t, 3*time.Minute, cfg.Scheduler.DeadManSwitchInterval)
	assert.Equal(t, 15*time.Minute, cfg.Scheduler.SLARecalculationInterval)
	assert.Equal(t, 4*time.Hour, cfg.Scheduler.PruneInterval)
	assert.Equal(t, 2*time.Minute, cfg.Scheduler.StartupGracePeriod)
}

func TestLoad_Flags_AllStorageOptions(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	err := flags.Set("storage.type", "mysql")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.host", "mysql.local")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.port", "3307")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.database", "guardian_db")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.username", "admin")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.password", "secret123")
	require.NoError(t, err)
	err = flags.Set("storage.log-storage-enabled", "true")
	require.NoError(t, err)
	err = flags.Set("storage.event-storage-enabled", "true")
	require.NoError(t, err)
	err = flags.Set("storage.max-log-size-kb", "500")
	require.NoError(t, err)
	err = flags.Set("storage.log-retention-days", "14")
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	assert.Equal(t, "mysql", cfg.Storage.Type)
	assert.Equal(t, "mysql.local", cfg.Storage.MySQL.Host)
	assert.Equal(t, 3307, cfg.Storage.MySQL.Port)
	assert.Equal(t, "guardian_db", cfg.Storage.MySQL.Database)
	assert.Equal(t, "admin", cfg.Storage.MySQL.Username)
	assert.Equal(t, "secret123", cfg.Storage.MySQL.Password)
	assert.True(t, cfg.Storage.LogStorageEnabled)
	assert.True(t, cfg.Storage.EventStorageEnabled)
	assert.Equal(t, 500, cfg.Storage.MaxLogSizeKB)
	assert.Equal(t, 14, cfg.Storage.LogRetentionDays)
}

func TestLoad_Flags_LeaderElection(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	err := flags.Set("leader-election.enabled", "true")
	require.NoError(t, err)
	err = flags.Set("leader-election.lease-duration", "30s")
	require.NoError(t, err)
	err = flags.Set("leader-election.renew-deadline", "25s")
	require.NoError(t, err)
	err = flags.Set("leader-election.retry-period", "5s")
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	assert.True(t, cfg.LeaderElection.Enabled)
	assert.Equal(t, 30*time.Second, cfg.LeaderElection.LeaseDuration)
	assert.Equal(t, 25*time.Second, cfg.LeaderElection.RenewDeadline)
	assert.Equal(t, 5*time.Second, cfg.LeaderElection.RetryPeriod)
}

// ============================================================================
// Environment Variable Tests
// ============================================================================

func TestLoad_Environment(t *testing.T) {
	// Set environment variables using t.Setenv which cleans up automatically
	t.Setenv("GUARDIAN_LOG_LEVEL", "warn")
	t.Setenv("GUARDIAN_STORAGE_TYPE", "postgres")
	t.Setenv("GUARDIAN_STORAGE_POSTGRES_HOST", "pg.example.com")
	t.Setenv("GUARDIAN_UI_PORT", "8888")
	t.Setenv("GUARDIAN_HISTORY_RETENTION_DEFAULT_DAYS", "45")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	cfg, err := Load(flags)
	require.NoError(t, err)

	// Environment variables should be respected
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, "postgres", cfg.Storage.Type)
	assert.Equal(t, "pg.example.com", cfg.Storage.PostgreSQL.Host)
	assert.Equal(t, 8888, cfg.UI.Port)
	assert.Equal(t, 45, cfg.HistoryRetention.DefaultDays)
}

func TestLoad_Environment_OverridesYAML(t *testing.T) {
	// Create a YAML with some values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
log-level: info
storage:
  type: sqlite
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	// Set environment to override
	t.Setenv("GUARDIAN_LOG_LEVEL", "error")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)
	err = flags.Set("config", configPath)
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	// Environment should override YAML
	assert.Equal(t, "error", cfg.LogLevel)
	// But YAML value for storage type should remain
	assert.Equal(t, "sqlite", cfg.Storage.Type)
}

// ============================================================================
// Storage Type Tests
// ============================================================================

func TestLoad_StorageTypes(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
	}{
		{"sqlite", "sqlite"},
		{"postgres", "postgres"},
		{"mysql", "mysql"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			BindFlags(flags)
			err := flags.Set("storage.type", tt.storageType)
			require.NoError(t, err)

			cfg, err := Load(flags)
			require.NoError(t, err)
			assert.Equal(t, tt.storageType, cfg.Storage.Type)
		})
	}
}

func TestLoad_StorageTypes_SQLite(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	err := flags.Set("storage.type", "sqlite")
	require.NoError(t, err)
	err = flags.Set("storage.sqlite.path", "/custom/path/guardian.db")
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	assert.Equal(t, "sqlite", cfg.Storage.Type)
	assert.Equal(t, "/custom/path/guardian.db", cfg.Storage.SQLite.Path)
}

func TestLoad_StorageTypes_PostgreSQL(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	err := flags.Set("storage.type", "postgres")
	require.NoError(t, err)
	err = flags.Set("storage.postgres.host", "pg.cluster.local")
	require.NoError(t, err)
	err = flags.Set("storage.postgres.port", "5433")
	require.NoError(t, err)
	err = flags.Set("storage.postgres.database", "cronjob_guardian")
	require.NoError(t, err)
	err = flags.Set("storage.postgres.username", "guardian_user")
	require.NoError(t, err)
	err = flags.Set("storage.postgres.password", "guardian_pass")
	require.NoError(t, err)
	err = flags.Set("storage.postgres.ssl-mode", "verify-full")
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	assert.Equal(t, "postgres", cfg.Storage.Type)
	assert.Equal(t, "pg.cluster.local", cfg.Storage.PostgreSQL.Host)
	assert.Equal(t, 5433, cfg.Storage.PostgreSQL.Port)
	assert.Equal(t, "cronjob_guardian", cfg.Storage.PostgreSQL.Database)
	assert.Equal(t, "guardian_user", cfg.Storage.PostgreSQL.Username)
	assert.Equal(t, "guardian_pass", cfg.Storage.PostgreSQL.Password)
	assert.Equal(t, "verify-full", cfg.Storage.PostgreSQL.SSLMode)
}

func TestLoad_StorageTypes_MySQL(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	err := flags.Set("storage.type", "mysql")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.host", "mysql.cluster.local")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.port", "3307")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.database", "guardian_db")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.username", "mysql_user")
	require.NoError(t, err)
	err = flags.Set("storage.mysql.password", "mysql_pass")
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	assert.Equal(t, "mysql", cfg.Storage.Type)
	assert.Equal(t, "mysql.cluster.local", cfg.Storage.MySQL.Host)
	assert.Equal(t, 3307, cfg.Storage.MySQL.Port)
	assert.Equal(t, "guardian_db", cfg.Storage.MySQL.Database)
	assert.Equal(t, "mysql_user", cfg.Storage.MySQL.Username)
	assert.Equal(t, "mysql_pass", cfg.Storage.MySQL.Password)
}

// ============================================================================
// Log Level Tests
// ============================================================================

func TestLoad_LogLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			BindFlags(flags)
			err := flags.Set("log-level", level)
			require.NoError(t, err)

			cfg, err := Load(flags)
			require.NoError(t, err)
			assert.Equal(t, level, cfg.LogLevel)
		})
	}
}

// ============================================================================
// Config File Used Tests
// ============================================================================

func TestConfigFileUsed(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "guardian-config.yaml")

	yamlContent := `log-level: debug`
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)
	err = flags.Set("config", configPath)
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	// Should report the config file path
	assert.Equal(t, configPath, cfg.ConfigFileUsed())
}

func TestConfigFileUsed_NoFile(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	cfg, err := Load(flags)
	require.NoError(t, err)

	// Should be empty when no config file is used
	assert.Equal(t, "", cfg.ConfigFileUsed())
}

// ============================================================================
// BindFlags Tests
// ============================================================================

func TestBindFlags_AllFlagsRegistered(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)

	// Check all expected flags exist
	expectedFlags := []string{
		"config",
		"log-level",
		"scheduler.dead-man-switch-interval",
		"scheduler.sla-recalculation-interval",
		"scheduler.prune-interval",
		"scheduler.startup-grace-period",
		"storage.type",
		"storage.sqlite.path",
		"storage.postgres.host",
		"storage.postgres.port",
		"storage.postgres.database",
		"storage.postgres.username",
		"storage.postgres.password",
		"storage.postgres.ssl-mode",
		"storage.mysql.host",
		"storage.mysql.port",
		"storage.mysql.database",
		"storage.mysql.username",
		"storage.mysql.password",
		"storage.log-storage-enabled",
		"storage.event-storage-enabled",
		"storage.max-log-size-kb",
		"storage.log-retention-days",
		"history-retention.default-days",
		"history-retention.max-days",
		"rate-limits.max-alerts-per-minute",
		"ui.enabled",
		"ui.port",
		"metrics.bind-address",
		"metrics.secure",
		"metrics.cert-path",
		"metrics.cert-name",
		"metrics.cert-key",
		"probes.bind-address",
		"leader-election.enabled",
		"leader-election.lease-duration",
		"leader-election.renew-deadline",
		"leader-election.retry-period",
		"webhook.cert-path",
		"webhook.cert-name",
		"webhook.cert-key",
		"webhook.enable-http2",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		assert.NotNil(t, flag, "Flag %s should be registered", flagName)
	}
}

// ============================================================================
// Complex Configuration Tests
// ============================================================================

func TestLoad_CompleteConfiguration(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
log-level: debug
scheduler:
  dead-man-switch-interval: 30s
  sla-recalculation-interval: 1m
  prune-interval: 30m
  startup-grace-period: 45s
storage:
  type: postgres
  sqlite:
    path: /tmp/test.db
  postgres:
    host: db.example.com
    port: 5432
    database: guardian
    username: guardian
    password: secret
    ssl-mode: require
  mysql:
    host: mysql.example.com
    port: 3306
    database: guardian
    username: root
    password: root
  log-storage-enabled: true
  event-storage-enabled: true
  max-log-size-kb: 256
  log-retention-days: 7
history-retention:
  default-days: 14
  max-days: 60
rate-limits:
  max-alerts-per-minute: 25
ui:
  enabled: true
  port: 3000
metrics:
  bind-address: ":9090"
  secure: false
  cert-path: /certs
  cert-name: metrics.crt
  cert-key: metrics.key
probes:
  bind-address: ":8082"
leader-election:
  enabled: true
  lease-duration: 20s
  renew-deadline: 15s
  retry-period: 3s
webhook:
  cert-path: /webhook-certs
  cert-name: webhook.crt
  cert-key: webhook.key
  enable-http2: true
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0600)
	require.NoError(t, err)

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	BindFlags(flags)
	err = flags.Set("config", configPath)
	require.NoError(t, err)

	cfg, err := Load(flags)
	require.NoError(t, err)

	// Verify all values
	assert.Equal(t, "debug", cfg.LogLevel)

	// Scheduler
	assert.Equal(t, 30*time.Second, cfg.Scheduler.DeadManSwitchInterval)
	assert.Equal(t, 1*time.Minute, cfg.Scheduler.SLARecalculationInterval)
	assert.Equal(t, 30*time.Minute, cfg.Scheduler.PruneInterval)
	assert.Equal(t, 45*time.Second, cfg.Scheduler.StartupGracePeriod)

	// Storage
	assert.Equal(t, "postgres", cfg.Storage.Type)
	assert.Equal(t, "db.example.com", cfg.Storage.PostgreSQL.Host)
	assert.Equal(t, 5432, cfg.Storage.PostgreSQL.Port)
	assert.Equal(t, "guardian", cfg.Storage.PostgreSQL.Database)
	assert.Equal(t, "guardian", cfg.Storage.PostgreSQL.Username)
	assert.Equal(t, "secret", cfg.Storage.PostgreSQL.Password)
	assert.Equal(t, "require", cfg.Storage.PostgreSQL.SSLMode)
	assert.True(t, cfg.Storage.LogStorageEnabled)
	assert.True(t, cfg.Storage.EventStorageEnabled)
	assert.Equal(t, 256, cfg.Storage.MaxLogSizeKB)
	assert.Equal(t, 7, cfg.Storage.LogRetentionDays)

	// History retention
	assert.Equal(t, 14, cfg.HistoryRetention.DefaultDays)
	assert.Equal(t, 60, cfg.HistoryRetention.MaxDays)

	// Rate limits
	assert.Equal(t, 25, cfg.RateLimits.MaxAlertsPerMinute)

	// UI
	assert.True(t, cfg.UI.Enabled)
	assert.Equal(t, 3000, cfg.UI.Port)

	// Metrics
	assert.Equal(t, ":9090", cfg.Metrics.BindAddress)
	assert.False(t, cfg.Metrics.Secure)
	assert.Equal(t, "/certs", cfg.Metrics.CertPath)
	assert.Equal(t, "metrics.crt", cfg.Metrics.CertName)
	assert.Equal(t, "metrics.key", cfg.Metrics.CertKey)

	// Probes
	assert.Equal(t, ":8082", cfg.Probes.BindAddress)

	// Leader election
	assert.True(t, cfg.LeaderElection.Enabled)
	assert.Equal(t, 20*time.Second, cfg.LeaderElection.LeaseDuration)
	assert.Equal(t, 15*time.Second, cfg.LeaderElection.RenewDeadline)
	assert.Equal(t, 3*time.Second, cfg.LeaderElection.RetryPeriod)

	// Webhook
	assert.Equal(t, "/webhook-certs", cfg.Webhook.CertPath)
	assert.Equal(t, "webhook.crt", cfg.Webhook.CertName)
	assert.Equal(t, "webhook.key", cfg.Webhook.CertKey)
	assert.True(t, cfg.Webhook.EnableHTTP2)
}
