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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// SLARecalcScheduler periodically recalculates SLA metrics
type SLARecalcScheduler struct {
	client     client.Client
	store      store.Store
	analyzer   analyzer.SLAAnalyzer
	dispatcher alerting.Dispatcher
	interval   time.Duration
	stopCh     chan struct{}
	running    bool
	mu         sync.Mutex
}

// NewSLARecalcScheduler creates a new SLA recalculation scheduler
func NewSLARecalcScheduler(c client.Client, st store.Store, a analyzer.SLAAnalyzer, d alerting.Dispatcher) *SLARecalcScheduler {
	return &SLARecalcScheduler{
		client:     c,
		store:      st,
		analyzer:   a,
		dispatcher: d,
		interval:   5 * time.Minute,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the scheduler loop
func (s *SLARecalcScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.mu.Unlock()

	logger := log.FromContext(ctx)
	logger.Info("starting SLA recalculation scheduler", "interval", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-ticker.C:
			s.recalculate(ctx)
		}
	}
}

// Stop halts the scheduler
func (s *SLARecalcScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// SetInterval changes the recalculation interval
func (s *SLARecalcScheduler) SetInterval(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = d
}

func (s *SLARecalcScheduler) recalculate(ctx context.Context) {
	logger := log.FromContext(ctx)

	monitors := &v1alpha1.CronJobMonitorList{}
	if err := s.client.List(ctx, monitors); err != nil {
		logger.Error(err, "failed to list monitors")
		return
	}

	for _, monitor := range monitors.Items {
		if monitor.Spec.SLA == nil || !isEnabled(monitor.Spec.SLA.Enabled) {
			continue
		}

		windowDays := int(getOrDefault(monitor.Spec.SLA.WindowDays, 7))

		for _, cjStatus := range monitor.Status.CronJobs {
			cronJobNN := types.NamespacedName{
				Namespace: cjStatus.Namespace,
				Name:      cjStatus.Name,
			}

			// Recalculate metrics
			metrics, err := s.analyzer.GetMetrics(ctx, cronJobNN, windowDays)
			if err != nil {
				logger.Error(err, "failed to get metrics", "cronjob", cjStatus.Name)
				continue
			}

			// Check SLA
			slaResult, err := s.analyzer.CheckSLA(ctx, cronJobNN, monitor.Spec.SLA)
			if err != nil {
				logger.Error(err, "failed to check SLA", "cronjob", cjStatus.Name)
				continue
			}

			// Check for violations
			if !slaResult.Passed {
				for _, v := range slaResult.Violations {
					alertKey := fmt.Sprintf("%s/%s/SLA/%s", cjStatus.Namespace, cjStatus.Name, v.Type)

					// Safely get severity override
					var slaBreachedSeverity string
					if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
						slaBreachedSeverity = monitor.Spec.Alerting.SeverityOverrides.SLABreached
					}

					alert := alerting.Alert{
						Key:      alertKey,
						Type:     "SLABreached",
						Severity: getSeverity(slaBreachedSeverity, "warning"),
						Title:    fmt.Sprintf("SLA breach: %s/%s", cjStatus.Namespace, cjStatus.Name),
						Message:  v.Message,
						CronJob:  cronJobNN,
						Context: alerting.AlertContext{
							SuccessRate: metrics.SuccessRate,
						},
						Timestamp: time.Now(),
					}

					if err := s.dispatcher.Dispatch(ctx, alert, monitor.Spec.Alerting); err != nil {
						logger.Error(err, "failed to dispatch SLA alert")
					}
				}
			}

			// Check duration regression
			regResult, err := s.analyzer.CheckDurationRegression(ctx, cronJobNN, monitor.Spec.SLA)
			if err == nil && regResult.Detected {
				// Safely get severity override
				var regressionSeverity string
				if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
					regressionSeverity = monitor.Spec.Alerting.SeverityOverrides.DurationRegression
				}

				alert := alerting.Alert{
					Key:       fmt.Sprintf("%s/%s/DurationRegression", cjStatus.Namespace, cjStatus.Name),
					Type:      "DurationRegression",
					Severity:  getSeverity(regressionSeverity, "warning"),
					Title:     fmt.Sprintf("Duration regression: %s/%s", cjStatus.Namespace, cjStatus.Name),
					Message:   regResult.Message,
					CronJob:   cronJobNN,
					Timestamp: time.Now(),
				}

				if err := s.dispatcher.Dispatch(ctx, alert, monitor.Spec.Alerting); err != nil {
					logger.Error(err, "failed to dispatch regression alert")
				}
			}
		}
	}
}
