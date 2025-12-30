"use client";

import { useCallback, useState, useEffect, useSyncExternalStore } from "react";
import Link from "next/link";
import { Timer, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { RelativeTime } from "@/components/relative-time";
import { StatusIndicator } from "@/components/status-indicator";
import { listMonitors, type MonitorsResponse, type Monitor } from "@/lib/api";
import { MonitorDetailClient } from "./monitor-detail";

// Subscribe function for useSyncExternalStore (no-op since pathname won't change without navigation)
const emptySubscribe = () => () => {};

// Check if we're on a detail route using useSyncExternalStore for proper SSR/hydration handling
function useIsDetailRoute() {
  return useSyncExternalStore(
    emptySubscribe,
    () => {
      const path = window.location.pathname;
      const parts = path.split("/").filter(Boolean);
      // /monitors/namespace/name has 3 parts
      return parts.length >= 3 && parts[0] === "monitors";
    },
    () => false // Server snapshot - default to list view on SSR
  );
}

export default function MonitorsPage() {
  const isDetailRoute = useIsDetailRoute();

  // If this is a detail route, render the detail view
  if (isDetailRoute) {
    return <MonitorDetailClient />;
  }

  // Otherwise render the list view
  return <MonitorsListView />;
}

function MonitorsListView() {
  const [monitors, setMonitors] = useState<MonitorsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const data = await listMonitors();
      setMonitors(data);
    } catch (error) {
      console.error("Failed to fetch monitors:", error);
      toast.error("Failed to load monitors");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(() => fetchData(), 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Monitors" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-48" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Monitors"
        description="CronJobMonitor resources"
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-6">
        {monitors?.items.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Timer className="mb-4 h-12 w-12 text-muted-foreground/50" />
              <p className="text-lg font-medium">No monitors configured</p>
              <p className="text-sm text-muted-foreground">
                Create a CronJobMonitor resource to start monitoring your CronJobs
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {monitors?.items.map((monitor) => (
              <MonitorCard key={`${monitor.namespace}/${monitor.name}`} monitor={monitor} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function MonitorCard({ monitor }: { monitor: Monitor }) {
  const totalJobs = monitor.summary.healthy + monitor.summary.warning + monitor.summary.critical;

  return (
    <Link href={`/monitors/${monitor.namespace}/${monitor.name}`}>
      <Card className="h-full transition-colors hover:bg-muted/50">
        <CardHeader className="pb-2">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3">
              <div className="rounded bg-primary/10 p-2">
                <Timer className="h-4 w-4 text-primary" />
              </div>
              <div>
                <CardTitle className="text-base font-medium">{monitor.name}</CardTitle>
                <Badge variant="outline" className="mt-1 font-normal">
                  {monitor.namespace}
                </Badge>
              </div>
            </div>
            <Badge
              variant="outline"
              className={
                monitor.phase === "Active"
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-muted-foreground"
              }
            >
              {monitor.phase}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* CronJob Count */}
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">CronJobs Matched</span>
            <span className="font-medium">{monitor.cronJobCount}</span>
          </div>

          {/* Health Summary */}
          <div className="space-y-2">
            <p className="text-sm text-muted-foreground">Health Summary</p>
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-1.5">
                <StatusIndicator status="healthy" size="sm" />
                <span className="text-sm font-medium">{monitor.summary.healthy}</span>
              </div>
              <div className="flex items-center gap-1.5">
                <StatusIndicator status="warning" size="sm" />
                <span className="text-sm font-medium">{monitor.summary.warning}</span>
              </div>
              <div className="flex items-center gap-1.5">
                <StatusIndicator status="critical" size="sm" />
                <span className="text-sm font-medium">{monitor.summary.critical}</span>
              </div>
            </div>
            {/* Progress bar */}
            {totalJobs > 0 && (
              <div className="flex h-2 overflow-hidden rounded-full bg-muted">
                {monitor.summary.healthy > 0 && (
                  <div
                    className="bg-emerald-500"
                    style={{ width: `${(monitor.summary.healthy / totalJobs) * 100}%` }}
                  />
                )}
                {monitor.summary.warning > 0 && (
                  <div
                    className="bg-amber-500"
                    style={{ width: `${(monitor.summary.warning / totalJobs) * 100}%` }}
                  />
                )}
                {monitor.summary.critical > 0 && (
                  <div
                    className="bg-red-500"
                    style={{ width: `${(monitor.summary.critical / totalJobs) * 100}%` }}
                  />
                )}
              </div>
            )}
          </div>

          {/* Active Alerts */}
          {monitor.activeAlerts > 0 && (
            <div className="flex items-center gap-2 rounded bg-red-500/10 px-3 py-2 text-sm">
              <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400" />
              <span className="text-red-700 dark:text-red-400">
                {monitor.activeAlerts} active alert{monitor.activeAlerts > 1 ? "s" : ""}
              </span>
            </div>
          )}

          {/* Last Reconcile */}
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">Last Reconcile</span>
            <RelativeTime date={monitor.lastReconcile} />
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
