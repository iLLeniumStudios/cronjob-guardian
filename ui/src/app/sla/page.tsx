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
} from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PageSkeleton } from "@/components/page-skeleton";
import { ExportButton } from "@/components/export/export-button";
import { exportSLAReportToCSV } from "@/lib/export/csv";
import { generateSLAPDFReport } from "@/lib/export/pdf";
import { listMonitors, listCronJobs, getMonitor, getCronJob } from "@/lib/api";
import { DataTable } from "@/components/data-table/data-table";
import type { ColumnDef } from "@/components/data-table/types";

type SLAStatus = "meeting" | "breaching" | "at-risk" | "no-sla";

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

const columns: ColumnDef<SLACronJob>[] = [
  {
    id: "name",
    header: "CronJob",
    accessorKey: "name",
    sortable: true,
    cell: (row) => (
      <div>
        <Link
          href={`/cronjob/${row.namespace}/${row.name}`}
          className="font-medium hover:underline"
        >
          {row.name}
        </Link>
        <div className="text-xs text-muted-foreground">{row.namespace}</div>
      </div>
    ),
  },
  {
    id: "monitor",
    header: "Monitor",
    accessorKey: "monitorName",
    sortable: true,
    cell: (row) => (
      <Link
        href={`/monitors/${row.monitorNamespace}/${row.monitorName}`}
        className="text-sm hover:underline"
      >
        {row.monitorName}
      </Link>
    ),
  },
  {
    id: "targetSLA",
    header: "Target SLA",
    accessorKey: "targetSLA",
    sortable: true,
    align: "right",
    cell: (row) =>
      row.status === "no-sla" ? (
        <span className="text-muted-foreground">-</span>
      ) : (
        <span className="font-mono">{row.targetSLA.toFixed(0)}%</span>
      ),
  },
  {
    id: "currentSuccessRate",
    header: "Current Rate",
    accessorKey: "currentSuccessRate",
    sortable: true,
    align: "right",
    cell: (row) => (
      <div className="flex flex-col items-end">
        <span
          className={`font-mono font-medium ${
            row.status === "meeting"
              ? "text-emerald-600 dark:text-emerald-400"
              : row.status === "at-risk"
              ? "text-amber-600 dark:text-amber-400"
              : row.status === "breaching"
              ? "text-red-600 dark:text-red-400"
              : ""
          }`}
        >
          {row.currentSuccessRate.toFixed(1)}%
        </span>
        {row.windowDays > 0 && (
          <span className="text-[10px] text-muted-foreground">
            {row.windowDays}d window
          </span>
        )}
      </div>
    ),
  },
  {
    id: "status",
    header: "Status",
    accessorKey: "status",
    sortable: true,
    align: "center",
    cell: (row) => <StatusBadge status={row.status} />,
    sortFn: (a, b) => statusOrder[a.status] - statusOrder[b.status],
  },
  {
    id: "trend",
    header: "Trend",
    accessorKey: "trend",
    sortable: true,
    align: "center",
    cell: (row) => <TrendIndicator trend={row.trend} />,
    sortFn: (a, b) => trendOrder[a.trend] - trendOrder[b.trend],
  },
];

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

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const [monitorsRes, cronJobsRes] = await Promise.all([
        listMonitors(),
        listCronJobs(),
      ]);

      // Fetch detailed monitor data to get SLA config
      const monitorDetails = await Promise.all(
        monitorsRes.items.map((m) =>
          getMonitor(m.namespace, m.name).catch(() => null)
        )
      );

      // First pass: collect all cronjobs with SLA and their window configs
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

      // Second pass: fetch cronjob details
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

      const slaMap = new Map<string, SLACronJob>();

      for (let i = 0; i < cronJobKeys.length; i++) {
        const key = cronJobKeys[i];
        const config = cronJobConfigs.get(key)!;
        const detail = cronJobDetails[i];
        const [namespace, name] = key.split("/");

        let successRate = 0;
        if (detail?.metrics) {
          if (config.windowDays <= 7) {
            successRate = detail.metrics.successRate7d;
          } else {
            successRate = detail.metrics.successRate30d;
          }
        } else {
          const listCronJob = cronJobsRes.items.find(
            (c) => c.name === name && c.namespace === namespace
          );
          successRate = listCronJob?.successRate ?? 0;
        }

        let status: SLAStatus;
        if (successRate >= config.targetSLA) {
          status = "meeting";
        } else if (successRate >= config.targetSLA * 0.9) {
          status = "at-risk";
        } else {
          status = "breaching";
        }

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

      // Check cronjobs without SLA
      for (const cj of cronJobsRes.items) {
        const key = `${cj.namespace}/${cj.name}`;
        if (!slaMap.has(key) && cj.monitorRef) {
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

        <DataTable
          data={slaData}
          columns={columns}
          getRowKey={(row) => `${row.namespace}/${row.name}`}
          title="SLA Compliance Details"
          pageSize={50}
          defaultSort={{ column: "status", direction: "asc" }}
          filters={[
            {
              type: "faceted",
              key: "status",
              label: "Status",
              options: [
                { label: "Meeting SLA", value: "meeting" },
                { label: "At Risk", value: "at-risk" },
                { label: "Breaching", value: "breaching" },
                { label: "No SLA", value: "no-sla" },
              ],
            },
          ]}
          search={{
            placeholder: "Filter cronjobs...",
            searchKeys: ["name", "monitorName"],
          }}
          enableViewOptions
          emptyState={{
            icon: Target,
            title: "No SLA data available",
            description: "Configure SLA targets to see compliance data",
          }}
        />
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