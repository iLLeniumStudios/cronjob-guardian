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

package alerting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

func TestSuggestedFix_OOMKilled(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "OOMKilled",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "out of memory")
	assert.Contains(t, suggestion, "resources.limits.memory")
}

func TestSuggestedFix_ExitCode137(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  137,
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "SIGKILL")
	assert.Contains(t, suggestion, "137")
}

func TestSuggestedFix_ExitCode143(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  143,
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "SIGTERM")
	assert.Contains(t, suggestion, "143")
}

func TestSuggestedFix_ImagePullBackOff(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "ImagePullBackOff",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "pull")
	assert.Contains(t, suggestion, "image")
}

func TestSuggestedFix_CrashLoopBackOff(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "CrashLoopBackOff",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "crash")
}

func TestSuggestedFix_CreateContainerConfigError(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "CreateContainerConfigError",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "Secret")
}

func TestSuggestedFix_DeadlineExceeded(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "DeadlineExceeded",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "activeDeadlineSeconds")
}

func TestSuggestedFix_BackoffLimitExceeded(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "BackoffLimitExceeded",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "backoffLimit")
}

func TestSuggestedFix_Evicted(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "Evicted",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "evicted")
}

func TestSuggestedFix_CustomPattern(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "custom-db-error",
			Match: v1alpha1.PatternMatch{
				Reason: "DatabaseConnectionError",
			},
			Suggestion: "Check database connection settings and credentials.",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "DatabaseConnectionError",
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Check database connection settings and credentials.", suggestion)
}

func TestSuggestedFix_LogPattern(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "auth-failure",
			Match: v1alpha1.PatternMatch{
				LogPattern: "(?i)authentication failed",
			},
			Suggestion: "Check API credentials and authentication tokens.",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Logs:      "Error: Authentication failed for user 'service-account'",
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Check API credentials and authentication tokens.", suggestion)
}

func TestSuggestedFix_EventPattern(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Events: []string{
			"Normal: Pod scheduled",
			"Warning: FailedScheduling - 0/3 nodes available: insufficient cpu",
		},
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "schedule")
}

func TestSuggestedFix_Priority(t *testing.T) {
	engine := NewSuggestedFixEngine()

	lowPriority := int32(10)
	highPriority := int32(200)

	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "low-priority-match",
			Match: v1alpha1.PatternMatch{
				ExitCode: ptr.To(int32(1)),
			},
			Suggestion: "Low priority suggestion",
			Priority:   &lowPriority,
		},
		{
			Name: "high-priority-match",
			Match: v1alpha1.PatternMatch{
				ExitCode: ptr.To(int32(1)),
			},
			Suggestion: "High priority suggestion",
			Priority:   &highPriority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  1,
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "High priority suggestion", suggestion)
}

func TestSuggestedFix_TemplateVariables(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "template-test",
			Match: v1alpha1.PatternMatch{
				ExitCode: ptr.To(int32(42)),
			},
			Suggestion: "Job {{.JobName}} in namespace {{.Namespace}} failed with exit code {{.ExitCode}}",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "production",
		Name:      "daily-report",
		JobName:   "daily-report-xyz789",
		ExitCode:  42,
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Job daily-report-xyz789 in namespace production failed with exit code 42", suggestion)
}

func TestSuggestedFix_ExitCodeRange(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  50,
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "error code")
	assert.Contains(t, suggestion, "kubectl logs")
}

func TestSuggestedFix_SignalRange(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  130,
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "signal")
}

func TestSuggestedFix_NoMatch_ReturnsDefault(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  0,
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Equal(t, "Check job logs and events for details.", suggestion)
}

func TestSuggestedFix_CustomOverridesBuiltin(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "oom-killed",
			Match: v1alpha1.PatternMatch{
				Reason: "OOMKilled",
			},
			Suggestion: "Custom OOM fix: Try increasing memory limits in values.yaml",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "OOMKilled",
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Custom OOM fix: Try increasing memory limits in values.yaml", suggestion)
}

func TestSuggestedFix_ReasonPatternRegex(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "error-pattern",
			Match: v1alpha1.PatternMatch{
				ReasonPattern: "Error.*Network",
			},
			Suggestion: "Network-related error detected",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "ErrorDueToNetwork",
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Network-related error detected", suggestion)
}

func TestSuggestedFix_CombinedMatchers(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "combined-match",
			Match: v1alpha1.PatternMatch{
				ExitCode:   ptr.To(int32(1)),
				LogPattern: "connection refused",
			},
			Suggestion: "Both exit code and log pattern matched",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  1,
		Logs:      "Error: connection refused to database",
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Both exit code and log pattern matched", suggestion)

	ctx2 := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  1,
		Logs:      "Some other error message",
	}

	suggestion2 := engine.GetBestSuggestion(ctx2, customPatterns)
	assert.NotEqual(t, "Both exit code and log pattern matched", suggestion2)
}

func TestSuggestedFix_InvalidRegex(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "invalid-regex",
			Match: v1alpha1.PatternMatch{
				LogPattern: "[invalid(regex",
			},
			Suggestion: "Should not match",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Logs:      "[invalid(regex",
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Check job logs and events for details.", suggestion)
}

func TestSuggestedFix_EmptyContext(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Equal(t, "Check job logs and events for details.", suggestion)
}

func TestSuggestedFix_CaseInsensitiveReason(t *testing.T) {
	engine := NewSuggestedFixEngine()

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Reason:    "oomkilled",
	}

	suggestion := engine.GetBestSuggestion(ctx, nil)
	assert.Contains(t, suggestion, "memory")
}

func TestSuggestedFix_TemplateError(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "bad-template",
			Match: v1alpha1.PatternMatch{
				ExitCode: ptr.To(int32(1)),
			},
			Suggestion: "Invalid template: {{.NonExistentField}}",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		ExitCode:  1,
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.NotEmpty(t, suggestion)
}

func TestSuggestedFix_MultipleEventsOneMatches(t *testing.T) {
	engine := NewSuggestedFixEngine()

	priority := int32(200)
	customPatterns := []v1alpha1.SuggestedFixPattern{
		{
			Name: "specific-event",
			Match: v1alpha1.PatternMatch{
				EventPattern: "QuotaExceeded",
			},
			Suggestion: "Resource quota exceeded",
			Priority:   &priority,
		},
	}

	ctx := MatchContext{
		Namespace: "default",
		Name:      "test-cron",
		JobName:   "test-cron-12345",
		Events: []string{
			"Normal: Scheduled pod",
			"Normal: Pulling image",
			"Warning: QuotaExceeded - CPU limit exceeded",
			"Normal: Started container",
		},
	}

	suggestion := engine.GetBestSuggestion(ctx, customPatterns)
	assert.Equal(t, "Resource quota exceeded", suggestion)
}
