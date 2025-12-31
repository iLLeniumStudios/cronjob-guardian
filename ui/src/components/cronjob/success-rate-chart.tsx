"use client";

import { useState } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
  ReferenceLine,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { TrendingDown, TrendingUp } from "lucide-react";
import type { CronJobExecution } from "@/lib/api";

interface SuccessRateChartProps {
  executions: CronJobExecution[];
  defaultDays?: number;
  targetSLA?: number; // Optional SLA target line
}

const RANGE_OPTIONS = [
  { value: 14, label: "14d" },
  { value: 30, label: "30d" },
  { value: 90, label: "90d" },
] as const;

export function SuccessRateChart({ executions, defaultDays = 14, targetSLA }: SuccessRateChartProps) {
  const [daysRange, setDaysRange] = useState(defaultDays);

  // Group executions by day
  const dayMap = new Map<string, { success: number; failed: number }>();

  executions.forEach((exec) => {
    const date = new Date(exec.startTime).toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
    });
    if (!dayMap.has(date)) {
      dayMap.set(date, { success: 0, failed: 0 });
    }
    const day = dayMap.get(date)!;
    if (exec.status === "success") {
      day.success++;
    } else {
      day.failed++;
    }
  });

  const chartData = Array.from(dayMap.entries())
    .map(([date, counts]) => {
      const total = counts.success + counts.failed;
      return {
        date,
        successRate: total > 0 ? (counts.success / total) * 100 : 0,
        success: counts.success,
        failed: counts.failed,
        total,
      };
    })
    .reverse()
    .slice(-daysRange);

  // Calculate week-over-week trend
  let weekTrend: { thisWeek: number; lastWeek: number; change: number } | null = null;
  if (chartData.length >= 14) {
    const thisWeekData = chartData.slice(-7);
    const lastWeekData = chartData.slice(-14, -7);

    const thisWeekTotal = thisWeekData.reduce((sum, d) => sum + d.total, 0);
    const thisWeekSuccess = thisWeekData.reduce((sum, d) => sum + d.success, 0);
    const lastWeekTotal = lastWeekData.reduce((sum, d) => sum + d.total, 0);
    const lastWeekSuccess = lastWeekData.reduce((sum, d) => sum + d.success, 0);

    const thisWeekRate = thisWeekTotal > 0 ? (thisWeekSuccess / thisWeekTotal) * 100 : 0;
    const lastWeekRate = lastWeekTotal > 0 ? (lastWeekSuccess / lastWeekTotal) * 100 : 0;

    weekTrend = {
      thisWeek: thisWeekRate,
      lastWeek: lastWeekRate,
      change: thisWeekRate - lastWeekRate,
    };
  }

  if (chartData.length === 0) {
    return (
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base font-medium">Success Rate</CardTitle>
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
            <CardTitle className="text-base font-medium">Success Rate</CardTitle>
            {weekTrend && (
              <Badge
                variant={weekTrend.change >= 0 ? "default" : "destructive"}
                className="flex items-center gap-1 text-xs"
              >
                {weekTrend.change >= 0 ? (
                  <TrendingUp className="h-3 w-3" />
                ) : (
                  <TrendingDown className="h-3 w-3" />
                )}
                {weekTrend.change >= 0 ? "+" : ""}
                {weekTrend.change.toFixed(1)}% vs last week
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
            <BarChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
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
                domain={[0, 100]}
                tickFormatter={(value) => `${value}%`}
                className="fill-muted-foreground"
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "var(--card)",
                  border: "1px solid var(--border)",
                  borderRadius: "4px",
                  fontSize: "12px",
                }}
                formatter={(value: number, _name: string, props) => {
                  const data = props.payload;
                  return [
                    `${value.toFixed(0)}% (${data.success}/${data.total})`,
                    "Success Rate",
                  ];
                }}
              />
              {targetSLA && (
                <ReferenceLine
                  y={targetSLA}
                  stroke="var(--chart-3)"
                  strokeDasharray="5 5"
                  label={{
                    value: `SLA Target: ${targetSLA}%`,
                    position: "right",
                    fontSize: 10,
                    fill: "var(--chart-3)",
                  }}
                />
              )}
              <Bar dataKey="successRate" radius={[2, 2, 0, 0]}>
                {chartData.map((entry, index) => (
                  <Cell
                    key={`cell-${index}`}
                    fill={
                      entry.successRate === 100
                        ? "var(--chart-2)"
                        : entry.successRate >= 80
                          ? "var(--chart-4)"
                          : "var(--destructive)"
                    }
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
        {weekTrend && (
          <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
            <div className="rounded-md bg-muted p-2">
              <span className="text-muted-foreground">This Week: </span>
              <span className="font-medium">{weekTrend.thisWeek.toFixed(1)}%</span>
            </div>
            <div className="rounded-md bg-muted p-2">
              <span className="text-muted-foreground">Last Week: </span>
              <span className="font-medium">{weekTrend.lastWeek.toFixed(1)}%</span>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
