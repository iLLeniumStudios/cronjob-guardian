// @title           CronJob Guardian API
// @version         1.0
// @description     Kubernetes CronJob monitoring and alerting API. Provides SLA tracking, dead-man switches, and intelligent alerting for CronJobs.

// @contact.name   CronJob Guardian
// @contact.url    https://github.com/iLLeniumStudios/cronjob-guardian

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @schemes http https

package api

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// Version is the operator version (set at build time)
var Version = "dev"

// UIAssets holds the embedded UI files (set from main)
var UIAssets embed.FS

// Server is the REST API server
type Server struct {
	client              client.Client
	clientset           *kubernetes.Clientset
	store               store.Store
	config              *config.Config
	alertDispatcher     alerting.Dispatcher
	startTime           time.Time
	port                int
	server              *http.Server
	leaderElectionCheck func() bool
	analyzerEnabled     bool
	schedulersRunning   []string
	log                 logr.Logger
}

// ServerOptions contains options for creating the server
type ServerOptions struct {
	Client              client.Client
	Clientset           *kubernetes.Clientset
	Store               store.Store
	Config              *config.Config
	AlertDispatcher     alerting.Dispatcher
	Port                int
	LeaderElectionCheck func() bool
	AnalyzerEnabled     bool
	SchedulersRunning   []string
}

// NewServer creates a new API server
func NewServer(opts ServerOptions) *Server {
	if opts.Port == 0 {
		opts.Port = 8080
	}

	return &Server{
		client:              opts.Client,
		clientset:           opts.Clientset,
		store:               opts.Store,
		config:              opts.Config,
		alertDispatcher:     opts.AlertDispatcher,
		startTime:           time.Now(),
		port:                opts.Port,
		leaderElectionCheck: opts.LeaderElectionCheck,
		analyzerEnabled:     opts.AnalyzerEnabled,
		schedulersRunning:   opts.SchedulersRunning,
		log:                 ctrl.Log.WithName("api-server"),
	}
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
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
		s.log.Info("starting API server", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error(err, "API server error")
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	s.log.Info("shutting down API server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.server.Shutdown(shutdownCtx)
}

// requestLoggerMiddleware returns a chi middleware that logs HTTP requests
func (s *Server) requestLoggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for static assets (UI files)
			if strings.HasPrefix(r.URL.Path, "/_next/") ||
				strings.HasSuffix(r.URL.Path, ".js") ||
				strings.HasSuffix(r.URL.Path, ".css") ||
				strings.HasSuffix(r.URL.Path, ".svg") ||
				strings.HasSuffix(r.URL.Path, ".ico") ||
				strings.HasSuffix(r.URL.Path, ".woff2") ||
				strings.HasSuffix(r.URL.Path, ".woff") ||
				strings.HasSuffix(r.URL.Path, ".png") ||
				strings.HasSuffix(r.URL.Path, ".jpg") {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				s.log.V(1).Info("http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration", time.Since(start).String(),
					"remote", r.RemoteAddr)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// setupRoutes configures the router
func (s *Server) setupRoutes() chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.requestLoggerMiddleware())

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
	h := NewHandlers(s.client, s.clientset, s.store, s.config, s.alertDispatcher, s.startTime, s.leaderElectionCheck)
	h.SetAnalyzerEnabled(s.analyzerEnabled)
	h.SetSchedulersRunning(s.schedulersRunning)

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
		r.Get("/cronjobs/{namespace}/{name}/executions/{jobName}", h.GetExecutionWithLogs)
		r.Get("/cronjobs/{namespace}/{name}/executions/{jobName}/logs", h.GetLogs)
		r.Delete("/cronjobs/{namespace}/{name}/history", h.DeleteCronJobHistory)
		r.Post("/cronjobs/{namespace}/{name}/trigger", h.TriggerCronJob)
		r.Post("/cronjobs/{namespace}/{name}/suspend", h.SuspendCronJob)
		r.Post("/cronjobs/{namespace}/{name}/resume", h.ResumeCronJob)

		// Alerts
		r.Get("/alerts", h.ListAlerts)
		r.Get("/alerts/history", h.GetAlertHistory)

		// Patterns
		r.Post("/patterns/test", h.TestPattern)

		// Channels
		r.Get("/channels", h.ListChannels)
		r.Get("/channels/{name}", h.GetChannel)
		r.Post("/channels/{name}/test", h.TestChannel)

		// Config
		r.Get("/config", h.GetConfig)

		// Admin endpoints
		r.Route("/admin", func(r chi.Router) {
			r.Get("/storage-stats", h.GetStorageStats)
			r.Post("/prune", h.TriggerPrune)
		})
	})

	// Serve UI
	s.serveUI(r)

	return r
}

// serveUI serves the embedded UI files
func (s *Server) serveUI(r chi.Router) {
	// Try to serve embedded UI files
	uiFS, err := fs.Sub(UIAssets, "ui/out")
	if err != nil {
		// No embedded UI, serve a simple message
		fallbackHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>CronJob Guardian</title></head>
<body>
<h1>CronJob Guardian</h1>
<p>API available at <a href="/api/v1/health">/api/v1/health</a></p>
</body>
</html>`))
		}
		r.Get("/*", fallbackHandler)
		r.Head("/*", fallbackHandler)
		return
	}

	// Serve static files
	fileServer := http.FileServer(http.FS(uiFS))
	uiHandler := func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if the file exists directly
		f, err := uiFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, req)
			return
		}

		// Try adding /index.html for directories (e.g., /cronjob -> /cronjob/index.html)
		if !strings.HasSuffix(path, "/") {
			indexPath := strings.TrimPrefix(path+"/index.html", "/")
			f, err = uiFS.Open(indexPath)
			if err == nil {
				_ = f.Close()
				req.URL.Path = path + "/index.html"
				fileServer.ServeHTTP(w, req)
				return
			}
		}

		// For dynamic routes like /cronjob/namespace/name, serve the base page
		// This handles SPA-style client-side routing
		// We directly serve the file content to avoid http.FileServer redirect behavior
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) > 0 {
			// Try to find a matching page for the base route
			basePath := parts[0] + "/index.html"
			if content, err := fs.ReadFile(uiFS, basePath); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(content)
				return
			}
		}

		// Fallback: serve root index.html directly
		if content, err := fs.ReadFile(uiFS, "index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(content)
			return
		}

		http.NotFound(w, req)
	}

	// Register handler for both GET and HEAD methods
	// HEAD is used by browsers/Next.js for link prefetching
	r.Get("/*", uiHandler)
	r.Head("/*", uiHandler)
}
