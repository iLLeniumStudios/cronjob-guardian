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

package main

import (
	"crypto/tls"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/api"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/controller"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/scheduler"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
	// +kubebuilder:scaffold:imports
)

//go:embed all:ui/out
var uiAssets embed.FS

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(guardianv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	// Set up pflags
	flags := pflag.NewFlagSet("cronjob-guardian", pflag.ExitOnError)
	config.BindFlags(flags)

	// Parse flags
	if err := flags.Parse(os.Args[1:]); err != nil {
		setupLog.Error(err, "failed to parse flags")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(flags)
	if err != nil {
		setupLog.Error(err, "failed to load configuration")
		os.Exit(1)
	}

	// Set up zerolog with configured log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	zl := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Logger()
	logger := zerologr.New(&zl)
	ctrl.SetLogger(logger)

	// Re-initialize setupLog with the configured logger
	setupLog = ctrl.Log.WithName("setup")
	if cfg.ConfigFileUsed() != "" {
		setupLog.Info("configuration loaded", "file", cfg.ConfigFileUsed(), "level", cfg.LogLevel)
	} else {
		setupLog.Info("no config file found, using defaults and flags", "level", cfg.LogLevel)
	}

	// Share zerolog with API server for chi middleware
	api.SetLogger(&zl)

	// TLS options
	var tlsOpts []func(*tls.Config)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !cfg.Webhook.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(cfg.Webhook.CertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", cfg.Webhook.CertPath,
			"webhook-cert-name", cfg.Webhook.CertName,
			"webhook-cert-key", cfg.Webhook.CertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(cfg.Webhook.CertPath, cfg.Webhook.CertName),
			filepath.Join(cfg.Webhook.CertPath, cfg.Webhook.CertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   cfg.Metrics.BindAddress,
		SecureServing: cfg.Metrics.Secure,
		TLSOpts:       tlsOpts,
	}

	if cfg.Metrics.Secure {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	if len(cfg.Metrics.CertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", cfg.Metrics.CertPath,
			"metrics-cert-name", cfg.Metrics.CertName,
			"metrics-cert-key", cfg.Metrics.CertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(cfg.Metrics.CertPath, cfg.Metrics.CertName),
			filepath.Join(cfg.Metrics.CertPath, cfg.Metrics.CertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.Probes.BindAddress,
		LeaderElection:         cfg.LeaderElection.Enabled,
		LeaderElectionID:       "59ab3636.illenium.net",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize the storage backend
	var dsn string
	switch cfg.Storage.Type {
	case "sqlite":
		dsn = cfg.Storage.SQLite.Path + "?_journal_mode=WAL&_busy_timeout=5000"
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Storage.PostgreSQL.Host, cfg.Storage.PostgreSQL.Port,
			cfg.Storage.PostgreSQL.Username, cfg.Storage.PostgreSQL.Password,
			cfg.Storage.PostgreSQL.Database, cfg.Storage.PostgreSQL.SSLMode)
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			cfg.Storage.MySQL.Username, cfg.Storage.MySQL.Password,
			cfg.Storage.MySQL.Host, cfg.Storage.MySQL.Port,
			cfg.Storage.MySQL.Database)
	default:
		setupLog.Error(nil, "unsupported storage type", "type", cfg.Storage.Type)
		os.Exit(1)
	}

	dataStore, err := store.NewGormStore(cfg.Storage.Type, dsn)
	if err != nil {
		setupLog.Error(err, "unable to create store")
		os.Exit(1)
	}

	if err := dataStore.Init(); err != nil {
		setupLog.Error(err, "unable to initialize store")
		os.Exit(1)
	}
	defer func() { _ = dataStore.Close() }()
	setupLog.Info("initialized store", "type", cfg.Storage.Type)

	// Initialize SLA analyzer (required for all SLA features)
	slaAnalyzer := analyzer.NewSLAAnalyzer(dataStore)
	setupLog.Info("initialized SLA analyzer")

	// Initialize and add history pruner to manager
	historyPruner := scheduler.NewHistoryPruner(dataStore, cfg.HistoryRetention.DefaultDays)
	historyPruner.SetInterval(cfg.Scheduler.PruneInterval)
	if cfg.Storage.LogRetentionDays > 0 {
		historyPruner.SetLogRetentionDays(cfg.Storage.LogRetentionDays)
	}
	if err := mgr.Add(historyPruner); err != nil {
		setupLog.Error(err, "unable to add history pruner to manager")
		os.Exit(1)
	}
	setupLog.Info("initialized history pruner",
		"retentionDays", cfg.HistoryRetention.DefaultDays,
		"logRetentionDays", cfg.Storage.LogRetentionDays,
		"interval", cfg.Scheduler.PruneInterval)

	// Create clientset for controllers that need raw API access
	clientset, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		setupLog.Error(err, "unable to create clientset")
		os.Exit(1)
	}

	// Create alert dispatcher and wire up the store
	alertDispatcher := alerting.NewDispatcher(mgr.GetClient())
	alertDispatcher.SetStore(dataStore)
	alertDispatcher.SetStartupGracePeriod(cfg.Scheduler.StartupGracePeriod)
	setupLog.Info("initialized alert dispatcher", "startupGracePeriod", cfg.Scheduler.StartupGracePeriod)

	if err := (&controller.CronJobMonitorReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("CronJobMonitor"),
		Scheme:          mgr.GetScheme(),
		Store:           dataStore,
		Config:          cfg,
		Analyzer:        slaAnalyzer,
		AlertDispatcher: alertDispatcher,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CronJobMonitor")
		os.Exit(1)
	}
	if err := (&controller.AlertChannelReconciler{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("AlertChannel"),
		Scheme:          mgr.GetScheme(),
		AlertDispatcher: alertDispatcher,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AlertChannel")
		os.Exit(1)
	}

	// Job handler watches for Job completions to record executions
	if err := (&controller.JobHandler{
		Client:          mgr.GetClient(),
		Log:             ctrl.Log.WithName("controllers").WithName("JobHandler"),
		Scheme:          mgr.GetScheme(),
		Clientset:       clientset,
		Store:           dataStore,
		Config:          cfg,
		AlertDispatcher: alertDispatcher,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "JobHandler")
		os.Exit(1)
	}

	// Create and register DeadManScheduler for periodic dead-man's switch checks
	deadManScheduler := scheduler.NewDeadManScheduler(mgr.GetClient(), slaAnalyzer, alertDispatcher)
	deadManScheduler.SetStartupDelay(cfg.Scheduler.StartupGracePeriod)
	deadManScheduler.SetInterval(cfg.Scheduler.DeadManSwitchInterval)
	if err := mgr.Add(deadManScheduler); err != nil {
		setupLog.Error(err, "unable to add dead-man scheduler")
		os.Exit(1)
	}
	setupLog.Info("initialized dead-man scheduler",
		"interval", cfg.Scheduler.DeadManSwitchInterval,
		"startupDelay", cfg.Scheduler.StartupGracePeriod)

	// Create and register SLARecalcScheduler for periodic SLA recalculation
	slaRecalcScheduler := scheduler.NewSLARecalcScheduler(mgr.GetClient(), dataStore, slaAnalyzer, alertDispatcher)
	if err := mgr.Add(slaRecalcScheduler); err != nil {
		setupLog.Error(err, "unable to add SLA recalc scheduler")
		os.Exit(1)
	}
	setupLog.Info("initialized SLA recalc scheduler", "interval", "5m")

	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Set up UI server with embedded UI assets (serves both web UI and REST API)
	if cfg.UI.Enabled {
		api.UIAssets = uiAssets

		// Create leader election check function
		var leaderElectionCheck func() bool
		if cfg.LeaderElection.Enabled {
			elected := mgr.Elected()
			leaderElectionCheck = func() bool {
				select {
				case <-elected:
					return true
				default:
					return false
				}
			}
		}

		apiServer := api.NewServer(api.ServerOptions{
			Client:              mgr.GetClient(),
			Clientset:           clientset,
			Store:               dataStore,
			Config:              cfg,
			AlertDispatcher:     alertDispatcher,
			Port:                cfg.UI.Port,
			LeaderElectionCheck: leaderElectionCheck,
			AnalyzerEnabled:     true, // Analyzer is always enabled (required dependency)
			SchedulersRunning:   []string{"dead-man-switch", "sla-recalc", "history-pruner"},
		})

		// Add API server to manager
		if err := mgr.Add(apiServer); err != nil {
			setupLog.Error(err, "unable to add API server to manager")
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
