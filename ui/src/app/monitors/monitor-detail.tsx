"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Timer,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Clock,
} from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusIndicator, StatusBadge } from "@/components/status-indicator";
import { RelativeTime } from "@/components/relative-time";
import { AggregateCharts } from "@/components/monitor/aggregate-charts";
import { getMonitor, type MonitorDetail } from "@/lib/api";

export function MonitorDetailClient() {
  const router = useRouter();

  // Parse namespace/name from URL path client-side
  const [namespace, setNamespace] = useState("");
  const [name, setName] = useState("");

  useEffect(() => {
    // Parse URL: /monitors/namespace/name
    const path = window.location.pathname;
    const parts = path.split("/").filter(Boolean);
    if (parts.length >= 3 && parts[0] === "monitors") {
      setNamespace(parts[1]);
      setName(parts[2]);
    }
  }, []);

  const [monitor, setMonitor] = useState<MonitorDetail | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const fetchData = useCallback(
    async (showRefreshing = false) => {
      if (!namespace || !name) {
        setIsLoading(false);
        return;
      }
      if (showRefreshing) setIsRefreshing(true);
      try {
        const data = await getMonitor(namespace, name);
        setMonitor(data);
      } catch (error) {
        console.error("Failed to fetch monitor data:", error);
        toast.error("Failed to load monitor data");
      } finally {
        setIsLoading(false);
        setIsRefreshing(false);
      }
    },
    [namespace, name]
  );

  useEffect(() => {
    if (namespace && name) {
      fetchData();
      const interval = setInterval(() => fetchData(), 5000);
      return () => clearInterval(interval);
    }
  }, [fetchData, namespace, name]);

  // If no URL params, return null (list view will be shown instead)
  if (!namespace || !name) {
    return null;
  }

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Monitor Details" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <Skeleton className="h-24 w-full" />
          <div className="grid gap-4 md:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          <Skeleton className="h-96" />
        </div>
      </div>
    );
  }

  if (!monitor) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Monitor Not Found" />
        <div className="flex flex-1 items-center justify-center">
          <div className="text-center">
            <p className="text-lg font-medium">Monitor not found</p>
            <p className="text-muted-foreground">
              {namespace}/{name} does not exist
            </p>
            <Button variant="outline" className="mt-4" onClick={() => router.push("/monitors")}>
              Back to Monitors
            </Button>
          </div>
        </div>
      </div>
    );
  }

  const totalJobs = monitor.status.summary.totalCronJobs;
  const overallStatus =
    monitor.status.summary.critical > 0
      ? "critical"
      : monitor.status.summary.warning > 0
        ? "warning"
        : "healthy";

  return (
    <div className="flex h-full flex-col">
      <Header
        title={monitor.metadata.name}
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
      />

      <div className="flex-1 space-y-6 overflow-auto p-6">
        {/* Back button and header info */}
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => router.push("/monitors")}
              className="mt-0.5"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-semibold">{monitor.metadata.name}</h2>
                <StatusBadge status={overallStatus as "healthy" | "warning" | "critical"} />
                <Badge
                  variant="outline"
                  className={
                    monitor.status.phase === "Active"
                      ? "text-emerald-600 dark:text-emerald-400"
                      : "text-muted-foreground"
                  }
                >
                  {monitor.status.phase}
                </Badge>
              </div>
              <div className="mt-1 flex items-center gap-4 text-sm text-muted-foreground">
                <span className="flex items-center gap-1">
                  <Badge variant="outline">{monitor.metadata.namespace}</Badge>
                </span>
                <span className="flex items-center gap-1">
                  <Clock className="h-3.5 w-3.5" />
                  Created <RelativeTime date={monitor.metadata.creationTimestamp} />
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Summary cards */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <div className="rounded bg-primary/10 p-2">
                  <Timer className="h-4 w-4 text-primary" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">CronJobs</p>
                  <p className="text-2xl font-bold">{totalJobs}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <div className="rounded bg-emerald-500/10 p-2">
                  <CheckCircle className="h-4 w-4 text-emerald-500" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Healthy</p>
                  <p className="text-2xl font-bold">{monitor.status.summary.healthy}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <div className="rounded bg-amber-500/10 p-2">
                  <AlertTriangle className="h-4 w-4 text-amber-500" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Warning</p>
                  <p className="text-2xl font-bold">{monitor.status.summary.warning}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <div className="rounded bg-red-500/10 p-2">
                  <XCircle className="h-4 w-4 text-red-500" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Critical</p>
                  <p className="text-2xl font-bold">{monitor.status.summary.critical}</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Active Alerts */}
        {monitor.status.summary.activeAlerts > 0 && (
          <Card className="border-red-500/50 bg-red-500/5">
            <CardContent className="flex items-center gap-3 py-4">
              <AlertTriangle className="h-5 w-5 text-red-600 dark:text-red-400" />
              <span className="font-medium text-red-700 dark:text-red-400">
                {monitor.status.summary.activeAlerts} active alert{monitor.status.summary.activeAlerts > 1 ? "s" : ""}
              </span>
              <Button variant="link" size="sm" className="ml-auto" onClick={() => router.push("/alerts")}>
                View Alerts
              </Button>
            </CardContent>
          </Card>
        )}

        {/* Aggregate Charts */}
        <AggregateCharts monitor={monitor} />

        {/* Configuration */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Configuration</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Selector */}
            <div>
              <p className="text-sm font-medium mb-2">Selector</p>
              <div className="flex flex-wrap gap-2">
                {monitor.spec.selector.matchLabels &&
                  Object.entries(monitor.spec.selector.matchLabels).map(([key, value]) => (
                    <Badge key={key} variant="secondary">
                      {key}={value}
                    </Badge>
                  ))}
                {monitor.spec.selector.matchExpressions?.map((expr, i) => (
                  <Badge key={i} variant="secondary">
                    {expr.key} {expr.operator}{expr.values?.length ? ` [${expr.values.join(", ")}]` : ""}
                  </Badge>
                ))}
              </div>
            </div>

            {/* Dead Man Switch */}
            {monitor.spec.deadManSwitch && (
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">Dead Man Switch</p>
                  <p className="text-sm text-muted-foreground">
                    Alert if no success within {monitor.spec.deadManSwitch.maxTimeSinceLastSuccess}
                  </p>
                </div>
                <Badge variant={monitor.spec.deadManSwitch.enabled ? "default" : "secondary"}>
                  {monitor.spec.deadManSwitch.enabled ? "Enabled" : "Disabled"}
                </Badge>
              </div>
            )}

            {/* SLA */}
            {monitor.spec.sla && (
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">SLA Monitoring</p>
                  <p className="text-sm text-muted-foreground">
                    Min {monitor.spec.sla.minSuccessRate}% success rate over {monitor.spec.sla.windowDays} days
                  </p>
                </div>
                <Badge variant={monitor.spec.sla.enabled ? "default" : "secondary"}>
                  {monitor.spec.sla.enabled ? "Enabled" : "Disabled"}
                </Badge>
              </div>
            )}
          </CardContent>
        </Card>

        {/* CronJobs table */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Monitored CronJobs</CardTitle>
          </CardHeader>
          <CardContent>
            {monitor.status.cronJobs.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-8">
                No CronJobs match the selector
              </p>
            ) : (
              <div className="rounded-md border">
                <table className="w-full">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-4 py-2 text-left text-sm font-medium">Name</th>
                      <th className="px-4 py-2 text-left text-sm font-medium">Namespace</th>
                      <th className="px-4 py-2 text-left text-sm font-medium">Status</th>
                      <th className="px-4 py-2 text-left text-sm font-medium">Success Rate</th>
                      <th className="px-4 py-2 text-left text-sm font-medium">Last Successful Run</th>
                      <th className="px-4 py-2 text-left text-sm font-medium">Next Run</th>
                    </tr>
                  </thead>
                  <tbody>
                    {monitor.status.cronJobs.map((cj) => (
                      <tr key={`${cj.namespace}/${cj.name}`} className="border-b last:border-0">
                        <td className="px-4 py-2">
                          <Link
                            href={`/cronjob/${cj.namespace}/${cj.name}`}
                            className="font-medium hover:underline"
                          >
                            {cj.name}
                          </Link>
                        </td>
                        <td className="px-4 py-2">
                          <Badge variant="outline">{cj.namespace}</Badge>
                        </td>
                        <td className="px-4 py-2">
                          <StatusIndicator status={cj.status as "healthy" | "warning" | "critical"} />
                        </td>
                        <td className="px-4 py-2">
                          <span className={cj.metrics.successRate < 90 ? "text-amber-600 dark:text-amber-400" : ""}>
                            {cj.metrics.successRate.toFixed(1)}%
                          </span>
                        </td>
                        <td className="px-4 py-2 text-sm text-muted-foreground">
                          {cj.lastSuccessfulTime ? (
                            <RelativeTime date={cj.lastSuccessfulTime} />
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </td>
                        <td className="px-4 py-2 text-sm text-muted-foreground">
                          {cj.nextScheduledTime ? (
                            <RelativeTime date={cj.nextScheduledTime} />
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Last reconcile */}
        <div className="text-sm text-muted-foreground text-center">
          Last reconciled <RelativeTime date={monitor.status.lastReconcileTime} />
        </div>
      </div>
    </div>
  );
}
