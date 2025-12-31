"use client";

import Link from "next/link";
import { AlertCircle, Bell } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { RelativeTime } from "@/components/relative-time";
import type { AlertsResponse, Alert } from "@/lib/api";
import { cn } from "@/lib/utils";

interface AlertsPanelProps {
  alerts: AlertsResponse | null;
  isLoading: boolean;
}

const severityStyles = {
  critical: {
    dot: "bg-red-500",
    text: "text-red-700 dark:text-red-400",
    bg: "bg-red-500/5",
  },
  warning: {
    dot: "bg-amber-500",
    text: "text-amber-700 dark:text-amber-400",
    bg: "bg-amber-500/5",
  },
  info: {
    dot: "bg-blue-500",
    text: "text-blue-700 dark:text-blue-400",
    bg: "bg-blue-500/5",
  },
};

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
    const severityOrder = { critical: 0, warning: 1, info: 2 };
    return severityOrder[a.severity] - severityOrder[b.severity];
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
          <div className="flex h-[520px] flex-col items-center justify-center text-center rounded-lg border border-dashed">
            <Bell className="mb-2 h-8 w-8 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">No active alerts</p>
            <p className="text-xs text-muted-foreground/70">
              All systems operating normally
            </p>
          </div>
        ) : (
          <ScrollArea className="h-[520px]">
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
  const normalizedSeverity = (alert.severity?.toLowerCase() || "warning") as keyof typeof severityStyles;
  const styles = severityStyles[normalizedSeverity] || severityStyles.warning;

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
