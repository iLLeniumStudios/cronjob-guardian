"use client";

import { cn } from "@/lib/utils";

type Status = "healthy" | "warning" | "critical" | "suspended" | "unknown";

interface StatusIndicatorProps {
  status: Status;
  size?: "sm" | "md" | "lg";
  className?: string;
}

const statusColors: Record<Status, string> = {
  healthy: "bg-emerald-500",
  warning: "bg-amber-500",
  critical: "bg-red-500",
  suspended: "bg-slate-400",
  unknown: "bg-slate-400",
};

const sizeClasses = {
  sm: "h-2 w-2",
  md: "h-2.5 w-2.5",
  lg: "h-3 w-3",
};

export function StatusIndicator({
  status,
  size = "md",
  className,
}: StatusIndicatorProps) {
  return (
    <span
      className={cn(
        "inline-block rounded-full",
        statusColors[status] || statusColors.unknown,
        sizeClasses[size],
        className
      )}
      title={status}
    />
  );
}

interface StatusBadgeProps {
  status: Status;
  className?: string;
}

const badgeColors: Record<Status, string> = {
  healthy: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  warning: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  critical: "bg-red-500/10 text-red-700 dark:text-red-400",
  suspended: "bg-slate-500/10 text-slate-700 dark:text-slate-400",
  unknown: "bg-slate-500/10 text-slate-700 dark:text-slate-400",
};

export function StatusBadge({ status, className }: StatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-medium capitalize",
        badgeColors[status] || badgeColors.unknown,
        className
      )}
    >
      <StatusIndicator status={status} size="sm" />
      {status}
    </span>
  );
}
