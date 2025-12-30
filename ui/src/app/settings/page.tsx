"use client";

import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, XCircle, Database, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  getConfig,
  getHealth,
  getStorageStats,
  triggerPrune,
  type Config,
  type HealthResponse,
  type StorageStatsResponse,
} from "@/lib/api";

function StatusIcon({ ok }: { ok: boolean }) {
  return ok ? (
    <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
  ) : (
    <XCircle className="h-4 w-4 text-red-600 dark:text-red-400" />
  );
}

function SettingRow({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="flex items-center justify-between py-2.5">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className={mono ? "font-mono text-sm" : "text-sm font-medium"}>{value}</span>
    </div>
  );
}

export default function SettingsPage() {
  const [config, setConfig] = useState<Config | null>(null);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [storageStats, setStorageStats] = useState<StorageStatsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const [pruneLoading, setPruneLoading] = useState(false);
  const [pruneDays, setPruneDays] = useState("30");

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const [configData, healthData, storageData] = await Promise.all([
        getConfig(),
        getHealth(),
        getStorageStats(),
      ]);
      setConfig(configData);
      setHealth(healthData);
      setStorageStats(storageData);
    } catch (error) {
      console.error("Failed to fetch config:", error);
      toast.error("Failed to load configuration");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  const handlePrune = async (dryRun: boolean) => {
    setPruneLoading(true);
    try {
      const days = parseInt(pruneDays, 10);
      if (isNaN(days) || days < 1) {
        toast.error("Please enter a valid number of days");
        return;
      }
      const result = await triggerPrune({ olderThanDays: days, dryRun });
      if (result.success) {
        if (dryRun) {
          toast.info(`Dry run: Would delete ${result.recordsPruned} records older than ${days} days`);
        } else {
          toast.success(`Pruned ${result.recordsPruned} records older than ${days} days`);
          setPruneDialogOpen(false);
          fetchData(true);
        }
      } else {
        toast.error(result.message || "Failed to prune records");
      }
    } catch {
      toast.error("Failed to prune execution history");
    } finally {
      setPruneLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(() => fetchData(), 60000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Settings" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <div className="grid gap-6 lg:grid-cols-2">
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
          </div>
        </div>
      </div>
    );
  }

  const storageLocation =
    config?.spec?.storage?.sqlite?.path ??
    config?.spec?.storage?.postgresql?.host ??
    config?.spec?.storage?.mysql?.host ??
    "-";

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Settings"
        description="System configuration and status"
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-6">
        <div className="grid gap-6 lg:grid-cols-2">
          {/* System Status */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base font-medium">System Status</CardTitle>
            </CardHeader>
            <CardContent className="space-y-0">
              <SettingRow
                label="Health"
                value={
                  <span className="flex items-center gap-2">
                    <StatusIcon ok={health?.status === "healthy"} />
                    <span className="capitalize">{health?.status ?? "Unknown"}</span>
                  </span>
                }
              />
              <Separator />
              <SettingRow label="Version" value={health?.version ?? "-"} mono />
              <Separator />
              <SettingRow label="Uptime" value={health?.uptime ?? "-"} />
              <Separator />
              <SettingRow
                label="Leader"
                value={
                  <Badge variant={health?.leader ? "default" : "secondary"} className="text-xs">
                    {health?.leader ? "Yes" : "No"}
                  </Badge>
                }
              />
              <Separator />
              <SettingRow
                label="Storage"
                value={
                  <span className="flex items-center gap-2">
                    <StatusIcon ok={config?.status?.storageStatus === "Ready"} />
                    <span>{config?.status?.storageStatus ?? "Unknown"}</span>
                  </span>
                }
              />
            </CardContent>
          </Card>

          {/* Configuration */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base font-medium">Configuration</CardTitle>
            </CardHeader>
            <CardContent className="space-y-0">
              <SettingRow
                label="Storage Type"
                value={
                  <span className="capitalize">{config?.spec?.storage?.type ?? "-"}</span>
                }
              />
              <Separator />
              <SettingRow
                label="Storage Location"
                value={<span className="truncate max-w-[200px]">{storageLocation}</span>}
                mono
              />
              <Separator />
              <SettingRow
                label="Dead-man Switch Interval"
                value={config?.spec?.deadManSwitchInterval ?? "-"}
              />
              <Separator />
              <SettingRow
                label="SLA Recalculation"
                value={config?.spec?.slaRecalculationInterval ?? "-"}
              />
              <Separator />
              <SettingRow
                label="History Retention"
                value={
                  config?.spec?.historyRetention
                    ? `${config.spec.historyRetention.defaultDays}d (max ${config.spec.historyRetention.maxDays}d)`
                    : "-"
                }
              />
            </CardContent>
          </Card>
        </div>

        {/* Activity Stats */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">Activity (24h)</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-6 md:grid-cols-4">
              <div className="text-center">
                <p className="text-3xl font-semibold">{config?.status?.totalMonitors ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">Monitors</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold">{config?.status?.totalCronJobsWatched ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">CronJobs</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold">{config?.status?.totalAlertsSent24h ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">Alerts Sent</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold">{config?.status?.totalRemediations24h ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">Remediations</p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Storage Management */}
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base font-medium flex items-center gap-2">
                <Database className="h-4 w-4" />
                Data Storage
              </CardTitle>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPruneDialogOpen(true)}
                className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-950"
              >
                <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                Prune Data
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-6 md:grid-cols-5">
              <div className="text-center">
                <p className="text-3xl font-semibold">{storageStats?.executionCount?.toLocaleString() ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">Executions</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold capitalize">{storageStats?.storageType ?? "-"}</p>
                <p className="mt-1 text-sm text-muted-foreground">Storage Type</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold">{storageStats?.retentionDays ?? "-"}</p>
                <p className="mt-1 text-sm text-muted-foreground">Retention Days</p>
              </div>
              <div className="text-center">
                <span className="flex items-center justify-center gap-2">
                  <StatusIcon ok={storageStats?.healthy ?? false} />
                  <span className="text-xl font-medium">{storageStats?.healthy ? "Healthy" : "Unhealthy"}</span>
                </span>
                <p className="mt-1 text-sm text-muted-foreground">Status</p>
              </div>
              <div className="text-center">
                <Badge variant={storageStats?.logStorageEnabled ? "default" : "secondary"}>
                  {storageStats?.logStorageEnabled ? "Enabled" : "Disabled"}
                </Badge>
                <p className="mt-1 text-sm text-muted-foreground">Log Storage</p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Ignored Namespaces - only show if there are any */}
        {config?.spec?.ignoredNamespaces && config.spec.ignoredNamespaces.length > 0 && (
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base font-medium">Ignored Namespaces</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {config.spec.ignoredNamespaces.map((ns) => (
                  <Badge key={ns} variant="outline" className="font-mono text-xs">
                    {ns}
                  </Badge>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Prune Dialog */}
      <Dialog open={pruneDialogOpen} onOpenChange={setPruneDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Prune Execution History</DialogTitle>
            <DialogDescription>
              Delete execution records older than a specified number of days.
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="pruneDays">Delete records older than (days)</Label>
              <Input
                id="pruneDays"
                type="number"
                min="1"
                value={pruneDays}
                onChange={(e) => setPruneDays(e.target.value)}
                placeholder="30"
              />
            </div>
          </div>
          <DialogFooter className="flex-col sm:flex-row gap-2">
            <Button
              variant="outline"
              onClick={() => handlePrune(true)}
              disabled={pruneLoading}
            >
              Dry Run
            </Button>
            <Button
              variant="destructive"
              onClick={() => handlePrune(false)}
              disabled={pruneLoading}
            >
              {pruneLoading ? "Processing..." : "Prune Data"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
