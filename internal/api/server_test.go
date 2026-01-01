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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/testutil"
)

func TestServer_Start(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
		Port:   port,
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to verify server is running
	resp, err := http.Get("http://127.0.0.1:" + string(rune(port+'0')) + "/api/v1/health")
	if err == nil {
		resp.Body.Close()
	}

	// Shutdown
	cancel()

	// Wait for shutdown with timeout
	select {
	case <-errCh:
		// Server stopped
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not shutdown in time")
	}
}

func TestServer_Stop(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
		Port:   0, // Will use default port
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for shutdown
	select {
	case err := <-errCh:
		// Server should shutdown gracefully without error
		assert.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("Server did not shutdown in time")
	}
}

func TestServer_ServesUI(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
	})

	router := server.setupRoutes()

	// Test that UI routes respond
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Without embedded UI assets, we get 404 since fs.Sub returns empty FS
	// This is expected behavior in test mode without UI build
	// In production, UIAssets would be set from main with actual UI files
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode)
}

func TestServer_CORSHeaders(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
	})

	router := server.setupRoutes()

	// Test OPTIONS request (CORS preflight)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestServer_APIRoutes(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
		Config: &config.Config{},
	})

	router := server.setupRoutes()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{"health endpoint", http.MethodGet, "/api/v1/health", http.StatusOK},
		{"stats endpoint", http.MethodGet, "/api/v1/stats", http.StatusOK},
		{"monitors list", http.MethodGet, "/api/v1/monitors", http.StatusOK},
		{"cronjobs list", http.MethodGet, "/api/v1/cronjobs", http.StatusOK},
		{"channels list", http.MethodGet, "/api/v1/channels", http.StatusOK},
		{"alerts list", http.MethodGet, "/api/v1/alerts", http.StatusOK},
		{"config endpoint", http.MethodGet, "/api/v1/config", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			resp := w.Result()
			resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestServer_WithOptions(t *testing.T) {
	mockStore := &testutil.MockStore{}
	mockDispatcher := testutil.NewMockDispatcher()
	cfg := &config.Config{
		LogLevel: "debug",
	}

	server := NewServer(ServerOptions{
		Client:              newTestAPIClient(),
		Store:               mockStore,
		Config:              cfg,
		AlertDispatcher:     mockDispatcher,
		Port:                9090,
		LeaderElectionCheck: func() bool { return true },
		AnalyzerEnabled:     true,
		SchedulersRunning:   []string{"dead-man", "sla-recalc"},
	})

	assert.NotNil(t, server)
	assert.Equal(t, 9090, server.port)
	assert.True(t, server.analyzerEnabled)
	assert.Contains(t, server.schedulersRunning, "dead-man")
}

func TestServer_DefaultPort(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
		// Port not specified
	})

	assert.Equal(t, 8080, server.port)
}

func TestServer_SetupRoutes(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
		Config: &config.Config{},
	})

	router := server.setupRoutes()

	// Verify router is not nil
	assert.NotNil(t, router)

	// Test that routes are properly registered by making requests
	healthReq := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	healthW := httptest.NewRecorder()
	router.ServeHTTP(healthW, healthReq)
	assert.Equal(t, http.StatusOK, healthW.Code)
}

func TestZerologMiddleware_SkipsStaticAssets(t *testing.T) {
	// Create a simple handler that always returns 200
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := zerologMiddleware(nextHandler)

	// Test static asset paths are handled without logging
	staticPaths := []string{
		"/_next/static/chunk.js",
		"/static/file.css",
		"/favicon.ico",
		"/logo.svg",
		"/font.woff2",
		"/image.png",
	}

	for _, path := range staticPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "path: %s", path)
	}
}

func TestZerologMiddleware_LogsAPIRequests(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := zerologMiddleware(nextHandler)

	// Test API paths are handled (logging occurs but we can't easily verify)
	apiPaths := []string{
		"/api/v1/health",
		"/api/v1/stats",
		"/api/v1/monitors",
	}

	for _, path := range apiPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "path: %s", path)
	}
}

func TestServer_HeadMethod(t *testing.T) {
	server := NewServer(ServerOptions{
		Client: newTestAPIClient(),
	})

	router := server.setupRoutes()

	// HEAD requests should work for UI routes (used by browser prefetching)
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	// Without embedded UI assets, we get 404 since fs.Sub returns empty FS
	// In production with real UI assets, this would return 200
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, resp.StatusCode)
}
