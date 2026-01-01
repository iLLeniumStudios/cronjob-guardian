"use client";

import { useState } from "react";
import { CheckCircle2, XCircle, FileText, Copy, Check, Database, Timer } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Badge } from "@/components/ui/badge";
import { RelativeTime } from "@/components/relative-time";
import { DataTable } from "@/components/data-table/data-table";
import type { ColumnDef } from "@/components/data-table/types";
import {
  getLogs,
  getExecutionDetail,
  type ExecutionHistoryResponse,
  type CronJobExecution,
} from "@/lib/api";
import { cn } from "@/lib/utils";

interface ExecutionHistoryProps {
  namespace: string;
  cronjobName: string;
  executions: ExecutionHistoryResponse | null;
  onRefresh: () => void;
}

function parseDuration(duration: string): number {
  let total = 0;
  const hours = duration.match(/(\d+)h/);
  const minutes = duration.match(/(\d+)m/);
  const seconds = duration.match(/(\d+)s/);
  if (hours) total += parseInt(hours[1]) * 3600;
  if (minutes) total += parseInt(minutes[1]) * 60;
  if (seconds) total += parseInt(seconds[1]);
  return total;
}

export function ExecutionHistory({
  namespace,
  cronjobName,
  executions,
}: ExecutionHistoryProps) {
  const [logsModal, setLogsModal] = useState<{
    open: boolean;
    jobName: string;
    logs: string;
    loading: boolean;
    isStored: boolean;
  }>({
    open: false,
    jobName: "",
    logs: "",
    loading: false,
    isStored: false,
  });
  const [copied, setCopied] = useState(false);

  const handleViewLogs = async (jobName: string) => {
    setLogsModal({ open: true, jobName, logs: "", loading: true, isStored: false });
    try {
      const detail = await getExecutionDetail(namespace, cronjobName, jobName);
      if (detail.storedLogs) {
        setLogsModal((prev) => ({
          ...prev,
          logs: detail.storedLogs ?? "",
          loading: false,
          isStored: true,
        }));
        return;
      }
    } catch {
      // Fallback
    }

    try {
      const result = await getLogs(namespace, cronjobName, jobName);
      setLogsModal((prev) => ({
        ...prev,
        logs: result.logs,
        loading: false,
        isStored: false,
      }));
    } catch (error) {
      console.error(error);
      setLogsModal((prev) => ({
        ...prev,
        logs: "Failed to fetch logs. The job may have been cleaned up.",
        loading: false,
        isStored: false,
      }));
    }
  };

  const handleCopyLogs = async () => {
    try {
      await navigator.clipboard.writeText(logsModal.logs);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
      toast.success("Logs copied to clipboard");
    } catch {
      toast.error("Failed to copy logs");
    }
  };

  const columns: ColumnDef<CronJobExecution>[] = [
    {
      id: "status",
      header: "",
      accessorKey: "status",
      className: "w-10",
      cell: (row) =>
        row.status === "success" ? (
          <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
        ) : (
          <XCircle className="h-4 w-4 text-red-600 dark:text-red-400" />
        ),
    },
    {
      id: "jobName",
      header: "Job Name",
      accessorKey: "jobName",
      sortable: true,
      className: "font-mono text-sm",
    },
    {
      id: "startTime",
      header: "Started",
      accessorKey: "startTime",
      sortable: true,
      cell: (row) => <RelativeTime date={row.startTime} />,
      sortFn: (a, b) => new Date(a.startTime).getTime() - new Date(b.startTime).getTime(),
    },
    {
      id: "duration",
      header: "Duration",
      accessorKey: "duration",
      sortable: true,
      sortFn: (a, b) => parseDuration(a.duration) - parseDuration(b.duration),
    },
    {
      id: "exitCode",
      header: "Exit Code",
      accessorKey: "exitCode",
      sortable: true,
      cell: (row) => (
        <Badge
          variant="outline"
          className={cn(
            "font-mono",
            row.exitCode !== 0 && "border-red-500/50 text-red-600 dark:text-red-400"
          )}
        >
          {row.exitCode}
        </Badge>
      ),
    },
    {
      id: "reason",
      header: "Reason",
      accessorKey: "reason",
      cell: (row) => <span className="text-muted-foreground">{row.reason || "-"}</span>,
    },
    {
      id: "actions",
      header: "",
      className: "w-24",
      cell: (row) => (
        <Button variant="ghost" size="sm" onClick={() => handleViewLogs(row.jobName)}>
          <FileText className="mr-1.5 h-3.5 w-3.5" />
          Logs
        </Button>
      ),
    },
  ];

  return (
    <>
      <DataTable
        data={executions?.items || []}
        columns={columns}
        getRowKey={(row) => row.jobName}
        title="Execution History"
        pageSize={20}
        defaultSort={{ column: "startTime", direction: "desc" }}
        filters={[
          {
            type: "select",
            key: "status",
            label: "Status",
            options: [
              { label: "Success", value: "success" },
              { label: "Failed", value: "failed" },
            ],
          },
        ]}
        emptyState={{
            icon: Timer,
            title: "No executions found",
            description: "Job execution history will appear here",
        }}
      />

      <Dialog
        open={logsModal.open}
        onOpenChange={(open) => setLogsModal((prev) => ({ ...prev, open }))}
      >
        <DialogContent className="max-w-4xl h-[80vh] flex flex-col">
          <DialogHeader className="flex-row items-center justify-between space-y-0 pb-2">
            <div className="flex items-center gap-2">
              <DialogTitle className="text-base font-medium">
                Logs: {logsModal.jobName}
              </DialogTitle>
              {logsModal.isStored && (
                <Badge variant="secondary" className="text-xs">
                  <Database className="mr-1 h-3 w-3" />
                  Stored
                </Badge>
              )}
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={handleCopyLogs}
              disabled={logsModal.loading}
            >
              {copied ? (
                <>
                  <Check className="mr-1.5 h-3.5 w-3.5" />
                  Copied
                </>
              ) : (
                <>
                  <Copy className="mr-1.5 h-3.5 w-3.5" />
                  Copy
                </>
              )}
            </Button>
          </DialogHeader>
          <ScrollArea className="flex-1 rounded border bg-muted/30">
            <pre className="p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all">
              {logsModal.loading ? "Loading logs..." : logsModal.logs || "No logs available"}
            </pre>
          </ScrollArea>
        </DialogContent>
      </Dialog>
    </>
  );
}