"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";

interface DataTablePaginationProps {
  /** Current page (0-indexed) */
  page: number;

  /** Total number of pages */
  totalPages: number;

  /** Total number of items */
  totalItems: number;

  /** Number of items per page */
  pageSize: number;

  /** Callback when page changes */
  onPageChange: (page: number) => void;
}

export function DataTablePagination({
  page,
  totalPages,
  totalItems,
  pageSize,
  onPageChange,
}: DataTablePaginationProps) {
  const effectivePage = Math.min(page, Math.max(0, totalPages - 1));
  const startItem = effectivePage * pageSize + 1;
  const endItem = Math.min((effectivePage + 1) * pageSize, totalItems);

  return (
    <div className="flex flex-col sm:flex-row items-center justify-between gap-3 border-t pt-4">
      <div className="text-sm text-muted-foreground order-2 sm:order-1">
        {totalItems > 0 ? (
          <>
            Showing {startItem}-{endItem} of {totalItems}
          </>
        ) : (
          "No items"
        )}
      </div>
      <div className="flex items-center gap-2 order-1 sm:order-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() => onPageChange(Math.max(0, page - 1))}
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
          onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))}
          disabled={effectivePage >= totalPages - 1}
          className="cursor-pointer disabled:cursor-not-allowed"
        >
          <span className="hidden sm:inline">Next</span>
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
