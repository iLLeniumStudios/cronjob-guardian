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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/config"
	prommetrics "github.com/iLLeniumStudios/cronjob-guardian/internal/metrics"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

const finalizerName = "guardian.illenium.net/finalizer"

// CronJob status constants (lowercase to match CRD enum)
const (
	statusHealthy  = "healthy"
	statusWarning  = "warning"
	statusCritical = "critical"
	statusUnknown  = "unknown"
)

// Monitor phase constants (to match CRD enum)
const (
	phaseInitializing = "Initializing"
	phaseActive       = "Active"
	phaseDegraded     = "Degraded"
	phaseError        = "Error"
)

// CronJobMonitorReconciler reconciles a CronJobMonitor object
type CronJobMonitorReconciler struct {
	client.Client
	Log             logr.Logger // Required - must be injected
	Scheme          *runtime.Scheme
	Store           store.Store
	Config          *config.Config
	Analyzer        analyzer.SLAAnalyzer
	AlertDispatcher alerting.Dispatcher
}

// +kubebuilder:rbac:groups=guardian.illenium.net,resources=cronjobmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=cronjobmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=cronjobmonitors/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *CronJobMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("monitor", req.NamespacedName)
	log.V(1).Info("reconciling CronJobMonitor")

	// 1. Fetch the CronJobMonitor
	monitor := &guardianv1alpha1.CronJobMonitor{}
	if err := r.Get(ctx, req.NamespacedName, monitor); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.V(1).Info("monitor not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get monitor")
		return ctrl.Result{}, err
	}
	log.V(1).Info("fetched monitor", "generation", monitor.Generation)

	// 2. Check if being deleted
	if !monitor.DeletionTimestamp.IsZero() {
		log.V(1).Info("monitor being deleted, handling deletion")
		return r.handleDeletion(ctx, monitor)
	}

	// 3. Add finalizer if needed
	if !controllerutil.ContainsFinalizer(monitor, finalizerName) {
		log.V(1).Info("adding finalizer")
		controllerutil.AddFinalizer(monitor, finalizerName)
		if err := r.Update(ctx, monitor); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, err
		}
		log.V(1).Info("finalizer added, requeueing")
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Validate spec
	if err := r.validateSpec(monitor); err != nil {
		log.Error(err, "spec validation failed")
		if updateErr := r.updateConditionWithRetry(ctx, req.NamespacedName, "Ready", metav1.ConditionFalse, "InvalidSpec", err.Error()); updateErr != nil {
			log.Error(updateErr, "failed to update status after validation error")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}
	log.V(1).Info("spec validation passed")

	// 5. Find matching CronJobs
	cronJobs, err := r.findMatchingCronJobs(ctx, monitor)
	if err != nil {
		log.Error(err, "failed to find matching CronJobs")
		return ctrl.Result{}, err
	}
	log.V(1).Info("found matching CronJobs", "count", len(cronJobs))

	// 6. Process each CronJob
	cronJobStatuses := []guardianv1alpha1.CronJobStatus{}
	for i := range cronJobs {
		status := r.processCronJob(ctx, monitor, &cronJobs[i])
		cronJobStatuses = append(cronJobStatuses, status)
	}

	// 6a. Handle CronJobs that were previously monitored but are now gone
	r.handleRemovedCronJobs(ctx, monitor, cronJobs)

	// 7. Calculate summary
	summary := r.calculateSummary(cronJobStatuses)
	log.V(1).Info("calculated summary",
		"total", summary.TotalCronJobs,
		"healthy", summary.Healthy,
		"warning", summary.Warning,
		"critical", summary.Critical,
		"suspended", summary.Suspended)

	// 8. Update status with retry to handle optimistic locking conflicts
	if err := r.updateStatusWithRetry(ctx, req.NamespacedName, summary, cronJobStatuses); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}
	log.Info("reconciled successfully",
		"phase", r.determinePhase(summary),
		"cronJobCount", len(cronJobStatuses))

	// 9. Requeue for periodic checks
	log.V(1).Info("requeueing for periodic check", "after", "30s")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// updateStatusWithRetry updates the monitor status with retry logic to handle optimistic locking conflicts.
