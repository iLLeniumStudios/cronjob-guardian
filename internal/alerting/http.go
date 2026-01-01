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
