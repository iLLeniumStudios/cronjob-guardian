"use client";

import { useMemo } from "react";
import {
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { MonitorDetail } from "@/lib/api";
import { formatDuration } from "@/lib/utils";

interface AggregateChartsProps {
  monitor: MonitorDetail;
}

export function AggregateCharts({ monitor }: AggregateChartsProps) {
  const { cronJobs } = monitor.status;

  // Calculate aggregate metrics
  const aggregateData = useMemo(() => {
    if (cronJobs.length === 0) return null;

    // Calculate overall success rate (weighted average)
    const totalSuccessRate =
      cronJobs.reduce((sum, cj) => sum + (cj.metrics?.successRate ?? 0), 0) / cronJobs.length;

    // Calculate average duration
    const avgDuration =
      cronJobs.reduce((sum, cj) => sum + (cj.metrics?.avgDurationSeconds ?? 0), 0) / cronJobs.length;

    return {
      totalSuccessRate: totalSuccessRate.toFixed(1),
      avgDuration: avgDuration.toFixed(1),
      cronJobCount: cronJobs.length,
    };
  }, [cronJobs]);

  // Comparison chart data - success rates by cronjob
  const comparisonData = useMemo(() => {
    return cronJobs
      .map((cj) => ({
        name: cj.name.length > 12 ? cj.name.substring(0, 12) + "..." : cj.name,
        fullName: cj.name,
        successRate: cj.metrics?.successRate ?? 0,
        avgDuration: cj.metrics?.avgDurationSeconds ?? 0,
        status: cj.status,
      }))
      .sort((a, b) => a.successRate - b.successRate); // Sort by success rate ascending
  }, [cronJobs]);

  // Health distribution pie chart data
  const healthDistribution = useMemo(() => {
    const healthy = cronJobs.filter((cj) => cj.status === "healthy").length;
    const warning = cronJobs.filter((cj) => cj.status === "warning").length;
    const critical = cronJobs.filter((cj) => cj.status === "critical").length;

    return [
      { name: "Healthy", value: healthy, color: "var(--chart-2)" },
      { name: "Warning", value: warning, color: "var(--chart-4)" },
      { name: "Critical", value: critical, color: "var(--destructive)" },
    ].filter((d) => d.value > 0);
  }, [cronJobs]);

  if (cronJobs.length === 0) {
    return null;
  }

  return (
    <div className="space-y-6">
      {/* Aggregate Summary */}
      {aggregateData && (
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardContent className="pt-6">
              <div className="text-center">
                <p className="text-sm text-muted-foreground">Avg Success Rate</p>
                <p
                  className={`text-3xl font-bold ${
                    parseFloat(aggregateData.totalSuccessRate) >= 95
                      ? "text-emerald-600 dark:text-emerald-400"
                      : parseFloat(aggregateData.totalSuccessRate) >= 80
                        ? "text-amber-600 dark:text-amber-400"
                        : "text-red-600 dark:text-red-400"
                  }`}
                >
                  {aggregateData.totalSuccessRate}%
                </p>
                <p className="text-xs text-muted-foreground">Across all CronJobs</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-center">
                <p className="text-sm text-muted-foreground">Avg Duration</p>
                <p className="text-3xl font-bold">{formatDuration(parseFloat(aggregateData.avgDuration))}</p>
                <p className="text-xs text-muted-foreground">Average execution time</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="text-center">
                <p className="text-sm text-muted-foreground">CronJobs Monitored</p>
                <p className="text-3xl font-bold">{aggregateData.cronJobCount}</p>
                <p className="text-xs text-muted-foreground">Matching selector</p>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Charts row */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Success Rate Comparison Chart */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-medium">Success Rate by CronJob</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart
                  data={comparisonData}
                  layout="vertical"
                  margin={{ top: 5, right: 30, left: 60, bottom: 5 }}
                >
                  <CartesianGrid strokeDasharray="3 3" className="stroke-border" horizontal={false} />
                  <XAxis
                    type="number"
                    domain={[0, 100]}
                    tickFormatter={(v) => `${v}%`}
                    tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }}
                  />
                  <YAxis
                    type="category"
                    dataKey="name"
                    tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }}
                    width={55}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "hsl(var(--card))",
                      border: "1px solid hsl(var(--border))",
                      borderRadius: "var(--radius)",
                      fontSize: "12px",
                      color: "hsl(var(--foreground))",
                    }}
                    itemStyle={{ color: "hsl(var(--foreground))" }}
                    labelStyle={{ color: "hsl(var(--muted-foreground))", marginBottom: "0.25rem" }}
                    formatter={(value: number, _name: string, props) => [
                      `${value.toFixed(1)}%`,
                      props.payload.fullName,
                    ]}
                  />
                  <Bar dataKey="successRate" radius={[0, 4, 4, 0]}>
                    {comparisonData.map((entry, index) => (
                      <Cell
                        key={`cell-${index}`}
                        fill={
                          entry.successRate >= 95
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
          </CardContent>
        </Card>

        {/* Health Distribution Pie Chart */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-medium">Health Distribution</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={healthDistribution}
                    cx="50%"
                    cy="50%"
                    innerRadius={60}
                    outerRadius={90}
                    paddingAngle={2}
                    dataKey="value"
                  >
                    {healthDistribution.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={entry.color} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "hsl(var(--card))",
                      border: "1px solid hsl(var(--border))",
                      borderRadius: "var(--radius)",
                      fontSize: "12px",
                      color: "hsl(var(--foreground))",
                    }}
                    itemStyle={{ color: "hsl(var(--foreground))" }}
                    labelStyle={{ color: "hsl(var(--muted-foreground))", marginBottom: "0.25rem" }}
                    formatter={(value: number, name: string) => [`${value} CronJob${value > 1 ? "s" : ""}`, name]}
                  />
                  <Legend
                    verticalAlign="bottom"
                    height={36}
                    iconType="circle"
                    iconSize={10}
                    wrapperStyle={{ color: "hsl(var(--foreground))" }}
                    formatter={(value) => <span className="text-sm">{value}</span>}
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Duration Comparison Chart */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base font-medium">Average Duration by CronJob</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={comparisonData} margin={{ top: 5, right: 10, left: 0, bottom: 20 }}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                <XAxis
                  dataKey="name"
                  tick={{ fontSize: 10, fill: "hsl(var(--muted-foreground))" }}
                  angle={-45}
                  textAnchor="end"
                  height={60}
                  interval={0}
                />
                <YAxis tick={{ fontSize: 11, fill: "hsl(var(--muted-foreground))" }} tickFormatter={(v) => formatDuration(v)} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "hsl(var(--card))",
                    border: "1px solid hsl(var(--border))",
                    borderRadius: "var(--radius)",
                    fontSize: "12px",
                    color: "hsl(var(--foreground))",
                  }}
                  itemStyle={{ color: "hsl(var(--foreground))" }}
                  labelStyle={{ color: "hsl(var(--muted-foreground))", marginBottom: "0.25rem" }}
                  formatter={(value: number, _name: string, props) => [
                    formatDuration(value),
                    props.payload.fullName,
                  ]}
                />
                <Bar dataKey="avgDuration" fill="var(--chart-1)" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