// This re-fetches the monitor on conflict and applies the new status values.
func (r *CronJobMonitorReconciler) updateStatusWithRetry(ctx context.Context, nn types.NamespacedName, summary *guardianv1alpha1.MonitorSummary, cronJobStatuses []guardianv1alpha1.CronJobStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch the latest version
		monitor := &guardianv1alpha1.CronJobMonitor{}
		if err := r.Get(ctx, nn, monitor); err != nil {
			return err
		}

		// Apply status updates
		monitor.Status.ObservedGeneration = monitor.Generation
		monitor.Status.Phase = r.determinePhase(summary)
		now := metav1.Now()
		monitor.Status.LastReconcileTime = &now
		monitor.Status.Summary = summary
		monitor.Status.CronJobs = cronJobStatuses
		r.setCondition(monitor, "Ready", metav1.ConditionTrue, "Reconciled", "Successfully reconciled")

		return r.Status().Update(ctx, monitor)
	})
}

// updateConditionWithRetry updates just a condition with retry logic.
func (r *CronJobMonitorReconciler) updateConditionWithRetry(ctx context.Context, nn types.NamespacedName, condType string, status metav1.ConditionStatus, reason, message string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		monitor := &guardianv1alpha1.CronJobMonitor{}
		if err := r.Get(ctx, nn, monitor); err != nil {
			return err
		}
		r.setCondition(monitor, condType, status, reason, message)
		return r.Status().Update(ctx, monitor)
	})
}

func (r *CronJobMonitorReconciler) validateSpec(_ *guardianv1alpha1.CronJobMonitor) error {
	// Basic validation - currently no validation needed
	// No selector means match all, which is valid
	return nil
}

func (r *CronJobMonitorReconciler) findMatchingCronJobs(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor) ([]batchv1.CronJob, error) {
	// Determine which namespaces to search
	namespaces, err := r.getTargetNamespaces(ctx, monitor)
	if err != nil {
		return nil, err
	}

	r.Log.V(1).Info("searching for CronJobs", "namespaces", namespaces)

	var result []batchv1.CronJob
	for _, ns := range namespaces {
		cronJobList := &batchv1.CronJobList{}
		if err := r.List(ctx, cronJobList, client.InNamespace(ns)); err != nil {
			r.Log.Error(err, "failed to list CronJobs in namespace", "namespace", ns)
			continue
		}
		r.Log.V(1).Info("found CronJobs in namespace", "namespace", ns, "count", len(cronJobList.Items))

		for _, cj := range cronJobList.Items {
			if MatchesSelector(&cj, monitor.Spec.Selector) {
				r.Log.V(1).Info("CronJob matches selector", "namespace", ns, "cronJob", cj.Name)
				result = append(result, cj)
			}
		}
	}

	return result, nil
}

// getTargetNamespaces determines which namespaces to search based on the selector
func (r *CronJobMonitorReconciler) getTargetNamespaces(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor) ([]string, error) {
	selector := monitor.Spec.Selector

	// No selector or empty selector - use monitor's namespace
	if selector == nil {
		return []string{monitor.Namespace}, nil
	}

	// AllNamespaces takes precedence
	if selector.AllNamespaces {
		return r.getAllNamespaces(ctx)
	}

	// Explicit namespace list
	if len(selector.Namespaces) > 0 {
		return selector.Namespaces, nil
	}

	// Namespace label selector
	if selector.NamespaceSelector != nil {
		return r.getNamespacesBySelector(ctx, selector.NamespaceSelector)
	}

	// Default: monitor's own namespace
	return []string{monitor.Namespace}, nil
}

