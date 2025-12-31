import { Header } from "@/components/header";
import { Skeleton } from "@/components/ui/skeleton";

type SkeletonVariant = "dashboard" | "grid" | "table" | "detail" | "settings";

interface PageSkeletonProps {
  /** Page title */
  title: string;
  /** Optional page description */
  description?: string;
  /** Skeleton layout variant */
  variant?: SkeletonVariant;
}

/**
 * A reusable page loading skeleton component.
 *
 * @example
 * if (isLoading) {
 *   return <PageSkeleton title="Dashboard" variant="dashboard" />;
 * }
 */
export function PageSkeleton({
  title,
  description,
  variant = "table",
}: PageSkeletonProps) {
  return (
    <div className="flex h-full flex-col">
      <Header title={title} description={description} />
      <div className="flex-1 space-y-6 overflow-auto p-4 md:p-6">
        <SkeletonContent variant={variant} />
      </div>
    </div>
  );
}

function SkeletonContent({ variant }: { variant: SkeletonVariant }) {
  switch (variant) {
    case "dashboard":
      return (
        <>
          {/* Stats row */}
          <div className="grid gap-4 grid-cols-2 md:grid-cols-5">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          {/* Main content */}
          <div className="grid gap-4 md:gap-6 xl:grid-cols-3">
            <div className="xl:col-span-2">
              <Skeleton className="h-[520px]" />
            </div>
            <Skeleton className="h-[520px]" />
          </div>
        </>
      );

    case "grid":
      return (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-48" />
          ))}
        </div>
      );

    case "table":
      return (
        <>
          <div className="grid gap-4 md:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          <Skeleton className="h-96" />
        </>
      );

    case "detail":
      return (
        <>
          {/* Metrics cards */}
          <div className="grid gap-3 grid-cols-2 md:grid-cols-3 lg:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          {/* Charts */}
          <div className="grid gap-4 lg:grid-cols-2">
            <Skeleton className="h-64" />
            <Skeleton className="h-64" />
          </div>
          {/* Heatmap */}
          <Skeleton className="h-48" />
          {/* History */}
          <Skeleton className="h-96" />
        </>
      );

    case "settings":
      return (
        <div className="grid gap-6 lg:grid-cols-2">
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
          <Skeleton className="h-64" />
        </div>
      );

    default:
      return <Skeleton className="h-96" />;
  }
}
