"use client";

import { useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { CronJobExecution } from "@/lib/api";

interface HealthHeatmapProps {
  executions: CronJobExecution[];
  onDayClick?: (date: string) => void;
}

const DAYS_IN_YEAR = 365;

interface DayData {
  date: string;
  dateObj: Date;
  successRate: number;
  success: number;
  failed: number;
  total: number;
}

// Helper to get local date key (YYYY-MM-DD) without timezone issues
function getLocalDateKey(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function HealthHeatmap({
  executions,
  onDayClick,
}: HealthHeatmapProps) {
  const { weeks } = useMemo(() => {
    // Group executions by day (using local date)
    const dayMap = new Map<string, { success: number; failed: number }>();

    executions.forEach((exec) => {
      const dateObj = new Date(exec.startTime);
      const dateKey = getLocalDateKey(dateObj);

      if (!dayMap.has(dateKey)) {
        dayMap.set(dateKey, { success: 0, failed: 0 });
      }
      const day = dayMap.get(dateKey)!;
      if (exec.status === "success") {
        day.success++;
      } else {
        day.failed++;
      }
    });

    // Generate all days in the past year
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const startDate = new Date(today);
    startDate.setDate(startDate.getDate() - DAYS_IN_YEAR + 1);

    const days: DayData[] = [];
    const current = new Date(startDate);

    while (current <= today) {
      const dateKey = getLocalDateKey(current);
      const counts = dayMap.get(dateKey) || { success: 0, failed: 0 };
      const total = counts.success + counts.failed;

      days.push({
        date: dateKey,
        dateObj: new Date(current),
        successRate: total > 0 ? (counts.success / total) * 100 : -1, // -1 means no data
        success: counts.success,
        failed: counts.failed,
        total,
      });

      current.setDate(current.getDate() + 1);
    }

    // Group into weeks (starting from Sunday)
    const weeksData: DayData[][] = [];
    let currentWeek: DayData[] = [];

    // Pad the start with empty days to align to week
    const firstDayOfWeek = days[0]?.dateObj.getDay() || 0;
    for (let i = 0; i < firstDayOfWeek; i++) {
      currentWeek.push({
        date: "",
        dateObj: new Date(0),
        successRate: -1,
        success: 0,
        failed: 0,
        total: 0,
      });
    }

    days.forEach((day) => {
      currentWeek.push(day);
      if (currentWeek.length === 7) {
        weeksData.push(currentWeek);
        currentWeek = [];
      }
    });

    // Push remaining days
    if (currentWeek.length > 0) {
      weeksData.push(currentWeek);
    }

    return { weeks: weeksData };
  }, [executions]);

  const getColorClass = (successRate: number): string => {
    if (successRate < 0) return "bg-muted/50"; // No data
    if (successRate === 100) return "bg-emerald-500";
    if (successRate >= 90) return "bg-emerald-400";
    if (successRate >= 75) return "bg-emerald-300 dark:bg-emerald-600";
    if (successRate >= 50) return "bg-amber-400";
    if (successRate >= 25) return "bg-amber-500";
    if (successRate > 0) return "bg-red-400";
    return "bg-red-500"; // 0% success rate
  };

  const formatDate = (date: Date): string => {
    return date.toLocaleDateString("en-US", {
      weekday: "short",
      month: "short",
      day: "numeric",
    });
  };

  const dayLabels = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

  // Generate month labels
  const monthLabels: { label: string; colStart: number }[] = [];
  let lastMonth = -1;
  weeks.forEach((week, weekIndex) => {
    const firstDayWithData = week.find((d) => d.date);
    if (firstDayWithData) {
      const month = firstDayWithData.dateObj.getMonth();
      if (month !== lastMonth) {
        monthLabels.push({
          label: firstDayWithData.dateObj.toLocaleDateString("en-US", {
            month: "short",
          }),
          colStart: weekIndex,
        });
        lastMonth = month;
      }
    }
  });

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-base font-medium">Health Heatmap</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <div className="inline-block min-w-full">
            {/* Month labels */}
            <div className="mb-1 flex">
              <div className="w-9" /> {/* Spacer for day labels */}
              <div className="relative flex-1">
                {monthLabels.map((month, i) => (
                  <span
                    key={i}
                    className="absolute text-xs text-muted-foreground"
                    style={{ left: `${month.colStart * 20}px` }}
                  >
                    {month.label}
                  </span>
                ))}
              </div>
            </div>

            {/* Heatmap grid */}
            <div className="mt-4 flex">
              {/* Day labels */}
              <div className="flex w-9 flex-col gap-1">
                {dayLabels.map((label, i) => (
                  <div
                    key={i}
                    className="flex h-4 items-center text-[10px] text-muted-foreground"
                  >
                    {i % 2 === 1 ? label : ""}
                  </div>
                ))}
              </div>

              {/* Weeks grid */}
              <TooltipProvider delayDuration={0}>
                <div className="flex gap-1">
                  {weeks.map((week, weekIndex) => (
                    <div key={weekIndex} className="flex flex-col gap-1">
                      {week.map((day, dayIndex) => (
                        <Tooltip key={`${weekIndex}-${dayIndex}`}>
                          <TooltipTrigger asChild>
                            <button
                              className={`h-4 w-4 rounded-sm transition-colors ${
                                day.date
                                  ? `${getColorClass(day.successRate)} hover:ring-2 hover:ring-foreground/20`
                                  : "bg-transparent"
                              }`}
                              onClick={() => day.date && onDayClick?.(day.date)}
                              disabled={!day.date}
                            />
                          </TooltipTrigger>
                          {day.date && (
                            <TooltipContent side="top" className="text-xs">
                              <div className="font-medium">
                                {formatDate(day.dateObj)}
                              </div>
                              {day.total > 0 ? (
                                <>
                                  <div className="text-muted-foreground">
                                    Success Rate: {day.successRate.toFixed(0)}%
                                  </div>
                                  <div className="text-muted-foreground">
                                    {day.success} success, {day.failed} failed
                                  </div>
                                </>
                              ) : (
                                <div className="text-muted-foreground">
                                  No executions
                                </div>
                              )}
                            </TooltipContent>
                          )}
                        </Tooltip>
                      ))}
                    </div>
                  ))}
                </div>
              </TooltipProvider>
            </div>

            {/* Legend */}
            <div className="mt-4 flex items-center justify-end gap-2 text-xs text-muted-foreground">
              <span>Less</span>
              <div className="flex gap-1">
                <div className="h-4 w-4 rounded-sm bg-muted/50" />
                <div className="h-4 w-4 rounded-sm bg-red-500" />
                <div className="h-4 w-4 rounded-sm bg-amber-400" />
                <div className="h-4 w-4 rounded-sm bg-emerald-300 dark:bg-emerald-600" />
                <div className="h-4 w-4 rounded-sm bg-emerald-500" />
              </div>
              <span>More</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
