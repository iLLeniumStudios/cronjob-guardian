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

package api

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/remediation"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// Version is the operator version (set at build time)
var Version = "dev"

// UIAssets holds the embedded UI files (set from main)
var UIAssets embed.FS

// Server is the REST API server
type Server struct {
	client            client.Client
	clientset         *kubernetes.Clientset
	store             store.Store
	alertDispatcher   alerting.Dispatcher
	remediationEngine remediation.Engine
	startTime         time.Time
	port              int
	server            *http.Server
}

// ServerOptions contains options for creating the server
type ServerOptions struct {
	Client            client.Client
	Clientset         *kubernetes.Clientset
	Store             store.Store
	AlertDispatcher   alerting.Dispatcher
	RemediationEngine remediation.Engine
	Port              int
}

// NewServer creates a new API server
func NewServer(opts ServerOptions) *Server {
	if opts.Port == 0 {
		opts.Port = 8080
	}

	return &Server{
		client:            opts.Client,
		clientset:         opts.Clientset,
		store:             opts.Store,
		alertDispatcher:   opts.AlertDispatcher,
		remediationEngine: opts.RemediationEngine,
		startTime:         time.Now(),
		port:              opts.Port,
	}
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
	logger := ctrl.Log.WithName("api-server")

	router := s.setupRoutes()

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting API server", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "API server error")
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	logger.Info("shutting down API server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

// setupRoutes configures the router
func (s *Server) setupRoutes() chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS for UI
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Create handlers
	h := NewHandlers(s.client, s.clientset, s.store, s.alertDispatcher, s.remediationEngine, s.startTime)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Health
		r.Get("/health", h.GetHealth)
		r.Get("/stats", h.GetStats)

		// Monitors
		r.Get("/monitors", h.ListMonitors)
		r.Get("/monitors/{namespace}/{name}", h.GetMonitor)

		// CronJobs
		r.Get("/cronjobs", h.ListCronJobs)
		r.Get("/cronjobs/{namespace}/{name}", h.GetCronJob)
		r.Get("/cronjobs/{namespace}/{name}/executions", h.GetExecutions)
		r.Get("/cronjobs/{namespace}/{name}/executions/{jobName}/logs", h.GetLogs)
		r.Post("/cronjobs/{namespace}/{name}/trigger", h.TriggerCronJob)
		r.Post("/cronjobs/{namespace}/{name}/suspend", h.SuspendCronJob)
		r.Post("/cronjobs/{namespace}/{name}/resume", h.ResumeCronJob)

		// Alerts
		r.Get("/alerts", h.ListAlerts)
		r.Get("/alerts/history", h.GetAlertHistory)

		// Channels
		r.Get("/channels", h.ListChannels)
		r.Get("/channels/{name}", h.GetChannel)
		r.Post("/channels/{name}/test", h.TestChannel)

		// Config
		r.Get("/config", h.GetConfig)
	})

	// Serve UI
	s.serveUI(r)

	return r
}

// serveUI serves the embedded UI files
func (s *Server) serveUI(r chi.Router) {
	// Try to serve embedded UI files
	uiFS, err := fs.Sub(UIAssets, "ui/dist")
	if err != nil {
		// No embedded UI, serve a simple message
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>CronJob Guardian</title></head>
<body>
<h1>CronJob Guardian</h1>
<p>API available at <a href="/api/v1/health">/api/v1/health</a></p>
</body>
</html>`))
		})
		return
	}

	// Serve static files
	fileServer := http.FileServer(http.FS(uiFS))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		f, err := uiFS.Open(path[1:]) // Remove leading /
		if err != nil {
			// Serve index.html for SPA routing
			f, err = uiFS.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = "/index.html"
		}
		_ = f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
