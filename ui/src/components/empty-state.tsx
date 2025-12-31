import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  /** Icon to display */
  icon: LucideIcon;
  /** Main title */
  title: string;
  /** Description text */
  description: string;
  /** Optional action element (button, link, etc.) */
  action?: React.ReactNode;
  /** Optional custom class name */
  className?: string;
  /** Whether to show a dashed border (default: true) */
  bordered?: boolean;
}

/**
 * A reusable empty state component for displaying when no data is available.
 *
 * @example
 * <EmptyState
 *   icon={Timer}
 *   title="No CronJobs found"
 *   description="Create a CronJob to get started"
 *   action={<Button>Create CronJob</Button>}
 * />
 */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
  bordered = true,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-12 text-center",
        bordered && "rounded-lg border border-dashed",
        className
      )}
    >
      <Icon className="mb-4 h-12 w-12 text-muted-foreground/50" />
      <p className="text-lg font-medium">{title}</p>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
