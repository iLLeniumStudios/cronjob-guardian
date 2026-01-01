"use client";

import { Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DataTableFacetedFilter } from "./data-table-faceted-filter";
import { DataTableViewOptions } from "./data-table-view-options";
import type { FilterConfig, SearchConfig, ColumnDef } from "./types";

interface DataTableToolbarProps<T> {
  /** Search configuration */
  search?: SearchConfig<T>;

  /** Current search value */
  searchValue: string;

  /** Callback when search value changes */
  onSearchChange: (value: string) => void;

  /** Filter configuration */
  filters?: FilterConfig<T>[];

  /** Current filter values */
  filterValues: Record<string, string | Set<string>>;

  /** Callback when filter values change */
  onFilterChange: (key: string, value: string | Set<string>) => void;

  /** Columns for view options */
  columns?: ColumnDef<T>[];

  /** Hidden columns state */
  hiddenColumns?: Set<string>;

  /** Callback to change hidden columns */
  onHiddenColumnsChange?: (hidden: Set<string>) => void;

  /** Whether to show view options */
  enableViewOptions?: boolean;

  /** Additional actions to display (e.g., export buttons) */
  headerActions?: React.ReactNode;
}

export function DataTableToolbar<T>({
  search,
  searchValue,
  onSearchChange,
  filters,
  filterValues,
  onFilterChange,
  columns,
  hiddenColumns,
  onHiddenColumnsChange,
  enableViewOptions,
  headerActions,
}: DataTableToolbarProps<T>) {
  const hasSearch = !!search;
  const hasFilters = filters && filters.length > 0;
  const hasActions = !!headerActions;
  const hasViewOptions = enableViewOptions && !!columns && !!hiddenColumns && !!onHiddenColumnsChange;

  if (!hasSearch && !hasFilters && !hasActions && !hasViewOptions) {
    return null;
  }

  const isFiltered = Object.values(filterValues).some((value) => {
    if (value instanceof Set) return value.size > 0;
    return value !== "all" && value !== "";
  });

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex flex-1 items-center gap-2 overflow-x-auto pb-2 sm:pb-0">
        {hasSearch && (
          <div className="relative flex-1 sm:max-w-xs min-w-[200px]">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder={search.placeholder || "Search..."}
              value={searchValue}
              onChange={(e) => onSearchChange(e.target.value)}
              className="pl-8 pr-8"
            />
            {searchValue && (
              <Button
                variant="ghost"
                size="sm"
                className="absolute right-1 top-1 h-7 w-7 p-0"
                onClick={() => onSearchChange("")}
              >
                <X className="h-4 w-4" />
                <span className="sr-only">Clear search</span>
              </Button>
            )}
          </div>
        )}

        {hasFilters &&
          filters.map((filter) => {
            const value = filterValues[filter.key as string];

            if (filter.type === "faceted") {
              return (
                <DataTableFacetedFilter
                  key={filter.key as string}
                  title={filter.label}
                  options={filter.options}
                  selectedValues={(value as Set<string>) || new Set()}
                  onSelect={(values) => onFilterChange(filter.key as string, values)}
                />
              );
            }

            // Default to select
            const selectValue = (value as string) || filter.defaultValue || "all";
            const showAll = filter.showAll !== false;
            const allLabel = filter.allLabel || "All";

            return (
              <Select
                key={filter.key as string}
                value={selectValue}
                onValueChange={(val) => onFilterChange(filter.key as string, val)}
              >
                <SelectTrigger className="w-[140px] h-8">
                  <SelectValue placeholder={filter.label || "Filter"} />
                </SelectTrigger>
                <SelectContent>
                  {showAll && <SelectItem value="all">{allLabel}</SelectItem>}
                  {filter.options.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            );
          })}

        {isFiltered && (
          <Button
            variant="ghost"
            onClick={() => {
              // Reset search
              if (searchValue) onSearchChange("");
              // Reset filters
              filters?.forEach((f) => {
                 if (f.type === "faceted") {
                   onFilterChange(f.key as string, new Set());
                 } else {
                   onFilterChange(f.key as string, "all");
                 }
              });
            }}
            className="h-8 px-2 lg:px-3"
          >
            Reset
            <X className="ml-2 h-4 w-4" />
          </Button>
        )}
      </div>

      <div className="flex items-center gap-2">
        {hasViewOptions && (
          <DataTableViewOptions
            columns={columns}
            hiddenColumns={hiddenColumns}
            onHiddenColumnsChange={onHiddenColumnsChange}
          />
        )}
        {hasActions && headerActions}
      </div>
    </div>
  );
}