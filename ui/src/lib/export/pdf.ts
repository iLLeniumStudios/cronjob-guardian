import type { CronJobMetrics, CronJobExecution } from "@/lib/api";

export interface PDFReportData {
  title: string;
  cronJobName: string;
  namespace: string;
  generatedAt: Date;
  dateRange?: {
    from: Date;
    to: Date;
  };
  metrics: CronJobMetrics;
  recentExecutions: CronJobExecution[];
  alerts?: Array<{
    severity: string;
    title: string;
    message: string;
    since: string;
  }>;
}

export interface SLAPDFReportData {
  title: string;
  generatedAt: Date;
  summary: {
    total: number;
    meeting: number;
    atRisk: number;
    breaching: number;
  };
  items: Array<{
    cronJobName: string;
    namespace: string;
    monitorName: string;
    targetSLA: number;
    currentSuccessRate: number;
    status: string;
  }>;
}

function formatDateTime(date: Date | string): string {
  const d = typeof date === "string" ? new Date(date) : date;
  return d.toLocaleString();
}

function formatDate(date: Date | string): string {
  const d = typeof date === "string" ? new Date(date) : date;
  return d.toLocaleDateString();
}

export function generateCronJobPDFReport(data: PDFReportData): void {
  const html = `
    <!DOCTYPE html>
    <html>
    <head>
      <title>${data.title}</title>
      <style>
        body {
          font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
          padding: 40px;
          max-width: 800px;
          margin: 0 auto;
          color: #1f2937;
        }
        h1 {
          color: #111827;
          border-bottom: 2px solid #e5e7eb;
          padding-bottom: 12px;
        }
        h2 {
          color: #374151;
          margin-top: 32px;
          font-size: 1.25rem;
        }
        .meta {
          color: #6b7280;
          font-size: 0.875rem;
          margin-bottom: 24px;
        }
        .metrics-grid {
          display: grid;
          grid-template-columns: repeat(3, 1fr);
          gap: 16px;
          margin: 24px 0;
        }
        .metric-card {
          background: #f9fafb;
          border: 1px solid #e5e7eb;
          border-radius: 8px;
          padding: 16px;
          text-align: center;
        }
        .metric-label {
          font-size: 0.75rem;
          color: #6b7280;
          text-transform: uppercase;
          letter-spacing: 0.05em;
        }
        .metric-value {
          font-size: 1.5rem;
          font-weight: 700;
          color: #111827;
          margin-top: 4px;
        }
        .metric-value.success { color: #059669; }
        .metric-value.warning { color: #d97706; }
        .metric-value.error { color: #dc2626; }
        table {
          width: 100%;
          border-collapse: collapse;
          margin: 16px 0;
          font-size: 0.875rem;
        }
        th, td {
          padding: 12px;
          text-align: left;
          border-bottom: 1px solid #e5e7eb;
        }
        th {
          background: #f9fafb;
          font-weight: 600;
          color: #374151;
        }
        .status-success { color: #059669; }
        .status-failed { color: #dc2626; }
        .alert-card {
          background: #fef2f2;
          border: 1px solid #fecaca;
          border-radius: 8px;
          padding: 12px;
          margin-bottom: 8px;
        }
        .alert-title {
          font-weight: 600;
          color: #991b1b;
        }
        .alert-message {
          font-size: 0.875rem;
          color: #7f1d1d;
          margin-top: 4px;
        }
        .footer {
          margin-top: 48px;
          padding-top: 16px;
          border-top: 1px solid #e5e7eb;
          color: #9ca3af;
          font-size: 0.75rem;
          text-align: center;
        }
        @media print {
          body { padding: 20px; }
          @page { margin: 1cm; }
        }
      </style>
    </head>
    <body>
      <h1>${data.title}</h1>
      <div class="meta">
        <strong>CronJob:</strong> ${data.namespace}/${data.cronJobName}<br>
        <strong>Generated:</strong> ${formatDateTime(data.generatedAt)}<br>
        ${data.dateRange ? `<strong>Period:</strong> ${formatDate(data.dateRange.from)} - ${formatDate(data.dateRange.to)}` : ""}
      </div>

      <h2>Performance Metrics</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <div class="metric-label">Success Rate (7d)</div>
          <div class="metric-value ${data.metrics.successRate7d >= 95 ? "success" : data.metrics.successRate7d >= 80 ? "warning" : "error"}">
            ${data.metrics.successRate7d.toFixed(1)}%
          </div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Success Rate (30d)</div>
          <div class="metric-value ${data.metrics.successRate30d >= 95 ? "success" : data.metrics.successRate30d >= 80 ? "warning" : "error"}">
            ${data.metrics.successRate30d.toFixed(1)}%
          </div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Total Runs (7d)</div>
          <div class="metric-value">${data.metrics.totalRuns7d}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Successful (7d)</div>
          <div class="metric-value success">${data.metrics.successfulRuns7d}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Failed (7d)</div>
          <div class="metric-value ${data.metrics.failedRuns7d > 0 ? "error" : ""}">
            ${data.metrics.failedRuns7d}
          </div>
        </div>
        <div class="metric-card">
          <div class="metric-label">Avg Duration</div>
          <div class="metric-value">${formatDuration(data.metrics.avgDurationSeconds)}</div>
        </div>
      </div>

      <h2>Duration Percentiles</h2>
      <div class="metrics-grid">
        <div class="metric-card">
          <div class="metric-label">P50</div>
          <div class="metric-value">${formatDuration(data.metrics.p50DurationSeconds)}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">P95</div>
          <div class="metric-value">${formatDuration(data.metrics.p95DurationSeconds)}</div>
        </div>
        <div class="metric-card">
          <div class="metric-label">P99</div>
          <div class="metric-value">${formatDuration(data.metrics.p99DurationSeconds)}</div>
        </div>
      </div>

      ${
        data.alerts && data.alerts.length > 0
          ? `
        <h2>Active Alerts</h2>
        ${data.alerts
          .map(
            (alert) => `
          <div class="alert-card">
            <div class="alert-title">[${alert.severity.toUpperCase()}] ${alert.title}</div>
            <div class="alert-message">${alert.message}</div>
          </div>
        `
          )
          .join("")}
      `
          : ""
      }

      <h2>Recent Executions</h2>
      <table>
        <thead>
          <tr>
            <th>Job Name</th>
            <th>Status</th>
            <th>Start Time</th>
            <th>Duration</th>
            <th>Exit Code</th>
          </tr>
        </thead>
        <tbody>
          ${data.recentExecutions
            .slice(0, 10)
            .map(
              (exec) => `
            <tr>
              <td>${exec.jobName}</td>
              <td class="${exec.status === "success" ? "status-success" : "status-failed"}">${exec.status}</td>
              <td>${formatDateTime(exec.startTime)}</td>
              <td>${exec.duration}</td>
              <td>${exec.exitCode}</td>
            </tr>
          `
            )
            .join("")}
        </tbody>
      </table>

      <div class="footer">
        Generated by CronJob Guardian
      </div>
    </body>
    </html>
  `;

  openPrintWindow(html, `${data.cronJobName}-report`);
}

