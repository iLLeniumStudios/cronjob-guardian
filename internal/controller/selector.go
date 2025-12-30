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
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// MatchesSelector checks if a CronJob matches a monitor's selector.
// This is a shared utility used by both CronJobMonitorReconciler and JobHandler
// to ensure consistent selector evaluation.
func MatchesSelector(cj *batchv1.CronJob, selector *guardianv1alpha1.CronJobSelector) bool {
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
		if !MatchExpression(cj.Labels, expr) {
			return false
		}
	}

	return true
}

// MatchExpression evaluates a single label selector expression against a label set.
func MatchExpression(labelSet map[string]string, expr metav1.LabelSelectorRequirement) bool {
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
