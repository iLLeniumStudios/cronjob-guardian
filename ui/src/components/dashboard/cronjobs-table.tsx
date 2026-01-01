"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Play, Timer } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StatusIndicator } from "@/components/status-indicator";
import { RelativeTime } from "@/components/relative-time";
import { DataTable } from "@/components/data-table/data-table";
import { CronSchedule } from "@/components/cron-schedule";
import type { CronJobListResponse, CronJob } from "@/lib/api";
import { cn } from "@/lib/utils";
import { getSuccessRateColor } from "@/lib/constants";
import type { ColumnDef } from "@/components/data-table/types";

interface CronJobsTableProps {
  cronJobs: CronJobListResponse | null;
  isLoading: boolean;
}

const columns: ColumnDef<CronJob>[] = [
  {
    id: "status",
    header: "",
    accessorFn: (row) => {
      const hasActiveJobs = row.activeJobs && row.activeJobs.length > 0;
      return hasActiveJobs ? "running" : row.status;
    },
    cell: (row) => {
      const hasActiveJobs = row.activeJobs && row.activeJobs.length > 0;
      const displayStatus = hasActiveJobs ? "running" : row.status;
      return <StatusIndicator status={displayStatus} />;
    },
    className: "w-10",
  },
  {
    id: "name",
    header: "Name",
    accessorKey: "name",
    sortable: true,
    className: "w-[30%] min-w-[180px] max-w-[250px]",
    cell: (row) => {
      const hasActiveJobs = row.activeJobs && row.activeJobs.length > 0;
      return (
        <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2 min-w-0">
          <Link
            href={`/cronjob/${row.namespace}/${row.name}`}
            className="font-medium hover:underline truncate"
            title={row.name}
            onClick={(e) => e.stopPropagation()}
          >
            {row.name}
          </Link>
          <span className="text-xs text-muted-foreground sm:hidden truncate">
            {row.namespace}
          </span>
          {hasActiveJobs && (
            <Badge
              variant="outline"
              className="bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20 gap-1 w-fit shrink-0"
            >
              <Play className="h-3 w-3 fill-current" />
              {row.activeJobs!.length} running
            </Badge>
          )}
        </div>
      );
    },
  },
  {
    id: "namespace",
    header: "Namespace",
    accessorKey: "namespace",
    sortable: true,
    hiddenBelow: "sm",
    className: "w-[120px]",
    cell: (row) => (
      <Badge variant="outline" className="font-normal max-w-full truncate">
        {row.namespace}
      </Badge>
    ),
  },
  {
    id: "schedule",
    header: "Schedule",
    accessorKey: "schedule",
    sortable: true,
    hiddenBelow: "md",
    className: "w-[110px]",
    cell: (row) => <CronSchedule schedule={row.schedule} />,
  },
  {
    id: "successRate",
    header: "Success (7d)",
    accessorKey: "successRate",
    sortable: true,
    align: "right",
    className: "w-[100px]",
    cell: (row) => (
      <span className={cn("font-medium", getSuccessRateColor(row.successRate))}>
        {row.successRate.toFixed(1)}%
      </span>
    ),
  },
  {
    id: "lastSuccess",
    header: "Last Successful Run",
    accessorKey: "lastSuccess",
    sortable: true,
    hiddenBelow: "lg",
    className: "w-[140px]",
    cell: (row) => <RelativeTime date={row.lastSuccess} />,
    sortFn: (a, b) => {
      const tA = a.lastSuccess ? new Date(a.lastSuccess).getTime() : 0;
      const tB = b.lastSuccess ? new Date(b.lastSuccess).getTime() : 0;
      return tA - tB;
    },
  },
  {
    id: "nextRun",
    header: "Next Run",
    accessorKey: "nextRun",
    sortable: true,
    hiddenBelow: "sm",
    className: "w-[120px]",
    cell: (row) => <RelativeTime date={row.nextRun} />,
    sortFn: (a, b) => {
      const tA = a.nextRun ? new Date(a.nextRun).getTime() : 0;
      const tB = b.nextRun ? new Date(b.nextRun).getTime() : 0;
      return tA - tB;
    },
  },
];

export function CronJobsTable({ cronJobs, isLoading }: CronJobsTableProps) {
  const router = useRouter();

  // Extract unique namespaces for filter
  const namespaces = Array.from(
    new Set(cronJobs?.items.map((j) => j.namespace) || [])
  ).sort();

  return (
    <div className="min-h-[500px] flex flex-col">
      <DataTable
        data={cronJobs?.items || []}
        columns={columns}
        getRowKey={(row) => `${row.namespace}/${row.name}`}
        isLoading={isLoading}
        title="CronJobs"
        pageSize={10}
        defaultSort={{ column: "name", direction: "asc" }}
        search={{
          placeholder: "Search cronjobs...",
          searchKeys: ["name", "namespace"],
        }}
        filters={[
          {
            type: "faceted",
            key: "namespace",
            label: "Namespace",
            options: namespaces.map((ns) => ({ label: ns, value: ns })),
          },
        ]}
        enableViewOptions
        onRowClick={(row) => router.push(`/cronjob/${row.namespace}/${row.name}`)}
        emptyState={{
          icon: Timer,
          title: "No CronJobs found",
          description: "CronJobs being monitored will appear here",
        }}
      />
    </div>
  );
}
