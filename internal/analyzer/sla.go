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

package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// SLAAnalyzer analyzes CronJob SLA compliance
type SLAAnalyzer interface {
	// GetMetrics returns SLA metrics for a CronJob
	GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*v1alpha1.CronJobMetrics, error)

	// CheckSLA checks if SLA thresholds are violated
	CheckSLA(ctx context.Context, cronJob types.NamespacedName, config *v1alpha1.SLAConfig) (*SLAResult, error)

	// CheckDeadManSwitch checks if dead-man's switch should trigger
	CheckDeadManSwitch(ctx context.Context, cronJob *batchv1.CronJob, config *v1alpha1.DeadManSwitchConfig) (*DeadManResult, error)

	// CheckDurationRegression checks for performance regression
	CheckDurationRegression(ctx context.Context, cronJob types.NamespacedName, config *v1alpha1.SLAConfig) (*RegressionResult, error)
}

// SLAResult contains SLA check results
type SLAResult struct {
	Passed      bool
	Violations  []Violation
	SuccessRate float64
	MinRequired float64
}

// Violation describes an SLA violation
type Violation struct {
	Type      string // "SuccessRate", "MaxDuration"
	Message   string
	Current   float64
	Threshold float64
}

// DeadManResult contains dead-man's switch check results
type DeadManResult struct {
	Triggered        bool
	LastSuccess      *time.Time
	ExpectedInterval time.Duration
	TimeSinceSuccess time.Duration
	Message          string
}

// RegressionResult contains regression check results
type RegressionResult struct {
	Detected           bool
	BaselineP95        time.Duration
	CurrentP95         time.Duration
	PercentageIncrease float64
	Threshold          float64
	Message            string
}

type analyzer struct {
	store store.Store
}

// NewSLAAnalyzer creates a new SLA analyzer
func NewSLAAnalyzer(s store.Store) SLAAnalyzer {
	return &analyzer{store: s}
}

func (a *analyzer) GetMetrics(ctx context.Context, cronJob types.NamespacedName, windowDays int) (*v1alpha1.CronJobMetrics, error) {
	metrics, err := a.store.GetMetrics(ctx, cronJob, windowDays)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.CronJobMetrics{
		SuccessRate:        metrics.SuccessRate,
		WindowDays:         metrics.WindowDays,
		TotalRuns:          metrics.TotalRuns,
		SuccessfulRuns:     metrics.SuccessfulRuns,
		FailedRuns:         metrics.FailedRuns,
		AvgDurationSeconds: metrics.AvgDurationSeconds,
		P50DurationSeconds: metrics.P50DurationSeconds,
		P95DurationSeconds: metrics.P95DurationSeconds,
		P99DurationSeconds: metrics.P99DurationSeconds,
	}, nil
}

func (a *analyzer) CheckSLA(ctx context.Context, cronJob types.NamespacedName, config *v1alpha1.SLAConfig) (*SLAResult, error) {
	if config == nil {
		return &SLAResult{Passed: true}, nil
	}

	windowDays := getOrDefaultInt32(config.WindowDays, 7)
	minSuccessRate := getOrDefaultFloat64(config.MinSuccessRate, 95.0)

	// Get current success rate
	successRate, err := a.store.GetSuccessRate(ctx, cronJob, int(windowDays))
	if err != nil {
		return nil, err
	}

	result := &SLAResult{
		Passed:      true,
		SuccessRate: successRate,
		MinRequired: minSuccessRate,
	}

	// Check success rate
	if successRate < minSuccessRate {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Type:      "SuccessRate",
			Message:   fmt.Sprintf("Success rate %.1f%% is below %.1f%% threshold", successRate, minSuccessRate),
			Current:   successRate,
			Threshold: minSuccessRate,
		})
	}

	// Check max duration if configured
	if config.MaxDuration != nil {
		lastExec, err := a.store.GetLastExecution(ctx, cronJob)
		if err == nil && lastExec != nil {
			if lastExec.Duration > config.MaxDuration.Duration {
				result.Passed = false
				result.Violations = append(result.Violations, Violation{
					Type:      "MaxDuration",
					Message:   fmt.Sprintf("Last duration %s exceeded max %s", lastExec.Duration, config.MaxDuration.Duration),
					Current:   lastExec.Duration.Seconds(),
					Threshold: config.MaxDuration.Seconds(),
				})
			}
		}
	}

	return result, nil
}