export function generateSLAPDFReport(data: SLAPDFReportData): void {
  const html = `
    <!DOCTYPE html>
    <html>
    <head>
      <title>${data.title}</title>
      <style>
        body {
          font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
          padding: 40px;
          max-width: 900px;
          margin: 0 auto;
          color: #1f2937;
        }
        h1 {
          color: #111827;
          border-bottom: 2px solid #e5e7eb;
          padding-bottom: 12px;
        }
        .meta {
          color: #6b7280;
          font-size: 0.875rem;
          margin-bottom: 24px;
        }
        .summary-grid {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: 16px;
          margin: 24px 0;
        }
        .summary-card {
          background: #f9fafb;
          border: 1px solid #e5e7eb;
          border-radius: 8px;
          padding: 20px;
          text-align: center;
        }
        .summary-card.meeting { border-color: #059669; background: #ecfdf5; }
        .summary-card.at-risk { border-color: #d97706; background: #fffbeb; }
        .summary-card.breaching { border-color: #dc2626; background: #fef2f2; }
        .summary-label {
          font-size: 0.75rem;
          color: #6b7280;
          text-transform: uppercase;
          letter-spacing: 0.05em;
        }
        .summary-value {
          font-size: 2rem;
          font-weight: 700;
          color: #111827;
          margin-top: 4px;
        }
        .summary-card.meeting .summary-value { color: #059669; }
        .summary-card.at-risk .summary-value { color: #d97706; }
        .summary-card.breaching .summary-value { color: #dc2626; }
        table {
          width: 100%;
          border-collapse: collapse;
          margin: 16px 0;
          font-size: 0.875rem;
        }
        th, td {
          padding: 12px;
          text-align: left;
          border-bottom: 1px solid #e5e7eb;
        }
        th {
          background: #f9fafb;
          font-weight: 600;
          color: #374151;
        }
        .status-meeting { color: #059669; font-weight: 500; }
        .status-at-risk { color: #d97706; font-weight: 500; }
        .status-breaching { color: #dc2626; font-weight: 500; }
        .footer {
          margin-top: 48px;
          padding-top: 16px;
          border-top: 1px solid #e5e7eb;
          color: #9ca3af;
          font-size: 0.75rem;
          text-align: center;
        }
        @media print {
          body { padding: 20px; }
          @page { margin: 1cm; }
        }
      </style>
    </head>
    <body>
      <h1>${data.title}</h1>
      <div class="meta">
        <strong>Generated:</strong> ${formatDateTime(data.generatedAt)}
      </div>

      <div class="summary-grid">
        <div class="summary-card">
          <div class="summary-label">Total Monitored</div>
          <div class="summary-value">${data.summary.total}</div>
        </div>
        <div class="summary-card meeting">
          <div class="summary-label">Meeting SLA</div>
          <div class="summary-value">${data.summary.meeting}</div>
        </div>
        <div class="summary-card at-risk">
          <div class="summary-label">At Risk</div>
          <div class="summary-value">${data.summary.atRisk}</div>
        </div>
        <div class="summary-card breaching">
          <div class="summary-label">Breaching SLA</div>
          <div class="summary-value">${data.summary.breaching}</div>
        </div>
      </div>

      <table>
        <thead>
          <tr>
            <th>CronJob</th>
            <th>Namespace</th>
            <th>Monitor</th>
            <th>Target SLA</th>
            <th>Current Rate</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          ${data.items
            .map(
              (item) => `
            <tr>
              <td>${item.cronJobName}</td>
              <td>${item.namespace}</td>
              <td>${item.monitorName}</td>
              <td>${item.targetSLA > 0 ? item.targetSLA.toFixed(0) + "%" : "-"}</td>
              <td>${item.currentSuccessRate.toFixed(1)}%</td>
              <td class="status-${item.status}">${formatStatus(item.status)}</td>
            </tr>
          `
            )
            .join("")}
        </tbody>
      </table>

      <div class="footer">
        Generated by CronJob Guardian
      </div>
    </body>
    </html>
  `;

  openPrintWindow(html, "sla-report");
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

function formatStatus(status: string): string {
  switch (status) {
    case "meeting":
      return "Meeting";
    case "at-risk":
      return "At Risk";
    case "breaching":
      return "Breaching";
    case "no-sla":
      return "No SLA";
    default:
      return status;
  }
}

function openPrintWindow(html: string, title: string): void {
  const printWindow = window.open("", "_blank");
  if (printWindow) {
    printWindow.document.write(html);
    printWindow.document.close();
    printWindow.document.title = title;

    // Wait for content to load, then trigger print
    printWindow.onload = () => {
      printWindow.print();
    };
  }
}