// getAllNamespaces returns all namespace names in the cluster
func (r *CronJobMonitorReconciler) getAllNamespaces(ctx context.Context) ([]string, error) {
	nsList := &corev1.NamespaceList{}
	if err := r.List(ctx, nsList); err != nil {
		return nil, err
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

// getNamespacesBySelector returns namespaces matching the label selector
func (r *CronJobMonitorReconciler) getNamespacesBySelector(ctx context.Context, selector *metav1.LabelSelector) ([]string, error) {
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}

	nsList := &corev1.NamespaceList{}
	if err := r.List(ctx, nsList, client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, err
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

func (r *CronJobMonitorReconciler) processCronJob(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, cj *batchv1.CronJob) guardianv1alpha1.CronJobStatus {
	log := r.Log.WithValues("cronJob", cj.Name)
	log.V(1).Info("processing CronJob")

	status := guardianv1alpha1.CronJobStatus{
		Name:      cj.Name,
		Namespace: cj.Namespace,
		Suspended: cj.Spec.Suspend != nil && *cj.Spec.Suspend,
	}
	log.V(1).Info("CronJob state", "suspended", status.Suspended)

	cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}

	// Get active jobs for this CronJob
	activeJobs, err := r.getActiveJobs(ctx, cj)
	if err != nil {
		log.V(1).Error(err, "failed to get active jobs")
	} else {
		status.ActiveJobs = activeJobs
		log.V(1).Info("found active jobs", "count", len(activeJobs))
	}

	// Get last successful execution
	if r.Store != nil {
		log.V(1).Info("fetching last successful execution from store")
		lastSuccess, _ := r.Store.GetLastSuccessfulExecution(ctx, cronJobNN)
		if lastSuccess != nil {
			status.LastSuccessfulTime = &metav1.Time{Time: lastSuccess.CompletionTime}
			status.LastRunDuration = &metav1.Duration{Duration: lastSuccess.Duration()}
			log.V(1).Info("found last successful execution",
				"completionTime", lastSuccess.CompletionTime,
				"duration", lastSuccess.Duration())
		} else {
			log.V(1).Info("no last successful execution found")
		}
	}

	// Calculate next scheduled time
	status.NextScheduledTime = calculateNextRun(cj.Spec.Schedule, cj.Spec.TimeZone)
	if status.NextScheduledTime != nil {
		log.V(1).Info("calculated next run", "nextScheduledTime", status.NextScheduledTime.Time)
	}

	// Get metrics - always fetch basic metrics, use SLA window if configured
	windowDays := 7 // Default window
	if monitor.Spec.SLA != nil && monitor.Spec.SLA.WindowDays != nil {
		windowDays = int(*monitor.Spec.SLA.WindowDays)
	}

	// Always try to get metrics from analyzer/store if available
	if r.Analyzer != nil {
		log.V(1).Info("fetching metrics", "windowDays", windowDays)
		metrics, err := r.Analyzer.GetMetrics(ctx, cronJobNN, windowDays)
		if err == nil && metrics != nil {
			status.Metrics = metrics
			log.V(1).Info("metrics retrieved",
				"successRate", metrics.SuccessRate,
				"totalRuns", metrics.TotalRuns)

			// Update Prometheus metrics
			prommetrics.UpdateSuccessRate(cj.Namespace, cj.Name, monitor.Name, metrics.SuccessRate)
			prommetrics.UpdateDuration(cj.Namespace, cj.Name, "p50", metrics.P50DurationSeconds)
			prommetrics.UpdateDuration(cj.Namespace, cj.Name, "p95", metrics.P95DurationSeconds)
			prommetrics.UpdateDuration(cj.Namespace, cj.Name, "p99", metrics.P99DurationSeconds)
		} else if err != nil {
			log.V(1).Error(err, "failed to get metrics")
		}
	} else if r.Store != nil {
		// Fallback: get metrics directly from store if no analyzer
		log.V(1).Info("fetching metrics from store", "windowDays", windowDays)
		metrics, err := r.Store.GetMetrics(ctx, cronJobNN, windowDays)
		if err == nil && metrics != nil {
			status.Metrics = &guardianv1alpha1.CronJobMetrics{
				SuccessRate:        metrics.SuccessRate,
				TotalRuns:          metrics.TotalRuns,
				SuccessfulRuns:     metrics.SuccessfulRuns,
				FailedRuns:         metrics.FailedRuns,
				AvgDurationSeconds: metrics.AvgDurationSeconds,
				P50DurationSeconds: metrics.P50DurationSeconds,
				P95DurationSeconds: metrics.P95DurationSeconds,
				P99DurationSeconds: metrics.P99DurationSeconds,
			}
			log.V(1).Info("metrics retrieved from store",
				"successRate", metrics.SuccessRate,
				"totalRuns", metrics.TotalRuns)

			// Update Prometheus metrics
			prommetrics.UpdateSuccessRate(cj.Namespace, cj.Name, monitor.Name, metrics.SuccessRate)
			prommetrics.UpdateDuration(cj.Namespace, cj.Name, "p50", metrics.P50DurationSeconds)
			prommetrics.UpdateDuration(cj.Namespace, cj.Name, "p95", metrics.P95DurationSeconds)
			prommetrics.UpdateDuration(cj.Namespace, cj.Name, "p99", metrics.P99DurationSeconds)
		} else if err != nil {
			log.V(1).Error(err, "failed to get metrics from store")
		}
	}

	// Check for active alerts
	status.ActiveAlerts = r.checkAlerts(ctx, monitor, cj, &status)
	log.V(1).Info("checked alerts", "activeAlertCount", len(status.ActiveAlerts))

	// Update active alerts Prometheus metric by severity
	alertsBySeverity := make(map[string]float64)
	for _, alert := range status.ActiveAlerts {
		alertsBySeverity[alert.Severity]++
	}
	for severity, count := range alertsBySeverity {
		prommetrics.UpdateActiveAlerts(cj.Namespace, cj.Name, severity, count)
	}

	// Determine overall status
	status.Status = r.determineStatus(&status)
	log.V(1).Info("determined CronJob status", "status", status.Status)

	return status
}

//nolint:gocyclo // complexity is acceptable for a function that checks multiple alert conditions
func (r *CronJobMonitorReconciler) checkAlerts(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, cj *batchv1.CronJob, _ *guardianv1alpha1.CronJobStatus) []guardianv1alpha1.ActiveAlert {
	var alerts []guardianv1alpha1.ActiveAlert
	cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}

	// Check for recent job failure
	if r.Store != nil {
		r.Log.V(1).Info("checking last execution for failure", "cronJob", cj.Name)
		lastExec, err := r.Store.GetLastExecution(ctx, cronJobNN)
		if err != nil {
			r.Log.V(1).Error(err, "failed to get last execution", "cronJob", cj.Name)
		} else if lastExec != nil && !lastExec.Succeeded {
			// Last execution failed - add a warning or critical alert
			severity := statusWarning
			if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
				severity = getSeverity(monitor.Spec.Alerting.SeverityOverrides.JobFailed, statusWarning)
			}
			message := "Last job execution failed"
			if lastExec.Reason != "" {
				message = "Last job execution failed: " + lastExec.Reason
			}
			// Use completion time if available, otherwise start time, otherwise now
			alertTime := metav1.Now()
			if !lastExec.CompletionTime.IsZero() {
				alertTime = metav1.Time{Time: lastExec.CompletionTime}
			} else if !lastExec.StartTime.IsZero() {
				alertTime = metav1.Time{Time: lastExec.StartTime}
			}
			r.Log.V(1).Info("last job execution failed", "cronJob", cj.Name, "severity", severity, "reason", lastExec.Reason)
			alerts = append(alerts, guardianv1alpha1.ActiveAlert{
				Type:     "JobFailed",
				Severity: severity,
				Message:  message,
				Since:    alertTime,
			})
		}
	}

	// Check dead-man's switch
	if monitor.Spec.DeadManSwitch != nil && isEnabled(monitor.Spec.DeadManSwitch.Enabled) && r.Analyzer != nil {
		r.Log.V(1).Info("checking dead-man's switch", "cronJob", cj.Name)
		result, err := r.Analyzer.CheckDeadManSwitch(ctx, cj, monitor.Spec.DeadManSwitch)
		if err != nil {
			r.Log.V(1).Error(err, "failed to check dead-man's switch", "cronJob", cj.Name)
		} else if result.Triggered {
			severity := "critical"
			if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
				severity = getSeverity(monitor.Spec.Alerting.SeverityOverrides.DeadManTriggered, "critical")
			}
			r.Log.V(1).Info("dead-man's switch triggered", "cronJob", cj.Name, "severity", severity, "message", result.Message)
			alerts = append(alerts, guardianv1alpha1.ActiveAlert{
				Type:     "DeadManTriggered",
				Severity: severity,
				Message:  result.Message,
				Since:    metav1.Now(),
			})
		}
	}

	// Check SLA
	if monitor.Spec.SLA != nil && isEnabled(monitor.Spec.SLA.Enabled) && r.Analyzer != nil {
		cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}
		r.Log.V(1).Info("checking SLA", "cronJob", cj.Name)
		result, err := r.Analyzer.CheckSLA(ctx, cronJobNN, monitor.Spec.SLA)
		if err != nil {
			r.Log.V(1).Error(err, "failed to check SLA", "cronJob", cj.Name)
		} else if !result.Passed {
			for _, v := range result.Violations {
				severity := statusWarning
				if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
					severity = getSeverity(monitor.Spec.Alerting.SeverityOverrides.SLABreached, statusWarning)
				}
				r.Log.V(1).Info("SLA violation detected", "cronJob", cj.Name, "type", v.Type, "severity", severity, "message", v.Message)
				alerts = append(alerts, guardianv1alpha1.ActiveAlert{
					Type:     v.Type,
					Severity: severity,
					Message:  v.Message,
					Since:    metav1.Now(),
				})
			}
		}
	}

	// Check duration regression
	if monitor.Spec.SLA != nil && r.Analyzer != nil {
		cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}
		r.Log.V(1).Info("checking duration regression", "cronJob", cj.Name)
		result, err := r.Analyzer.CheckDurationRegression(ctx, cronJobNN, monitor.Spec.SLA)
		if err != nil {
			r.Log.V(1).Error(err, "failed to check duration regression", "cronJob", cj.Name)
		} else if result.Detected {
			severity := "warning"
			if monitor.Spec.Alerting != nil && monitor.Spec.Alerting.SeverityOverrides != nil {
				severity = getSeverity(monitor.Spec.Alerting.SeverityOverrides.DurationRegression, "warning")
			}
			r.Log.V(1).Info("duration regression detected", "cronJob", cj.Name, "severity", severity, "message", result.Message)
			alerts = append(alerts, guardianv1alpha1.ActiveAlert{
				Type:     "DurationRegression",
				Severity: severity,
				Message:  result.Message,
				Since:    metav1.Now(),
			})
		}
	}

	return alerts
}

