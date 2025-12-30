"use client";

import { useState } from "react";
import { CheckCircle2, XCircle, FileText, Copy, Check, Database } from "lucide-react";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Badge } from "@/components/ui/badge";
import { RelativeTime } from "@/components/relative-time";
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

export function ExecutionHistory({
  namespace,
  cronjobName,
  executions,
}: ExecutionHistoryProps) {
  const [statusFilter, setStatusFilter] = useState<string>("all");
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

  const filteredExecutions =
    statusFilter === "all"
      ? executions?.items ?? []
      : (executions?.items ?? []).filter((e) => e.status === statusFilter);

  const handleViewLogs = async (jobName: string) => {
    setLogsModal({ open: true, jobName, logs: "", loading: true, isStored: false });
    try {
      // First, try to get stored logs from execution detail
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
      // Execution detail not found, fall back to live logs
    }

    // Fall back to fetching live logs from K8s
    try {
      const result = await getLogs(namespace, cronjobName, jobName);
      setLogsModal((prev) => ({
        ...prev,
        logs: result.logs,
        loading: false,
        isStored: false,
      }));
    } catch (error) {
      console.error("Failed to fetch logs:", error);
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

  return (
    <>
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base font-medium">Execution History</CardTitle>
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-32">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="success">Success</SelectItem>
                <SelectItem value="failed">Failed</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          <div className="rounded border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10"></TableHead>
                  <TableHead>Job Name</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Exit Code</TableHead>
                  <TableHead>Reason</TableHead>
                  <TableHead className="w-24"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredExecutions.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="h-24 text-center">
                      No executions found
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredExecutions.map((exec) => (
                    <ExecutionRow
                      key={exec.jobName}
                      execution={exec}
                      onViewLogs={handleViewLogs}
                    />
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          {executions && executions.pagination.hasMore && (
            <div className="mt-4 text-center">
              <p className="text-sm text-muted-foreground">
                Showing {filteredExecutions.length} of {executions.pagination.total} executions
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Logs Modal */}
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

function ExecutionRow({
  execution,
  onViewLogs,
}: {
  execution: CronJobExecution;
  onViewLogs: (jobName: string) => void;
}) {
  const isSuccess = execution.status === "success";

  return (
    <TableRow>
      <TableCell>
        {isSuccess ? (
          <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
        ) : (
          <XCircle className="h-4 w-4 text-red-600 dark:text-red-400" />
        )}
      </TableCell>
      <TableCell className="font-mono text-sm">{execution.jobName}</TableCell>
      <TableCell>
        <RelativeTime date={execution.startTime} />
      </TableCell>
      <TableCell>{execution.duration}</TableCell>
      <TableCell>
        <Badge
          variant="outline"
          className={cn(
            "font-mono",
            execution.exitCode !== 0 && "border-red-500/50 text-red-600 dark:text-red-400"
          )}
        >
          {execution.exitCode}
        </Badge>
      </TableCell>
      <TableCell className="text-muted-foreground">{execution.reason || "-"}</TableCell>
      <TableCell>
        <Button variant="ghost" size="sm" onClick={() => onViewLogs(execution.jobName)}>
          <FileText className="mr-1.5 h-3.5 w-3.5" />
          Logs
        </Button>
      </TableCell>
    </TableRow>
  );
}
