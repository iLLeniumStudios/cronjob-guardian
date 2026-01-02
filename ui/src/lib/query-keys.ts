/**
 * Query key factory for React Query.
 * Provides consistent query keys for caching and invalidation.
 *
 * Usage with React Query:
 * @example
 * const { data } = useQuery({
 *   queryKey: queryKeys.cronJobs.detail('default', 'my-job'),
 *   queryFn: () => getCronJob('default', 'my-job'),
 * });
 */
export const queryKeys = {
  // Dashboard stats
  stats: ["stats"] as const,

  // CronJobs
  cronJobs: {
    all: ["cronJobs"] as const,
    detail: (namespace: string, name: string) =>
      ["cronJobs", namespace, name] as const,
    executions: (namespace: string, name: string) =>
      ["cronJobs", namespace, name, "executions"] as const,
  },

  // Monitors
  monitors: {
    all: ["monitors"] as const,
    detail: (namespace: string, name: string) =>
      ["monitors", namespace, name] as const,
  },

  // Alert Channels
  channels: ["channels"] as const,

  // Alerts
  alerts: {
    active: ["alerts", "active"] as const,
    history: ["alerts", "history"] as const,
  },

  // SLA
  sla: ["sla"] as const,

  // Settings
  settings: ["settings"] as const,
};

/**
 * Helper type to extract query key type from factory functions
 */
export type QueryKey = ReturnType<typeof queryKeys.cronJobs.detail>;