// getActiveJobs returns currently running jobs for a CronJob
func (r *CronJobMonitorReconciler) getActiveJobs(ctx context.Context, cj *batchv1.CronJob) ([]guardianv1alpha1.ActiveJob, error) {
	// List all jobs in the namespace
	jobList := &batchv1.JobList{}
	if err := r.List(ctx, jobList, client.InNamespace(cj.Namespace)); err != nil {
		return nil, err
	}

	activeJobs := make([]guardianv1alpha1.ActiveJob, 0, len(jobList.Items))
	now := time.Now()

	for _, job := range jobList.Items {
		// Check if this job is owned by the CronJob
		isOwned := false
		for _, ref := range job.OwnerReferences {
			if ref.Kind == kindCronJob && ref.Name == cj.Name {
				isOwned = true
				break
			}
		}
		if !isOwned {
			continue
		}

		// Check if job is still active (not completed or failed)
		if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
			continue
		}

		// Skip jobs that haven't started yet (StartTime is required in the CRD)
		if job.Status.StartTime == nil {
			continue
		}

		// Job is active and running
		duration := now.Sub(job.Status.StartTime.Time)
		activeJob := guardianv1alpha1.ActiveJob{
			Name:            job.Name,
			StartTime:       *job.Status.StartTime,
			RunningDuration: &metav1.Duration{Duration: duration},
		}

		// Calculate ready status
		parallelism := int32(1)
		if job.Spec.Parallelism != nil {
			parallelism = *job.Spec.Parallelism
		}
		ready := int32(0)
		if job.Status.Ready != nil {
			ready = *job.Status.Ready
		}
		activeJob.Ready = fmt.Sprintf("%d/%d", ready, parallelism)

		// Get pod information
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.InNamespace(cj.Namespace), client.MatchingLabels{"job-name": job.Name}); err == nil && len(podList.Items) > 0 {
			// Get the most recent pod
			pod := &podList.Items[0]
			for i := range podList.Items {
				if podList.Items[i].CreationTimestamp.After(pod.CreationTimestamp.Time) {
					pod = &podList.Items[i]
				}
			}
			activeJob.PodName = pod.Name
			activeJob.PodPhase = string(pod.Status.Phase)
		}

		activeJobs = append(activeJobs, activeJob)
	}

	return activeJobs, nil
}

