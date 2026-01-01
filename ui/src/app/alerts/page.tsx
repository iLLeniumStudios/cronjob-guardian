"use client";

import { useCallback, useMemo } from "react";
import Link from "next/link";
import { Bell, AlertCircle, History } from "lucide-react";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ScrollArea } from "@/components/ui/scroll-area";
import { RelativeTime } from "@/components/relative-time";
import { EmptyState } from "@/components/empty-state";
import { StatCard } from "@/components/stat-card";
import { PageSkeleton } from "@/components/page-skeleton";
import { SuggestedFix } from "@/components/suggested-fix";
import { useFetchData } from "@/hooks/use-fetch-data";
import {
  listAlerts,
  getAlertHistory,
  type AlertsResponse,
  type AlertHistoryResponse,
  type Alert,
  type AlertHistoryItem,
} from "@/lib/api";
import { cn } from "@/lib/utils";
import { SEVERITY_STYLES, SEVERITY_ORDER, type Severity } from "@/lib/constants";

interface AlertsData {
  activeAlerts: AlertsResponse;
  alertHistory: AlertHistoryResponse;
}

export default function AlertsPage() {
  const fetchAlertsData = useCallback(async (): Promise<AlertsData> => {
    const [activeAlerts, alertHistory] = await Promise.all([
      listAlerts(),
      getAlertHistory({ limit: 50 }),
    ]);
    return { activeAlerts, alertHistory };
  }, []);

  const { data, isLoading, isRefreshing, refetch } = useFetchData(fetchAlertsData);

  const activeAlerts = data?.activeAlerts;
  const alertHistory = data?.alertHistory;

  // Sort active alerts: critical first, then by namespace, then by name, then by type
  const sortedActiveAlerts = useMemo(() => {
    const items = activeAlerts?.items;
    if (!items) return [];
    return [...items].sort((a, b) => {
      const aSeverity = (a.severity || "info") as Severity;
      const bSeverity = (b.severity || "info") as Severity;
      // Primary sort: severity (critical first)
      const severityDiff = SEVERITY_ORDER[aSeverity] - SEVERITY_ORDER[bSeverity];
      if (severityDiff !== 0) return severityDiff;
      // Secondary sort: namespace
      const namespaceDiff = a.cronjob.namespace.localeCompare(b.cronjob.namespace);
      if (namespaceDiff !== 0) return namespaceDiff;
      // Tertiary sort: name
      const nameDiff = a.cronjob.name.localeCompare(b.cronjob.name);
      if (nameDiff !== 0) return nameDiff;
      // Quaternary sort: alert type (for stability)
      return (a.type || "").localeCompare(b.type || "");
    });
  }, [activeAlerts?.items]);

  if (isLoading) {
    return <PageSkeleton title="Alerts" variant="table" />;
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Alerts"
        description="Active alerts and history"
        onRefresh={refetch}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-4 md:p-6">
        {/* Summary */}
        <div className="grid gap-4 md:grid-cols-3">
          <StatCard
            icon={AlertCircle}
            iconColor="red"
            label="Critical"
            value={activeAlerts?.bySeverity.critical ?? 0}
          />
          <StatCard
            icon={AlertCircle}
            iconColor="amber"
            label="Warning"
            value={activeAlerts?.bySeverity.warning ?? 0}
          />
          <StatCard
            icon={AlertCircle}
            iconColor="blue"
            label="Info"
            value={activeAlerts?.bySeverity.info ?? 0}
          />
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
                {sortedActiveAlerts.length === 0 ? (
                  <EmptyState
                    icon={Bell}
                    title="No active alerts"
                    description="All systems operating normally"
                    bordered={false}
                  />
                ) : (
                  <ScrollArea className="h-[500px]">
                    <div className="space-y-3 pr-3">
                      {sortedActiveAlerts.map((alert) => (
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
                  <EmptyState
                    icon={History}
                    title="No alert history"
                    description="Historical alerts will appear here"
                    bordered={false}
                  />
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
  const severity = (alert.severity || "info") as Severity;
  const styles = SEVERITY_STYLES[severity] || SEVERITY_STYLES.info;

  return (
    <div className={cn("rounded border p-4", styles.bg)}>
      <Link
        href={`/cronjob/${alert.cronjob.namespace}/${alert.cronjob.name}`}
        className="block transition-colors hover:bg-muted/30 -m-4 p-4 rounded"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <span className={cn("mt-1.5 h-2.5 w-2.5 flex-shrink-0 rounded-full", styles.dot)} />
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="outline" className={cn("text-xs", styles.badge)}>
                  {alert.severity}
                </Badge>
                <span className="text-xs text-muted-foreground">{alert.type}</span>
                {/* Show exit code if present */}
                {alert.context?.exitCode !== undefined && alert.context.exitCode !== 0 && (
                  <Badge variant="outline" className="text-xs bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400">
                    Exit {alert.context.exitCode}
                  </Badge>
                )}
                {/* Show reason if present */}
                {alert.context?.reason && (
                  <Badge variant="outline" className="text-xs bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400">
                    {alert.context.reason}
                  </Badge>
                )}
              </div>
              <p className="mt-1 font-medium">{alert.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{alert.message}</p>
              <p className="mt-2 text-xs text-muted-foreground">
                {alert.cronjob.namespace}/{alert.cronjob.name}
              </p>
            </div>
          </div>
          <div className="text-right text-xs text-muted-foreground shrink-0">
            <p>Since</p>
            <RelativeTime date={alert.since} showTooltip={false} />
          </div>
        </div>
      </Link>
      {/* Suggested fix displayed below the link area */}
      {alert.context?.suggestedFix && (
        <div className="mt-3 pt-3 border-t">
          <SuggestedFix
            fix={alert.context.suggestedFix}
            exitCode={alert.context.exitCode}
            reason={alert.context.reason}
            namespace={alert.cronjob.namespace}
            name={alert.cronjob.name}
          />
        </div>
      )}
    </div>
  );
}

function HistoryAlertCard({ alert }: { alert: AlertHistoryItem }) {
  const severity = (alert.severity || "info") as Severity;
  const styles = SEVERITY_STYLES[severity] || SEVERITY_STYLES.info;

  return (
    <div className="rounded border p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <span className={cn("mt-1.5 h-2.5 w-2.5 flex-shrink-0 rounded-full", styles.dot)} />
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <Badge variant="outline" className={cn("text-xs", styles.badge)}>
                {alert.severity}
              </Badge>
              <span className="text-xs text-muted-foreground">{alert.type}</span>
              {alert.resolvedAt && (
                <Badge variant="outline" className="text-xs text-emerald-600">
                  Resolved
                </Badge>
              )}
              {/* Show exit code if present */}
              {alert.exitCode !== undefined && alert.exitCode !== 0 && (
                <Badge variant="outline" className="text-xs bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400">
                  Exit {alert.exitCode}
                </Badge>
              )}
              {/* Show reason if present */}
              {alert.reason && (
                <Badge variant="outline" className="text-xs bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400">
                  {alert.reason}
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
            {/* Show suggested fix if present (compact mode for history) */}
            {alert.suggestedFix && (
              <div className="mt-3">
                <SuggestedFix
                  fix={alert.suggestedFix}
                  exitCode={alert.exitCode}
                  reason={alert.reason}
                  namespace={alert.cronjob.namespace}
                  name={alert.cronjob.name}
                  compact
                />
              </div>
            )}
          </div>
        </div>
        <div className="text-right text-xs text-muted-foreground shrink-0">
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
