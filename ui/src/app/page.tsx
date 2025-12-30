"use client";

import { useCallback, useEffect, useState } from "react";
import { Header } from "@/components/header";
import { StatsCards } from "@/components/dashboard/stats-cards";
import { CronJobsTable } from "@/components/dashboard/cronjobs-table";
import { AlertsPanel } from "@/components/dashboard/alerts-panel";
import {
  getStats,
  listCronJobs,
  listAlerts,
  type StatsResponse,
  type CronJobListResponse,
  type AlertsResponse,
} from "@/lib/api";

export default function DashboardPage() {
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [cronJobs, setCronJobs] = useState<CronJobListResponse | null>(null);
  const [alerts, setAlerts] = useState<AlertsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const [statsData, cronJobsData, alertsData] = await Promise.all([
        getStats(),
        listCronJobs(),
        listAlerts(),
      ]);
      setStats(statsData);
      setCronJobs(cronJobsData);
      setAlerts(alertsData);
    } catch (error) {
      console.error("Failed to fetch dashboard data:", error);
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    // Auto-refresh every 30 seconds
    const interval = setInterval(() => fetchData(), 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const handleRefresh = () => fetchData(true);

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Dashboard"
        description="Overview of your CronJob health"
        onRefresh={handleRefresh}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-6">
        <StatsCards stats={stats} isLoading={isLoading} />
        <div className="grid gap-6 lg:grid-cols-3">
          <div className="lg:col-span-2">
            <CronJobsTable cronJobs={cronJobs} isLoading={isLoading} />
          </div>
          <div>
            <AlertsPanel alerts={alerts} isLoading={isLoading} />
          </div>
        </div>
      </div>
    </div>
  );
}
