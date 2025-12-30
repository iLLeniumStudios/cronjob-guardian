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

package remediation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"golang.org/x/time/rate"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
)

// Engine handles auto-remediation actions
type Engine interface {
	// KillStuckJob terminates a job that has been running too long
	KillStuckJob(ctx context.Context, job *batchv1.Job, killCfg *v1alpha1.KillStuckJobsConfig) (*RemediationResult, error)

	// TryRetry attempts to retry a failed job
	TryRetry(ctx context.Context, monitor *v1alpha1.CronJobMonitor, failedJob *batchv1.Job, cronJobName string) (*RemediationResult, error)

	// CanRemediate checks if remediation is allowed
	CanRemediate(ctx context.Context, monitor *v1alpha1.CronJobMonitor, action string) (bool, string)

	// SetGlobalRateLimits updates global rate limits
	SetGlobalRateLimits(limits config.RateLimitsConfig)

	// GetRemediationCount24h returns remediations in last 24h
	GetRemediationCount24h() int32

	// ResetRetryCount resets the retry counter for a CronJob (called when job succeeds)
	ResetRetryCount(namespace, cronJobName string)
}

// RemediationResult describes the outcome of a remediation action
type RemediationResult struct {
	Action    string // "KillStuckJob", "RetryJob"
	Success   bool
	Message   string
	JobName   string // Name of affected/created job
	Timestamp time.Time
	DryRun    bool
}

type engine struct {
	client       client.Client
	rateLimiter  *rate.Limiter
	retryTracker map[string]int // cronJob -> retry count
	count24h     int32
	mu           sync.Mutex
}

// NewEngine creates a new remediation engine
func NewEngine(c client.Client) Engine {
	return &engine{
		client:       c,
		rateLimiter:  rate.NewLimiter(rate.Limit(100.0/3600), 10), // 100/hour, burst 10
		retryTracker: make(map[string]int),
	}
}

func (e *engine) KillStuckJob(ctx context.Context, job *batchv1.Job, killCfg *v1alpha1.KillStuckJobsConfig) (*RemediationResult, error) {
	logger := log.FromContext(ctx)
	result := &RemediationResult{
		Action:    "KillStuckJob",
		JobName:   job.Name,
		Timestamp: time.Now(),
	}

	// Check if actually stuck
	if job.Status.StartTime == nil {
		result.Success = false
		result.Message = "Job has not started yet"
		return result, nil
	}

	runningDuration := time.Since(job.Status.StartTime.Time)
	if runningDuration < killCfg.AfterDuration.Duration {
		result.Success = false
		result.Message = fmt.Sprintf("Job running for %s, threshold is %s", runningDuration, killCfg.AfterDuration.Duration)
		return result, nil
	}

	// Check rate limit
	if !e.rateLimiter.Allow() {
		result.Success = false
		result.Message = "Rate limit exceeded"
		return result, nil
	}

	logger.Info("killing stuck job", "job", job.Name, "namespace", job.Namespace, "runningDuration", runningDuration)

	// Determine delete policy
	deletePolicy := metav1.DeletePropagationForeground
	if killCfg.DeletePolicy == "Orphan" {
		deletePolicy = metav1.DeletePropagationOrphan
	}

	// Delete the job
	err := e.client.Delete(ctx, job, &client.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to delete job: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Killed stuck job after %s", runningDuration.Round(time.Second))

	e.mu.Lock()
	e.count24h++
	e.mu.Unlock()

	return result, nil
}

func (e *engine) TryRetry(ctx context.Context, monitor *v1alpha1.CronJobMonitor, failedJob *batchv1.Job, cronJobName string) (*RemediationResult, error) {
	logger := log.FromContext(ctx)
	result := &RemediationResult{
		Action:    "RetryJob",
		Timestamp: time.Now(),
	}

	remediationCfg := monitor.Spec.Remediation
	if remediationCfg == nil || !isEnabled(remediationCfg.Enabled) {
		result.Success = false
		result.Message = "Remediation not enabled"
		return result, nil
	}

	retryConfig := remediationCfg.AutoRetry
	if retryConfig == nil || !retryConfig.Enabled {
		result.Success = false
		result.Message = "Auto-retry not enabled"
		return result, nil
	}

	// Check dry run
	if isEnabled(remediationCfg.DryRun) {
		result.DryRun = true
		result.Success = true
		result.Message = "Would retry job (dry run)"
		return result, nil
	}

	// Check retry count
	key := fmt.Sprintf("%s/%s", failedJob.Namespace, cronJobName)
	e.mu.Lock()
	retryCount := e.retryTracker[key]
	maxRetries := int(getOrDefault(retryConfig.MaxRetries, 2))
	if retryCount >= maxRetries {
		e.mu.Unlock()
		result.Success = false
		result.Message = fmt.Sprintf("Max retries (%d) reached", maxRetries)
		return result, nil
	}
	e.retryTracker[key] = retryCount + 1
	e.mu.Unlock()

	// Check exit code filter
	if len(retryConfig.OnlyForExitCodes) > 0 {
		exitCode := getExitCode(ctx, e.client, failedJob)
		if !containsInt32(retryConfig.OnlyForExitCodes, exitCode) {
			result.Success = false
			result.Message = fmt.Sprintf("Exit code %d not in retry list", exitCode)
			return result, nil
		}
	}

	// Check rate limit
	if !e.rateLimiter.Allow() {
		result.Success = false
		result.Message = "Rate limit exceeded"
		return result, nil
	}

	// Get the CronJob to get the job template
	cronJob := &batchv1.CronJob{}
	err := e.client.Get(ctx, types.NamespacedName{
		Namespace: failedJob.Namespace,
		Name:      cronJobName,
	}, cronJob)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to get CronJob: %v", err)
		return result, err
	}

	// Wait for delay if configured
	if retryConfig.DelayBetweenRetries != nil {
		delay := retryConfig.DelayBetweenRetries.Duration
		logger.Info("waiting before retry", "delay", delay)
		time.Sleep(delay)
	}

	// Create a new job
	newJob := e.createRetryJob(cronJob, failedJob, retryCount+1)

	logger.Info("creating retry job", "job", newJob.Name, "namespace", newJob.Namespace, "retryCount", retryCount+1)

	err = e.client.Create(ctx, newJob)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to create retry job: %v", err)
		return result, err
	}

	result.Success = true
	result.JobName = newJob.Name
	result.Message = fmt.Sprintf("Created retry job %s (attempt %d/%d)", newJob.Name, retryCount+1, maxRetries)

	e.mu.Lock()
	e.count24h++
	e.mu.Unlock()

	return result, nil
}

