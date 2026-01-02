package analyzer

import (
	"context"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// scheduleCache caches parsed cron schedules to avoid repeated parsing.
// Uses bounded LRU cache to prevent unbounded memory growth.
var (
	scheduleCache     *lru.Cache[string, cron.Schedule]
	scheduleCacheOnce sync.Once
)

// getScheduleCache returns the schedule cache, initializing it on first use.
func getScheduleCache() *lru.Cache[string, cron.Schedule] {
	scheduleCacheOnce.Do(func() {
		// Max 1000 unique schedules cached - LRU eviction handles bounds
		cache, _ := lru.New[string, cron.Schedule](1000)
		scheduleCache = cache
	})
	return scheduleCache
}

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
	Triggered            bool
	LastSuccess          *time.Time
	ExpectedInterval     time.Duration
	TimeSinceSuccess     time.Duration
	Message              string
	MissedScheduleCount  int32
	ShouldIncrementCount bool
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

	successRate, err := a.store.GetSuccessRate(ctx, cronJob, int(windowDays))
	if err != nil {
		return nil, err
	}

	result := &SLAResult{
		Passed:      true,
		SuccessRate: successRate,
		MinRequired: minSuccessRate,
	}

	if successRate < minSuccessRate {
		result.Passed = false
		result.Violations = append(result.Violations, Violation{
			Type:      "SuccessRate",
			Message:   fmt.Sprintf("Success rate %.1f%% is below %.1f%% threshold", successRate, minSuccessRate),
			Current:   successRate,
			Threshold: minSuccessRate,
		})
	}

	if config.MaxDuration != nil {
		lastExec, err := a.store.GetLastExecution(ctx, cronJob)
		if err == nil && lastExec != nil {
			if lastExec.Duration() > config.MaxDuration.Duration {
				result.Passed = false
				result.Violations = append(result.Violations, Violation{
					Type:      "MaxDuration",
					Message:   fmt.Sprintf("Last duration %s exceeded max %s", lastExec.Duration(), config.MaxDuration.Duration),
					Current:   lastExec.Duration().Seconds(),
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

	cronJobNN := types.NamespacedName{
		Namespace: cronJob.Namespace,
		Name:      cronJob.Name,
	}

	lastExec, err := a.store.GetLastExecution(ctx, cronJobNN)
	if err != nil {
		return nil, fmt.Errorf("failed to get last execution: %w", err)
	}

	lastSuccess, _ := a.store.GetLastSuccessfulExecution(ctx, cronJobNN)

	result := &DeadManResult{}

	if lastSuccess != nil && !lastSuccess.CompletionTime.IsZero() {
		result.LastSuccess = &lastSuccess.CompletionTime
		result.TimeSinceSuccess = time.Since(lastSuccess.CompletionTime)
	}

	var timeSinceLastRun time.Duration
	if lastExec != nil {
		refTime := lastExec.CompletionTime
		if refTime.IsZero() {
			refTime = lastExec.StartTime
		}
		if !refTime.IsZero() {
			timeSinceLastRun = time.Since(refTime)
		} else {
			lastExec = nil
		}
	}

	var expectedInterval time.Duration

	if config.MaxTimeSinceLastSuccess != nil {
		expectedInterval = config.MaxTimeSinceLastSuccess.Duration
	} else if config.AutoFromSchedule != nil && config.AutoFromSchedule.Enabled {
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
		return &DeadManResult{Triggered: false}, nil
	}

	result.ExpectedInterval = expectedInterval

	var missedCount int32
	threshold := int32(1)

	if config.AutoFromSchedule != nil && config.AutoFromSchedule.MissedScheduleThreshold != nil {
		threshold = *config.AutoFromSchedule.MissedScheduleThreshold
	}

	if lastExec == nil {
		if cronJob.CreationTimestamp.Add(expectedInterval).Before(time.Now()) {
			elapsed := time.Since(cronJob.CreationTimestamp.Time)
			missedCount = int32(elapsed / expectedInterval)
			result.ShouldIncrementCount = true
		}
	} else if timeSinceLastRun > expectedInterval {
		missedCount = int32(timeSinceLastRun / expectedInterval)
		result.ShouldIncrementCount = true
	}

	result.MissedScheduleCount = missedCount

	if missedCount >= threshold {
		result.Triggered = true
		if lastExec == nil {
			result.Message = fmt.Sprintf("No jobs have run since creation. Missed %d scheduled run(s) (threshold: %d, expected interval: %s)",
				missedCount, threshold, expectedInterval)
		} else {
			result.Message = fmt.Sprintf("No jobs have run for %s. Missed %d scheduled run(s) (threshold: %d)",
				timeSinceLastRun.Round(time.Minute), missedCount, threshold)
		}
	}

	return result, nil
}

func (a *analyzer) CheckDurationRegression(ctx context.Context, cronJob types.NamespacedName, config *v1alpha1.SLAConfig) (*RegressionResult, error) {
	if config == nil {
		return &RegressionResult{Detected: false}, nil
	}

	threshold := float64(getOrDefaultInt32(config.DurationRegressionThreshold, 50))
	baselineWindowDays := int(getOrDefaultInt32(config.DurationBaselineWindowDays, 14))
	recentWindowDays := 1

	baselineP95, err := a.store.GetDurationPercentile(ctx, cronJob, 95, baselineWindowDays)
	if err != nil {
		return nil, err
	}

	currentP95, err := a.store.GetDurationPercentile(ctx, cronJob, 95, recentWindowDays)
	if err != nil {
		return nil, err
	}

	result := &RegressionResult{
		BaselineP95: baselineP95,
		CurrentP95:  currentP95,
		Threshold:   threshold,
	}

	if baselineP95 == 0 {
		return result, nil
	}

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

// parseScheduleInterval parses a cron schedule and returns the expected interval.
// Uses a bounded LRU cache to avoid repeated parsing of the same schedule string.
func parseScheduleInterval(schedule string) (time.Duration, error) {
	cache := getScheduleCache()

	if sched, ok := cache.Get(schedule); ok {
		now := time.Now()
		next := sched.Next(now)
		nextNext := sched.Next(next)
		return nextNext.Sub(next), nil
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return 0, err
	}

	cache.Add(schedule, sched) // LRU eviction handles bounds

	now := time.Now()
	next := sched.Next(now)
	nextNext := sched.Next(next)

	return nextNext.Sub(next), nil
}

// Helper functions

func isEnabled(b *bool) bool {
	return b == nil || *b
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
