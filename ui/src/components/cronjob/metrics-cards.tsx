"use client";

import { Card, CardContent } from "@/components/ui/card";
import { RelativeTime } from "@/components/relative-time";
import type { CronJobMetrics } from "@/lib/api";
import { cn } from "@/lib/utils";
import { getSuccessRateColor } from "@/lib/constants";

interface MetricsCardsProps {
  metrics: CronJobMetrics | null | undefined;
  nextRun: string | null;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) {
    // For sub-minute, show up to 2 decimal places
    const rounded = Math.round(seconds * 100) / 100;
    return `${rounded}s`;
  }
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

export function MetricsCards({ metrics, nextRun }: MetricsCardsProps) {
  // Use default values if metrics is null/undefined
  const m = metrics ?? {
    successRate7d: 0,
    successRate30d: 0,
    totalRuns7d: 0,
    successfulRuns7d: 0,
    failedRuns7d: 0,
    avgDurationSeconds: 0,
    p50DurationSeconds: 0,
    p95DurationSeconds: 0,
    p99DurationSeconds: 0,
  };

  const cards = [
    {
      label: "Success Rate (7d)",
      value: `${m.successRate7d.toFixed(1)}%`,
      subtext: `${m.successfulRuns7d}/${m.totalRuns7d} runs`,
      className: getSuccessRateColor(m.successRate7d),
    },
    {
      label: "Success Rate (30d)",
      value: `${m.successRate30d.toFixed(1)}%`,
      className: getSuccessRateColor(m.successRate30d),
    },
    {
      label: "Avg Duration",
      value: formatDuration(m.avgDurationSeconds),
    },
    {
      label: "P95 Duration",
      value: formatDuration(m.p95DurationSeconds),
    },
    {
      label: "P99 Duration",
      value: formatDuration(m.p99DurationSeconds),
    },
    {
      label: "Next Run",
      value: nextRun ? <RelativeTime date={nextRun} /> : "-",
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 md:gap-4 md:grid-cols-3 lg:grid-cols-6">
      {cards.map((card) => (
        <Card key={card.label}>
          <CardContent className="p-3 md:p-4">
            <p className="text-xs text-muted-foreground">{card.label}</p>
            <p className={cn("mt-1 text-lg md:text-xl font-semibold", card.className)}>
              {card.value}
            </p>
            {card.subtext && (
              <p className="mt-0.5 text-xs text-muted-foreground">{card.subtext}</p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
