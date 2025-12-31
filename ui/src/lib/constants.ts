/**
 * Centralized styling constants for the UI.
 * These are shared across multiple components to ensure consistency.
 */

// =============================================================================
// SEVERITY STYLES (for alerts)
// =============================================================================

export type Severity = "critical" | "warning" | "info";

export const SEVERITY_STYLES = {
  critical: {
    dot: "bg-red-500",
    text: "text-red-700 dark:text-red-400",
    bg: "bg-red-500/5",
    badge: "bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/20",
    icon: "text-red-600 dark:text-red-400",
  },
  warning: {
    dot: "bg-amber-500",
    text: "text-amber-700 dark:text-amber-400",
    bg: "bg-amber-500/5",
    badge: "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/20",
    icon: "text-amber-600 dark:text-amber-400",
  },
  info: {
    dot: "bg-blue-500",
    text: "text-blue-700 dark:text-blue-400",
    bg: "bg-blue-500/5",
    badge: "bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20",
    icon: "text-blue-600 dark:text-blue-400",
  },
} as const;

export const SEVERITY_ORDER: Record<Severity, number> = {
  critical: 0,
  warning: 1,
  info: 2,
};

// =============================================================================
// STATUS STYLES (for overall health status)
// =============================================================================

export type Status = "healthy" | "warning" | "critical" | "suspended" | "running" | "unknown";

export const STATUS_COLORS: Record<Status, string> = {
  healthy: "bg-emerald-500",
  warning: "bg-amber-500",
  critical: "bg-red-500",
  suspended: "bg-slate-400",
  running: "bg-blue-500",
  unknown: "bg-slate-400",
};

export const STATUS_BADGE_COLORS: Record<Status, string> = {
  healthy: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  warning: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  critical: "bg-red-500/10 text-red-700 dark:text-red-400",
  suspended: "bg-slate-500/10 text-slate-700 dark:text-slate-400",
  running: "bg-blue-500/10 text-blue-700 dark:text-blue-400",
  unknown: "bg-slate-500/10 text-slate-700 dark:text-slate-400",
};

// =============================================================================
// SUCCESS RATE STYLES
// =============================================================================

/**
 * Get the color class for a success rate percentage.
 * - >= 99%: Excellent (green)
 * - >= 95%: Good (amber)
 * - < 95%: Poor (red)
 */
export function getSuccessRateColor(rate: number): string {
  if (rate >= 99) return "text-emerald-600 dark:text-emerald-400";
  if (rate >= 95) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

/**
 * Get the background color class for a success rate percentage.
 */
export function getSuccessRateBgColor(rate: number): string {
  if (rate >= 99) return "bg-emerald-500/10";
  if (rate >= 95) return "bg-amber-500/10";
  return "bg-red-500/10";
}

// =============================================================================
// ICON COLORS (for stat cards and icons)
// =============================================================================

export type IconColor = "emerald" | "amber" | "red" | "blue" | "gray" | "purple";

export const ICON_COLORS: Record<IconColor, { bg: string; icon: string }> = {
  emerald: {
    bg: "bg-emerald-500/10",
    icon: "text-emerald-600 dark:text-emerald-400",
  },
  amber: {
    bg: "bg-amber-500/10",
    icon: "text-amber-600 dark:text-amber-400",
  },
  red: {
    bg: "bg-red-500/10",
    icon: "text-red-600 dark:text-red-400",
  },
  blue: {
    bg: "bg-blue-500/10",
    icon: "text-blue-600 dark:text-blue-400",
  },
  gray: {
    bg: "bg-slate-500/10",
    icon: "text-slate-600 dark:text-slate-400",
  },
  purple: {
    bg: "bg-purple-500/10",
    icon: "text-purple-600 dark:text-purple-400",
  },
};

// =============================================================================
// REFRESH INTERVAL
// =============================================================================

/** Default auto-refresh interval in milliseconds */
export const DEFAULT_REFRESH_INTERVAL = 5000;
