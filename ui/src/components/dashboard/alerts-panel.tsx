"use client";

import Link from "next/link";
import { AlertCircle, Bell } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { RelativeTime } from "@/components/relative-time";
import { EmptyState } from "@/components/empty-state";
import type { AlertsResponse, Alert } from "@/lib/api";
import { cn } from "@/lib/utils";
import { SEVERITY_STYLES, SEVERITY_ORDER, type Severity } from "@/lib/constants";

interface AlertsPanelProps {
  alerts: AlertsResponse | null;
  isLoading: boolean;
}

export function AlertsPanel({ alerts, isLoading }: AlertsPanelProps) {
  if (isLoading) {
    return (
      <Card className="h-full">
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">Active Alerts</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  // Deduplicate alerts by ID to prevent duplicates from appearing
  const uniqueAlerts = alerts?.items?.reduce((acc, alert) => {
    const key = alert.id || `${alert.cronjob.namespace}-${alert.cronjob.name}-${alert.type}`;
    if (!acc.has(key)) {
      acc.set(key, alert);
    }
    return acc;
  }, new Map<string, Alert>());

  const sortedAlerts = [...(uniqueAlerts?.values() ?? [])].sort((a, b) => {
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

  return (
    <Card className="h-full">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-medium">Active Alerts</CardTitle>
          {alerts && alerts.total > 0 && (
            <span className="flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-xs font-medium text-white">
              {alerts.total}
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {sortedAlerts.length === 0 ? (
          <EmptyState
            icon={Bell}
            title="No active alerts"
            description="All systems operating normally"
            className="h-[300px] xl:h-[520px]"
          />
        ) : (
          <ScrollArea className="h-[300px] xl:h-[520px]">
            <div className="space-y-2 pr-3">
              {sortedAlerts.map((alert) => (
                <AlertItem key={alert.id || `${alert.cronjob.namespace}-${alert.cronjob.name}-${alert.type}`} alert={alert} />
              ))}
            </div>
          </ScrollArea>
        )}
      </CardContent>
    </Card>
  );
}

function AlertItem({ alert }: { alert: Alert }) {
  // Normalize severity to lowercase and default to warning for unknown values
  const severity = (alert.severity?.toLowerCase() || "warning") as Severity;
  const styles = SEVERITY_STYLES[severity] || SEVERITY_STYLES.warning;

  return (
    <Link
      href={`/cronjob/${alert.cronjob.namespace}/${alert.cronjob.name}`}
      className={cn(
        "block rounded border p-3 transition-colors hover:bg-muted/50",
        styles.bg
      )}
    >
      <div className="flex items-start gap-2">
        <span className={cn("mt-1.5 h-2 w-2 flex-shrink-0 rounded-full", styles.dot)} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <AlertCircle className={cn("h-3.5 w-3.5 flex-shrink-0", styles.text)} />
            <span className={cn("text-xs font-medium capitalize", styles.text)}>
              {alert.type}
            </span>
          </div>
          <p className="mt-1 text-sm font-medium truncate">{alert.title}</p>
          <p className="mt-0.5 text-xs text-muted-foreground truncate">
            {alert.cronjob.namespace}/{alert.cronjob.name}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            <RelativeTime date={alert.since} showTooltip={false} />
          </p>
        </div>
      </div>
    </Link>
  );
}
