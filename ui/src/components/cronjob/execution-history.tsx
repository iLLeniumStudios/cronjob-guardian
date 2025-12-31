"use client";

import { useState, useMemo } from "react";
import { CheckCircle2, XCircle, FileText, Copy, Check, Database, ChevronLeft, ChevronRight } from "lucide-react";
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
import { SortableTableHeader } from "@/components/sortable-table-header";
import {
  getLogs,
  getExecutionDetail,
  type ExecutionHistoryResponse,
  type CronJobExecution,
} from "@/lib/api";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;
type SortColumn = "jobName" | "startTime" | "duration" | "exitCode" | "status";
type SortDirection = "asc" | "desc";

function parseDuration(duration: string): number {
  // Parse duration strings like "1m30s", "45s", "2h15m"
  let total = 0;
  const hours = duration.match(/(\d+)h/);
  const minutes = duration.match(/(\d+)m/);
  const seconds = duration.match(/(\d+)s/);
  if (hours) total += parseInt(hours[1]) * 3600;
  if (minutes) total += parseInt(minutes[1]) * 60;
  if (seconds) total += parseInt(seconds[1]);
  return total;
}

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
  const [page, setPage] = useState(0);
  const [sortColumn, setSortColumn] = useState<SortColumn>("startTime");
  const [sortDirection, setSortDirection] = useState<SortDirection>("desc");
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

  // Filter, sort, and paginate
  const { paginatedExecutions, totalFiltered, totalPages } = useMemo(() => {
    // Filter by status
    const filtered = statusFilter === "all"
      ? executions?.items ?? []
      : (executions?.items ?? []).filter((e) => e.status === statusFilter);

    // Sort
    const multiplier = sortDirection === "asc" ? 1 : -1;
    const sorted = [...filtered].sort((a, b) => {
      let comparison = 0;
      switch (sortColumn) {
        case "jobName":
          comparison = a.jobName.localeCompare(b.jobName);
          break;
        case "startTime":
          comparison = new Date(a.startTime).getTime() - new Date(b.startTime).getTime();
          break;
        case "duration":
          comparison = parseDuration(a.duration) - parseDuration(b.duration);
          break;
        case "exitCode":
          comparison = a.exitCode - b.exitCode;
          break;
        case "status":
          comparison = a.status.localeCompare(b.status);
          break;
      }
      return comparison * multiplier;
    });

    // Paginate
    const total = sorted.length;
    const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    const effectivePage = Math.min(page, Math.max(0, pages - 1));
    const start = effectivePage * PAGE_SIZE;
    const paginated = sorted.slice(start, start + PAGE_SIZE);

    return {
      paginatedExecutions: paginated,
      totalFiltered: total,
      totalPages: pages,
    };
  }, [executions?.items, statusFilter, sortColumn, sortDirection, page]);

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(column);
      setSortDirection("desc");
    }
    setPage(0);
  };

  const handleFilterChange = (value: string) => {
    setStatusFilter(value);
    setPage(0);
  };

  const effectivePage = Math.min(page, Math.max(0, totalPages - 1));

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
            <Select value={statusFilter} onValueChange={handleFilterChange}>
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
          <div className="rounded border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10"></TableHead>
                  <SortableTableHeader
                    column="jobName"
                    label="Job Name"
                    currentSort={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableTableHeader
                    column="startTime"
                    label="Started"
                    currentSort={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableTableHeader
                    column="duration"
                    label="Duration"
                    currentSort={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <SortableTableHeader
                    column="exitCode"
                    label="Exit Code"
                    currentSort={sortColumn}
                    direction={sortDirection}
                    onSort={handleSort}
                  />
                  <TableHead>Reason</TableHead>
                  <TableHead className="w-24"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {paginatedExecutions.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={7} className="h-24 text-center">
                      No executions found
                    </TableCell>
                  </TableRow>
                ) : (
                  paginatedExecutions.map((exec) => (
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
          {/* Pagination */}
          <div className="flex flex-col sm:flex-row items-center justify-between gap-3 border-t pt-4 mt-4">
            <div className="text-sm text-muted-foreground order-2 sm:order-1">
              {totalFiltered > 0 ? (
                <>
                  Showing {effectivePage * PAGE_SIZE + 1}-
                  {Math.min((effectivePage + 1) * PAGE_SIZE, totalFiltered)} of {totalFiltered}
                </>
              ) : (
                "No items"
              )}
            </div>
            <div className="flex items-center gap-2 order-1 sm:order-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={effectivePage === 0}
                className="cursor-pointer disabled:cursor-not-allowed"
              >
                <ChevronLeft className="h-4 w-4" />
                <span className="hidden sm:inline">Previous</span>
              </Button>
              <span className="text-sm text-muted-foreground whitespace-nowrap">
                {effectivePage + 1} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                disabled={effectivePage >= totalPages - 1}
                className="cursor-pointer disabled:cursor-not-allowed"
              >
                <span className="hidden sm:inline">Next</span>
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
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
