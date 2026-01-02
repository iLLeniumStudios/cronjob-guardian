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
	"sync"
	"text/template"

	"k8s.io/utils/ptr"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// compiledPattern holds a pattern with pre-compiled regex
type compiledPattern struct {
	Original v1alpha1.SuggestedFixPattern
	ReasonRe *regexp.Regexp
	LogRe    *regexp.Regexp
	EventRe  *regexp.Regexp
}

// SuggestedFixEngine matches failure context against patterns to suggest fixes
type SuggestedFixEngine struct {
	builtinPatterns  []v1alpha1.SuggestedFixPattern
	compiledBuiltins []compiledPattern
	compileOnce      sync.Once
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

// compilePatterns pre-compiles regex patterns for efficiency
func compilePatterns(patterns []v1alpha1.SuggestedFixPattern) []compiledPattern {
	result := make([]compiledPattern, 0, len(patterns))
	for _, p := range patterns {
		compiled := compiledPattern{Original: p}

		if p.Match.ReasonPattern != "" {
			if re, err := regexp.Compile(p.Match.ReasonPattern); err == nil {
				compiled.ReasonRe = re
			}
		}
		if p.Match.LogPattern != "" {
			if re, err := regexp.Compile(p.Match.LogPattern); err == nil {
				compiled.LogRe = re
			}
		}
		if p.Match.EventPattern != "" {
			if re, err := regexp.Compile(p.Match.EventPattern); err == nil {
				compiled.EventRe = re
			}
		}
		result = append(result, compiled)
	}
	return result
}

// getCompiledBuiltins returns pre-compiled builtin patterns (lazy initialization)
func (e *SuggestedFixEngine) getCompiledBuiltins() []compiledPattern {
	e.compileOnce.Do(func() {
		e.compiledBuiltins = compilePatterns(e.builtinPatterns)
	})
	return e.compiledBuiltins
}

// GetBestSuggestion returns the highest priority matching suggestion
func (e *SuggestedFixEngine) GetBestSuggestion(ctx MatchContext, customPatterns []v1alpha1.SuggestedFixPattern) string {
	patterns := e.mergePatterns(customPatterns)

	sort.Slice(patterns, func(i, j int) bool {
		pi := int32(0)
		pj := int32(0)
		if patterns[i].Original.Priority != nil {
			pi = *patterns[i].Original.Priority
		}
		if patterns[j].Original.Priority != nil {
			pj = *patterns[j].Original.Priority
		}
		return pi > pj
	})

	for _, pattern := range patterns {
		if e.matchesCompiled(ctx, pattern) {
			return e.renderSuggestion(pattern.Original.Suggestion, ctx)
		}
	}

	return "Check job logs and events for details."
}

// mergePatterns merges custom patterns with builtins, custom overrides by name
// Returns compiled patterns for efficient matching
func (e *SuggestedFixEngine) mergePatterns(custom []v1alpha1.SuggestedFixPattern) []compiledPattern {
	builtins := e.getCompiledBuiltins()
	customCompiled := compilePatterns(custom)

	result := make([]compiledPattern, 0, len(builtins)+len(customCompiled))

	customNames := make(map[string]bool)
	for _, p := range customCompiled {
		result = append(result, p)
		customNames[p.Original.Name] = true
	}

	for _, p := range builtins {
		if !customNames[p.Original.Name] {
			result = append(result, p)
		}
	}

	return result
}

// matchesCompiled checks if context matches the compiled pattern (uses pre-compiled regex)
func (e *SuggestedFixEngine) matchesCompiled(ctx MatchContext, cp compiledPattern) bool {
	match := cp.Original.Match
	matched := false

	if match.ExitCode != nil {
		if ctx.ExitCode == *match.ExitCode {
			matched = true
		} else {
			return false
		}
	}

	if match.ExitCodeRange != nil {
		if ctx.ExitCode >= match.ExitCodeRange.Min && ctx.ExitCode <= match.ExitCodeRange.Max {
			matched = true
		} else {
			return false
		}
	}

	if match.Reason != "" {
		if strings.EqualFold(ctx.Reason, match.Reason) {
			matched = true
		} else {
			return false
		}
	}

	// Use pre-compiled regex for ReasonPattern
	if match.ReasonPattern != "" {
		if cp.ReasonRe != nil && cp.ReasonRe.MatchString(ctx.Reason) {
			matched = true
		} else {
			return false
		}
	}

	// Use pre-compiled regex for LogPattern
	if match.LogPattern != "" {
		if cp.LogRe != nil && cp.LogRe.MatchString(ctx.Logs) {
			matched = true
		} else {
			return false
		}
	}

	// Use pre-compiled regex for EventPattern
	if match.EventPattern != "" {
		if cp.EventRe == nil {
			return false
		}
		eventMatched := false
		for _, evt := range ctx.Events {
			if cp.EventRe.MatchString(evt) {
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
	return []v1alpha1.SuggestedFixPattern{
		{
			Name:       "oom-killed",
			Match:      v1alpha1.PatternMatch{Reason: "OOMKilled"},
			Suggestion: "Container ran out of memory. Increase resources.limits.memory in the CronJob spec.",
			Priority:   ptr.To(int32(100)),
		},
		{
			Name:       "oom-signal",
			Match:      v1alpha1.PatternMatch{ExitCode: ptr.To(int32(137))},
			Suggestion: "Container killed by SIGKILL (exit 137), often due to OOM. Run: kubectl describe pod -n {{.Namespace}} -l job-name={{.JobName}} | grep -A5 'Last State'",
			Priority:   ptr.To(int32(95)),
		},
		{
			Name:       "sigterm",
			Match:      v1alpha1.PatternMatch{ExitCode: ptr.To(int32(143))},
			Suggestion: "Container received SIGTERM (exit 143). Job may have exceeded activeDeadlineSeconds or been evicted.",
			Priority:   ptr.To(int32(90)),
		},
		{
			Name:       "image-pull",
			Match:      v1alpha1.PatternMatch{Reason: "ImagePullBackOff"},
			Suggestion: "Failed to pull container image. Check image name/tag and registry credentials (imagePullSecrets).",
			Priority:   ptr.To(int32(85)),
		},
		{
			Name:       "crash-loop",
			Match:      v1alpha1.PatternMatch{Reason: "CrashLoopBackOff"},
			Suggestion: "Container keeps crashing on startup. Check application logs for initialization errors.",
			Priority:   ptr.To(int32(80)),
		},
		{
			Name:       "config-error",
			Match:      v1alpha1.PatternMatch{Reason: "CreateContainerConfigError"},
			Suggestion: "Container config error. Verify Secret/ConfigMap references exist and have correct keys.",
			Priority:   ptr.To(int32(75)),
		},
		{
			Name:       "deadline-exceeded",
			Match:      v1alpha1.PatternMatch{Reason: "DeadlineExceeded"},
			Suggestion: "Job exceeded activeDeadlineSeconds. Either increase the deadline or optimize job performance.",
			Priority:   ptr.To(int32(70)),
		},
		{
			Name:       "backoff-limit",
			Match:      v1alpha1.PatternMatch{Reason: "BackoffLimitExceeded"},
			Suggestion: "Job failed too many times (backoffLimit reached). Check logs from failed attempts for root cause.",
			Priority:   ptr.To(int32(65)),
		},
		{
			Name:       "evicted",
			Match:      v1alpha1.PatternMatch{Reason: "Evicted"},
			Suggestion: "Pod was evicted from node. Check node resource pressure and consider setting pod priority.",
			Priority:   ptr.To(int32(60)),
		},
		{
			Name:       "scheduling-failed",
			Match:      v1alpha1.PatternMatch{EventPattern: "FailedScheduling"},
			Suggestion: "Pod could not be scheduled. Check node resources, taints/tolerations, and affinity rules.",
			Priority:   ptr.To(int32(55)),
		},
		{
			Name:       "app-error-range",
			Match:      v1alpha1.PatternMatch{ExitCodeRange: &v1alpha1.ExitCodeRange{Min: 1, Max: 125}},
			Suggestion: "Application exited with error code {{.ExitCode}}. Check application logs: kubectl logs -n {{.Namespace}} -l job-name={{.JobName}} --tail=100",
			Priority:   ptr.To(int32(10)),
		},
		{
			Name:       "signal-range",
			Match:      v1alpha1.PatternMatch{ExitCodeRange: &v1alpha1.ExitCodeRange{Min: 128, Max: 255}},
			Suggestion: "Container terminated by signal (exit {{.ExitCode}} = signal {{.ExitCode}}-128). Check for resource limits or external termination.",
			Priority:   ptr.To(int32(5)),
		},
	}
}