func (a *analyzer) CheckDeadManSwitch(ctx context.Context, cronJob *batchv1.CronJob, config *v1alpha1.DeadManSwitchConfig) (*DeadManResult, error) {
	if config == nil || !isEnabled(config.Enabled) {
		return &DeadManResult{Triggered: false}, nil
	}

	// Get last successful execution
	lastSuccess, err := a.store.GetLastSuccessfulExecution(ctx, types.NamespacedName{
		Namespace: cronJob.Namespace,
		Name:      cronJob.Name,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get last successful execution: %w", err)
	}

	result := &DeadManResult{}

	if lastSuccess != nil {
		result.LastSuccess = &lastSuccess.CompletionTime
		result.TimeSinceSuccess = time.Since(lastSuccess.CompletionTime)
	}

	// Determine expected interval
	var expectedInterval time.Duration

	if config.MaxTimeSinceLastSuccess != nil {
		// Explicit configuration
		expectedInterval = config.MaxTimeSinceLastSuccess.Duration
	} else if config.AutoFromSchedule != nil && config.AutoFromSchedule.Enabled {
		// Auto-detect from schedule
		interval, err := parseScheduleInterval(cronJob.Spec.Schedule)
		if err != nil {
			return nil, fmt.Errorf("failed to parse schedule: %w", err)
		}
		buffer := 1 * time.Hour
		if config.AutoFromSchedule.Buffer != nil {
			buffer = config.AutoFromSchedule.Buffer.Duration
		}
		expectedInterval = interval + buffer
	} else {
		// No dead-man's switch configured effectively
		return &DeadManResult{Triggered: false}, nil
	}

	result.ExpectedInterval = expectedInterval

	// Check if triggered
	if lastSuccess == nil {
		// Never succeeded - check if CronJob is old enough
		if cronJob.CreationTimestamp.Add(expectedInterval).Before(time.Now()) {
			result.Triggered = true
			result.Message = fmt.Sprintf("No successful runs since creation (expected within %s)", expectedInterval)
		}
	} else if result.TimeSinceSuccess > expectedInterval {
		result.Triggered = true
		result.Message = fmt.Sprintf("No successful run in %s (expected within %s)",
			result.TimeSinceSuccess.Round(time.Minute),
			expectedInterval)
	}

	return result, nil
}

func (a *analyzer) CheckDurationRegression(ctx context.Context, cronJob types.NamespacedName, config *v1alpha1.SLAConfig) (*RegressionResult, error) {
	if config == nil {
		return &RegressionResult{Detected: false}, nil
	}

	threshold := float64(getOrDefaultInt32(config.DurationRegressionThreshold, 50))
	baselineWindowDays := int(getOrDefaultInt32(config.DurationBaselineWindowDays, 14))
	recentWindowDays := 1 // Last 24 hours

	// Get baseline P95
	baselineP95, err := a.store.GetDurationPercentile(ctx, cronJob, 95, baselineWindowDays)
	if err != nil {
		return nil, err
	}

	// Get recent P95
	currentP95, err := a.store.GetDurationPercentile(ctx, cronJob, 95, recentWindowDays)
	if err != nil {
		return nil, err
	}

	result := &RegressionResult{
		BaselineP95: baselineP95,
		CurrentP95:  currentP95,
		Threshold:   threshold,
	}

	// Can't detect regression without baseline
	if baselineP95 == 0 {
		return result, nil
	}

	// Calculate increase
	if currentP95 > baselineP95 {
		increase := (float64(currentP95) - float64(baselineP95)) / float64(baselineP95) * 100
		result.PercentageIncrease = increase

		if increase >= threshold {
			result.Detected = true
			result.Message = fmt.Sprintf("P95 duration increased %.0f%% (from %s to %s)",
				increase, baselineP95, currentP95)
		}
	}

	return result, nil
}

// parseScheduleInterval parses a cron schedule and returns the expected interval
func parseScheduleInterval(schedule string) (time.Duration, error) {
	// Use robfig/cron parser
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return 0, err
	}

	// Calculate interval between two consecutive runs
	now := time.Now()
	next := sched.Next(now)
	nextNext := sched.Next(next)

	return nextNext.Sub(next), nil
}

// Helper functions

func isEnabled(b *bool) bool {
	return b == nil || *b // Default to true if not set
}

func getOrDefaultInt32(val *int32, def int32) int32 {
	if val == nil {
		return def
	}
	return *val
}

func getOrDefaultFloat64(val *float64, def float64) float64 {
	if val == nil {
		return def
	}
	return *val
}
