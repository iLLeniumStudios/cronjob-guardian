"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { ChevronLeft, ChevronRight, Search } from "lucide-react";
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

function getSuccessRateColor(rate: number): string {
  if (rate >= 99) return "text-emerald-600 dark:text-emerald-400";
  if (rate >= 95) return "text-amber-600 dark:text-amber-400";
  return "text-red-600 dark:text-red-400";
}

// Pure function to filter, sort, and paginate - no side effects
function getDisplayData(
  items: CronJob[] | undefined,
  namespace: string,
  search: string,
  page: number
): { jobs: CronJob[]; totalFiltered: number; namespaces: string[] } {
  if (!items || items.length === 0) {
    return { jobs: [], totalFiltered: 0, namespaces: [] };
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

  // Sort by namespace, then name
  filtered.sort((a, b) => {
    const nsCompare = a.namespace.localeCompare(b.namespace);
    if (nsCompare !== 0) return nsCompare;
    return a.name.localeCompare(b.name);
  });

  // Paginate
  const start = page * PAGE_SIZE;
  const jobs = filtered.slice(start, start + PAGE_SIZE);

  return { jobs, totalFiltered: filtered.length, namespaces };
}

export function CronJobsTable({ cronJobs, isLoading }: CronJobsTableProps) {
  const [search, setSearch] = useState("");
  const [namespace, setNamespace] = useState("all");
  const [page, setPage] = useState(0);

  // Compute display data - pure function, no side effects
  const { jobs, totalFiltered, namespaces } = getDisplayData(
    cronJobs?.items,
    namespace,
    search,
    page
  );

  const totalPages = Math.max(1, Math.ceil(totalFiltered / PAGE_SIZE));

  // Clamp page when filters reduce available pages (useEffect to avoid setState during render)
  useEffect(() => {
    if (totalPages > 0 && page >= totalPages) {
      setPage(totalPages - 1);
    }
  }, [page, totalPages]);

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

  // Calculate empty rows needed for consistent height
  const emptyRowCount = PAGE_SIZE - jobs.length;

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-4">
          <CardTitle className="text-base font-medium">CronJobs</CardTitle>
          <div className="flex items-center gap-2">
            <Select value={namespace} onValueChange={handleNamespaceChange}>
              <SelectTrigger className="w-40">
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
            <div className="relative w-56">
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
        <div className="rounded border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-8"></TableHead>
                <TableHead>Name</TableHead>
                <TableHead>Namespace</TableHead>
                <TableHead>Schedule</TableHead>
                <TableHead className="text-right">Success Rate</TableHead>
                <TableHead>Last Successful Run</TableHead>
                <TableHead>Next Run</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {jobs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-12 text-center">
                    {search || namespace !== "all"
                      ? "No cronjobs match your filters"
                      : "No cronjobs found"}
                  </TableCell>
                </TableRow>
              ) : (
                jobs.map((job) => (
                  <CronJobRow key={`${job.namespace}/${job.name}`} job={job} />
                ))
              )}
              {/* Empty rows for consistent height */}
              {emptyRowCount > 0 &&
                Array.from({ length: jobs.length === 0 ? PAGE_SIZE - 1 : emptyRowCount }).map((_, i) => (
                  <TableRow key={`empty-${i}`} className="pointer-events-none">
                    <TableCell colSpan={7} className="h-12">
                      &nbsp;
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </div>
        {/* Pagination */}
        <div className="flex items-center justify-between border-t pt-4 mt-4">
          <div className="text-sm text-muted-foreground">
            {totalFiltered > 0 ? (
              <>
                Showing {page * PAGE_SIZE + 1}-
                {Math.min((page + 1) * PAGE_SIZE, totalFiltered)} of {totalFiltered}
              </>
            ) : (
              "No items"
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handlePrevPage}
              disabled={page === 0}
            >
              <ChevronLeft className="h-4 w-4" />
              Previous
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {page + 1} of {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={handleNextPage}
              disabled={page >= totalPages - 1}
            >
              Next
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function CronJobRow({ job }: { job: CronJob }) {
  return (
    <TableRow className="cursor-pointer hover:bg-muted/50 h-12">
      <TableCell>
        <StatusIndicator status={job.status} />
      </TableCell>
      <TableCell>
        <Link
          href={`/cronjob/${job.namespace}/${job.name}`}
          className="font-medium hover:underline"
        >
          {job.name}
        </Link>
      </TableCell>
      <TableCell>
        <Badge variant="outline" className="font-normal">
          {job.namespace}
        </Badge>
      </TableCell>
      <TableCell className="font-mono text-sm">{job.schedule}</TableCell>
      <TableCell className="text-right">
        <span className={cn("font-medium", getSuccessRateColor(job.successRate))}>
          {job.successRate.toFixed(1)}%
        </span>
      </TableCell>
      <TableCell>
        <RelativeTime date={job.lastSuccess} />
      </TableCell>
      <TableCell>
        <RelativeTime date={job.nextRun} />
      </TableCell>
    </TableRow>
  );
}
