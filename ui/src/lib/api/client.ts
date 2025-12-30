import type {
  CronJobListResponse,
  CronJobDetail,
  ExecutionHistoryResponse,
  LogsResponse,
  AlertsResponse,
  AlertHistoryResponse,
  MonitorsResponse,
  MonitorDetail,
  ChannelsResponse,
  ChannelDetail,
  Config,
  HealthResponse,
  StatsResponse,
  ActionResponse,
  DeleteHistoryResponse,
  StorageStatsResponse,
  PruneRequest,
  PruneResponse,
  ExecutionDetail,
} from "./types";

// Use relative URLs - the API is served on the same host/port as the UI
const API_BASE = "/api/v1";

class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string
  ) {
    super(message);
    this.name = "APIError";
  }
}

async function fetchAPI<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE}${endpoint}`;
  const response = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    let errorData: { error?: { code?: string; message?: string } } = {};
    try {
      errorData = await response.json();
    } catch {
      // Ignore JSON parse errors
    }
    throw new APIError(
      response.status,
      errorData.error?.code || "UNKNOWN",
      errorData.error?.message || `HTTP ${response.status}`
    );
  }

  return response.json();
}

// Health & Stats
export async function getHealth(): Promise<HealthResponse> {
  return fetchAPI<HealthResponse>("/health");
}

export async function getStats(): Promise<StatsResponse> {
  return fetchAPI<StatsResponse>("/stats");
}

// CronJobs
export async function listCronJobs(params?: {
  namespace?: string;
  status?: string;
  search?: string;
}): Promise<CronJobListResponse> {
  const searchParams = new URLSearchParams();
  if (params?.namespace) searchParams.set("namespace", params.namespace);
  if (params?.status) searchParams.set("status", params.status);
  if (params?.search) searchParams.set("search", params.search);

  const query = searchParams.toString();
  return fetchAPI<CronJobListResponse>(`/cronjobs${query ? `?${query}` : ""}`);
}

export async function getCronJob(
  namespace: string,
  name: string
): Promise<CronJobDetail> {
  return fetchAPI<CronJobDetail>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`
  );
}

export async function getExecutions(
  namespace: string,
  name: string,
  params?: {
    limit?: number;
    offset?: number;
    status?: string;
    since?: string;
  }
): Promise<ExecutionHistoryResponse> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set("limit", String(params.limit));
  if (params?.offset) searchParams.set("offset", String(params.offset));
  if (params?.status) searchParams.set("status", params.status);
  if (params?.since) searchParams.set("since", params.since);

  const query = searchParams.toString();
  return fetchAPI<ExecutionHistoryResponse>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/executions${query ? `?${query}` : ""}`
  );
}

export async function getLogs(
  namespace: string,
  cronjobName: string,
  jobName: string,
  params?: {
    container?: string;
    tailLines?: number;
  }
): Promise<LogsResponse> {
  const searchParams = new URLSearchParams();
  if (params?.container) searchParams.set("container", params.container);
  if (params?.tailLines) searchParams.set("tailLines", String(params.tailLines));

  const query = searchParams.toString();
  return fetchAPI<LogsResponse>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(cronjobName)}/executions/${encodeURIComponent(jobName)}/logs${query ? `?${query}` : ""}`
  );
}

export async function triggerCronJob(
  namespace: string,
  name: string
): Promise<ActionResponse> {
  return fetchAPI<ActionResponse>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/trigger`,
    { method: "POST" }
  );
}

export async function suspendCronJob(
  namespace: string,
  name: string
): Promise<ActionResponse> {
  return fetchAPI<ActionResponse>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/suspend`,
    { method: "POST" }
  );
}

export async function resumeCronJob(
  namespace: string,
  name: string
): Promise<ActionResponse> {
  return fetchAPI<ActionResponse>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/resume`,
    { method: "POST" }
  );
}

// Alerts
export async function listAlerts(params?: {
  severity?: string;
  type?: string;
  namespace?: string;
  cronjob?: string;
}): Promise<AlertsResponse> {
  const searchParams = new URLSearchParams();
  if (params?.severity) searchParams.set("severity", params.severity);
  if (params?.type) searchParams.set("type", params.type);
  if (params?.namespace) searchParams.set("namespace", params.namespace);
  if (params?.cronjob) searchParams.set("cronjob", params.cronjob);

  const query = searchParams.toString();
  return fetchAPI<AlertsResponse>(`/alerts${query ? `?${query}` : ""}`);
}

export async function getAlertHistory(params?: {
  limit?: number;
  offset?: number;
  since?: string;
  severity?: string;
}): Promise<AlertHistoryResponse> {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set("limit", String(params.limit));
  if (params?.offset) searchParams.set("offset", String(params.offset));
  if (params?.since) searchParams.set("since", params.since);
  if (params?.severity) searchParams.set("severity", params.severity);

  const query = searchParams.toString();
  return fetchAPI<AlertHistoryResponse>(
    `/alerts/history${query ? `?${query}` : ""}`
  );
}

// Monitors
export async function listMonitors(params?: {
  namespace?: string;
}): Promise<MonitorsResponse> {
  const searchParams = new URLSearchParams();
  if (params?.namespace) searchParams.set("namespace", params.namespace);

  const query = searchParams.toString();
  return fetchAPI<MonitorsResponse>(`/monitors${query ? `?${query}` : ""}`);
}

export async function getMonitor(
  namespace: string,
  name: string
): Promise<MonitorDetail> {
  return fetchAPI<MonitorDetail>(
    `/monitors/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`
  );
}

// Channels
export async function listChannels(): Promise<ChannelsResponse> {
  return fetchAPI<ChannelsResponse>("/channels");
}

export async function getChannel(name: string): Promise<ChannelDetail> {
  return fetchAPI<ChannelDetail>(`/channels/${encodeURIComponent(name)}`);
}

export async function testChannel(name: string): Promise<ActionResponse> {
  return fetchAPI<ActionResponse>(
    `/channels/${encodeURIComponent(name)}/test`,
    { method: "POST" }
  );
}

// Config
export async function getConfig(): Promise<Config> {
  return fetchAPI<Config>("/config");
}

// Data Management
export async function deleteHistory(
  namespace: string,
  name: string
): Promise<DeleteHistoryResponse> {
  return fetchAPI<DeleteHistoryResponse>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/history`,
    { method: "DELETE" }
  );
}

export async function getStorageStats(): Promise<StorageStatsResponse> {
  return fetchAPI<StorageStatsResponse>("/admin/storage-stats");
}

export async function triggerPrune(
  request?: PruneRequest
): Promise<PruneResponse> {
  return fetchAPI<PruneResponse>("/admin/prune", {
    method: "POST",
    body: JSON.stringify(request || {}),
  });
}

export async function getExecutionDetail(
  namespace: string,
  cronJobName: string,
  jobName: string
): Promise<ExecutionDetail> {
  return fetchAPI<ExecutionDetail>(
    `/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(cronJobName)}/executions/${encodeURIComponent(jobName)}`
  );
}

export { APIError };
