"use client";

import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { CronJobExecution } from "@/lib/api";

interface SuccessRateChartProps {
  executions: CronJobExecution[];
}

export function SuccessRateChart({ executions }: SuccessRateChartProps) {
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
    .slice(-14);

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
        <CardTitle className="text-base font-medium">Success Rate (14 days)</CardTitle>
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
                  backgroundColor: "hsl(var(--card))",
                  border: "1px solid hsl(var(--border))",
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
              <Bar dataKey="successRate" radius={[2, 2, 0, 0]}>
                {chartData.map((entry, index) => (
                  <Cell
                    key={`cell-${index}`}
                    fill={
                      entry.successRate === 100
                        ? "hsl(var(--chart-2))"
                        : entry.successRate >= 80
                          ? "hsl(var(--chart-4))"
                          : "hsl(var(--destructive))"
                    }
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  );
}
