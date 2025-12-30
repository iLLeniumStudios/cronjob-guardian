"use client";

import { CronJobDetailClient } from "./cronjob-detail";

// Simple page that handles URL parsing client-side
// The Go server will serve this page for all /cronjob/* paths
export default function CronJobPage() {
  return <CronJobDetailClient />;
}