func (r *CronJobMonitorReconciler) calculateSummary(statuses []guardianv1alpha1.CronJobStatus) *guardianv1alpha1.MonitorSummary {
	summary := &guardianv1alpha1.MonitorSummary{
		TotalCronJobs: int32(len(statuses)),
	}

	for _, s := range statuses {
		switch s.Status {
		case statusHealthy:
			summary.Healthy++
		case statusWarning:
			summary.Warning++
		case statusCritical:
			summary.Critical++
		}
		if s.Suspended {
			summary.Suspended++
		}
		// Count CronJobs that have at least one active job running
		if len(s.ActiveJobs) > 0 {
			summary.Running++
		}
	}

	return summary
}

// handleRemovedCronJobs handles CronJobs that were previously monitored but are now gone
func (r *CronJobMonitorReconciler) handleRemovedCronJobs(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, currentCronJobs []batchv1.CronJob) {
	if r.Store == nil {
		return
	}

	// Build a set of current CronJob names
	currentNames := make(map[string]bool)
	for _, cj := range currentCronJobs {
		currentNames[cj.Name] = true
	}

	// Check which CronJobs from the previous status are no longer present
	for _, prevCJ := range monitor.Status.CronJobs {
		if !currentNames[prevCJ.Name] {
			// This CronJob was previously monitored but is now gone
			r.handleCronJobRemoval(ctx, monitor, prevCJ.Namespace, prevCJ.Name)
		}
	}
}

