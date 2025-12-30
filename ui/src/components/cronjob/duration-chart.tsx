"use client";

import { useState } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  ReferenceLine,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { TrendingUp, AlertTriangle } from "lucide-react";
import type { CronJobExecution } from "@/lib/api";

interface DurationChartProps {
  executions: CronJobExecution[];
  defaultDays?: number;
}

function parseDuration(duration: string): number {
  // Parse duration strings like "12m34s", "1h30m", "45s"
  let totalSeconds = 0;

  const hoursMatch = duration.match(/(\d+)h/);
  const minsMatch = duration.match(/(\d+)m/);
  const secsMatch = duration.match(/(\d+)s/);

  if (hoursMatch) totalSeconds += parseInt(hoursMatch[1]) * 3600;
  if (minsMatch) totalSeconds += parseInt(minsMatch[1]) * 60;
  if (secsMatch) totalSeconds += parseInt(secsMatch[1]);

  return totalSeconds;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

const RANGE_OPTIONS = [
  { value: 14, label: "14d" },
  { value: 30, label: "30d" },
  { value: 90, label: "90d" },
] as const;

export function DurationChart({ executions, defaultDays = 14 }: DurationChartProps) {
  const [daysRange, setDaysRange] = useState(defaultDays);

  // Group executions by day and calculate p50/p95
  const dayMap = new Map<string, number[]>();

  executions.forEach((exec) => {
    if (!exec.completionTime) return;
    const date = new Date(exec.startTime).toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
    });
    const duration = parseDuration(exec.duration);
    if (!dayMap.has(date)) {
      dayMap.set(date, []);
    }
    dayMap.get(date)!.push(duration);
  });

  const chartData = Array.from(dayMap.entries())
    .map(([date, durations]) => {
      durations.sort((a, b) => a - b);
      const p50Index = Math.floor(durations.length * 0.5);
      const p95Index = Math.floor(durations.length * 0.95);
      return {
        date,
        p50: durations[p50Index] || 0,
        p95: durations[p95Index] || durations[durations.length - 1] || 0,
      };
    })
    .reverse()
    .slice(-daysRange);

  // Calculate regression detection
  const hasRegression = chartData.length >= 4;
  let regressionInfo: { baseline: number; current: number; percentChange: number } | null = null;

  if (hasRegression) {
    const midPoint = Math.floor(chartData.length / 2);
    const firstHalf = chartData.slice(0, midPoint);
    const secondHalf = chartData.slice(midPoint);

    const baselineP95 = firstHalf.reduce((sum, d) => sum + d.p95, 0) / firstHalf.length;
    const currentP95 = secondHalf.reduce((sum, d) => sum + d.p95, 0) / secondHalf.length;
    const percentChange = ((currentP95 - baselineP95) / baselineP95) * 100;

    if (percentChange > 30) {
      regressionInfo = { baseline: baselineP95, current: currentP95, percentChange };
    }
  }

  if (chartData.length === 0) {
    return (
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base font-medium">Duration Trend</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex h-48 items-center justify-center text-muted-foreground">
            No execution data available
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base font-medium">Duration Trend</CardTitle>
            {regressionInfo && (
              <Badge variant="destructive" className="flex items-center gap-1 text-xs">
                <AlertTriangle className="h-3 w-3" />
                +{regressionInfo.percentChange.toFixed(0)}% regression
              </Badge>
            )}
          </div>
          <div className="flex gap-1">
            {RANGE_OPTIONS.map((option) => (
              <Button
                key={option.value}
                variant={daysRange === option.value ? "default" : "outline"}
                size="sm"
                className="h-7 px-2 text-xs"
                onClick={() => setDaysRange(option.value)}
              >
                {option.label}
              </Button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="h-48">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                className="fill-muted-foreground"
                interval={daysRange > 30 ? Math.floor(daysRange / 10) : "preserveStartEnd"}
              />
              <YAxis
                tick={{ fontSize: 11 }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(value) => formatDuration(value)}
                className="fill-muted-foreground"
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "hsl(var(--card))",
                  border: "1px solid hsl(var(--border))",
                  borderRadius: "4px",
                  fontSize: "12px",
                }}
                formatter={(value: number) => [formatDuration(value), ""]}
              />
              <Legend
                verticalAlign="top"
                height={30}
                iconType="line"
                iconSize={10}
                wrapperStyle={{ fontSize: "12px" }}
              />
              {regressionInfo && (
                <ReferenceLine
                  y={regressionInfo.baseline}
                  stroke="hsl(var(--destructive))"
                  strokeDasharray="5 5"
                  label={{
                    value: `Baseline: ${formatDuration(regressionInfo.baseline)}`,
                    position: "right",
                    fontSize: 10,
                    fill: "hsl(var(--destructive))",
                  }}
                />
              )}
              <Line
                type="monotone"
                dataKey="p50"
                name="P50"
                stroke="hsl(var(--chart-1))"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="p95"
                name="P95"
                stroke="hsl(var(--chart-2))"
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
        {regressionInfo && (
          <div className="mt-2 flex items-center gap-2 rounded-md bg-destructive/10 p-2 text-xs text-destructive">
            <TrendingUp className="h-3.5 w-3.5" />
            <span>
              P95 duration increased from {formatDuration(regressionInfo.baseline)} to{" "}
              {formatDuration(regressionInfo.current)} (+{regressionInfo.percentChange.toFixed(0)}%)
            </span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
