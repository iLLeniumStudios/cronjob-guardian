package framework

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// AlertPayload represents an alert received by the mock receiver
type AlertPayload struct {
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	CronJob    string    `json:"cronjob"`
	Namespace  string    `json:"namespace"`
	ReceivedAt time.Time `json:"received_at"`
	RawBody    []byte    `json:"-"`
}

// MockWebhookReceiver is a test HTTP server that receives webhook alerts
type MockWebhookReceiver struct {
	server    *http.Server
	alerts    []AlertPayload
	mu        sync.RWMutex
	port      int
	listening bool
}

// NewMockWebhookReceiver creates a new mock webhook receiver
func NewMockWebhookReceiver(port int) *MockWebhookReceiver {
	return &MockWebhookReceiver{
		port:   port,
		alerts: make([]AlertPayload, 0),
	}
}

// Start starts the mock webhook receiver
func (r *MockWebhookReceiver) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", r.handleWebhook)
	mux.HandleFunc("/health", r.handleHealth)

	r.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", r.port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		r.listening = true
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			r.listening = false
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the mock webhook receiver
func (r *MockWebhookReceiver) Stop() error {
	if r.server != nil {
		return r.server.Close()
	}
	return nil
}

// handleWebhook handles incoming webhook requests
func (r *MockWebhookReceiver) handleWebhook(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer func() { _ = req.Body.Close() }()

	var payload AlertPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		// Store raw body even if parsing fails
		payload = AlertPayload{RawBody: body}
	}
	payload.ReceivedAt = time.Now()
	payload.RawBody = body

	r.mu.Lock()
	r.alerts = append(r.alerts, payload)
	r.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "received"}`))
}

// handleHealth handles health check requests
func (r *MockWebhookReceiver) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}

// GetAlerts returns all received alerts
func (r *MockWebhookReceiver) GetAlerts() []AlertPayload {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]AlertPayload, len(r.alerts))
	copy(result, r.alerts)
	return result
}

// GetAlertCount returns the number of received alerts
func (r *MockWebhookReceiver) GetAlertCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.alerts)
}

// ClearAlerts clears all received alerts
func (r *MockWebhookReceiver) ClearAlerts() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.alerts = make([]AlertPayload, 0)
}

// WaitForAlert waits for at least one alert to be received
func (r *MockWebhookReceiver) WaitForAlert(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.GetAlertCount() > 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// WaitForAlertCount waits for a specific number of alerts
func (r *MockWebhookReceiver) WaitForAlertCount(count int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.GetAlertCount() >= count {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// GetURL returns the webhook URL
func (r *MockWebhookReceiver) GetURL() string {
	return fmt.Sprintf("http://localhost:%d/webhook", r.port)
}
