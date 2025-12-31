import type { LucideIcon } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { ICON_COLORS, type IconColor } from "@/lib/constants";

interface StatCardProps {
  /** Icon to display */
  icon: LucideIcon;
  /** Color theme for the icon */
  iconColor: IconColor;
  /** Label text */
  label: string;
  /** Value to display (can be string, number, or React node) */
  value: React.ReactNode;
  /** Optional sub-value or additional info */
  subValue?: React.ReactNode;
  /** Optional trend indicator */
  trend?: {
    direction: "up" | "down" | "stable";
    value: string;
  };
  /** Optional custom class name for the value */
  valueClassName?: string;
}

/**
 * A reusable stat card component for displaying metrics.
 *
 * @example
 * <StatCard
 *   icon={AlertCircle}
 *   iconColor="red"
 *   label="Critical Alerts"
 *   value={5}
 * />
 */
export function StatCard({
  icon: Icon,
  iconColor,
  label,
  value,
  subValue,
  valueClassName,
}: StatCardProps) {
  const colors = ICON_COLORS[iconColor];

  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-3">
          <div className={cn("rounded p-2 md:p-2.5", colors.bg)}>
            <Icon className={cn("h-4 w-4 md:h-5 md:w-5", colors.icon)} />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm text-muted-foreground truncate">{label}</p>
            <p className={cn("text-xl md:text-2xl font-semibold", valueClassName)}>
              {value}
            </p>
            {subValue && (
              <p className="text-xs text-muted-foreground truncate">{subValue}</p>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

interface SimpleStatCardProps {
  /** Label text */
  label: string;
  /** Value to display */
  value: React.ReactNode;
  /** Optional custom class name for the value */
  valueClassName?: string;
}

/**
 * A simple stat card without an icon.
 */
export function SimpleStatCard({ label, value, valueClassName }: SimpleStatCardProps) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className={cn("mt-1 text-xl md:text-2xl font-semibold", valueClassName)}>
          {value}
        </p>
      </CardContent>
    </Card>
  );
}
