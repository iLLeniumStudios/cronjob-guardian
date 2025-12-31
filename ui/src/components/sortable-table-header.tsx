import { ChevronUp, ChevronDown } from "lucide-react";
import { TableHead } from "@/components/ui/table";
import { cn } from "@/lib/utils";

interface SortableTableHeaderProps<T extends string> {
  /** Column identifier */
  column: T;
  /** Display label */
  label: string;
  /** Currently sorted column */
  currentSort: T;
  /** Current sort direction */
  direction: "asc" | "desc";
  /** Callback when header is clicked */
  onSort: (column: T) => void;
  /** Optional additional class name */
  className?: string;
  /** Whether the column is hidden on certain breakpoints */
  hideOnMobile?: boolean;
  hideOnTablet?: boolean;
  /** Text alignment */
  align?: "left" | "center" | "right";
}

/**
 * A reusable sortable table header component.
 *
 * @example
 * <SortableTableHeader
 *   column="name"
 *   label="Name"
 *   currentSort={sortColumn}
 *   direction={sortDirection}
 *   onSort={handleSort}
 * />
 */
export function SortableTableHeader<T extends string>({
  column,
  label,
  currentSort,
  direction,
  onSort,
  className,
  hideOnMobile,
  hideOnTablet,
  align = "left",
}: SortableTableHeaderProps<T>) {
  const isActive = currentSort === column;

  const alignmentClasses = {
    left: "",
    center: "text-center",
    right: "text-right",
  };

  const flexAlignment = {
    left: "justify-start",
    center: "justify-center",
    right: "justify-end",
  };

  return (
    <TableHead
      className={cn(
        "cursor-pointer select-none group",
        hideOnMobile && "hidden sm:table-cell",
        hideOnTablet && "hidden md:table-cell",
        alignmentClasses[align],
        className
      )}
      onClick={() => onSort(column)}
    >
      <span className={cn("flex items-center gap-1", flexAlignment[align])}>
        {label}
        <SortIcon isActive={isActive} direction={direction} />
      </span>
    </TableHead>
  );
}

function SortIcon({
  isActive,
  direction,
}: {
  isActive: boolean;
  direction: "asc" | "desc";
}) {
  if (!isActive) {
    return <ChevronUp className="h-3 w-3 opacity-0 group-hover:opacity-30" />;
  }
  return direction === "asc" ? (
    <ChevronUp className="h-3 w-3" />
  ) : (
    <ChevronDown className="h-3 w-3" />
  );
}