// handleCronJobRemoval handles the removal of a specific CronJob from monitoring
func (r *CronJobMonitorReconciler) handleCronJobRemoval(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, namespace, name string) {
	log := r.Log.WithValues("cronJob", name, "namespace", namespace)

	// Determine the action based on DataRetention config
	onDeletion := "retain" // default
	if monitor.Spec.DataRetention != nil && monitor.Spec.DataRetention.OnCronJobDeletion != "" {
		onDeletion = monitor.Spec.DataRetention.OnCronJobDeletion
	}

	cronJobNN := types.NamespacedName{Namespace: namespace, Name: name}

	switch onDeletion {
	case "purge":
		// Delete executions immediately
		deleted, err := r.Store.DeleteExecutionsByCronJob(ctx, cronJobNN)
		if err != nil {
			log.Error(err, "failed to purge executions for removed CronJob")
		} else if deleted > 0 {
			log.Info("purged executions for removed CronJob", "count", deleted)
		}

	case "purge-after-days":
		// This would require tracking the removal time and processing in the pruner
		// For now, log that this is not yet implemented
		purgeAfterDays := int32(7) // default
		if monitor.Spec.DataRetention != nil && monitor.Spec.DataRetention.PurgeAfterDays != nil {
			purgeAfterDays = *monitor.Spec.DataRetention.PurgeAfterDays
		}
		log.V(1).Info("CronJob marked for delayed purge (not yet fully implemented)",
			"purgeAfterDays", purgeAfterDays)
		// TODO: Implement delayed purge tracking in Phase 9 (Enhanced Pruner)

	case "retain":
		log.V(1).Info("retaining executions for removed CronJob per config")

	default:
		log.V(1).Info("unknown onCronJobDeletion value, defaulting to retain", "value", onDeletion)
	}
}

func (r *CronJobMonitorReconciler) determinePhase(summary *guardianv1alpha1.MonitorSummary) string {
	if summary.Critical > 0 {
		return phaseError
	}
	if summary.Warning > 0 {
		return phaseDegraded
	}
	if summary.TotalCronJobs == 0 {
		return phaseActive // No targets is still active, just empty
	}
	return phaseActive
}

func (r *CronJobMonitorReconciler) determineStatus(status *guardianv1alpha1.CronJobStatus) string {
	for _, alert := range status.ActiveAlerts {
		if alert.Severity == "critical" {
			return statusCritical
		}
	}
	for _, alert := range status.ActiveAlerts {
		if alert.Severity == "warning" {
			return statusWarning
		}
	}
	return statusHealthy
}