func (e *engine) createRetryJob(cronJob *batchv1.CronJob, failedJob *batchv1.Job, retryNum int) *batchv1.Job {
	// Generate name
	name := fmt.Sprintf("%s-retry-%d-%d", cronJob.Name, time.Now().Unix(), retryNum)
	if len(name) > 63 {
		name = name[:63]
	}

	// Create job from template
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cronJob.Namespace,
			Labels: map[string]string{
				"guardian.illenium.net/retry":   "true",
				"guardian.illenium.net/cronjob": cronJob.Name,
			},
			Annotations: map[string]string{
				"guardian.illenium.net/retry-of":    failedJob.Name,
				"guardian.illenium.net/retry-count": fmt.Sprintf("%d", retryNum),
			},
		},
		Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
	}

	// Copy labels from CronJob
	for k, v := range cronJob.Spec.JobTemplate.Labels {
		job.Labels[k] = v
	}

	return job
}

func (e *engine) CanRemediate(ctx context.Context, monitor *v1alpha1.CronJobMonitor, action string) (bool, string) {
	remediationCfg := monitor.Spec.Remediation

	// Check if enabled
	if remediationCfg == nil || !isEnabled(remediationCfg.Enabled) {
		return false, "remediation not enabled"
	}

	// Check dry run
	if isEnabled(remediationCfg.DryRun) {
		return true, "dry run mode"
	}

	// Check maintenance window
	if inMaintenanceWindow(monitor.Spec.MaintenanceWindows, time.Now(), monitor.Spec.Timezone) {
		return false, "in maintenance window"
	}

	return true, ""
}

func (e *engine) SetGlobalRateLimits(limits config.RateLimitsConfig) {
	maxPerHour := limits.MaxRemediationsPerHour
	if maxPerHour <= 0 {
		maxPerHour = 100
	}

	e.rateLimiter = rate.NewLimiter(rate.Limit(float64(maxPerHour)/3600), 10)
}

func (e *engine) GetRemediationCount24h() int32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.count24h
}

// ResetRetryCount resets the retry counter for a CronJob (called when job succeeds)
func (e *engine) ResetRetryCount(namespace, cronJobName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := fmt.Sprintf("%s/%s", namespace, cronJobName)
	delete(e.retryTracker, key)
}

// Helper functions

func getExitCode(ctx context.Context, c client.Client, job *batchv1.Job) int32 {
	// Look at pod status to get actual exit code
	// For simplicity, return 1 if failed
	// In real implementation, examine pod container statuses
	return 1
}

func inMaintenanceWindow(windows []v1alpha1.MaintenanceWindow, t time.Time, timezone string) bool {
	if len(windows) == 0 {
		return false
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)

	for _, w := range windows {
		wLoc := loc
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				wLoc = l
			}
		}

		// Parse the schedule and check if we're in the window
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sched, err := parser.Parse(w.Schedule)
		if err != nil {
			continue
		}

		// Find the most recent window start
		// Go back at most 1 day to find a window
		checkTime := t.In(wLoc).Add(-24 * time.Hour)
		for checkTime.Before(t.In(wLoc)) {
			windowStart := sched.Next(checkTime)
			windowEnd := windowStart.Add(w.Duration.Duration)

			if t.In(wLoc).After(windowStart) && t.In(wLoc).Before(windowEnd) {
				return true
			}

			checkTime = windowStart
		}
	}

	return false
}

func isEnabled(b *bool) bool {
	return b != nil && *b
}

func getOrDefault[T any](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}
	return def
}

func containsInt32(slice []int32, item int32) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
