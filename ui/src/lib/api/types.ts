// API Response Types

export interface CronJobSummary {
  total: number;
  healthy: number;
  warning: number;
  critical: number;
  running: number;
}

export interface ActiveJob {
  name: string;
  startTime: string;
  runningDuration?: string;
  podPhase?: string;
  podName?: string;
  ready?: string;
}

export interface CronJob {
  name: string;
  namespace: string;
  status: "healthy" | "warning" | "critical" | "suspended" | "running";
  schedule: string;
  timezone?: string;
  suspended: boolean;
  successRate: number;
  lastSuccess: string | null;
  lastRunDuration: string | null;
  nextRun: string | null;
  activeJobs?: ActiveJob[];
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
  activeJobs?: ActiveJob[];
  activeAlerts: Alert[];
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

// AlertContext contains context data for an alert (suggested fixes, exit codes, etc.)
export interface AlertContext {
  exitCode?: number;
  reason?: string;
  suggestedFix?: string;
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
  context?: AlertContext;
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
  // Context fields for failure alerts (stored at alert time)
  exitCode?: number;
  reason?: string;
  suggestedFix?: string;
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
    running: number;
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
      // metrics is optional - will be null/undefined if no executions recorded yet
      metrics?: {
        successRate: number;
        totalRuns: number;
        successfulRuns: number;
        failedRuns: number;
        avgDurationSeconds: number;
        p50DurationSeconds?: number;
        p95DurationSeconds?: number;
        p99DurationSeconds?: number;
      } | null;
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
    alertsSentTotal: number;
    alertsFailedTotal: number;
    lastAlertTime: string | null;
    lastFailedTime: string | null;
    lastFailedError: string | null;
    consecutiveFailures: number;
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
    lastAlertTime: string | null;
  };
}

// Config matches the actual API response from /api/v1/config
export interface Config {
  logLevel: string;
  storage: {
    type: string;
    sqlite?: {
      path: string;
    };
    postgres?: {
      host: string;
      port: number;
      database: string;
      username?: string;
      sslMode?: string;
    };
    mysql?: {
      host: string;
      port: number;
      database: string;
      username?: string;
    };
    logStorageEnabled: boolean;
    eventStorageEnabled: boolean;
    maxLogSizeKB: number;
    logRetentionDays: number;
  };
  historyRetention: {
    defaultDays: number;
    maxDays: number;
  };
  rateLimits: {
    maxAlertsPerMinute: number;
  };
  ui: {
    enabled: boolean;
    port: number;
  };
  scheduler: {
    deadManSwitchInterval: number; // Duration in nanoseconds
    slaRecalculationInterval: number; // Duration in nanoseconds
    pruneInterval: number; // Duration in nanoseconds
  };
}

export interface HealthResponse {
  status: string;
  storage: string;
  leader: boolean;
  version: string;
  uptime: string;
  analyzerEnabled: boolean;
  schedulersRunning: string[];
}

export interface StatsResponse {
  totalMonitors: number;
  totalCronJobs: number;
  summary: CronJobSummary;
  activeAlerts: number;
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

// Pattern Testing Types (for pattern tester component)
export interface PatternMatch {
  exitCode?: number;
  exitCodeRange?: { min: number; max: number };
  reason?: string;
  reasonPattern?: string;
  logPattern?: string;
  eventPattern?: string;
}

export interface SuggestedFixPattern {
  name: string;
  match: PatternMatch;
  suggestion: string;
  priority?: number;
}

export interface PatternTestRequest {
  pattern: SuggestedFixPattern;
  testData: {
    exitCode: number;
    reason: string;
    logs: string;
    events: string[];
    namespace: string;
    name: string;
    jobName: string;
  };
}

export interface PatternTestResponse {
  matched: boolean;
  renderedSuggestion?: string;
  error?: string;
}
