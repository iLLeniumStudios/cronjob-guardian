import type { LucideIcon } from "lucide-react";

export type ColumnType =
  | "text"
  | "link"
  | "badge"
  | "status"
  | "percentage"
  | "relativeTime"
  | "duration"
  | "trend"
  | "actions"
  | "custom";

export type SortDirection = "asc" | "desc";

export type BadgeVariant = "default" | "secondary" | "outline" | "destructive";

export type StatusVariant = "healthy" | "warning" | "critical" | "running" | "suspended";

export type TrendDirection = "improving" | "declining" | "stable";

export interface ColumnDef<T> {
  /** Unique identifier for the column */
  id: string;

  /** Header text displayed in the table header */
  header: string;

  /** Key to access the value from the row data */
  accessorKey?: keyof T;

  /** Function to derive the value from the row data */
  accessorFn?: (row: T) => unknown;

  /** Column type determines rendering - defaults to "text" */
  type?: ColumnType;

  // --- Type-specific options ---

  /** For type="link": Function to generate the href */
  linkHref?: (row: T) => string;

  /** For type="link": Optional subtitle text below the link */
  subtitle?: (row: T) => string | undefined;

  /** For type="badge": Function to determine badge variant */
  badgeVariant?: (row: T) => BadgeVariant;

  /** For type="badge": Optional className for the badge */
  badgeClassName?: (row: T) => string | undefined;

  /** For type="status": Function to determine status variant */
  statusVariant?: (row: T) => StatusVariant;

  /** For type="percentage": Thresholds for color coding */
  percentageThresholds?: {
    good: number;   // >= this is green
    warning: number; // >= this is amber, below is red
  };

  /** For type="trend": Function to determine trend direction */
  trendDirection?: (row: T) => TrendDirection;

  // --- Sorting ---

  /** Whether the column is sortable */
  sortable?: boolean;

  /** Custom sort function for complex sorting logic */
  sortFn?: (a: T, b: T) => number;

  // --- Custom rendering ---

  /** Custom cell renderer for complex cases */
  cell?: (row: T) => React.ReactNode;

  // --- Responsive visibility ---

  /** Hide this column below the specified breakpoint */
  hiddenBelow?: "sm" | "md" | "lg" | "xl";

  // --- Layout ---

  /** Text alignment within the cell */
  align?: "left" | "center" | "right";

  /** Additional className for the column cells */
  className?: string;

  /** Additional className for the header cell */
  headerClassName?: string;
}

export interface BaseFilterConfig<T> {
  /** Key in the row data to filter on */
  key: keyof T;

  /** Label shown next to the filter dropdown */
  label?: string;

  /** Available filter options */
  options: { value: string; label: string; icon?: React.ComponentType<{ className?: string }> }[];
}

export interface SelectFilterConfig<T> extends BaseFilterConfig<T> {
  type: "select";
  /** Default selected value (defaults to first option or "all") */
  defaultValue?: string;
  /** Whether to include an "All" option (defaults to true) */
  showAll?: boolean;
  /** Label for the "All" option (defaults to "All") */
  allLabel?: string;
}

export interface FacetedFilterConfig<T> extends BaseFilterConfig<T> {
  type: "faceted";
}

export type FilterConfig<T> = SelectFilterConfig<T> | FacetedFilterConfig<T>;

export interface SearchConfig<T> {
  /** Placeholder text for the search input */
  placeholder?: string;

  /** Keys in the row data to search against */
  searchKeys: (keyof T)[];
}

export interface EmptyStateConfig {
  /** Icon to display */
  icon: LucideIcon;

  /** Title text */
  title: string;

  /** Description text */
  description: string;

  /** Optional action button/link */
  action?: React.ReactNode;
}

export interface DataTableProps<T> {
  /** Array of data to display */
  data: T[];

  /** Column definitions */
  columns: ColumnDef<T>[];

  /** Function to generate a unique key for each row */
  getRowKey: (row: T) => string;

  // --- Pagination ---

  /** Number of items per page. Set to 0 to disable pagination. Defaults to 20. */
  pageSize?: number;

  // --- Sorting ---

  /** Default sort configuration */
  defaultSort?: {
    column: string;
    direction: SortDirection;
  };

  // --- Filtering ---

  /** Filter configuration */
  filters?: FilterConfig<T>[];

  // --- Search ---

  /** Search input configuration */
  search?: SearchConfig<T>;

  // --- View Options ---

  /** Whether to show the column visibility toggle */
  enableViewOptions?: boolean;

  // --- Empty State ---

  /** Configuration for the empty state display */
  emptyState?: EmptyStateConfig;

  /** Empty state to show when filters return no results (uses emptyState if not provided) */
  noResultsState?: EmptyStateConfig;

  // --- Header ---

  /** Title displayed above the table */
  title?: string;

  /** Actions to display in the header (e.g., export buttons) */
  headerActions?: React.ReactNode;

  // --- Wrapper ---

  /** Whether to wrap the table in a Card component. Defaults to true. */
  showCard?: boolean;

  // --- Loading ---

  /** Whether the table is in a loading state */
  isLoading?: boolean;

  // --- Callbacks ---

  /** Called when a row is clicked */
  onRowClick?: (row: T) => void;
}

export interface SortState {
  column: string;
  direction: SortDirection;
}

export interface PaginationState {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
}
