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
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// Default rate limiting values
const (
	DefaultMaxAlertsPerHour = 100
	DefaultBurstLimit       = 10
)

// NewRateLimiter creates a rate limiter from AlertChannel rate limiting config.
func NewRateLimiter(rl *guardianv1alpha1.RateLimitConfig) *rate.Limiter {
	maxPerHour := int32(DefaultMaxAlertsPerHour)
	burst := int32(DefaultBurstLimit)

	if rl != nil {
		if rl.MaxAlertsPerHour != nil {
			maxPerHour = *rl.MaxAlertsPerHour
		}
		if rl.BurstLimit != nil {
			burst = *rl.BurstLimit
		}
	}

	return rate.NewLimiter(rate.Limit(float64(maxPerHour)/3600), int(burst))
}

// AlertHTTPClient is a shared HTTP client with sensible timeouts for alert delivery.
var AlertHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	},
}

// Default retry configuration
const (
	DefaultMaxRetries     = 3
	DefaultInitialBackoff = 1 * time.Second
	DefaultMaxBackoff     = 30 * time.Second
)

// RetryConfig configures retry behavior for HTTP requests
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     DefaultMaxRetries,
		InitialBackoff: DefaultInitialBackoff,
		MaxBackoff:     DefaultMaxBackoff,
	}
}

// SendWithRetry executes an HTTP request with exponential backoff retry.
// It retries on network errors and 5xx status codes.
// The request body is read and buffered to allow retries.
func SendWithRetry(ctx context.Context, req *http.Request, config RetryConfig) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		_ = req.Body.Close()
	}

	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}

			// Double the backoff for next attempt, capped at max
			backoff *= 2
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		}

		// Reset the body for this attempt
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		resp, err := AlertHTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		// Success or client error (4xx) - don't retry
		if resp.StatusCode < 500 {
			return resp, nil
		}

		// Server error (5xx) - retry
		lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
		_ = resp.Body.Close()
	}

	return nil, fmt.Errorf("after %d retries: %w", config.MaxRetries, lastErr)
}
