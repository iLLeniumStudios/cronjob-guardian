"use client";

import { useCallback, useState, useEffect, useMemo } from "react";
import Link from "next/link";
import {
  Target,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  TrendingUp,
  TrendingDown,
  Minus,
  Filter,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyState } from "@/components/empty-state";
import { SortableTableHeader } from "@/components/sortable-table-header";
import { PageSkeleton } from "@/components/page-skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ExportButton } from "@/components/export/export-button";
import { exportSLAReportToCSV } from "@/lib/export/csv";
import { generateSLAPDFReport } from "@/lib/export/pdf";
import { listMonitors, listCronJobs, getMonitor, getCronJob } from "@/lib/api";

const PAGE_SIZE = 50;

type SLAStatus = "meeting" | "breaching" | "at-risk" | "no-sla";
type SLASortColumn = "name" | "monitor" | "targetSLA" | "currentRate" | "status" | "trend";
type SortDirection = "asc" | "desc";

const statusOrder: Record<SLAStatus, number> = {
  breaching: 0,
  "at-risk": 1,
  meeting: 2,
  "no-sla": 3,
};

const trendOrder: Record<string, number> = {
  declining: 0,
  stable: 1,
  improving: 2,
};

interface SLACronJob {
  name: string;
  namespace: string;
  monitorName: string;
  monitorNamespace: string;
  targetSLA: number;
  currentSuccessRate: number;
  status: SLAStatus;
  trend: "improving" | "declining" | "stable";
  windowDays: number;
}

interface SLASummary {
  total: number;
  meeting: number;
  breaching: number;
  atRisk: number;
  noSLA: number;
}

