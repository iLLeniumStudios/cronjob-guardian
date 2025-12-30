// API Response Types

export interface CronJobSummary {
  total: number;
  healthy: number;
  warning: number;
  critical: number;
}

export interface CronJob {
  name: string;
  namespace: string;
  status: "healthy" | "warning" | "critical" | "suspended";
  schedule: string;
  timezone?: string;
  suspended: boolean;
  successRate: number;
  lastSuccess: string | null;
  lastRunDuration: string | null;
  nextRun: string | null;
  activeAlerts: number;
  monitorRef?: {
    name: string;
    namespace: string;
  };
}

export interface CronJobListResponse {
  items: CronJob[];
  summary: CronJobSummary;
}

export interface CronJobMetrics {
  successRate7d: number;
  successRate30d: number;
  totalRuns7d: number;
  successfulRuns7d: number;
  failedRuns7d: number;
  avgDurationSeconds: number;
  p50DurationSeconds: number;
  p95DurationSeconds: number;
  p99DurationSeconds: number;
}

export interface CronJobExecution {
  jobName: string;
  status: "success" | "failed";
  startTime: string;
  completionTime: string | null;
  duration: string;
  exitCode: number;
  reason: string;
}

export interface CronJobDetail extends Omit<CronJob, 'activeAlerts'> {
  metrics: CronJobMetrics;
  lastExecution: CronJobExecution | null;
  activeAlerts: Alert[];
  lastRemediation: {
    action: string;
    time: string;
    result: string;
    message: string;
  } | null;
}

export interface ExecutionHistoryResponse {
  items: CronJobExecution[];
  pagination: {
    total: number;
    limit: number;
    offset: number;
    hasMore: boolean;
  };
}

export interface LogsResponse {
  jobName: string;
  container: string;
  logs: string;
  truncated: boolean;
}

export interface Alert {
  id: string;
  type: string;
  severity: "critical" | "warning" | "info";
  title: string;
  message: string;
  cronjob: {
    namespace: string;
    name: string;
  };
  monitor?: {
    namespace: string;
    name: string;
  };
  since: string;
  lastNotified: string;
}

export interface AlertsResponse {
  items: Alert[];
  total: number;
  bySeverity: {
    critical: number;
    warning: number;
    info: number;
  };
}

export interface AlertHistoryItem extends Alert {
  occurredAt: string;
  resolvedAt: string | null;
  channelsNotified: string[];
}

export interface AlertHistoryResponse {
  items: AlertHistoryItem[];
  pagination: {
    total: number;
    limit: number;
    offset: number;
  };
}

export interface Monitor {
  name: string;
  namespace: string;
  cronJobCount: number;
  summary: {
    healthy: number;
    warning: number;
    critical: number;
  };
  activeAlerts: number;
  lastReconcile: string;
  phase: string;
}

export interface MonitorsResponse {
  items: Monitor[];
}

export interface MonitorDetail {
  metadata: {
    name: string;
    namespace: string;
    creationTimestamp: string;
  };
  spec: {
    selector: {
      matchLabels?: Record<string, string>;
      matchExpressions?: Array<{
        key: string;
        operator: string;
        values: string[];
      }>;
    };
    deadManSwitch?: {
      enabled: boolean;
      maxTimeSinceLastSuccess: string;
    };
    sla?: {
      enabled: boolean;
      minSuccessRate: number;
      windowDays: number;
    };
  };
  status: {
    phase: string;
    summary: {
      totalCronJobs: number;
      healthy: number;
      warning: number;
      critical: number;
      activeAlerts: number;
    };
    cronJobs: Array<{
      name: string;
      namespace: string;
      status: string;
      lastSuccessfulTime: string | null;
      nextScheduledTime: string | null;
      metrics: {
        successRate: number;
        avgDurationSeconds: number;
      };
    }>;
    lastReconcileTime: string;
  };
}

export interface Channel {
  name: string;
  type: "slack" | "pagerduty" | "webhook" | "email";
  ready: boolean;
  config: Record<string, string>;
  stats: {
    alertsSent24h: number;
    alertsSentTotal: number;
  };
  lastTest: {
    time: string;
    result: "success" | "failed";
  } | null;
}

export interface ChannelsResponse {
  items: Channel[];
  summary: {
    total: number;
    ready: number;
    notReady: number;
  };
}

export interface ChannelDetail {
  metadata: {
    name: string;
    creationTimestamp: string;
  };
  spec: {
    type: string;
    slack?: {
      webhookSecretRef: {
        name: string;
        namespace: string;
        key: string;
      };
      defaultChannel: string;
    };
    pagerduty?: {
      routingKeySecretRef: {
        name: string;
        namespace: string;
        key: string;
      };
    };
    webhook?: {
      url?: string;
      urlSecretRef?: {
        name: string;
        namespace: string;
        key: string;
      };
    };
    email?: {
      smtpSecretRef: {
        name: string;
        namespace: string;
        key: string;
      };
      from: string;
      to: string[];
    };
    rateLimiting?: {
      maxAlertsPerHour: number;
      burstLimit: number;
    };
  };
  status: {
    ready: boolean;
    lastTestTime: string | null;
    lastTestResult: string | null;
    alertsSentTotal: number;
    alertsSentLast24h: number;
    lastAlertTime: string | null;
  };
}

export interface Config {
  metadata: {
    name: string;
  };
  spec: {
    deadManSwitchInterval: string;
    slaRecalculationInterval: string;
    historyRetention: {
      defaultDays: number;
      maxDays: number;
    };
    storage: {
      type: "sqlite" | "postgresql" | "mysql";
      sqlite?: {
        path: string;
      };
      postgresql?: {
        host: string;
        port: number;
        database: string;
      };
      mysql?: {
        host: string;
        port: number;
        database: string;
      };
    };
    ignoredNamespaces: string[];
  };
  status: {
    activeLeader: string;
    totalMonitors: number;
    totalCronJobsWatched: number;
    totalAlertsSent24h: number;
    totalRemediations24h: number;
    storageStatus: string;
  };
}

export interface HealthResponse {
  status: string;
  storage: string;
  leader: boolean;
  version: string;
  uptime: string;
}

export interface StatsResponse {
  totalMonitors: number;
  totalCronJobs: number;
  summary: CronJobSummary;
  activeAlerts: number;
  alertsSent24h: number;
  remediations24h: number;
  executionsRecorded24h: number;
}

export interface ActionResponse {
  success: boolean;
  message?: string;
  error?: string;
  jobName?: string;
}

// Data Management Types
export interface DeleteHistoryResponse {
  success: boolean;
  recordsDeleted: number;
  message: string;
}

export interface StorageStatsResponse {
  executionCount: number;
  storageType: string;
  healthy: boolean;
  retentionDays: number;
  logStorageEnabled: boolean;
}

export interface PruneRequest {
  olderThanDays?: number;
  dryRun?: boolean;
  pruneLogsOnly?: boolean;
}

export interface PruneResponse {
  success: boolean;
  recordsPruned: number;
  dryRun: boolean;
  cutoff: string;
  olderThanDays: number;
  message: string;
}

export interface ExecutionDetail extends CronJobExecution {
  id: number;
  cronJobNamespace: string;
  cronJobName: string;
  cronJobUID?: string;
  isRetry: boolean;
  retryOf?: string;
  storedLogs?: string;
  storedEvents?: string;
}
