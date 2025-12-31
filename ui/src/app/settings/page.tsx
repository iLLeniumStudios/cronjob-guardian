"use client";

import { useCallback, useState } from "react";
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
import { useFetchData } from "@/hooks/use-fetch-data";
import {
  getConfig,
  getHealth,
  getStats,
  getStorageStats,
  triggerPrune,
  type Config,
  type HealthResponse,
  type StatsResponse,
  type StorageStatsResponse,
} from "@/lib/api";

// Format nanoseconds duration to human readable string
function formatDuration(nanoseconds: number | undefined): string {
  if (!nanoseconds) return "-";
  const seconds = nanoseconds / 1_000_000_000;
  if (seconds < 60) return `${seconds}s`;
  const minutes = seconds / 60;
  if (minutes < 60) return `${Math.round(minutes)}m`;
  const hours = minutes / 60;
  return `${Math.round(hours)}h`;
}

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

interface SettingsData {
  config: Config;
  health: HealthResponse;
  stats: StatsResponse;
  storageStats: StorageStatsResponse;
}

export default function SettingsPage() {
  const [pruneDialogOpen, setPruneDialogOpen] = useState(false);
  const [pruneLoading, setPruneLoading] = useState(false);
  const [pruneDays, setPruneDays] = useState("30");

  const fetchSettingsData = useCallback(async (): Promise<SettingsData> => {
    const [config, health, stats, storageStats] = await Promise.all([
      getConfig(),
      getHealth(),
      getStats(),
      getStorageStats(),
    ]);
    return { config, health, stats, storageStats };
  }, []);

  const { data, isLoading, isRefreshing, refetch } = useFetchData(fetchSettingsData);

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
          refetch();
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

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Settings" />
        <div className="flex-1 space-y-6 overflow-auto p-4 md:p-6">
          <div className="grid gap-6 lg:grid-cols-2">
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
          </div>
        </div>
      </div>
    );
  }

  const config = data?.config;
  const health = data?.health;
  const stats = data?.stats;
  const storageStats = data?.storageStats;

  const storageLocation =
    config?.storage?.sqlite?.path ??
    config?.storage?.postgres?.host ??
    config?.storage?.mysql?.host ??
    "-";

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Settings"
        description="System configuration and status"
        onRefresh={refetch}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-4 md:p-6">
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
                    <StatusIcon ok={health?.storage === "connected"} />
                    <span className="capitalize">{health?.storage ?? "Unknown"}</span>
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
                  <span className="capitalize">{config?.storage?.type ?? "-"}</span>
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
                value={formatDuration(config?.scheduler?.deadManSwitchInterval)}
              />
              <Separator />
              <SettingRow
                label="SLA Recalculation"
                value={formatDuration(config?.scheduler?.slaRecalculationInterval)}
              />
              <Separator />
              <SettingRow
                label="History Retention"
                value={
                  config?.historyRetention
                    ? `${config.historyRetention.defaultDays}d (max ${config.historyRetention.maxDays}d)`
                    : "-"
                }
              />
              <Separator />
              <SettingRow
                label="Log Storage"
                value={
                  <Badge variant={config?.storage?.logStorageEnabled ? "default" : "secondary"}>
                    {config?.storage?.logStorageEnabled ? "Enabled" : "Disabled"}
                  </Badge>
                }
              />
              <Separator />
              <SettingRow
                label="Max Alerts/min"
                value={config?.rateLimits?.maxAlertsPerMinute ?? "-"}
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
            <div className="grid grid-cols-3 gap-6">
              <div className="text-center">
                <p className="text-3xl font-semibold">{stats?.totalMonitors ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">Monitors</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold">{stats?.totalCronJobs ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">CronJobs</p>
              </div>
              <div className="text-center">
                <p className="text-3xl font-semibold">{stats?.executionsRecorded24h ?? 0}</p>
                <p className="mt-1 text-sm text-muted-foreground">Executions (24h)</p>
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
