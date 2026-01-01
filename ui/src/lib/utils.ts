import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
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
