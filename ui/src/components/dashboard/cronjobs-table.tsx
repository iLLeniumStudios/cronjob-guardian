"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { ChevronLeft, ChevronRight, ChevronUp, ChevronDown, Play, Search, Timer } from "lucide-react";
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
import type { CronJobListResponse, CronJob } from "@/lib/api";
import { cn } from "@/lib/utils";

interface CronJobsTableProps {
  cronJobs: CronJobListResponse | null;
  isLoading: boolean;
}

const PAGE_SIZE = 10;

type SortColumn = "name" | "namespace" | "schedule" | "successRate" | "lastSuccess" | "nextRun";
type SortDirection = "asc" | "desc";

function getSuccessRateColor(rate: number): string {
  if (rate >= 99) return "text-emerald-600 dark:text-emerald-400";
  if (rate >= 95) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

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

  // Sort
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

  const SortIcon = ({ column }: { column: SortColumn }) => {
    if (sortColumn !== column) {
      return <ChevronUp className="h-3 w-3 opacity-0 group-hover:opacity-30" />;
    }
    return sortDirection === "asc" ? (
      <ChevronUp className="h-3 w-3" />
    ) : (
      <ChevronDown className="h-3 w-3" />
    );
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
            <div className="flex flex-1 flex-col items-center justify-center rounded-lg border border-dashed">
              <Timer className="mb-3 h-12 w-12 text-muted-foreground/50" />
              <p className="text-lg font-medium text-muted-foreground">
                {search || namespace !== "all"
                  ? "No CronJobs match your filters"
                  : "No CronJobs found"}
              </p>
              <p className="mt-1 text-sm text-muted-foreground/70">
                {search || namespace !== "all"
                  ? "Try adjusting your search or filter criteria"
                  : "CronJobs being monitored will appear here"}
              </p>
              {(search || namespace !== "all") && (
                <Button
                  variant="outline"
                  size="sm"
                  className="mt-4"
                  onClick={() => {
                    setSearch("");
                    setNamespace("all");
                    setPage(0);
                  }}
                >
                  Clear filters
                </Button>
              )}
            </div>
          ) : (
            <div className="flex-1 rounded border overflow-hidden">
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-8"></TableHead>
                      <TableHead
                        className="cursor-pointer select-none group"
                        onClick={() => handleSort("name")}
                      >
                        <span className="flex items-center gap-1">
                          Name
                          <SortIcon column="name" />
                        </span>
                      </TableHead>
                      <TableHead
                        className="hidden sm:table-cell cursor-pointer select-none group"
                        onClick={() => handleSort("namespace")}
                      >
                        <span className="flex items-center gap-1">
                          Namespace
                          <SortIcon column="namespace" />
                        </span>
                      </TableHead>
                      <TableHead
                        className="hidden md:table-cell cursor-pointer select-none group"
                        onClick={() => handleSort("schedule")}
                      >
                        <span className="flex items-center gap-1">
                          Schedule
                          <SortIcon column="schedule" />
                        </span>
                      </TableHead>
                      <TableHead
                        className="text-right cursor-pointer select-none group"
                        onClick={() => handleSort("successRate")}
                      >
                        <span className="flex items-center justify-end gap-1">
                          Success Rate
                          <SortIcon column="successRate" />
                        </span>
                      </TableHead>
                      <TableHead
                        className="hidden lg:table-cell cursor-pointer select-none group"
                        onClick={() => handleSort("lastSuccess")}
                      >
                        <span className="flex items-center gap-1">
                          Last Successful Run
                          <SortIcon column="lastSuccess" />
                        </span>
                      </TableHead>
                      <TableHead
                        className="hidden sm:table-cell cursor-pointer select-none group"
                        onClick={() => handleSort("nextRun")}
                      >
                        <span className="flex items-center gap-1">
                          Next Run
                          <SortIcon column="nextRun" />
                        </span>
                      </TableHead>
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
      <TableCell>
        <StatusIndicator status={displayStatus} />
      </TableCell>
      <TableCell>
        <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2">
          <Link
            href={`/cronjob/${job.namespace}/${job.name}`}
            className="font-medium hover:underline"
          >
            {job.name}
          </Link>
          {/* Show namespace inline on mobile since column is hidden */}
          <span className="text-xs text-muted-foreground sm:hidden">{job.namespace}</span>
          {hasActiveJobs && (
            <Badge variant="outline" className="bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20 gap-1 w-fit">
              <Play className="h-3 w-3 fill-current" />
              {job.activeJobs!.length} running
            </Badge>
          )}
        </div>
      </TableCell>
      <TableCell className="hidden sm:table-cell">
        <Badge variant="outline" className="font-normal">
          {job.namespace}
        </Badge>
      </TableCell>
      <TableCell className="hidden md:table-cell font-mono text-sm">{job.schedule}</TableCell>
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
