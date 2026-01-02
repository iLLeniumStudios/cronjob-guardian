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
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// ==================== SendWithRetry Tests ====================

func TestSendWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "should only make one request on success")
}

func TestSendWithRetry_RetriesOn5xx(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&requestCount), "should retry until success")
}

func TestSendWithRetry_DoesNotRetryOn4xx(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "should not retry 4xx errors")
}

func TestSendWithRetry_ExhaustsRetries(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "after 3 retries")
	assert.Equal(t, int32(4), atomic.LoadInt32(&requestCount), "should make initial + 3 retry attempts")
}

func TestSendWithRetry_RespectsContextCancellation(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     10,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	resp, err := SendWithRetry(ctx, req, config)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.LessOrEqual(t, atomic.LoadInt32(&requestCount), int32(2), "should stop retrying after context cancellation")
}

func TestSendWithRetry_PreservesRequestBody(t *testing.T) {
	var receivedBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))
		if len(receivedBodies) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	expectedBody := "test-body-content"
	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte(expectedBody)))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Len(t, receivedBodies, 3)
	for i, body := range receivedBodies {
		assert.Equal(t, expectedBody, body, "body should be preserved on attempt %d", i+1)
	}
}

func TestSendWithRetry_ExponentialBackoff(t *testing.T) {
	var requestTimes []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		if len(requestTimes) < 4 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 20 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Len(t, requestTimes, 4)

	// Check that delays are approximately exponential
	// First retry: ~20ms, Second: ~40ms, Third: ~80ms
	gap1 := requestTimes[1].Sub(requestTimes[0])
	gap2 := requestTimes[2].Sub(requestTimes[1])
	gap3 := requestTimes[3].Sub(requestTimes[2])

	// Allow some tolerance for timing
	assert.GreaterOrEqual(t, gap1, 15*time.Millisecond, "first backoff should be ~20ms")
	assert.GreaterOrEqual(t, gap2, 30*time.Millisecond, "second backoff should be ~40ms")
	assert.GreaterOrEqual(t, gap3, 60*time.Millisecond, "third backoff should be ~80ms")

	// Each gap should be roughly double the previous (with tolerance)
	assert.Greater(t, gap2, gap1, "backoff should increase")
	assert.Greater(t, gap3, gap2, "backoff should increase")
}

func TestSendWithRetry_BackoffCappedAtMax(t *testing.T) {
	var requestTimes []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		if len(requestTimes) < 5 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     30 * time.Millisecond, // Low cap
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Len(t, requestTimes, 5)

	// Later gaps should be capped at MaxBackoff
	gap4 := requestTimes[4].Sub(requestTimes[3])

	// Allow tolerance but should not exceed max by much
	assert.LessOrEqual(t, gap4, 50*time.Millisecond, "backoff should be capped at MaxBackoff")
}

func TestSendWithRetry_NilBody(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		body, _ := io.ReadAll(r.Body)
		assert.Empty(t, body, "body should be empty")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount))
}

func TestSendWithRetry_ZeroRetries(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	config := RetryConfig{
		MaxRetries:     0, // No retries
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}

	resp, err := SendWithRetry(context.Background(), req, config)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "after 0 retries")
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "should make only one attempt with 0 retries")
}

// ==================== DefaultRetryConfig Tests ====================

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultInitialBackoff, config.InitialBackoff)
	assert.Equal(t, DefaultMaxBackoff, config.MaxBackoff)
}

// ==================== NewRateLimiter Tests ====================

func TestNewRateLimiter_NilConfig(t *testing.T) {
	limiter := NewRateLimiter(nil)
	assert.NotNil(t, limiter)

	// Should use defaults - allow burst of DefaultBurstLimit
	for i := 0; i < DefaultBurstLimit; i++ {
		assert.True(t, limiter.Allow(), "should allow up to burst limit")
	}
}

func TestNewRateLimiter_CustomConfig(t *testing.T) {
	maxPerHour := int32(3600) // 1 per second
	burst := int32(5)

	config := &guardianv1alpha1.RateLimitConfig{
		MaxAlertsPerHour: &maxPerHour,
		BurstLimit:       &burst,
	}

	limiter := NewRateLimiter(config)
	assert.NotNil(t, limiter)

	// Should allow burst
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow())
	}

	// Next should fail (burst exhausted)
	assert.False(t, limiter.Allow())
}
