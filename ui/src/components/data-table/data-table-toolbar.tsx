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
import type { FilterConfig, SearchConfig } from "./types";

interface DataTableToolbarProps<T> {
  /** Search configuration */
  search?: SearchConfig<T>;

  /** Current search value */
  searchValue: string;

  /** Callback when search value changes */
  onSearchChange: (value: string) => void;

  /** Filter configuration */
  filter?: FilterConfig<T>;

  /** Current filter value */
  filterValue: string;

  /** Callback when filter value changes */
  onFilterChange: (value: string) => void;

  /** Additional actions to display (e.g., export buttons) */
  headerActions?: React.ReactNode;
}

export function DataTableToolbar<T>({
  search,
  searchValue,
  onSearchChange,
  filter,
  filterValue,
  onFilterChange,
  headerActions,
}: DataTableToolbarProps<T>) {
  const hasSearch = !!search;
  const hasFilter = !!filter;
  const hasActions = !!headerActions;

  if (!hasSearch && !hasFilter && !hasActions) {
    return null;
  }

  const showAll = filter?.showAll !== false;
  const allLabel = filter?.allLabel || "All";

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex flex-1 items-center gap-2">
        {hasSearch && (
          <div className="relative flex-1 sm:max-w-xs">
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

        {hasFilter && (
          <Select value={filterValue} onValueChange={onFilterChange}>
            <SelectTrigger className="w-[140px]">
              <SelectValue placeholder={filter.label || "Filter"} />
            </SelectTrigger>
            <SelectContent>
              {showAll && (
                <SelectItem value="all">{allLabel}</SelectItem>
              )}
              {filter.options.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      {hasActions && (
        <div className="flex items-center gap-2">
          {headerActions}
        </div>
      )}
    </div>
  );
}
