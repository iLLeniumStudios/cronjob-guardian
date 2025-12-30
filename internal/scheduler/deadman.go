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
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
)

// DeadManScheduler periodically checks for dead-man's switch violations
type DeadManScheduler struct {
	client     client.Client
	analyzer   analyzer.SLAAnalyzer
	dispatcher alerting.Dispatcher
	interval   time.Duration
	stopCh     chan struct{}
	running    bool
	mu         sync.Mutex
}

// NewDeadManScheduler creates a new dead-man's switch scheduler
func NewDeadManScheduler(c client.Client, a analyzer.SLAAnalyzer, d alerting.Dispatcher) *DeadManScheduler {
	return &DeadManScheduler{
		client:     c,
		analyzer:   a,
		dispatcher: d,
		interval:   1 * time.Minute,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the scheduler loop
func (s *DeadManScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	logger := log.FromContext(ctx)
	logger.Info("starting dead-man's switch scheduler", "interval", s.interval)

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

// Stop halts the scheduler
func (s *DeadManScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// SetInterval changes the check interval
func (s *DeadManScheduler) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = d
}

func (s *DeadManScheduler) check(ctx context.Context) {
	logger := log.FromContext(ctx)

	// List all CronJobMonitors
	monitors := &v1alpha1.CronJobMonitorList{}
	if err := s.client.List(ctx, monitors); err != nil {
		logger.Error(err, "failed to list monitors")
		return
	}

	for _, monitor := range monitors.Items {
		if monitor.Spec.DeadManSwitch == nil || !isEnabled(monitor.Spec.DeadManSwitch.Enabled) {
			continue
		}

		// Check each CronJob in the monitor
		for _, cjStatus := range monitor.Status.CronJobs {
			// Skip suspended CronJobs if configured
			if cjStatus.Suspended && isEnabled(monitor.Spec.SuspendedHandling.PauseMonitoring) {
				continue
			}

			// Skip if in maintenance window
			if inMaintenanceWindow(monitor.Spec.MaintenanceWindows, time.Now(), monitor.Spec.Timezone) {
				continue
			}

			// Get the CronJob
			cronJob := &batchv1.CronJob{}
			err := s.client.Get(ctx, types.NamespacedName{
				Namespace: cjStatus.Namespace,
				Name:      cjStatus.Name,
			}, cronJob)
			if err != nil {
				continue
			}

			// Check dead-man's switch
			result, err := s.analyzer.CheckDeadManSwitch(ctx, cronJob, monitor.Spec.DeadManSwitch)
			if err != nil {
				logger.Error(err, "failed to check dead-man's switch", "cronjob", cjStatus.Name)
				continue
			}

			if result.Triggered {
				// Check if we already have an active alert for this
				if hasActiveAlert(cjStatus.ActiveAlerts, "DeadManTriggered") {
					continue
				}

				// Send alert
				alert := alerting.Alert{
					Type:     "DeadManTriggered",
					Severity: getSeverity(monitor.Spec.Alerting.SeverityOverrides.DeadManTriggered, "critical"),
					Title:    fmt.Sprintf("Dead-man's switch triggered: %s/%s", cjStatus.Namespace, cjStatus.Name),
					Message:  result.Message,
					CronJob: types.NamespacedName{
						Namespace: cjStatus.Namespace,
						Name:      cjStatus.Name,
					},
					MonitorRef: types.NamespacedName{
						Namespace: monitor.Namespace,
						Name:      monitor.Name,
					},
					Timestamp: time.Now(),
				}

				if err := s.dispatcher.Dispatch(ctx, alert, monitor.Spec.Alerting); err != nil {
					logger.Error(err, "failed to dispatch dead-man's switch alert")
				}
			}
		}
	}
}

func hasActiveAlert(alerts []v1alpha1.ActiveAlert, alertType string) bool {
	for _, a := range alerts {
		if a.Type == alertType {
			return true
		}
	}
	return false
}
