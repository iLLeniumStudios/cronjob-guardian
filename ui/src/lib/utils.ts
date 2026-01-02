import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { SEVERITY_ORDER, type Severity } from "@/lib/constants"
import type { Alert } from "@/lib/api"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Sorts alerts by severity (critical first), then namespace, name, and type.
 * Creates a new sorted array without modifying the input.
 *
 * @example
 * const sorted = sortAlerts(alerts);
 */
export function sortAlerts(alerts: Alert[]): Alert[] {
  return [...alerts].sort((a, b) => {
    const aSeverity = (a.severity || "info") as Severity
    const bSeverity = (b.severity || "info") as Severity

    // Primary sort: severity (critical first)
    const severityDiff = SEVERITY_ORDER[aSeverity] - SEVERITY_ORDER[bSeverity]
    if (severityDiff !== 0) return severityDiff

    // Secondary sort: namespace
    const namespaceDiff = a.cronjob.namespace.localeCompare(b.cronjob.namespace)
    if (namespaceDiff !== 0) return namespaceDiff

    // Tertiary sort: name
    const nameDiff = a.cronjob.name.localeCompare(b.cronjob.name)
    if (nameDiff !== 0) return nameDiff

    // Quaternary sort: alert type (for stability)
    return (a.type || "").localeCompare(b.type || "")
  })
}

/**
 * Formats a duration given in seconds to a human-readable string.
 * Examples: "1.5s", "45s", "2m 30s", "1h 15m", "2d 3h"
 */
export function formatDuration(seconds: number): string {
  if (seconds < 0) return "0s"
  if (seconds < 60) {
    return seconds < 10 ? `${seconds.toFixed(1)}s` : `${Math.round(seconds)}s`
  }
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60)
    const secs = Math.round(seconds % 60)
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`
  }
  if (seconds < 86400) {
    const hours = Math.floor(seconds / 3600)
    const mins = Math.round((seconds % 3600) / 60)
    return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`
  }
  const days = Math.floor(seconds / 86400)
  const hours = Math.round((seconds % 86400) / 3600)
  return hours > 0 ? `${days}d ${hours}h` : `${days}d`
}

/**
 * Formats a duration given in nanoseconds to a human-readable string.
 * Used for Go's time.Duration values which are in nanoseconds.
 */
export function formatDurationNano(nanoseconds: number | undefined): string {
  if (nanoseconds === undefined || nanoseconds === 0) return "N/A"
  return formatDuration(nanoseconds / 1_000_000_000)
}

/**
 * Formats a date/time value to a localized string with full date and time.
 * Example: "Jan 2, 2025, 3:45:30 PM"
 */
export function formatDateTime(date: Date | string): string {
  const d = typeof date === "string" ? new Date(date) : date
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  })
}

/**
 * Formats a date value to a localized string with date only.
 * Example: "Jan 2, 2025"
 */
export function formatDate(date: Date | string): string {
  const d = typeof date === "string" ? new Date(date) : date
  return d.toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  })
}
