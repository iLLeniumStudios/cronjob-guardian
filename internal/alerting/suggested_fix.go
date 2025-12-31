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
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// SuggestedFixEngine matches failure context against patterns to suggest fixes
type SuggestedFixEngine struct {
	builtinPatterns []v1alpha1.SuggestedFixPattern
}

// MatchContext contains the data to match against patterns
type MatchContext struct {
	Namespace string
	Name      string // CronJob name
	JobName   string // Job name
	ExitCode  int32
	Reason    string
	Logs      string
	Events    []string
}

// NewSuggestedFixEngine creates a new engine with built-in patterns
func NewSuggestedFixEngine() *SuggestedFixEngine {
	return &SuggestedFixEngine{
		builtinPatterns: getBuiltinPatterns(),
	}
}

// GetBestSuggestion returns the highest priority matching suggestion
func (e *SuggestedFixEngine) GetBestSuggestion(ctx MatchContext, customPatterns []v1alpha1.SuggestedFixPattern) string {
	// Merge patterns: custom patterns can override builtins by name
	patterns := e.mergePatterns(customPatterns)

	// Sort by priority (descending)
	sort.Slice(patterns, func(i, j int) bool {
		pi := int32(0)
		pj := int32(0)
		if patterns[i].Priority != nil {
			pi = *patterns[i].Priority
		}
		if patterns[j].Priority != nil {
			pj = *patterns[j].Priority
		}
		return pi > pj
	})

	// Find first match
	for _, pattern := range patterns {
		if e.matches(ctx, pattern.Match) {
			return e.renderSuggestion(pattern.Suggestion, ctx)
		}
	}

	return "Check job logs and events for details."
}

// mergePatterns merges custom patterns with builtins, custom overrides by name
func (e *SuggestedFixEngine) mergePatterns(custom []v1alpha1.SuggestedFixPattern) []v1alpha1.SuggestedFixPattern {
	result := make([]v1alpha1.SuggestedFixPattern, 0, len(e.builtinPatterns)+len(custom))

	// Add custom patterns first
	customNames := make(map[string]bool)
	for _, p := range custom {
		result = append(result, p)
		customNames[p.Name] = true
	}

	// Add builtins that aren't overridden
	for _, p := range e.builtinPatterns {
		if !customNames[p.Name] {
			result = append(result, p)
		}
	}

	return result
}

// matches checks if context matches the pattern
func (e *SuggestedFixEngine) matches(ctx MatchContext, match v1alpha1.PatternMatch) bool {
	matched := false

	// Exit code exact match
	if match.ExitCode != nil {
		if ctx.ExitCode == *match.ExitCode {
			matched = true
		} else {
			return false // Explicit mismatch
		}
	}

	// Exit code range
	if match.ExitCodeRange != nil {
		if ctx.ExitCode >= match.ExitCodeRange.Min && ctx.ExitCode <= match.ExitCodeRange.Max {
			matched = true
		} else {
			return false
		}
	}

	// Reason exact match (case-insensitive)
	if match.Reason != "" {
		if strings.EqualFold(ctx.Reason, match.Reason) {
			matched = true
		} else {
			return false
		}
	}

	// Reason pattern (regex)
	if match.ReasonPattern != "" {
		re, err := regexp.Compile(match.ReasonPattern)
		if err == nil && re.MatchString(ctx.Reason) {
			matched = true
		} else {
			return false
		}
	}

	// Log pattern (regex)
	if match.LogPattern != "" {
		re, err := regexp.Compile(match.LogPattern)
		if err == nil && re.MatchString(ctx.Logs) {
			matched = true
		} else {
			return false
		}
	}

	// Event pattern (regex) - match any event
	if match.EventPattern != "" {
		re, err := regexp.Compile(match.EventPattern)
		if err != nil {
			return false
		}
		eventMatched := false
		for _, evt := range ctx.Events {
			if re.MatchString(evt) {
				eventMatched = true
				break
			}
		}
		if eventMatched {
			matched = true
		} else {
			return false
		}
	}

	return matched
}

