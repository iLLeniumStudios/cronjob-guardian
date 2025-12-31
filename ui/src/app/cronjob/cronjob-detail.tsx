"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Play,
  Pause,
  PlayCircle,
  Copy,
  Clock,
  Calendar,
  Timer,
  Trash2,
  AlertTriangle,
  Loader2,
  Container,
} from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { StatusBadge } from "@/components/status-indicator";
import { MetricsCards } from "@/components/cronjob/metrics-cards";
import { DurationChart } from "@/components/cronjob/duration-chart";
import { SuccessRateChart } from "@/components/cronjob/success-rate-chart";
import { HealthHeatmap } from "@/components/cronjob/health-heatmap";
import { ExecutionHistory } from "@/components/cronjob/execution-history";
import { ExportButton } from "@/components/export/export-button";
import { exportExecutionsToCSV } from "@/lib/export/csv";
import { generateCronJobPDFReport } from "@/lib/export/pdf";
import {
  getCronJob,
  getExecutions,
  triggerCronJob,
  suspendCronJob,
  resumeCronJob,
  deleteHistory,
  type CronJobDetail,
  type ExecutionHistoryResponse,
} from "@/lib/api";

export function CronJobDetailClient() {
  const router = useRouter();

  // Parse namespace/name from URL path client-side
  const [namespace, setNamespace] = useState("");
  const [name, setName] = useState("");

  useEffect(() => {
    // Parse URL: /cronjob/namespace/name
    const path = window.location.pathname;
    const parts = path.split("/").filter(Boolean);
    if (parts.length >= 3 && parts[0] === "cronjob") {
      setNamespace(parts[1]);
      setName(parts[2]);
    }
  }, []);

  const [cronJob, setCronJob] = useState<CronJobDetail | null>(null);
  const [executions, setExecutions] = useState<ExecutionHistoryResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const fetchData = useCallback(
    async (showRefreshing = false) => {
      if (!namespace || !name) {
        setIsLoading(false);
        return;
      }
      if (showRefreshing) setIsRefreshing(true);
      try {
        const [cronJobData, executionsData] = await Promise.all([
          getCronJob(namespace, name),
          getExecutions(namespace, name, { limit: 20 }),
        ]);
        setCronJob(cronJobData);
        setExecutions(executionsData);
      } catch (error) {
        console.error("Failed to fetch cronjob data:", error);
        toast.error("Failed to load CronJob data");
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

  const handleTrigger = async () => {
    setActionLoading("trigger");
    try {
      const result = await triggerCronJob(namespace, name);
      if (result.success) {
        toast.success(`Job triggered: ${result.jobName}`);
        fetchData(true);
      } else {
        toast.error(result.error || "Failed to trigger job");
      }
    } catch {
      toast.error("Failed to trigger job");
    } finally {
      setActionLoading(null);
    }
  };

  const handleSuspend = async () => {
    setActionLoading("suspend");
    try {
      const result = await suspendCronJob(namespace, name);
      if (result.success) {
        toast.success("CronJob suspended");
        fetchData(true);
      } else {
        toast.error(result.error || "Failed to suspend");
      }
    } catch {
      toast.error("Failed to suspend CronJob");
    } finally {
      setActionLoading(null);
    }
  };

  const handleResume = async () => {
    setActionLoading("resume");
    try {
      const result = await resumeCronJob(namespace, name);
      if (result.success) {
        toast.success("CronJob resumed");
        fetchData(true);
      } else {
        toast.error(result.error || "Failed to resume");
      }
    } catch {
      toast.error("Failed to resume CronJob");
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeleteHistory = async () => {
    setActionLoading("delete");
    try {
      const result = await deleteHistory(namespace, name);
      if (result.success) {
        toast.success(`Deleted ${result.recordsDeleted} execution records`);
        setDeleteDialogOpen(false);
        fetchData(true);
      } else {
        toast.error(result.message || "Failed to delete history");
      }
    } catch {
      toast.error("Failed to delete execution history");
    } finally {
      setActionLoading(null);
    }
  };

  const handleExportCSV = () => {
    if (executions?.items && executions.items.length > 0) {
      exportExecutionsToCSV(executions.items, cronJob?.name || name);
      toast.success("CSV exported successfully");
    } else {
      toast.error("No execution data to export");
    }
  };

  const handleExportPDF = () => {
    if (cronJob && executions?.items) {
      generateCronJobPDFReport({
        title: `CronJob Report: ${cronJob.name}`,
        cronJobName: cronJob.name,
        namespace: cronJob.namespace,
        generatedAt: new Date(),
        metrics: cronJob.metrics,
        recentExecutions: executions.items,
        alerts: cronJob.activeAlerts?.map((a) => ({
          severity: a.severity,
          title: a.title,
          message: a.message,
          since: a.since,
        })),
      });
    } else {
      toast.error("No data available for PDF export");
    }
  };

  // If no URL params, show a message
  if (!namespace || !name) {
    return (
      <div className="flex h-full flex-col">
        <Header title="CronJob Details" />
        <div className="flex flex-1 items-center justify-center">
          <div className="text-center">
            <p className="text-lg font-medium">Select a CronJob</p>
            <p className="text-muted-foreground">
              Choose a CronJob from the dashboard to view details
            </p>
            <Button variant="outline" className="mt-4" onClick={() => router.push("/")}>
              Go to Dashboard
            </Button>
          </div>
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="CronJob Details" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <Skeleton className="h-24 w-full" />
          <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          <div className="grid gap-4 md:gap-6 lg:grid-cols-2">
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
          </div>
          <Skeleton className="h-96" />
        </div>
      </div>
    );
  }

  if (!cronJob) {
    return (
      <div className="flex h-full flex-col">
        <Header title="CronJob Not Found" />
        <div className="flex flex-1 items-center justify-center">
          <div className="text-center">
            <p className="text-lg font-medium">CronJob not found</p>
            <p className="text-muted-foreground">
              {namespace}/{name} does not exist
            </p>
            <Button variant="outline" className="mt-4" onClick={() => router.push("/")}>
              Back to Dashboard
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title={cronJob.name}
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
        actions={
          <div className="flex items-center gap-2">
            <ExportButton
              onExportCSV={handleExportCSV}
              onExportPDF={handleExportPDF}
              isLoading={!!actionLoading}
            />
            <Button
              variant="outline"
              size="sm"
              onClick={handleTrigger}
              disabled={!!actionLoading}
            >
              <PlayCircle className="mr-1.5 h-4 w-4" />
              Trigger Now
            </Button>
            {cronJob.suspended ? (
              <Button
                variant="outline"
                size="sm"
                onClick={handleResume}
                disabled={!!actionLoading}
              >
                <Play className="mr-1.5 h-4 w-4" />
                Resume
              </Button>
            ) : (
              <Button
                variant="outline"
                size="sm"
                onClick={handleSuspend}
                disabled={!!actionLoading}
              >
                <Pause className="mr-1.5 h-4 w-4" />
                Suspend
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => setDeleteDialogOpen(true)}
              disabled={!!actionLoading}
              className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-950"
            >
              <Trash2 className="mr-1.5 h-4 w-4" />
              Clear History
            </Button>
          </div>
        }
      />

      <div className="flex-1 space-y-4 md:space-y-6 overflow-auto p-4 md:p-6">
        {/* Back button and header info */}
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-3 md:gap-4">
            <Button
              variant="ghost"
              size="icon"
              onClick={() => router.push("/")}
              className="mt-0.5 h-8 w-8 md:h-9 md:w-9 cursor-pointer"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <div>
              <div className="flex flex-wrap items-center gap-2 md:gap-3">
                <h2 className="text-lg md:text-xl font-semibold">{cronJob.name}</h2>
                <StatusBadge status={cronJob.status} />
                {cronJob.suspended && (
                  <Badge variant="secondary">Suspended</Badge>
                )}
              </div>
              <div className="mt-1 flex items-center gap-4 text-sm text-muted-foreground">
                <span className="flex items-center gap-1">
                  <Badge variant="outline">{cronJob.namespace}</Badge>
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Info bar */}
        <Card>
          <CardContent className="flex flex-wrap items-center gap-4 md:gap-6 py-3">
            <div className="flex items-center gap-2">
              <Calendar className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm">
                <span className="text-muted-foreground">Schedule:</span>{" "}
                <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                  {cronJob.schedule}
                </code>
              </span>
            </div>
            {cronJob.timezone && (
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm">
                  <span className="text-muted-foreground">Timezone:</span>{" "}
                  {cronJob.timezone}
                </span>
              </div>
            )}
            {cronJob.monitorRef && (
              <div className="flex items-center gap-2">
                <Timer className="h-4 w-4 text-muted-foreground" />
                <span className="text-sm">
                  <span className="text-muted-foreground">Monitor:</span>{" "}
                  <Link
                    href={`/monitors/${cronJob.monitorRef.namespace}/${cronJob.monitorRef.name}`}
                    className="hover:underline"
                  >
                    {cronJob.monitorRef.name}
                  </Link>
                </span>
              </div>
            )}
            <button
              onClick={() => {
                const cmd = `kubectl get cronjob ${cronJob.name} -n ${cronJob.namespace} -o yaml`;
                navigator.clipboard.writeText(cmd);
                toast.success("kubectl command copied to clipboard");
              }}
              className="ml-auto flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
            >
              <Copy className="h-3.5 w-3.5" />
              Copy kubectl
            </button>
          </CardContent>
        </Card>

        {/* Active Jobs */}
        {cronJob.activeJobs && cronJob.activeJobs.length > 0 && (
          <Card className="border-blue-200 dark:border-blue-800 bg-blue-50/50 dark:bg-blue-950/20">
            <CardHeader className="py-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <Loader2 className="h-4 w-4 text-blue-500 animate-spin" />
                Running Jobs ({cronJob.activeJobs.length})
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="space-y-3">
                {cronJob.activeJobs.map((job) => (
                  <div
                    key={job.name}
                    className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 rounded-lg border bg-background p-3"
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <Play className="h-4 w-4 text-blue-500 fill-blue-500 shrink-0" />
                      <span className="font-mono text-sm truncate">{job.name}</span>
                    </div>
                    <div className="flex flex-wrap items-center gap-2 sm:gap-3 text-sm text-muted-foreground">
                      {job.runningDuration && (
                        <Badge variant="outline" className="bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20">
                          <Clock className="h-3 w-3 mr-1" />
                          {job.runningDuration}
                        </Badge>
                      )}
                      {job.podPhase && (
                        <Badge variant="outline" className={
                          job.podPhase === "Running"
                            ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20"
                            : job.podPhase === "Pending"
                            ? "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/20"
                            : "bg-slate-500/10 text-slate-700 dark:text-slate-400 border-slate-500/20"
                        }>
                          {job.podPhase}
                        </Badge>
                      )}
                      {job.podName && (
                        <span className="flex items-center gap-1 text-xs">
                          <Container className="h-3 w-3" />
                          <span className="font-mono truncate max-w-[200px]">{job.podName}</span>
                        </span>
                      )}
                      {job.ready && (
                        <span className="text-xs">
                          Ready: {job.ready}
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Metrics cards */}
        <MetricsCards metrics={cronJob.metrics} nextRun={cronJob.nextRun} />

        {/* Active Alerts */}
        {cronJob.activeAlerts && cronJob.activeAlerts.length > 0 && (
          <Card className="border-orange-200 dark:border-orange-800">
            <CardHeader className="py-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <AlertTriangle className="h-4 w-4 text-orange-500" />
                Active Alerts ({cronJob.activeAlerts.length})
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="space-y-3">
                {cronJob.activeAlerts.map((alert) => (
                  <div
                    key={alert.id}
                    className="flex items-start gap-3 rounded-lg border p-3"
                  >
                    <Badge
                      variant={
                        alert.severity === "critical"
                          ? "destructive"
                          : alert.severity === "warning"
                          ? "default"
                          : "secondary"
                      }
                      className={
                        alert.severity === "warning"
                          ? "bg-orange-500 hover:bg-orange-600"
                          : undefined
                      }
                    >
                      {alert.severity}
                    </Badge>
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-sm">{alert.title}</p>
                      <p className="text-sm text-muted-foreground mt-0.5 break-words">
                        {alert.message}
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">
                        Since {new Date(alert.since).toLocaleString()}
                      </p>
                    </div>
                    <Link
                      href="/alerts"
                      className="text-sm text-blue-500 hover:underline shrink-0"
                    >
                      View
                    </Link>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Charts */}
        <div className="grid gap-4 md:gap-6 lg:grid-cols-2">
          <DurationChart executions={executions?.items ?? []} />
          <SuccessRateChart executions={executions?.items ?? []} />
        </div>

        {/* Health Heatmap */}
        <HealthHeatmap executions={executions?.items ?? []} />

        {/* Execution history */}
        <ExecutionHistory
          namespace={namespace}
          cronjobName={name}
          executions={executions}
          onRefresh={() => fetchData(true)}
        />
      </div>

      {/* Delete History Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Clear Execution History</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete all execution history for{" "}
              <span className="font-semibold">{namespace}/{name}</span>?
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteDialogOpen(false)}
              disabled={actionLoading === "delete"}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteHistory}
              disabled={actionLoading === "delete"}
            >
              {actionLoading === "delete" ? "Deleting..." : "Delete History"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
