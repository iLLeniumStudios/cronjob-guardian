"use client";

import { useState, useMemo, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/empty-state";
import { SortableTableHeader } from "@/components/sortable-table-header";
import { DataTableCell } from "./data-table-cell";
import { DataTableToolbar } from "./data-table-toolbar";
import { DataTablePagination } from "./data-table-pagination";
import { cn } from "@/lib/utils";
import type { ColumnDef, DataTableProps, SortDirection } from "./types";

const DEFAULT_PAGE_SIZE = 20;

function getHiddenClass(hiddenBelow?: "sm" | "md" | "lg" | "xl"): string {
  switch (hiddenBelow) {
    case "sm":
      return "hidden sm:table-cell";
    case "md":
      return "hidden md:table-cell";
    case "lg":
      return "hidden lg:table-cell";
    case "xl":
      return "hidden xl:table-cell";
    default:
      return "";
  }
}

function getAlignClass(align?: "left" | "center" | "right"): string {
  switch (align) {
    case "center":
      return "text-center";
    case "right":
      return "text-right";
    default:
      return "text-left";
  }
}

function getValue<T>(row: T, column: ColumnDef<T>): unknown {
  if (column.accessorFn) {
    return column.accessorFn(row);
  }
  if (column.accessorKey) {
    return row[column.accessorKey];
  }
  return undefined;
}

function defaultSort<T>(
  a: T,
  b: T,
  column: ColumnDef<T>,
  direction: SortDirection
): number {
  const aValue = getValue(a, column);
  const bValue = getValue(b, column);
  const multiplier = direction === "asc" ? 1 : -1;

  // Handle null/undefined
  if (aValue == null && bValue == null) return 0;
  if (aValue == null) return 1 * multiplier;
  if (bValue == null) return -1 * multiplier;

  // Compare based on type
  if (typeof aValue === "number" && typeof bValue === "number") {
    return (aValue - bValue) * multiplier;
  }

  if (aValue instanceof Date && bValue instanceof Date) {
    return (aValue.getTime() - bValue.getTime()) * multiplier;
  }

  // String comparison for everything else
  return String(aValue).localeCompare(String(bValue)) * multiplier;
}

export function DataTable<T>({
  data,
  columns,
  getRowKey,
  pageSize = DEFAULT_PAGE_SIZE,
  defaultSort: defaultSortConfig,
  filter,
  search,
  emptyState,
  noResultsState,
  title,
  headerActions,
  showCard = true,
  isLoading = false,
  onRowClick,
}: DataTableProps<T>) {
  // State
  const [page, setPage] = useState(0);
  const [sortColumn, setSortColumn] = useState<string>(
    defaultSortConfig?.column || ""
  );
  const [sortDirection, setSortDirection] = useState<SortDirection>(
    defaultSortConfig?.direction || "asc"
  );
  const [searchValue, setSearchValue] = useState("");
  const [filterValue, setFilterValue] = useState(
    filter?.defaultValue || "all"
  );

  // Get column by ID
  const getColumn = useCallback(
    (id: string) => columns.find((c) => c.id === id),
    [columns]
  );

  // Process data: filter, search, sort, paginate
  const { processedData, totalFiltered, totalPages } = useMemo(() => {
    let result = [...data];

    // Apply filter
    if (filter && filterValue !== "all") {
      result = result.filter((row) => {
        const value = row[filter.key];
        return String(value) === filterValue;
      });
    }

    // Apply search
    if (search && searchValue.trim()) {
      const searchLower = searchValue.toLowerCase().trim();
      result = result.filter((row) => {
        return search.searchKeys.some((key) => {
          const value = row[key];
          return String(value ?? "")
            .toLowerCase()
            .includes(searchLower);
        });
      });
    }

    // Apply sort
    if (sortColumn) {
      const column = getColumn(sortColumn);
      if (column) {
        result.sort((a, b) => {
          if (column.sortFn) {
            const comparison = column.sortFn(a, b);
            return sortDirection === "asc" ? comparison : -comparison;
          }
          return defaultSort(a, b, column, sortDirection);
        });
      }
    }

    // Calculate pagination
    const totalFiltered = result.length;
    const totalPages = pageSize > 0 ? Math.max(1, Math.ceil(totalFiltered / pageSize)) : 1;
    const effectivePage = Math.min(page, Math.max(0, totalPages - 1));

    // Apply pagination
    if (pageSize > 0) {
      const start = effectivePage * pageSize;
      result = result.slice(start, start + pageSize);
    }

    return {
      processedData: result,
      totalFiltered,
      totalPages,
    };
  }, [data, filter, filterValue, search, searchValue, sortColumn, sortDirection, getColumn, page, pageSize]);

  // Handlers
  const handleSort = useCallback(
    (column: string) => {
      if (sortColumn === column) {
        setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
      } else {
        setSortColumn(column);
        setSortDirection("asc");
      }
      setPage(0);
    },
    [sortColumn]
  );

  const handleSearchChange = useCallback((value: string) => {
    setSearchValue(value);
    setPage(0);
  }, []);

  const handleFilterChange = useCallback((value: string) => {
    setFilterValue(value);
    setPage(0);
  }, []);

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage);
  }, []);

  // Loading skeleton
  if (isLoading) {
    const content = (
      <div className="space-y-4">
        <div className="flex gap-2">
          <Skeleton className="h-10 w-64" />
          <Skeleton className="h-10 w-32" />
        </div>
        <div className="rounded border">
          <div className="border-b p-3">
            <Skeleton className="h-4 w-full" />
          </div>
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="border-b p-3">
              <Skeleton className="h-4 w-full" />
            </div>
          ))}
        </div>
      </div>
    );

    if (!showCard) return content;

    return (
      <Card>
        {title && (
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">{title}</CardTitle>
          </CardHeader>
        )}
        <CardContent>{content}</CardContent>
      </Card>
    );
  }

  // Determine which empty state to show
  const isEmpty = data.length === 0;
  const hasNoResults = !isEmpty && processedData.length === 0;
  const showEmptyState = isEmpty && emptyState;
  const showNoResultsState = hasNoResults && (noResultsState || emptyState);

  // Build table content
  const tableContent = (
    <div className="space-y-4">
      {/* Toolbar */}
      <DataTableToolbar
        search={search}
        searchValue={searchValue}
        onSearchChange={handleSearchChange}
        filter={filter}
        filterValue={filterValue}
        onFilterChange={handleFilterChange}
        headerActions={headerActions}
      />

      {/* Empty/No Results State */}
      {(showEmptyState || showNoResultsState) ? (
        <EmptyState
          icon={(noResultsState || emptyState)!.icon}
          title={(noResultsState || emptyState)!.title}
          description={(noResultsState || emptyState)!.description}
          action={(noResultsState || emptyState)!.action}
          bordered={false}
        />
      ) : (
        <>
          {/* Table */}
          <div className="rounded border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  {columns.map((column) => {
                    const hiddenClass = getHiddenClass(column.hiddenBelow);
                    const alignClass = getAlignClass(column.align);

                    if (column.sortable) {
                      return (
                        <SortableTableHeader
                          key={column.id}
                          column={column.id}
                          label={column.header}
                          currentSort={sortColumn}
                          direction={sortDirection}
                          onSort={handleSort}
                          className={cn(hiddenClass, column.headerClassName)}
                          align={column.align}
                        />
                      );
                    }

                    return (
                      <TableHead
                        key={column.id}
                        className={cn(hiddenClass, alignClass, column.headerClassName)}
                      >
                        {column.header}
                      </TableHead>
                    );
                  })}
                </TableRow>
              </TableHeader>
              <TableBody>
                {processedData.map((row) => (
                  <TableRow
                    key={getRowKey(row)}
                    className={onRowClick ? "cursor-pointer" : undefined}
                    onClick={onRowClick ? () => onRowClick(row) : undefined}
                  >
                    {columns.map((column) => {
                      const hiddenClass = getHiddenClass(column.hiddenBelow);
                      const alignClass = getAlignClass(column.align);

                      return (
                        <TableCell
                          key={column.id}
                          className={cn(hiddenClass, alignClass, column.className)}
                        >
                          <DataTableCell row={row} column={column} />
                        </TableCell>
                      );
                    })}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          {/* Pagination */}
          {pageSize > 0 && totalFiltered > 0 && (
            <DataTablePagination
              page={page}
              totalPages={totalPages}
              totalItems={totalFiltered}
              pageSize={pageSize}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}
    </div>
  );

  // Wrap in card if needed
  if (!showCard) {
    return tableContent;
  }

  return (
    <Card>
      {title && (
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">{title}</CardTitle>
        </CardHeader>
      )}
      <CardContent>{tableContent}</CardContent>
    </Card>
  );
}
