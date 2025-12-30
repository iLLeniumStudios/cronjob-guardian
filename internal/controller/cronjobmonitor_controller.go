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
	"time"

	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/analyzer"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

const finalizerName = "guardian.illenium.net/finalizer"

// Status constants
const (
	statusHealthy  = "Healthy"
	statusWarning  = "Warning"
	statusCritical = "Critical"
)

// CronJobMonitorReconciler reconciles a CronJobMonitor object
type CronJobMonitorReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Store           store.Store
	Analyzer        analyzer.SLAAnalyzer
	AlertDispatcher alerting.Dispatcher
}

// +kubebuilder:rbac:groups=guardian.illenium.net,resources=cronjobmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=cronjobmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=cronjobmonitors/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *CronJobMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch the CronJobMonitor
	monitor := &guardianv1alpha1.CronJobMonitor{}
	if err := r.Get(ctx, req.NamespacedName, monitor); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Check if being deleted
	if !monitor.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, monitor)
	}

	// 3. Add finalizer if needed
	if !controllerutil.ContainsFinalizer(monitor, finalizerName) {
		controllerutil.AddFinalizer(monitor, finalizerName)
		if err := r.Update(ctx, monitor); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Validate spec
	if err := r.validateSpec(monitor); err != nil {
		r.setCondition(monitor, "Ready", metav1.ConditionFalse, "InvalidSpec", err.Error())
		if err := r.Status().Update(ctx, monitor); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 5. Find matching CronJobs
	cronJobs, err := r.findMatchingCronJobs(ctx, monitor)
	if err != nil {
		logger.Error(err, "failed to find matching CronJobs")
		return ctrl.Result{}, err
	}

	// 6. Process each CronJob
	cronJobStatuses := []guardianv1alpha1.CronJobStatus{}
	for i := range cronJobs {
		status := r.processCronJob(ctx, monitor, &cronJobs[i])
		cronJobStatuses = append(cronJobStatuses, status)
	}

	// 7. Calculate summary
	summary := r.calculateSummary(cronJobStatuses)

	// 8. Update status
	monitor.Status.ObservedGeneration = monitor.Generation
	monitor.Status.Phase = r.determinePhase(summary)
	now := metav1.Now()
	monitor.Status.LastReconcileTime = &now
	monitor.Status.Summary = summary
	monitor.Status.CronJobs = cronJobStatuses
	r.setCondition(monitor, "Ready", metav1.ConditionTrue, "Reconciled", "Successfully reconciled")

	if err := r.Status().Update(ctx, monitor); err != nil {
		return ctrl.Result{}, err
	}

	// 9. Requeue for periodic checks
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *CronJobMonitorReconciler) validateSpec(_ *guardianv1alpha1.CronJobMonitor) error {
	// Basic validation - currently no validation needed
	// No selector means match all, which is valid
	return nil
}

func (r *CronJobMonitorReconciler) findMatchingCronJobs(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor) ([]batchv1.CronJob, error) {
	cronJobList := &batchv1.CronJobList{}
	if err := r.List(ctx, cronJobList, client.InNamespace(monitor.Namespace)); err != nil {
		return nil, err
	}

	var result []batchv1.CronJob
	for _, cj := range cronJobList.Items {
		if r.matchesSelector(&cj, monitor.Spec.Selector) {
			result = append(result, cj)
		}
	}

	return result, nil
}

func (r *CronJobMonitorReconciler) matchesSelector(cj *batchv1.CronJob, selector *guardianv1alpha1.CronJobSelector) bool {
	if selector == nil {
		return true // No selector = match all
	}

	// Check matchNames
	if len(selector.MatchNames) > 0 {
		found := false
		for _, name := range selector.MatchNames {
			if name == cj.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check matchLabels
	for k, v := range selector.MatchLabels {
		if cj.Labels[k] != v {
			return false
		}
	}

	// Check matchExpressions
	for _, expr := range selector.MatchExpressions {
		if !r.matchExpression(cj.Labels, expr) {
			return false
		}
	}

	return true
}

func (r *CronJobMonitorReconciler) matchExpression(labelSet map[string]string, expr metav1.LabelSelectorRequirement) bool {
	switch expr.Operator {
	case metav1.LabelSelectorOpIn:
		val, ok := labelSet[expr.Key]
		if !ok {
			return false
		}
		for _, v := range expr.Values {
			if v == val {
				return true
			}
		}
		return false
	case metav1.LabelSelectorOpNotIn:
		val, ok := labelSet[expr.Key]
		if !ok {
			return true
		}
		for _, v := range expr.Values {
			if v == val {
				return false
			}
		}
		return true
	case metav1.LabelSelectorOpExists:
		_, ok := labelSet[expr.Key]
		return ok
	case metav1.LabelSelectorOpDoesNotExist:
		_, ok := labelSet[expr.Key]
		return !ok
	}
	return false
}

func (r *CronJobMonitorReconciler) processCronJob(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, cj *batchv1.CronJob) guardianv1alpha1.CronJobStatus {
	status := guardianv1alpha1.CronJobStatus{
		Name:      cj.Name,
		Namespace: cj.Namespace,
		Suspended: cj.Spec.Suspend != nil && *cj.Spec.Suspend,
	}

	cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}

	// Get last successful execution
	if r.Store != nil {
		lastSuccess, _ := r.Store.GetLastSuccessfulExecution(ctx, cronJobNN)
		if lastSuccess != nil {
			status.LastSuccessfulTime = &metav1.Time{Time: lastSuccess.CompletionTime}
			status.LastRunDuration = &metav1.Duration{Duration: lastSuccess.Duration}
		}
	}

	// Calculate next scheduled time
	status.NextScheduledTime = calculateNextRun(cj.Spec.Schedule, cj.Spec.TimeZone)

	// Get SLA metrics
	if monitor.Spec.SLA != nil && isEnabled(monitor.Spec.SLA.Enabled) && r.Analyzer != nil {
		windowDays := int(getOrDefault(monitor.Spec.SLA.WindowDays, 7))
		metrics, err := r.Analyzer.GetMetrics(ctx, cronJobNN, windowDays)
		if err == nil && metrics != nil {
			status.Metrics = metrics
		}
	}

	// Check for active alerts
	status.ActiveAlerts = r.checkAlerts(ctx, monitor, cj, &status)

	// Determine overall status
	status.Status = r.determineStatus(&status)

	return status
}

func (r *CronJobMonitorReconciler) checkAlerts(ctx context.Context, monitor *guardianv1alpha1.CronJobMonitor, cj *batchv1.CronJob, _ *guardianv1alpha1.CronJobStatus) []guardianv1alpha1.ActiveAlert {
	var alerts []guardianv1alpha1.ActiveAlert

	// Check dead-man's switch
	if monitor.Spec.DeadManSwitch != nil && isEnabled(monitor.Spec.DeadManSwitch.Enabled) && r.Analyzer != nil {
		result, err := r.Analyzer.CheckDeadManSwitch(ctx, cj, monitor.Spec.DeadManSwitch)
		if err == nil && result.Triggered {
			alerts = append(alerts, guardianv1alpha1.ActiveAlert{
				Type:     "DeadManTriggered",
				Severity: getSeverity(monitor.Spec.Alerting.SeverityOverrides.DeadManTriggered, "critical"),
				Message:  result.Message,
				Since:    metav1.Now(),
			})
		}
	}

	// Check SLA
	if monitor.Spec.SLA != nil && isEnabled(monitor.Spec.SLA.Enabled) && r.Analyzer != nil {
		cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}
		result, err := r.Analyzer.CheckSLA(ctx, cronJobNN, monitor.Spec.SLA)
		if err == nil && !result.Passed {
			for _, v := range result.Violations {
				alerts = append(alerts, guardianv1alpha1.ActiveAlert{
					Type:     v.Type,
					Severity: getSeverity(monitor.Spec.Alerting.SeverityOverrides.SLABreached, "warning"),
					Message:  v.Message,
					Since:    metav1.Now(),
				})
			}
		}
	}

	// Check duration regression
	if monitor.Spec.SLA != nil && r.Analyzer != nil {
		cronJobNN := types.NamespacedName{Namespace: cj.Namespace, Name: cj.Name}
		result, err := r.Analyzer.CheckDurationRegression(ctx, cronJobNN, monitor.Spec.SLA)
		if err == nil && result.Detected {
			alerts = append(alerts, guardianv1alpha1.ActiveAlert{
				Type:     "DurationRegression",
				Severity: getSeverity(monitor.Spec.Alerting.SeverityOverrides.DurationRegression, "warning"),
				Message:  result.Message,
				Since:    metav1.Now(),
			})
		}
	}

	return alerts
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
	}

	return summary
}