func (r *CronJobMonitorReconciler) handleDeletion(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(monitor, finalizerName) {
		// Cancel any active alerts
		if r.AlertDispatcher != nil {
			r.Log.V(1).Info("clearing alerts for deleted monitor", "monitor", monitor.Name)
			r.AlertDispatcher.ClearAlertsForMonitor(monitor.Namespace, monitor.Name)
		}

		// Remove finalizer
		r.Log.V(1).Info("removing finalizer", "monitor", monitor.Name)
		controllerutil.RemoveFinalizer(monitor, finalizerName)
		if err := r.Update(ctx, monitor); err != nil {
			r.Log.Error(err, "failed to remove finalizer", "monitor", monitor.Name)
			return ctrl.Result{}, err
		}
		r.Log.Info("finalizer removed, deletion complete", "monitor", monitor.Name)
	}
	return ctrl.Result{}, nil
}

func (r *CronJobMonitorReconciler) setCondition(monitor *guardianv1alpha1.CronJobMonitor, condType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	found := false
	for i, c := range monitor.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				monitor.Status.Conditions[i] = condition
			}
			found = true
			break
		}
	}
	if !found {
		monitor.Status.Conditions = append(monitor.Status.Conditions, condition)
	}
}

// findMonitorsForCronJob returns reconcile requests for monitors that match the CronJob.
// This searches all monitors cluster-wide since monitors can watch CronJobs across namespaces.
func (r *CronJobMonitorReconciler) findMonitorsForCronJob(ctx context.Context, obj client.Object) []reconcile.Request {
	log := r.Log.V(1)
	cj, ok := obj.(*batchv1.CronJob)
	if !ok {
		return nil
	}

	log.Info("finding monitors for CronJob", "cronJob", cj.Name, "namespace", cj.Namespace)

	// List ALL monitors cluster-wide since monitors can watch CronJobs from other namespaces
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := r.List(ctx, monitors); err != nil {
		log.Error(err, "failed to list monitors")
		return nil
	}

	var requests []reconcile.Request
	for _, monitor := range monitors.Items {
		// Check if this monitor is watching the CronJob's namespace
		if !r.monitorWatchesNamespace(ctx, &monitor, cj.Namespace) {
			continue
		}

		// Check if the CronJob matches the monitor's selector
		if MatchesSelector(cj, monitor.Spec.Selector) {
			log.Info("CronJob matches monitor", "monitor", monitor.Name, "monitorNamespace", monitor.Namespace)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: monitor.Namespace,
					Name:      monitor.Name,
				},
			})
		}
	}
	log.Info("found monitors for CronJob", "count", len(requests))
	return requests
}

// monitorWatchesNamespace checks if a monitor is configured to watch a given namespace
func (r *CronJobMonitorReconciler) monitorWatchesNamespace(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, namespace string) bool {
	selector := monitor.Spec.Selector

	// No selector - monitor only watches its own namespace
	if selector == nil {
		return monitor.Namespace == namespace
	}

	// AllNamespaces - watches everything
	if selector.AllNamespaces {
		return true
	}

	// Explicit namespace list
	if len(selector.Namespaces) > 0 {
		for _, ns := range selector.Namespaces {
			if ns == namespace {
				return true
			}
		}
		return false
	}

	// Namespace label selector - need to check if namespace has matching labels
	if selector.NamespaceSelector != nil {
		labelSelector, err := metav1.LabelSelectorAsSelector(selector.NamespaceSelector)
		if err != nil {
			return false
		}

		ns := &corev1.Namespace{}
		if err := r.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
			return false
		}

		return labelSelector.Matches(labels.Set(ns.Labels))
	}

	// Default: monitor only watches its own namespace
	return monitor.Namespace == namespace
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("setting up CronJobMonitor controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&guardianv1alpha1.CronJobMonitor{},
			// Only reconcile on spec changes (generation changes), not status-only updates.
			// This prevents duplicate reconciles when we update status at the end of Reconcile().
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&batchv1.CronJob{},
			handler.EnqueueRequestsFromMapFunc(r.findMonitorsForCronJob),
		).
		Named("cronjobmonitor").
		Complete(r)
}

// Helper functions

func calculateNextRun(schedule string, timezone *string) *metav1.Time {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return nil
	}

	loc := time.UTC
	if timezone != nil && *timezone != "" {
		if l, err := time.LoadLocation(*timezone); err == nil {
			loc = l
		}
	}

	next := sched.Next(time.Now().In(loc))
	return &metav1.Time{Time: next}
}

func isEnabled(b *bool) bool {
	return b == nil || *b
}

func getSeverity(override string, defaultSeverity string) string {
	if override != "" {
		return override
	}
	return defaultSeverity
}
