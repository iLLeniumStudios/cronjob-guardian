"use client";

import { Timer, CheckCircle2, AlertTriangle, XCircle, Play } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { StatsResponse } from "@/lib/api";

interface StatsCardsProps {
  stats: StatsResponse | null;
  isLoading: boolean;
}

export function StatsCards({ stats, isLoading }: StatsCardsProps) {
  if (isLoading) {
    return (
      <div className="grid grid-cols-2 gap-3 md:gap-4 lg:grid-cols-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <Card key={i}>
            <CardContent className="p-3 md:p-4">
              <div className="flex items-center gap-3 md:gap-4">
                <Skeleton className="h-9 w-9 md:h-10 md:w-10 rounded" />
                <div className="space-y-2">
                  <Skeleton className="h-3 md:h-4 w-16 md:w-20" />
                  <Skeleton className="h-5 md:h-6 w-10 md:w-12" />
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  const cards = [
    {
      label: "Total CronJobs",
      value: stats?.totalCronJobs ?? 0,
      icon: Timer,
      color: "text-blue-600 dark:text-blue-400",
      bgColor: "bg-blue-500/10",
    },
    {
      label: "Healthy",
      value: stats?.summary.healthy ?? 0,
      icon: CheckCircle2,
      color: "text-emerald-600 dark:text-emerald-400",
      bgColor: "bg-emerald-500/10",
    },
    {
      label: "Warning",
      value: stats?.summary.warning ?? 0,
      icon: AlertTriangle,
      color: "text-amber-600 dark:text-amber-400",
      bgColor: "bg-amber-500/10",
    },
    {
      label: "Critical",
      value: stats?.summary.critical ?? 0,
      icon: XCircle,
      color: "text-red-600 dark:text-red-400",
      bgColor: "bg-red-500/10",
    },
    {
      label: "Running",
      value: stats?.summary.running ?? 0,
      icon: Play,
      color: "text-blue-600 dark:text-blue-400",
      bgColor: "bg-blue-500/10",
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 md:gap-4 lg:grid-cols-5">
      {cards.map((card) => (
        <Card key={card.label}>
          <CardContent className="p-3 md:p-4">
            <div className="flex items-center gap-3 md:gap-4">
              <div className={`rounded p-2 md:p-2.5 ${card.bgColor}`}>
                <card.icon className={`h-4 w-4 md:h-5 md:w-5 ${card.color}`} />
              </div>
              <div>
                <p className="text-xs md:text-sm text-muted-foreground">{card.label}</p>
                <p className="text-xl md:text-2xl font-semibold">{card.value}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