func (r *CronJobMonitorReconciler) determinePhase(summary *guardianv1alpha1.MonitorSummary) string {
	if summary.Critical > 0 {
		return statusCritical
	}
	if summary.Warning > 0 {
		return statusWarning
	}
	if summary.TotalCronJobs == 0 {
		return "NoTargets"
	}
	return statusHealthy
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
			r.AlertDispatcher.ClearAlertsForMonitor(monitor.Namespace, monitor.Name)
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(monitor, finalizerName)
		if err := r.Update(ctx, monitor); err != nil {
			return ctrl.Result{}, err
		}
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

// findMonitorsForCronJob returns reconcile requests for monitors that match the CronJob
func (r *CronJobMonitorReconciler) findMonitorsForCronJob(ctx context.Context, obj client.Object) []reconcile.Request {
	cj, ok := obj.(*batchv1.CronJob)
	if !ok {
		return nil
	}

	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := r.List(ctx, monitors, client.InNamespace(cj.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, monitor := range monitors.Items {
		if r.matchesSelector(cj, monitor.Spec.Selector) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: monitor.Namespace,
					Name:      monitor.Name,
				},
			})
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobMonitorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&guardianv1alpha1.CronJobMonitor{}).
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

func getOrDefault[T any](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}
	return def
}

func getSeverity(override string, defaultSeverity string) string {
	if override != "" {
		return override
	}
	return defaultSeverity
}