// renderSuggestion renders template variables in the suggestion
func (e *SuggestedFixEngine) renderSuggestion(suggestion string, ctx MatchContext) string {
	tmpl, err := template.New("suggestion").Parse(suggestion)
	if err != nil {
		return suggestion
	}

	var buf strings.Builder
	data := map[string]interface{}{
		"Namespace": ctx.Namespace,
		"Name":      ctx.Name,
		"JobName":   ctx.JobName,
		"ExitCode":  ctx.ExitCode,
		"Reason":    ctx.Reason,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return suggestion
	}

	return buf.String()
}

// getBuiltinPatterns returns the default built-in patterns
func getBuiltinPatterns() []v1alpha1.SuggestedFixPattern {
	ptr := func(i int32) *int32 { return &i }

	return []v1alpha1.SuggestedFixPattern{
		{
			Name:       "oom-killed",
			Match:      v1alpha1.PatternMatch{Reason: "OOMKilled"},
			Suggestion: "Container ran out of memory. Increase resources.limits.memory in the CronJob spec.",
			Priority:   ptr(100),
		},
		{
			Name:       "oom-signal",
			Match:      v1alpha1.PatternMatch{ExitCode: ptr(137)},
			Suggestion: "Container killed by SIGKILL (exit 137), often due to OOM. Run: kubectl describe pod -n {{.Namespace}} -l job-name={{.JobName}} | grep -A5 'Last State'",
			Priority:   ptr(95),
		},
		{
			Name:       "sigterm",
			Match:      v1alpha1.PatternMatch{ExitCode: ptr(143)},
			Suggestion: "Container received SIGTERM (exit 143). Job may have exceeded activeDeadlineSeconds or been evicted.",
			Priority:   ptr(90),
		},
		{
			Name:       "image-pull",
			Match:      v1alpha1.PatternMatch{Reason: "ImagePullBackOff"},
			Suggestion: "Failed to pull container image. Check image name/tag and registry credentials (imagePullSecrets).",
			Priority:   ptr(85),
		},
		{
			Name:       "crash-loop",
			Match:      v1alpha1.PatternMatch{Reason: "CrashLoopBackOff"},
			Suggestion: "Container keeps crashing on startup. Check application logs for initialization errors.",
			Priority:   ptr(80),
		},
		{
			Name:       "config-error",
			Match:      v1alpha1.PatternMatch{Reason: "CreateContainerConfigError"},
			Suggestion: "Container config error. Verify Secret/ConfigMap references exist and have correct keys.",
			Priority:   ptr(75),
		},
		{
			Name:       "deadline-exceeded",
			Match:      v1alpha1.PatternMatch{Reason: "DeadlineExceeded"},
			Suggestion: "Job exceeded activeDeadlineSeconds. Either increase the deadline or optimize job performance.",
			Priority:   ptr(70),
		},
		{
			Name:       "backoff-limit",
			Match:      v1alpha1.PatternMatch{Reason: "BackoffLimitExceeded"},
			Suggestion: "Job failed too many times (backoffLimit reached). Check logs from failed attempts for root cause.",
			Priority:   ptr(65),
		},
		{
			Name:       "evicted",
			Match:      v1alpha1.PatternMatch{Reason: "Evicted"},
			Suggestion: "Pod was evicted from node. Check node resource pressure and consider setting pod priority.",
			Priority:   ptr(60),
		},
		{
			Name:       "scheduling-failed",
			Match:      v1alpha1.PatternMatch{EventPattern: "FailedScheduling"},
			Suggestion: "Pod could not be scheduled. Check node resources, taints/tolerations, and affinity rules.",
			Priority:   ptr(55),
		},
		{
			Name:       "app-error-range",
			Match:      v1alpha1.PatternMatch{ExitCodeRange: &v1alpha1.ExitCodeRange{Min: 1, Max: 125}},
			Suggestion: "Application exited with error code {{.ExitCode}}. Check application logs: kubectl logs -n {{.Namespace}} -l job-name={{.JobName}} --tail=100",
			Priority:   ptr(10),
		},
		{
			Name:       "signal-range",
			Match:      v1alpha1.PatternMatch{ExitCodeRange: &v1alpha1.ExitCodeRange{Min: 128, Max: 255}},
			Suggestion: "Container terminated by signal (exit {{.ExitCode}} = signal {{.ExitCode}}-128). Check for resource limits or external termination.",
			Priority:   ptr(5),
		},
	}
}
