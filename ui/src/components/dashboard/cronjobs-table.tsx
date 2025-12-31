"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { ChevronLeft, ChevronRight, Play, Search, Timer } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusIndicator } from "@/components/status-indicator";
import { RelativeTime } from "@/components/relative-time";
import { EmptyState } from "@/components/empty-state";
import { SortableTableHeader } from "@/components/sortable-table-header";
import type { CronJobListResponse, CronJob } from "@/lib/api";
import { cn } from "@/lib/utils";
import { getSuccessRateColor } from "@/lib/constants";

interface CronJobsTableProps {
  cronJobs: CronJobListResponse | null;
  isLoading: boolean;
}

const PAGE_SIZE = 10;

type SortColumn = "name" | "namespace" | "schedule" | "successRate" | "lastSuccess" | "nextRun";
type SortDirection = "asc" | "desc";

function parseDate(dateStr: string | null): number {
  if (!dateStr) return 0;
  return new Date(dateStr).getTime();
}

// Pure function to filter and sort - no side effects
function getDisplayData(
  items: CronJob[] | undefined,
  namespace: string,
  search: string,
  sortColumn: SortColumn,
  sortDirection: SortDirection
): { filtered: CronJob[]; namespaces: string[] } {
  if (!items || items.length === 0) {
    return { filtered: [], namespaces: [] };
  }

  // Get unique namespaces from original data
  const namespaceSet = new Set<string>();
  for (const job of items) {
    namespaceSet.add(job.namespace);
  }
  const namespaces = Array.from(namespaceSet).sort();

  // Filter
  const searchLower = search.toLowerCase();
  const filtered: CronJob[] = [];
  for (const job of items) {
    // Namespace filter
    if (namespace !== "all" && job.namespace !== namespace) {
      continue;
    }
    // Search filter
    if (search) {
      if (
        !job.name.toLowerCase().includes(searchLower) &&
        !job.namespace.toLowerCase().includes(searchLower)
      ) {
        continue;
      }
    }
    filtered.push(job);
  }

  // Sort with stable secondary sort by name
  const multiplier = sortDirection === "asc" ? 1 : -1;
  filtered.sort((a, b) => {
    let comparison = 0;
    switch (sortColumn) {
      case "name":
        comparison = a.name.localeCompare(b.name);
        break;
      case "namespace":
        comparison = a.namespace.localeCompare(b.namespace);
        break;
      case "schedule":
        comparison = a.schedule.localeCompare(b.schedule);
        break;
      case "successRate":
        comparison = a.successRate - b.successRate;
        break;
      case "lastSuccess":
        comparison = parseDate(a.lastSuccess) - parseDate(b.lastSuccess);
        break;
      case "nextRun":
        comparison = parseDate(a.nextRun) - parseDate(b.nextRun);
        break;
    }
    // Stable sort: use name as tiebreaker when primary sort values are equal
    if (comparison === 0 && sortColumn !== "name") {
      comparison = a.name.localeCompare(b.name);
    }
    return comparison * multiplier;
  });

  return { filtered, namespaces };
}

// Paginate filtered results with automatic page clamping
function paginateResults(
  filtered: CronJob[],
  page: number
): { jobs: CronJob[]; totalFiltered: number; effectivePage: number } {
  const totalFiltered = filtered.length;
  const totalPages = Math.max(1, Math.ceil(totalFiltered / PAGE_SIZE));
  // Clamp page to valid range
  const effectivePage = Math.min(page, Math.max(0, totalPages - 1));
  const start = effectivePage * PAGE_SIZE;
  const jobs = filtered.slice(start, start + PAGE_SIZE);

  return { jobs, totalFiltered, effectivePage };
}

