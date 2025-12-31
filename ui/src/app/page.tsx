"use client";

import { useCallback } from "react";
import { Header } from "@/components/header";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { CronJobsTable } from "@/components/dashboard/cronjobs-table";
import { AlertsPanel } from "@/components/dashboard/alerts-panel";
import { useFetchData } from "@/hooks/use-fetch-data";
import {
  getStats,
  listCronJobs,
  listAlerts,
  type StatsResponse,
  type CronJobListResponse,
  type AlertsResponse,
} from "@/lib/api";

interface DashboardData {
  stats: StatsResponse;
  cronJobs: CronJobListResponse;
  alerts: AlertsResponse;
}

export default function DashboardPage() {
  const fetchDashboardData = useCallback(async (): Promise<DashboardData> => {
    const [stats, cronJobs, alerts] = await Promise.all([
      getStats(),
      listCronJobs(),
      listAlerts(),
    ]);
    return { stats, cronJobs, alerts };
  }, []);

  const { data, isLoading, isRefreshing, refetch } = useFetchData(fetchDashboardData);

  return (
    <div className="flex h-full flex-col min-w-0">
      <Header
        title="Dashboard"
        description="Overview of your CronJob health"
        onRefresh={refetch}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-4 md:space-y-6 overflow-auto p-4 md:p-6 min-w-0">
        <StatsCards stats={data?.stats ?? null} isLoading={isLoading} />
        <div className="grid gap-4 md:gap-6 xl:grid-cols-3 min-w-0">
          <div className="xl:col-span-2 min-w-0">
            <CronJobsTable cronJobs={data?.cronJobs ?? null} isLoading={isLoading} />
          </div>
          <div className="min-w-0">
            <AlertsPanel alerts={data?.alerts ?? null} isLoading={isLoading} />
          </div>
        </div>
      </div>
    </div>
  );
}
