package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// isEnabled returns true if the pointer is nil (default true) or points to true
func isEnabled(b *bool) bool {
	return b == nil || *b
}

// getOrDefault returns the value if the pointer is non-nil, otherwise returns the default
func getOrDefault[T any](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}
	return def
}

// getSeverity returns the override severity if non-empty, otherwise returns the default
func getSeverity(override string, defaultSeverity string) string {
	if override != "" {
		return override
	}
	return defaultSeverity
}

// inMaintenanceWindow checks if the given time falls within any maintenance window
func inMaintenanceWindow(windows []v1alpha1.MaintenanceWindow, t time.Time, timezone string) bool {
	if len(windows) == 0 {
		return false
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	t = t.In(loc)

	for _, w := range windows {
		wLoc := loc
		if w.Timezone != "" {
			if l, err := time.LoadLocation(w.Timezone); err == nil {
				wLoc = l
			}
		}

		// Parse the schedule and check if we're in the window
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sched, err := parser.Parse(w.Schedule)
		if err != nil {
			continue
		}

		// Find the most recent window start
		// Go back at most 1 day to find a window
		checkTime := t.In(wLoc).Add(-24 * time.Hour)
		for checkTime.Before(t.In(wLoc)) {
			windowStart := sched.Next(checkTime)
			windowEnd := windowStart.Add(w.Duration.Duration)

			if t.In(wLoc).After(windowStart) && t.In(wLoc).Before(windowEnd) {
				return true
			}

			checkTime = windowStart
		}
	}

	return false
}
