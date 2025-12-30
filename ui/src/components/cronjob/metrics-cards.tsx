"use client";

import { Card, CardContent } from "@/components/ui/card";
import { RelativeTime } from "@/components/relative-time";
import type { CronJobMetrics } from "@/lib/api";
import { cn } from "@/lib/utils";

interface MetricsCardsProps {
  metrics: CronJobMetrics;
  nextRun: string | null;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

function getSuccessRateColor(rate: number): string {
  if (rate >= 99) return "text-emerald-600 dark:text-emerald-400";
  if (rate >= 95) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

export function MetricsCards({ metrics, nextRun }: MetricsCardsProps) {
  const cards = [
    {
      label: "Success Rate (7d)",
      value: `${metrics.successRate7d.toFixed(1)}%`,
      subtext: `${metrics.successfulRuns7d}/${metrics.totalRuns7d} runs`,
      className: getSuccessRateColor(metrics.successRate7d),
    },
    {
      label: "Success Rate (30d)",
      value: `${metrics.successRate30d.toFixed(1)}%`,
      className: getSuccessRateColor(metrics.successRate30d),
    },
    {
      label: "Avg Duration",
      value: formatDuration(metrics.avgDurationSeconds),
    },
    {
      label: "P95 Duration",
      value: formatDuration(metrics.p95DurationSeconds),
    },
    {
      label: "P99 Duration",
      value: formatDuration(metrics.p99DurationSeconds),
    },
    {
      label: "Next Run",
      value: nextRun ? <RelativeTime date={nextRun} /> : "-",
    },
  ];

  return (
    <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-6">
      {cards.map((card) => (
        <Card key={card.label}>
          <CardContent className="p-4">
            <p className="text-xs text-muted-foreground">{card.label}</p>
            <p className={cn("mt-1 text-xl font-semibold", card.className)}>
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
