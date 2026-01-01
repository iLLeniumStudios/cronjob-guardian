"use client";

import {
  ArrowDown,
  ArrowUp,
  ChevronsUpDown,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ColumnDef, SortDirection } from "./types";

interface DataTableColumnHeaderProps<TData>
  extends React.HTMLAttributes<HTMLDivElement> {
  column: ColumnDef<TData>;
  title: string;
  currentSortColumn: string;
  currentSortDirection: SortDirection;
  onSort: (column: string, direction: SortDirection) => void;
  // In the future, we can add column visibility toggling here
}

export function DataTableColumnHeader<TData>({
  column,
  title,
  currentSortColumn,
  currentSortDirection,
  onSort,
  className,
}: DataTableColumnHeaderProps<TData>) {
  if (!column.sortable) {
    return <div className={cn(className)}>{title}</div>;
  }

  const isSorted = currentSortColumn === column.id;
  const direction = isSorted ? currentSortDirection : null;

  return (
    <div className={cn("flex items-center space-x-2", className)}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="-ml-3 h-8 data-[state=open]:bg-accent"
          >
            <span>{title}</span>
            {direction === "desc" ? (
              <ArrowDown className="ml-2 h-4 w-4" />
            ) : direction === "asc" ? (
              <ArrowUp className="ml-2 h-4 w-4" />
            ) : (
              <ChevronsUpDown className="ml-2 h-4 w-4" />
            )}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem onClick={() => onSort(column.id, "asc")}>
            <ArrowUp className="mr-2 h-3.5 w-3.5 text-muted-foreground/70" />
            Asc
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => onSort(column.id, "desc")}>
            <ArrowDown className="mr-2 h-3.5 w-3.5 text-muted-foreground/70" />
            Desc
          </DropdownMenuItem>
          {/* 
            TODO: Implement column visibility hiding here when we have a central state for it.
            For now, we just support sorting.
          */}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