export default function SLAPage() {
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [slaData, setSLAData] = useState<SLACronJob[]>([]);
  const [summary, setSummary] = useState<SLASummary>({
    total: 0,
    meeting: 0,
    breaching: 0,
    atRisk: 0,
    noSLA: 0,
  });
  const [filter, setFilter] = useState<SLAStatus | "all">("all");
  const [page, setPage] = useState(0);
  const [sortColumn, setSortColumn] = useState<SLASortColumn>("status");
  const [sortDirection, setSortDirection] = useState<SortDirection>("asc");

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const [monitorsRes, cronJobsRes] = await Promise.all([
        listMonitors(),
        listCronJobs(),
      ]);

      // Fetch detailed monitor data to get SLA config
      const monitorDetails = await Promise.all(
        monitorsRes.items.map((m) => getMonitor(m.namespace, m.name).catch(() => null))
      );

      // First pass: collect all cronjobs with SLA and their window configs
      // Key: namespace/name, Value: { windowDays, targetSLA, monitorName, monitorNamespace }
      type CronJobSLAConfig = {
        windowDays: number;
        targetSLA: number;
        monitorName: string;
        monitorNamespace: string;
      };
      const cronJobConfigs = new Map<string, CronJobSLAConfig>();

      for (const detail of monitorDetails) {
        if (!detail || !detail.spec.sla?.enabled) continue;

        const targetSLA = detail.spec.sla.minSuccessRate;
        const windowDays = detail.spec.sla.windowDays || 7;

        for (const cj of detail.status.cronJobs) {
          const key = `${cj.namespace}/${cj.name}`;
          const existing = cronJobConfigs.get(key);
          // Keep the strictest (highest) SLA target
          if (!existing || targetSLA > existing.targetSLA) {
            cronJobConfigs.set(key, {
              windowDays,
              targetSLA,
              monitorName: detail.metadata.name,
              monitorNamespace: detail.metadata.namespace,
            });
          }
        }
      }

      // Second pass: fetch cronjob details to get metrics for the correct window
      // Use batched fetching to avoid too many concurrent requests
      const cronJobKeys = Array.from(cronJobConfigs.keys());
      const cronJobDetails = await Promise.all(
        cronJobKeys.map(async (key) => {
          const [namespace, name] = key.split("/");
          try {
            return await getCronJob(namespace, name);
          } catch {
            return null;
          }
        })
      );

      // Build SLA data with correct window metrics
      const slaMap = new Map<string, SLACronJob>();

      for (let i = 0; i < cronJobKeys.length; i++) {
        const key = cronJobKeys[i];
        const config = cronJobConfigs.get(key)!;
        const detail = cronJobDetails[i];
        const [namespace, name] = key.split("/");

        // Get success rate based on configured window
        // For windows <= 7 days, use 7d metrics
        // For windows > 7 days, use 30d metrics (closest we have)
        let successRate = 0;
        if (detail?.metrics) {
          if (config.windowDays <= 7) {
            successRate = detail.metrics.successRate7d;
          } else {
            // Use 30d metrics for windows > 7 days
            successRate = detail.metrics.successRate30d;
          }
        } else {
          // Fallback to list data
          const listCronJob = cronJobsRes.items.find(
            (c) => c.name === name && c.namespace === namespace
          );
          successRate = listCronJob?.successRate ?? 0;
        }

        // Determine SLA status
        let status: SLAStatus;
        if (successRate >= config.targetSLA) {
          status = "meeting";
        } else if (successRate >= config.targetSLA * 0.9) {
          // Within 10% of target
          status = "at-risk";
        } else {
          status = "breaching";
        }

        // Determine trend (simplified - would need historical data for real trend)
        let trend: "improving" | "declining" | "stable" = "stable";
        const gap = successRate - config.targetSLA;
        if (gap > 5) trend = "improving";
        else if (gap < -10) trend = "declining";

        slaMap.set(key, {
          name,
          namespace,
          monitorName: config.monitorName,
          monitorNamespace: config.monitorNamespace,
          targetSLA: config.targetSLA,
          currentSuccessRate: successRate,
          status,
          trend,
          windowDays: config.windowDays,
        });
      }

      // Also check cronjobs without SLA configured
      for (const cj of cronJobsRes.items) {
        const key = `${cj.namespace}/${cj.name}`;
        if (!slaMap.has(key) && cj.monitorRef) {
          // Has monitor but no SLA configured
          slaMap.set(key, {
            name: cj.name,
            namespace: cj.namespace,
            monitorName: cj.monitorRef.name,
            monitorNamespace: cj.monitorRef.namespace,
            targetSLA: 0,
            currentSuccessRate: cj.successRate,
            status: "no-sla",
            trend: "stable",
            windowDays: 0,
          });
        }
      }

      const slaItems = Array.from(slaMap.values());

      // Calculate summary
      const newSummary: SLASummary = {
        total: slaItems.length,
        meeting: slaItems.filter((s) => s.status === "meeting").length,
        breaching: slaItems.filter((s) => s.status === "breaching").length,
        atRisk: slaItems.filter((s) => s.status === "at-risk").length,
        noSLA: slaItems.filter((s) => s.status === "no-sla").length,
      };

      setSLAData(slaItems);
      setSummary(newSummary);
    } catch (error) {
      console.error("Failed to fetch SLA data:", error);
      toast.error("Failed to load SLA compliance data");
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

  // Filter, sort, and paginate data
  const { paginatedData, totalFiltered, totalPages } = useMemo(() => {
    // Filter by status
    const filtered = filter === "all" ? slaData : slaData.filter((s) => s.status === filter);

    // Sort
    const multiplier = sortDirection === "asc" ? 1 : -1;
    const sorted = [...filtered].sort((a, b) => {
      let comparison = 0;
      switch (sortColumn) {
        case "name":
          comparison = a.name.localeCompare(b.name);
          break;
        case "monitor":
          comparison = a.monitorName.localeCompare(b.monitorName);
          break;
        case "targetSLA":
          comparison = a.targetSLA - b.targetSLA;
          break;
        case "currentRate":
          comparison = a.currentSuccessRate - b.currentSuccessRate;
          break;
        case "status":
          comparison = statusOrder[a.status] - statusOrder[b.status];
          break;
        case "trend":
          comparison = trendOrder[a.trend] - trendOrder[b.trend];
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
      paginatedData: paginated,
      totalFiltered: total,
      totalPages: pages,
    };
  }, [slaData, filter, sortColumn, sortDirection, page]);

  const effectivePage = Math.min(page, Math.max(0, totalPages - 1));

  const handleSort = (column: SLASortColumn) => {
    if (sortColumn === column) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(column);
      setSortDirection("asc");
    }
    setPage(0);
  };

  const handleFilterChange = (value: SLAStatus | "all") => {
    setFilter(value);
    setPage(0);
  };

  const handleExportCSV = () => {
    if (slaData.length > 0) {
      exportSLAReportToCSV(
        slaData.map((item) => ({
          cronJobName: item.name,
          namespace: item.namespace,
          targetSLA: item.targetSLA,
          currentSuccessRate: item.currentSuccessRate,
          status: item.status,
          monitorName: item.monitorName,
        }))
      );
      toast.success("CSV exported successfully");
    } else {
      toast.error("No SLA data to export");
    }
  };

  const handleExportPDF = () => {
    if (slaData.length > 0) {
      generateSLAPDFReport({
        title: "SLA Compliance Report",
        generatedAt: new Date(),
        summary: {
          total: summary.total,
          meeting: summary.meeting,
          atRisk: summary.atRisk,
          breaching: summary.breaching,
        },
        items: slaData.map((item) => ({
          cronJobName: item.name,
          namespace: item.namespace,
          monitorName: item.monitorName,
          targetSLA: item.targetSLA,
          currentSuccessRate: item.currentSuccessRate,
          status: item.status,
        })),
      });
    } else {
      toast.error("No SLA data to export");
    }
  };

  if (isLoading) {
    return <PageSkeleton title="SLA Compliance" variant="table" />;
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title="SLA Compliance"
        description="Monitor your CronJob SLA targets and compliance"
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
        actions={
          <ExportButton
            onExportCSV={handleExportCSV}
            onExportPDF={handleExportPDF}
          />
        }
      />
      <div className="flex-1 space-y-6 overflow-auto p-6">
        {/* Summary Cards */}
        <div className="grid gap-4 md:grid-cols-4">
          <SummaryCard
            title="Total Monitored"
            value={summary.total}
            icon={Target}
            description="CronJobs with SLA tracking"
          />
          <SummaryCard
            title="Meeting SLA"
            value={summary.meeting}
            icon={CheckCircle2}
            description="Above target threshold"
            variant="success"
          />
          <SummaryCard
            title="At Risk"
            value={summary.atRisk}
            icon={AlertTriangle}
            description="Within 10% of target"
            variant="warning"
          />
          <SummaryCard
            title="Breaching SLA"
            value={summary.breaching}
            icon={XCircle}
            description="Below target threshold"
            variant="destructive"
          />
        </div>

        {/* Compliance Table */}
        <Card>
          <CardHeader className="pb-4">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base font-medium">
                SLA Compliance Details
              </CardTitle>
              <div className="flex items-center gap-2">
                <Filter className="h-4 w-4 text-muted-foreground" />
                <Select
                  value={filter}
                  onValueChange={(v) => handleFilterChange(v as SLAStatus | "all")}
                >
                  <SelectTrigger className="w-[140px]">
                    <SelectValue placeholder="Filter" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Status</SelectItem>
                    <SelectItem value="meeting">Meeting SLA</SelectItem>
                    <SelectItem value="at-risk">At Risk</SelectItem>
                    <SelectItem value="breaching">Breaching</SelectItem>
                    <SelectItem value="no-sla">No SLA</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {totalFiltered === 0 ? (
              <EmptyState
                icon={Target}
                title="No SLA data available"
                description={filter === "all"
                  ? "Configure SLA targets on your CronJobMonitor resources"
                  : "No CronJobs match the selected filter"}
                bordered={false}
              />
            ) : (
              <div className="space-y-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <SortableTableHeader
                      column="name"
                      label="CronJob"
                      currentSort={sortColumn}
                      direction={sortDirection}
                      onSort={handleSort}
                    />
                    <SortableTableHeader
                      column="monitor"
                      label="Monitor"
                      currentSort={sortColumn}
                      direction={sortDirection}
                      onSort={handleSort}
                    />
                    <SortableTableHeader
                      column="targetSLA"
                      label="Target SLA"
                      currentSort={sortColumn}
                      direction={sortDirection}
                      onSort={handleSort}
                      align="right"
                    />
                    <SortableTableHeader
                      column="currentRate"
                      label="Current Rate"
                      currentSort={sortColumn}
                      direction={sortDirection}
                      onSort={handleSort}
                      align="right"
                    />
                    <SortableTableHeader
                      column="status"
                      label="Status"
                      currentSort={sortColumn}
                      direction={sortDirection}
                      onSort={handleSort}
                      align="center"
                    />
                    <SortableTableHeader
                      column="trend"
                      label="Trend"
                      currentSort={sortColumn}
                      direction={sortDirection}
                      onSort={handleSort}
                      align="center"
                    />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {paginatedData.map((item) => (
                    <TableRow key={`${item.namespace}/${item.name}`}>
                      <TableCell>
                        <Link
                          href={`/cronjob/${item.namespace}/${item.name}`}
                          className="font-medium hover:underline"
                        >
                          {item.name}
                        </Link>
                        <div className="text-xs text-muted-foreground">
                          {item.namespace}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Link
                          href={`/monitors/${item.monitorNamespace}/${item.monitorName}`}
                          className="text-sm hover:underline"
                        >
                          {item.monitorName}
                        </Link>
                      </TableCell>
                      <TableCell className="text-right">
                        {item.status === "no-sla" ? (
                          <span className="text-muted-foreground">-</span>
                        ) : (
                          <span className="font-mono">
                            {item.targetSLA.toFixed(0)}%
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex flex-col items-end">
                          <span
                            className={`font-mono font-medium ${
                              item.status === "meeting"
                                ? "text-emerald-600 dark:text-emerald-400"
                                : item.status === "at-risk"
                                  ? "text-amber-600 dark:text-amber-400"
                                  : item.status === "breaching"
                                    ? "text-red-600 dark:text-red-400"
                                    : ""
                            }`}
                          >
                            {item.currentSuccessRate.toFixed(1)}%
                          </span>
                          {item.windowDays > 0 && (
                            <span className="text-[10px] text-muted-foreground">
                              {item.windowDays}d window
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-center">
                        <StatusBadge status={item.status} />
                      </TableCell>
                      <TableCell className="text-center">
                        <TrendIndicator trend={item.trend} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              {/* Pagination */}
              <div className="flex flex-col sm:flex-row items-center justify-between gap-3 border-t pt-4">
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
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function SummaryCard({
  title,
  value,
  icon: Icon,
  description,
  variant,
}: {
  title: string;
  value: number;
  icon: React.ComponentType<{ className?: string }>;
  description: string;
  variant?: "success" | "warning" | "destructive";
}) {
  const iconColors = {
    success: "text-emerald-600 dark:text-emerald-400",
    warning: "text-amber-600 dark:text-amber-400",
    destructive: "text-red-600 dark:text-red-400",
    default: "text-primary",
  };

  const bgColors = {
    success: "bg-emerald-500/10",
    warning: "bg-amber-500/10",
    destructive: "bg-red-500/10",
    default: "bg-primary/10",
  };

  const color = iconColors[variant || "default"];
  const bg = bgColors[variant || "default"];

  return (
    <Card>
      <CardContent className="pt-6">
        <div className="flex items-center gap-4">
          <div className={`rounded-lg p-3 ${bg}`}>
            <Icon className={`h-5 w-5 ${color}`} />
          </div>
          <div>
            <p className="text-sm text-muted-foreground">{title}</p>
            <p className="text-2xl font-bold">{value}</p>
            <p className="text-xs text-muted-foreground">{description}</p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: SLAStatus }) {
  switch (status) {
    case "meeting":
      return (
        <Badge className="bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/20 dark:text-emerald-400">
          Meeting
        </Badge>
      );
    case "at-risk":
      return (
        <Badge className="bg-amber-500/10 text-amber-700 hover:bg-amber-500/20 dark:text-amber-400">
          At Risk
        </Badge>
      );
    case "breaching":
      return (
        <Badge className="bg-red-500/10 text-red-700 hover:bg-red-500/20 dark:text-red-400">
          Breaching
        </Badge>
      );
    case "no-sla":
      return (
        <Badge variant="outline" className="text-muted-foreground">
          No SLA
        </Badge>
      );
  }
}

function TrendIndicator({ trend }: { trend: "improving" | "declining" | "stable" }) {
  switch (trend) {
    case "improving":
      return (
        <div className="flex items-center justify-center gap-1 text-emerald-600 dark:text-emerald-400">
          <TrendingUp className="h-4 w-4" />
          <span className="text-xs">Up</span>
        </div>
      );
    case "declining":
      return (
        <div className="flex items-center justify-center gap-1 text-red-600 dark:text-red-400">
          <TrendingDown className="h-4 w-4" />
          <span className="text-xs">Down</span>
        </div>
      );
    case "stable":
      return (
        <div className="flex items-center justify-center gap-1 text-muted-foreground">
          <Minus className="h-4 w-4" />
          <span className="text-xs">Stable</span>
        </div>
      );
  }
}
