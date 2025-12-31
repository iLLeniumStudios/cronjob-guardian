"use client";

import { useCallback, useState, useEffect } from "react";
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
  ChevronUp,
  ChevronDown,
} from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
import { listMonitors, listCronJobs, getMonitor } from "@/lib/api";

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

      // Build SLA data by cross-referencing monitors and cronjobs
      // Use a Map to deduplicate by cronjob (keeping the strictest SLA)
      const slaMap = new Map<string, SLACronJob>();

      for (const detail of monitorDetails) {
        if (!detail || !detail.spec.sla?.enabled) continue;

        const targetSLA = detail.spec.sla.minSuccessRate;
        const windowDays = detail.spec.sla.windowDays || 7;

        for (const cj of detail.status.cronJobs) {
          const key = `${cj.namespace}/${cj.name}`;

          // Find the cronjob in the list to get full metrics
          const fullCronJob = cronJobsRes.items.find(
            (c) => c.name === cj.name && c.namespace === cj.namespace
          );

          const successRate = cj.metrics?.successRate ?? fullCronJob?.successRate ?? 0;

          // Determine SLA status
          let status: SLAStatus;
          if (successRate >= targetSLA) {
            status = "meeting";
          } else if (successRate >= targetSLA * 0.9) {
            // Within 10% of target
            status = "at-risk";
          } else {
            status = "breaching";
          }

          // Determine trend (simplified - would need historical data for real trend)
          // For now, use current rate vs target as proxy
          let trend: "improving" | "declining" | "stable" = "stable";
          const gap = successRate - targetSLA;
          if (gap > 5) trend = "improving";
          else if (gap < -10) trend = "declining";

          const newItem: SLACronJob = {
            name: cj.name,
            namespace: cj.namespace,
            monitorName: detail.metadata.name,
            monitorNamespace: detail.metadata.namespace,
            targetSLA,
            currentSuccessRate: successRate,
            status,
            trend,
            windowDays,
          };

          // If cronjob already exists, keep the entry with the strictest (highest) SLA target
          const existing = slaMap.get(key);
          if (!existing || targetSLA > existing.targetSLA) {
            slaMap.set(key, newItem);
          }
        }
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

  // Filter and sort data
  const filteredData = (() => {
    const filtered = filter === "all" ? slaData : slaData.filter((s) => s.status === filter);
    const multiplier = sortDirection === "asc" ? 1 : -1;

    return [...filtered].sort((a, b) => {
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
  })();

  const handleSort = (column: SLASortColumn) => {
    if (sortColumn === column) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(column);
      setSortDirection("asc");
    }
  };

  const SortIcon = ({ column }: { column: SLASortColumn }) => {
    if (sortColumn !== column) {
      return <ChevronUp className="h-3 w-3 opacity-0 group-hover:opacity-30" />;
    }
    return sortDirection === "asc" ? (
      <ChevronUp className="h-3 w-3" />
    ) : (
      <ChevronDown className="h-3 w-3" />
    );
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
    return (
      <div className="flex h-full flex-col">
        <Header title="SLA Compliance" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <div className="grid gap-4 md:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-24" />
            ))}
          </div>
          <Skeleton className="h-96" />
        </div>
      </div>
    );
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
                  onValueChange={(v) => setFilter(v as SLAStatus | "all")}
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
            {filteredData.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Target className="mb-4 h-12 w-12 text-muted-foreground/50" />
                <p className="text-lg font-medium">No SLA data available</p>
                <p className="text-sm text-muted-foreground">
                  {filter === "all"
                    ? "Configure SLA targets on your CronJobMonitor resources"
                    : "No CronJobs match the selected filter"}
                </p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead
                      className="cursor-pointer select-none group"
                      onClick={() => handleSort("name")}
                    >
                      <span className="flex items-center gap-1">
                        CronJob
                        <SortIcon column="name" />
                      </span>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer select-none group"
                      onClick={() => handleSort("monitor")}
                    >
                      <span className="flex items-center gap-1">
                        Monitor
                        <SortIcon column="monitor" />
                      </span>
                    </TableHead>
                    <TableHead
                      className="text-right cursor-pointer select-none group"
                      onClick={() => handleSort("targetSLA")}
                    >
                      <span className="flex items-center justify-end gap-1">
                        Target SLA
                        <SortIcon column="targetSLA" />
                      </span>
                    </TableHead>
                    <TableHead
                      className="text-right cursor-pointer select-none group"
                      onClick={() => handleSort("currentRate")}
                    >
                      <span className="flex items-center justify-end gap-1">
                        Current Rate
                        <SortIcon column="currentRate" />
                      </span>
                    </TableHead>
                    <TableHead
                      className="text-center cursor-pointer select-none group"
                      onClick={() => handleSort("status")}
                    >
                      <span className="flex items-center justify-center gap-1">
                        Status
                        <SortIcon column="status" />
                      </span>
                    </TableHead>
                    <TableHead
                      className="text-center cursor-pointer select-none group"
                      onClick={() => handleSort("trend")}
                    >
                      <span className="flex items-center justify-center gap-1">
                        Trend
                        <SortIcon column="trend" />
                      </span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredData.map((item) => (
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
