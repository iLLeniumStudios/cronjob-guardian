"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface RelativeTimeProps {
  date: string | Date | null | undefined;
  className?: string;
  showTooltip?: boolean;
}

function formatRelativeTime(date: Date): string {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  const isFuture = diffMs < 0;
  const absDiffMin = Math.abs(diffMin);
  const absDiffHour = Math.abs(diffHour);
  const absDiffDay = Math.abs(diffDay);

  if (Math.abs(diffSec) < 60) {
    return isFuture ? "in a few seconds" : "just now";
  }
  if (absDiffMin < 60) {
    return isFuture
      ? `in ${absDiffMin}m`
      : `${absDiffMin}m ago`;
  }
  if (absDiffHour < 24) {
    return isFuture
      ? `in ${absDiffHour}h`
      : `${absDiffHour}h ago`;
  }
  if (absDiffDay < 7) {
    return isFuture
      ? `in ${absDiffDay}d`
      : `${absDiffDay}d ago`;
  }
  if (absDiffDay < 30) {
    const weeks = Math.floor(absDiffDay / 7);
    return isFuture
      ? `in ${weeks}w`
      : `${weeks}w ago`;
  }
  const months = Math.floor(absDiffDay / 30);
  return isFuture
    ? `in ${months}mo`
    : `${months}mo ago`;
}

function formatAbsoluteTime(date: Date): string {
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// Custom hook for relative time with automatic updates
function useRelativeTime(date: Date | string | null | undefined): string {
  // Memoize the Date object to avoid recreating on every render
  const d = useMemo(
    () => (date ? (typeof date === "string" ? new Date(date) : date) : null),
    [date]
  );

  // Subscribe to time updates (every minute)
  const subscribe = useCallback((callback: () => void) => {
    const interval = setInterval(callback, 60000);
    return () => clearInterval(interval);
  }, []);

  // Get current snapshot
  const getSnapshot = useCallback(() => {
    return d ? formatRelativeTime(d) : "";
  }, [d]);

  // Server snapshot (same as client for this case)
  const getServerSnapshot = useCallback(() => {
    return d ? formatRelativeTime(d) : "";
  }, [d]);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

export function RelativeTime({
  date,
  className,
  showTooltip = true,
}: RelativeTimeProps) {
  const relativeTime = useRelativeTime(date);

  if (!date) {
    return <span className={className}>-</span>;
  }

  const d = typeof date === "string" ? new Date(date) : date;
  const absoluteTime = formatAbsoluteTime(d);

  if (!showTooltip) {
    return <span className={className}>{relativeTime}</span>;
  }

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className={className}>{relativeTime}</span>
        </TooltipTrigger>
        <TooltipContent>
          <p>{absoluteTime}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
