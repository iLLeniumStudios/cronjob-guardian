"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Bell, AlertCircle, History } from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { RelativeTime } from "@/components/relative-time";
import {
  listAlerts,
  getAlertHistory,
  type AlertsResponse,
  type AlertHistoryResponse,
  type Alert,
  type AlertHistoryItem,
} from "@/lib/api";
import { cn } from "@/lib/utils";

const severityStyles = {
  critical: {
    dot: "bg-red-500",
    text: "text-red-700 dark:text-red-400",
    bg: "bg-red-500/5",
    badge: "bg-red-500/10 text-red-700 dark:text-red-400",
  },
  warning: {
    dot: "bg-amber-500",
    text: "text-amber-700 dark:text-amber-400",
    bg: "bg-amber-500/5",
    badge: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  },
  info: {
    dot: "bg-blue-500",
    text: "text-blue-700 dark:text-blue-400",
    bg: "bg-blue-500/5",
    badge: "bg-blue-500/10 text-blue-700 dark:text-blue-400",
  },
};

export default function AlertsPage() {
  const [activeAlerts, setActiveAlerts] = useState<AlertsResponse | null>(null);
  const [alertHistory, setAlertHistory] = useState<AlertHistoryResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const [activeData, historyData] = await Promise.all([
        listAlerts(),
        getAlertHistory({ limit: 50 }),
      ]);
      setActiveAlerts(activeData);
      setAlertHistory(historyData);
    } catch (error) {
      console.error("Failed to fetch alerts:", error);
      toast.error("Failed to load alerts");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(() => fetchData(), 5000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Alerts" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <div className="grid gap-4 md:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          <Skeleton className="h-96" />
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Alerts"
        description="Active alerts and history"
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-6">
        {/* Summary */}
        <div className="grid gap-4 md:grid-cols-3">
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-3">
                <div className="rounded bg-red-500/10 p-2.5">
                  <AlertCircle className="h-5 w-5 text-red-600 dark:text-red-400" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Critical</p>
                  <p className="text-2xl font-semibold">
                    {activeAlerts?.bySeverity.critical ?? 0}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-3">
                <div className="rounded bg-amber-500/10 p-2.5">
                  <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Warning</p>
                  <p className="text-2xl font-semibold">
                    {activeAlerts?.bySeverity.warning ?? 0}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <div className="flex items-center gap-3">
                <div className="rounded bg-blue-500/10 p-2.5">
                  <AlertCircle className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Info</p>
                  <p className="text-2xl font-semibold">
                    {activeAlerts?.bySeverity.info ?? 0}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Tabs */}
        <Tabs defaultValue="active">
          <TabsList>
            <TabsTrigger value="active" className="gap-2">
              <Bell className="h-4 w-4" />
              Active ({activeAlerts?.total ?? 0})
            </TabsTrigger>
            <TabsTrigger value="history" className="gap-2">
              <History className="h-4 w-4" />
              History
            </TabsTrigger>
          </TabsList>

          <TabsContent value="active" className="mt-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-medium">Active Alerts</CardTitle>
              </CardHeader>
              <CardContent>
                {activeAlerts?.items.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12 text-center">
                    <Bell className="mb-4 h-12 w-12 text-muted-foreground/50" />
                    <p className="text-lg font-medium">No active alerts</p>
                    <p className="text-sm text-muted-foreground">
                      All systems operating normally
                    </p>
                  </div>
                ) : (
                  <ScrollArea className="h-[500px]">
                    <div className="space-y-3 pr-3">
                      {activeAlerts?.items.map((alert) => (
                        <ActiveAlertCard key={alert.id} alert={alert} />
                      ))}
                    </div>
                  </ScrollArea>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="history" className="mt-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-medium">Alert History</CardTitle>
              </CardHeader>
              <CardContent>
                {alertHistory?.items.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12 text-center">
                    <History className="mb-4 h-12 w-12 text-muted-foreground/50" />
                    <p className="text-lg font-medium">No alert history</p>
                    <p className="text-sm text-muted-foreground">
                      Historical alerts will appear here
                    </p>
                  </div>
                ) : (
                  <ScrollArea className="h-[500px]">
                    <div className="space-y-3 pr-3">
                      {alertHistory?.items.map((alert) => (
                        <HistoryAlertCard key={alert.id} alert={alert} />
                      ))}
                    </div>
                  </ScrollArea>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}

function ActiveAlertCard({ alert }: { alert: Alert }) {
  const styles = severityStyles[alert.severity] || severityStyles.info;

  return (
    <Link
      href={`/cronjob/${alert.cronjob.namespace}/${alert.cronjob.name}`}
      className={cn("block rounded border p-4 transition-colors hover:bg-muted/50", styles.bg)}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <span className={cn("mt-1.5 h-2.5 w-2.5 flex-shrink-0 rounded-full", styles.dot)} />
          <div>
            <div className="flex items-center gap-2">
              <Badge variant="outline" className={cn("text-xs", styles.badge)}>
                {alert.severity}
              </Badge>
              <span className="text-xs text-muted-foreground">{alert.type}</span>
            </div>
            <p className="mt-1 font-medium">{alert.title}</p>
            <p className="mt-1 text-sm text-muted-foreground">{alert.message}</p>
            <p className="mt-2 text-xs text-muted-foreground">
              {alert.cronjob.namespace}/{alert.cronjob.name}
            </p>
          </div>
        </div>
        <div className="text-right text-xs text-muted-foreground">
          <p>Since</p>
          <RelativeTime date={alert.since} showTooltip={false} />
        </div>
      </div>
    </Link>
  );
}

function HistoryAlertCard({ alert }: { alert: AlertHistoryItem }) {
  const styles = severityStyles[alert.severity] || severityStyles.info;

  return (
    <div className="rounded border p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <span className={cn("mt-1.5 h-2.5 w-2.5 flex-shrink-0 rounded-full", styles.dot)} />
          <div>
            <div className="flex items-center gap-2">
              <Badge variant="outline" className={cn("text-xs", styles.badge)}>
                {alert.severity}
              </Badge>
              <span className="text-xs text-muted-foreground">{alert.type}</span>
              {alert.resolvedAt && (
                <Badge variant="outline" className="text-xs text-emerald-600">
                  Resolved
                </Badge>
              )}
            </div>
            <p className="mt-1 font-medium">{alert.title}</p>
            <p className="mt-1 text-sm text-muted-foreground">{alert.message}</p>
            <p className="mt-2 text-xs text-muted-foreground">
              {alert.cronjob.namespace}/{alert.cronjob.name}
            </p>
            {alert.channelsNotified.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-1">
                {alert.channelsNotified.map((channel) => (
                  <Badge key={channel} variant="secondary" className="text-xs">
                    {channel}
                  </Badge>
                ))}
              </div>
            )}
          </div>
        </div>
        <div className="text-right text-xs text-muted-foreground">
          <p>Occurred</p>
          <RelativeTime date={alert.occurredAt} showTooltip={false} />
          {alert.resolvedAt && (
            <>
              <p className="mt-2">Resolved</p>
              <RelativeTime date={alert.resolvedAt} showTooltip={false} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
