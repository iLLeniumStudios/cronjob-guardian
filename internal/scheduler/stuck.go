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

package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/remediation"
)

// StuckJobChecker periodically checks for jobs running too long
type StuckJobChecker struct {
	client            client.Client
	remediationEngine remediation.Engine
	dispatcher        alerting.Dispatcher
	interval          time.Duration
	stopCh            chan struct{}
	running           bool
	mu                sync.Mutex
}

// NewStuckJobChecker creates a new stuck job checker
func NewStuckJobChecker(c client.Client, r remediation.Engine, d alerting.Dispatcher) *StuckJobChecker {
	return &StuckJobChecker{
		client:            c,
		remediationEngine: r,
		dispatcher:        d,
		interval:          1 * time.Minute,
		stopCh:            make(chan struct{}),
	}
}

// Start begins the checker loop
func (s *StuckJobChecker) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	logger := log.FromContext(ctx)
	logger.Info("starting stuck job checker", "interval", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-ticker.C:
			s.check(ctx)
		}
	}
}

// Stop halts the checker
func (s *StuckJobChecker) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// SetInterval changes the check interval
func (s *StuckJobChecker) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = d
}

func (s *StuckJobChecker) check(ctx context.Context) {
	logger := log.FromContext(ctx)

	// List all jobs that are currently running
	jobs := &batchv1.JobList{}
	if err := s.client.List(ctx, jobs); err != nil {
		logger.Error(err, "failed to list jobs")
		return
	}

	for _, job := range jobs.Items {
		// Skip if not running
		if job.Status.Active == 0 {
			continue
		}

		// Skip if no start time
		if job.Status.StartTime == nil {
			continue
		}

		// Find the CronJob owner
		cronJobName := getCronJobOwner(&job)
		if cronJobName == "" {
			continue
		}

		// Find the monitor
		monitor := s.findMonitorForCronJob(ctx, job.Namespace, cronJobName)
		if monitor == nil {
			continue
		}

		// Check if remediation is configured
		config := monitor.Spec.Remediation
		if config == nil || !isEnabled(config.Enabled) {
			continue
		}
		if config.KillStuckJobs == nil || !config.KillStuckJobs.Enabled {
			continue
		}

		// Check if stuck
		runningDuration := time.Since(job.Status.StartTime.Time)
		if runningDuration <= config.KillStuckJobs.AfterDuration.Duration {
			continue
		}

		// Skip if in maintenance window
		if inMaintenanceWindow(monitor.Spec.MaintenanceWindows, time.Now(), monitor.Spec.Timezone) {
			continue
		}

		logger.Info("found stuck job",
			"job", job.Name,
			"namespace", job.Namespace,
			"runningDuration", runningDuration,
			"threshold", config.KillStuckJobs.AfterDuration.Duration)

		// Send alert first
		alert := alerting.Alert{
			Type:     "StuckJob",
			Severity: getSeverity(monitor.Spec.Alerting.SeverityOverrides.StuckJob, "warning"),
			Title:    fmt.Sprintf("Stuck job detected: %s/%s", job.Namespace, job.Name),
			Message:  fmt.Sprintf("Job has been running for %s (threshold: %s)", runningDuration.Round(time.Second), config.KillStuckJobs.AfterDuration.Duration),
			CronJob: types.NamespacedName{
				Namespace: job.Namespace,
				Name:      cronJobName,
			},
			Timestamp: time.Now(),
		}
		if err := s.dispatcher.Dispatch(ctx, alert, monitor.Spec.Alerting); err != nil {
			logger.Error(err, "failed to dispatch stuck job alert")
		}

		// Kill the job
		result, err := s.remediationEngine.KillStuckJob(ctx, &job, config.KillStuckJobs)
		if err != nil {
			logger.Error(err, "failed to kill stuck job", "job", job.Name)
		} else if result.Success {
			logger.Info("killed stuck job", "job", job.Name, "message", result.Message)
		}
	}
}

func (s *StuckJobChecker) findMonitorForCronJob(ctx context.Context, namespace, cronJobName string) *v1alpha1.CronJobMonitor {
	monitors := &v1alpha1.CronJobMonitorList{}
	if err := s.client.List(ctx, monitors, client.InNamespace(namespace)); err != nil {
		return nil
	}

	for _, monitor := range monitors.Items {
		for _, cj := range monitor.Status.CronJobs {
			if cj.Name == cronJobName && cj.Namespace == namespace {
				return &monitor
			}
		}
	}
	return nil
}

func getCronJobOwner(job *batchv1.Job) string {
	for _, ref := range job.OwnerReferences {
		if ref.Kind == "CronJob" {
			return ref.Name
		}
	}
	return ""
}
