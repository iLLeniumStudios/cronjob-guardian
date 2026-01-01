"use client";

import { DropdownMenuTrigger } from "@radix-ui/react-dropdown-menu";
import { Settings2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import type { ColumnDef } from "./types";

interface DataTableViewOptionsProps<TData> {
  columns: ColumnDef<TData>[];
  hiddenColumns: Set<string>;
  onHiddenColumnsChange: (hiddenColumns: Set<string>) => void;
}

export function DataTableViewOptions<TData>({
  columns,
  hiddenColumns,
  onHiddenColumnsChange,
}: DataTableViewOptionsProps<TData>) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto hidden h-8 lg:flex"
        >
          <Settings2 className="mr-2 h-4 w-4" />
          View
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-[150px]">
        <DropdownMenuLabel>Toggle columns</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {columns
          .filter(
            (column) =>
              typeof column.accessorKey !== "undefined" ||
              typeof column.accessorFn !== "undefined" ||
              column.id
          )
          .map((column) => {
            return (
              <DropdownMenuCheckboxItem
                key={column.id}
                className="capitalize"
                checked={!hiddenColumns.has(column.id)}
                onCheckedChange={(value) => {
                  const newHidden = new Set(hiddenColumns);
                  if (value) {
                    newHidden.delete(column.id);
                  } else {
                    newHidden.add(column.id);
                  }
                  onHiddenColumnsChange(newHidden);
                }}
              >
                {column.header}
              </DropdownMenuCheckboxItem>
            );
          })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