export function CronJobsTable({ cronJobs, isLoading }: CronJobsTableProps) {
  const [search, setSearch] = useState("");
  const [namespace, setNamespace] = useState("all");
  const [page, setPage] = useState(0);
  const [sortColumn, setSortColumn] = useState<SortColumn>("name");
  const [sortDirection, setSortDirection] = useState<SortDirection>("asc");

  // Compute filtered data - memoized to avoid recomputation
  const { filtered, namespaces } = useMemo(
    () => getDisplayData(cronJobs?.items, namespace, search, sortColumn, sortDirection),
    [cronJobs?.items, namespace, search, sortColumn, sortDirection]
  );

  // Paginate with automatic page clamping (handles when filters reduce available pages)
  const { jobs, totalFiltered, effectivePage } = useMemo(
    () => paginateResults(filtered, page),
    [filtered, page]
  );

  const totalPages = Math.max(1, Math.ceil(totalFiltered / PAGE_SIZE));

  const handleNamespaceChange = (value: string) => {
    setNamespace(value);
    setPage(0);
  };

  const handleSearchChange = (value: string) => {
    setSearch(value);
    setPage(0);
  };

  const handlePrevPage = () => {
    setPage((p) => Math.max(0, p - 1));
  };

  const handleNextPage = () => {
    setPage((p) => Math.min(totalPages - 1, p + 1));
  };

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(column);
      setSortDirection("asc");
    }
    setPage(0);
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">CronJobs</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <Skeleton className="h-9 w-full" />
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <CardTitle className="text-base font-medium">CronJobs</CardTitle>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
            <Select value={namespace} onValueChange={handleNamespaceChange}>
              <SelectTrigger className="w-full sm:w-40">
                <SelectValue placeholder="All namespaces" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All namespaces</SelectItem>
                {namespaces.map((ns) => (
                  <SelectItem key={ns} value={ns}>
                    {ns}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="relative w-full sm:w-56">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search cronjobs..."
                value={search}
                onChange={(e) => handleSearchChange(e.target.value)}
                className="pl-8"
              />
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="h-[400px] md:h-[520px] flex flex-col">
          {jobs.length === 0 ? (
            <EmptyState
              icon={Timer}
              title={search || namespace !== "all"
                ? "No CronJobs match your filters"
                : "No CronJobs found"}
              description={search || namespace !== "all"
                ? "Try adjusting your search or filter criteria"
                : "CronJobs being monitored will appear here"}
              action={(search || namespace !== "all") ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setSearch("");
                    setNamespace("all");
                    setPage(0);
                  }}
                >
                  Clear filters
                </Button>
              ) : undefined}
              className="flex-1"
            />
          ) : (
            <div className="flex-1 rounded border overflow-hidden">
              <div className="overflow-x-auto">
                <Table className="table-fixed">
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-10"></TableHead>
                      <SortableTableHeader
                        column="name"
                        label="Name"
                        currentSort={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                        className="w-[30%] min-w-[180px]"
                      />
                      <SortableTableHeader
                        column="namespace"
                        label="Namespace"
                        currentSort={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                        hideOnMobile
                        className="w-[120px]"
                      />
                      <SortableTableHeader
                        column="schedule"
                        label="Schedule"
                        currentSort={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                        hideOnTablet
                        className="w-[110px]"
                      />
                      <SortableTableHeader
                        column="successRate"
                        label="Success (7d)"
                        currentSort={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                        align="right"
                        className="w-[100px]"
                      />
                      <SortableTableHeader
                        column="lastSuccess"
                        label="Last Successful Run"
                        currentSort={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                        className="hidden lg:table-cell w-[140px]"
                      />
                      <SortableTableHeader
                        column="nextRun"
                        label="Next Run"
                        currentSort={sortColumn}
                        direction={sortDirection}
                        onSort={handleSort}
                        hideOnMobile
                        className="w-[120px]"
                      />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {jobs.map((job) => (
                      <CronJobRow key={`${job.namespace}/${job.name}`} job={job} />
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          )}
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
              onClick={handlePrevPage}
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
              onClick={handleNextPage}
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
  );
}

function CronJobRow({ job }: { job: CronJob }) {
  const hasActiveJobs = job.activeJobs && job.activeJobs.length > 0;
  const displayStatus = hasActiveJobs ? "running" : job.status;

  return (
    <TableRow className="cursor-pointer hover:bg-muted/50 h-12">
      <TableCell className="w-10">
        <StatusIndicator status={displayStatus} />
      </TableCell>
      <TableCell className="max-w-0">
        <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2 min-w-0">
          <Link
            href={`/cronjob/${job.namespace}/${job.name}`}
            className="font-medium hover:underline truncate"
            title={job.name}
          >
            {job.name}
          </Link>
          {/* Show namespace inline on mobile since column is hidden */}
          <span className="text-xs text-muted-foreground sm:hidden truncate">{job.namespace}</span>
          {hasActiveJobs && (
            <Badge variant="outline" className="bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20 gap-1 w-fit shrink-0">
              <Play className="h-3 w-3 fill-current" />
              {job.activeJobs!.length} running
            </Badge>
          )}
        </div>
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        <Badge variant="outline" className="font-normal max-w-full truncate">
          {job.namespace}
        </Badge>
      </TableCell>
      <TableCell className="hidden md:table-cell font-mono text-sm truncate">{job.schedule}</TableCell>
      <TableCell className="text-right">
        <span className={cn("font-medium", getSuccessRateColor(job.successRate))}>
          {job.successRate.toFixed(1)}%
        </span>
      </TableCell>
      <TableCell className="hidden lg:table-cell">
        <RelativeTime date={job.lastSuccess} />
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        <RelativeTime date={job.nextRun} />
      </TableCell>
    </TableRow>
  );
}
