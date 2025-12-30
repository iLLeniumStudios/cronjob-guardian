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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/alerting"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/remediation"
	"github.com/iLLeniumStudios/cronjob-guardian/internal/store"
)

// GuardianConfigReconciler reconciles a GuardianConfig object
type GuardianConfigReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Store             store.Store
	AlertDispatcher   alerting.Dispatcher
	RemediationEngine remediation.Engine
	IgnoredNamespaces map[string]bool
}

// +kubebuilder:rbac:groups=guardian.illenium.net,resources=guardianconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=guardianconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=guardian.illenium.net,resources=guardianconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles GuardianConfig reconciliation
func (r *GuardianConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Only process the "default" config
	if req.Name != "default" {
		logger.V(1).Info("ignoring non-default GuardianConfig", "name", req.Name)
		return ctrl.Result{}, nil
	}

	// 1. Fetch the GuardianConfig
	config := &guardianv1alpha1.GuardianConfig{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Use defaults
			r.applyDefaults()
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// 2. Apply configuration
	r.applyConfig(config)

	// 3. Initialize/update storage backend
	if err := r.initStorage(ctx, config); err != nil {
		logger.Error(err, "failed to initialize storage")
		config.Status.StorageStatus = "Error: " + err.Error()
		r.setCondition(config, "StorageReady", metav1.ConditionFalse, "InitFailed", err.Error())
	} else {
		config.Status.StorageStatus = "Ready"
		r.setCondition(config, "StorageReady", metav1.ConditionTrue, "Initialized", "Storage backend ready")
	}

	// 4. Update global rate limits
	if r.AlertDispatcher != nil {
		r.AlertDispatcher.SetGlobalRateLimits(config.Spec.GlobalRateLimits)
	}
	if r.RemediationEngine != nil {
		r.RemediationEngine.SetGlobalRateLimits(config.Spec.GlobalRateLimits)
	}

	// 5. Update ignored namespaces
	r.setIgnoredNamespaces(config.Spec.IgnoredNamespaces)

	// 6. Gather stats
	config.Status.TotalMonitors = r.countMonitors(ctx)
	config.Status.TotalCronJobsWatched = r.countWatchedCronJobs(ctx)
	if r.AlertDispatcher != nil {
		config.Status.TotalAlertsSent24h = r.AlertDispatcher.GetAlertCount24h()
	}
	if r.RemediationEngine != nil {
		config.Status.TotalRemediations24h = r.RemediationEngine.GetRemediationCount24h()
	}
	now := metav1.Now()
	config.Status.LastReconcileTime = &now

	// 7. Update status
	if err := r.Status().Update(ctx, config); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *GuardianConfigReconciler) applyDefaults() {
	// Set default ignored namespaces
	r.IgnoredNamespaces = map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
	}
}

func (r *GuardianConfigReconciler) applyConfig(config *guardianv1alpha1.GuardianConfig) {
	// Apply ignored namespaces
	r.setIgnoredNamespaces(config.Spec.IgnoredNamespaces)
}

func (r *GuardianConfigReconciler) initStorage(ctx context.Context, config *guardianv1alpha1.GuardianConfig) error {
	// Storage is initialized by the store package via factory
	// This is just for status reporting
	if r.Store != nil {
		return r.Store.Health(ctx)
	}
	return nil
}

func (r *GuardianConfigReconciler) setIgnoredNamespaces(namespaces []string) {
	r.IgnoredNamespaces = make(map[string]bool)
	for _, ns := range namespaces {
		r.IgnoredNamespaces[ns] = true
	}
	// Always ignore system namespaces
	r.IgnoredNamespaces["kube-system"] = true
	r.IgnoredNamespaces["kube-public"] = true
	r.IgnoredNamespaces["kube-node-lease"] = true
}

func (r *GuardianConfigReconciler) countMonitors(ctx context.Context) int32 {
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := r.List(ctx, monitors); err != nil {
		return 0
	}
	return int32(len(monitors.Items))
}

func (r *GuardianConfigReconciler) countWatchedCronJobs(ctx context.Context) int32 {
	monitors := &guardianv1alpha1.CronJobMonitorList{}
	if err := r.List(ctx, monitors); err != nil {
		return 0
	}

	count := int32(0)
	for _, m := range monitors.Items {
		count += int32(len(m.Status.CronJobs))
	}
	return count
}

func (r *GuardianConfigReconciler) setCondition(config *guardianv1alpha1.GuardianConfig, condType string, status metav1.ConditionStatus, reason, message string) {
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
	for i, c := range config.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				config.Status.Conditions[i] = condition
			}
			found = true
			break
		}
	}
	if !found {
		config.Status.Conditions = append(config.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GuardianConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&guardianv1alpha1.GuardianConfig{}).
		Named("guardianconfig").
		Complete(r)
}
