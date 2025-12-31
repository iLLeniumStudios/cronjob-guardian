"use client";

import Link from "next/link";
import { TrendingUp, TrendingDown, Minus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { StatusIndicator } from "@/components/status-indicator";
import { RelativeTime } from "@/components/relative-time";
import { cn } from "@/lib/utils";
import type { ColumnDef, StatusVariant, TrendDirection } from "./types";

interface DataTableCellProps<T> {
  row: T;
  column: ColumnDef<T>;
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

function getPercentageColor(
  value: number,
  thresholds?: { good: number; warning: number }
): string {
  if (!thresholds) {
    return "";
  }
  if (value >= thresholds.good) {
    return "text-emerald-600 dark:text-emerald-400";
  }
  if (value >= thresholds.warning) {
    return "text-amber-600 dark:text-amber-400";
  }
  return "text-red-600 dark:text-red-400";
}

function TrendIndicator({ direction }: { direction: TrendDirection }) {
  switch (direction) {
    case "improving":
      return (
        <div className="flex items-center justify-center gap-1 text-emerald-600 dark:text-emerald-400">
          <TrendingUp className="h-4 w-4" />
          <span className="text-xs">Up</span>
        </div>
      );
    case "declining":
      return (
        <div className="flex items-center justify-center gap-1 text-red-600 dark:text-red-400">
          <TrendingDown className="h-4 w-4" />
          <span className="text-xs">Down</span>
        </div>
      );
    case "stable":
    default:
      return (
        <div className="flex items-center justify-center gap-1 text-muted-foreground">
          <Minus className="h-4 w-4" />
          <span className="text-xs">Stable</span>
        </div>
      );
  }
}

export function DataTableCell<T>({ row, column }: DataTableCellProps<T>) {
  // Use custom renderer if provided
  if (column.cell) {
    return <>{column.cell(row)}</>;
  }

  const value = getValue(row, column);
  const type = column.type || "text";

  switch (type) {
    case "link": {
      const href = column.linkHref?.(row) || "#";
      const subtitle = column.subtitle?.(row);
      const displayValue = String(value ?? "");
      return (
        <div>
          <Link href={href} className="font-medium hover:underline">
            {displayValue}
          </Link>
          {subtitle && (
            <div className="text-xs text-muted-foreground">{subtitle}</div>
          )}
        </div>
      );
    }

    case "badge": {
      const variant = column.badgeVariant?.(row) || "secondary";
      const badgeClassName = column.badgeClassName?.(row);
      return (
        <Badge variant={variant} className={badgeClassName}>
          {String(value ?? "")}
        </Badge>
      );
    }

    case "status": {
      const status = (column.statusVariant?.(row) ||
        (value as StatusVariant) ||
        "unknown") as StatusVariant;
      return <StatusIndicator status={status} />;
    }

    case "percentage": {
      const numValue = typeof value === "number" ? value : parseFloat(String(value));
      const colorClass = getPercentageColor(numValue, column.percentageThresholds);
      return (
        <span className={cn("font-mono", colorClass)}>
          {isNaN(numValue) ? "-" : `${numValue.toFixed(1)}%`}
        </span>
      );
    }

    case "relativeTime": {
      const dateValue = value as string | Date | null | undefined;
      return <RelativeTime date={dateValue} />;
    }

    case "duration": {
      return <span className="font-mono text-sm">{String(value ?? "-")}</span>;
    }

    case "trend": {
      const direction = column.trendDirection?.(row) || (value as TrendDirection) || "stable";
      return <TrendIndicator direction={direction} />;
    }

    case "actions": {
      // Actions should always use custom cell renderer
      return null;
    }

    case "custom": {
      // Custom should always use custom cell renderer
      return <span>{String(value ?? "")}</span>;
    }

    case "text":
    default: {
      return <span>{String(value ?? "")}</span>;
    }
  }
}
