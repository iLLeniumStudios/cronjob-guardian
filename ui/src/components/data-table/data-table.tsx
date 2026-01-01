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
import { DataTableColumnHeader } from "./data-table-column-header";
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

  if (aValue == null && bValue == null) return 0;
  if (aValue == null) return 1 * multiplier;
  if (bValue == null) return -1 * multiplier;

  if (typeof aValue === "number" && typeof bValue === "number") {
    return (aValue - bValue) * multiplier;
  }

  if (aValue instanceof Date && bValue instanceof Date) {
    return (aValue.getTime() - bValue.getTime()) * multiplier;
  }

  return String(aValue).localeCompare(String(bValue)) * multiplier;
}

interface ExtendedDataTableProps<T> extends DataTableProps<T> {
  className?: string;
}

export function DataTable<T>({
  data,
  columns,
  getRowKey,
  pageSize = DEFAULT_PAGE_SIZE,
  defaultSort: defaultSortConfig,
  filters,
  search,
  emptyState,
  noResultsState,
  title,
  headerActions,
  enableViewOptions = false,
  showCard = true,
  isLoading = false,
  onRowClick,
  className,
}: ExtendedDataTableProps<T>) {
  // State
  const [page, setPage] = useState(0);
  const [sortColumn, setSortColumn] = useState<string>(
    defaultSortConfig?.column || ""
  );
  const [sortDirection, setSortDirection] = useState<SortDirection>(
    defaultSortConfig?.direction || "asc"
  );
  const [searchValue, setSearchValue] = useState("");
  
  // Filter state
  const [filterValues, setFilterValues] = useState<Record<string, string | Set<string>>>(
    () => {
      const initial: Record<string, string | Set<string>> = {};
      filters?.forEach((f) => {
        if (f.type === "faceted") {
          initial[f.key as string] = new Set<string>();
        } else {
          initial[f.key as string] = f.defaultValue || "all";
        }
      });
      return initial;
    }
  );

  const [hiddenColumns, setHiddenColumns] = useState<Set<string>>(new Set());

  // Get column by ID
  const getColumn = useCallback(
    (id: string) => columns.find((c) => c.id === id),
    [columns]
  );

  // Process data
  const { processedData, totalFiltered, totalPages } = useMemo(() => {
    let result = [...data];

    // Apply filters
    if (filters && filters.length > 0) {
      result = result.filter((row) => {
        return filters.every((filter) => {
          const rowValue = String(row[filter.key]);
          const filterValue = filterValues[filter.key as string];

          if (filter.type === "faceted") {
            const selected = filterValue as Set<string>;
            if (!selected || selected.size === 0) return true;
            return selected.has(rowValue);
          } else {
            const selected = filterValue as string;
            if (!selected || selected === "all") return true;
            return rowValue === selected;
          }
        });
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
          let comparison: number;
          if (column.sortFn) {
            comparison = column.sortFn(a, b);
            if (sortDirection === "desc") comparison = -comparison;
          } else {
            comparison = defaultSort(a, b, column, sortDirection);
          }
          if (comparison === 0) {
            comparison = getRowKey(a).localeCompare(getRowKey(b));
          }
          return comparison;
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
  }, [
    data,
    filters,
    filterValues,
    search,
    searchValue,
    sortColumn,
    sortDirection,
    getColumn,
    getRowKey,
    page,
    pageSize,
  ]);

  // Handlers
  const handleSort = useCallback(
    (column: string, direction: SortDirection) => {
      if (sortColumn === column && direction === sortDirection) {
         setSortDirection(direction === "asc" ? "desc" : "asc");
      } else {
        setSortColumn(column);
        setSortDirection(direction);
      }
      setPage(0);
    },
    [sortColumn, sortDirection]
  );

  const handleSearchChange = useCallback((value: string) => {
    setSearchValue(value);
    setPage(0);
  }, []);

  const handleFilterChange = useCallback((key: string, value: string | Set<string>) => {
    setFilterValues((prev) => ({
      ...prev,
      [key]: value,
    }));
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
      <Card className={cn("flex flex-col", className)}>
        {title && (
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-medium">{title}</CardTitle>
          </CardHeader>
        )}
        <CardContent className="flex-1">{content}</CardContent>
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
    <div className="flex flex-col h-full space-y-4">
      {/* Toolbar */}
      <DataTableToolbar
        search={search}
        searchValue={searchValue}
        onSearchChange={handleSearchChange}
        filters={filters}
        filterValues={filterValues}
        onFilterChange={handleFilterChange}
        columns={columns}
        hiddenColumns={hiddenColumns}
        onHiddenColumnsChange={setHiddenColumns}
        enableViewOptions={enableViewOptions}
        headerActions={headerActions}
      />

      {/* Empty/No Results State */}
      {(showEmptyState || showNoResultsState) ? (
        <div className="flex-1 flex flex-col">
          <EmptyState
            icon={(noResultsState || emptyState)!.icon}
            title={(noResultsState || emptyState)!.title}
            description={(noResultsState || emptyState)!.description}
            action={(noResultsState || emptyState)!.action}
            bordered={false}
            className="flex-1"
          />
        </div>
      ) : (
        <>
          {/* Table Container - Takes remaining space */}
          <div className="rounded border overflow-x-auto flex-1">
            <Table>
              <TableHeader>
                <TableRow>
                  {columns
                    .filter((col) => !hiddenColumns.has(col.id))
                    .map((column) => {
                      const hiddenClass = getHiddenClass(column.hiddenBelow);
                      const alignClass = getAlignClass(column.align);

                      return (
                        <TableHead
                          key={column.id}
                          className={cn(
                            hiddenClass,
                            alignClass,
                            column.headerClassName
                          )}
                        >
                          <DataTableColumnHeader
                            column={column}
                            title={column.header}
                            currentSortColumn={sortColumn}
                            currentSortDirection={sortDirection}
                            onSort={handleSort}
                          />
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
                    {columns
                      .filter((col) => !hiddenColumns.has(col.id))
                      .map((column) => {
                        const hiddenClass = getHiddenClass(column.hiddenBelow);
                        const alignClass = getAlignClass(column.align);

                        return (
                          <TableCell
                            key={column.id}
                            className={cn(
                              hiddenClass,
                              alignClass,
                              column.className
                            )}
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

          {/* Pagination - Pushed to bottom */}
          {pageSize > 0 && totalFiltered > 0 && (
            <div className="mt-auto">
              <DataTablePagination
                page={page}
                totalPages={totalPages}
                totalItems={totalFiltered}
                pageSize={pageSize}
                onPageChange={handlePageChange}
              />
            </div>
          )}
        </>
      )}
    </div>
  );

  // Wrap in card if needed
  if (!showCard) {
    return <div className={cn("h-full", className)}>{tableContent}</div>;
  }

  return (
    <Card className={cn("flex flex-col", className)}>
      {title && (
        <CardHeader className="pb-3">
          <CardTitle className="text-base font-medium">{title}</CardTitle>
        </CardHeader>
      )}
      <CardContent className="flex-1 flex flex-col overflow-hidden">{tableContent}</CardContent>
    </Card>
  );
}
