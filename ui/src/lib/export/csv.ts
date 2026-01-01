import type { CronJobExecution } from "@/lib/api";
import { formatDateTime } from "@/lib/utils";

interface ExportColumn {
  key: string;
  label: string;
  format?: (value: unknown) => string;
}

function formatDateTimeOrEmpty(value: unknown): string {
  if (!value) return "-";
  return formatDateTime(value as string);
}

const DEFAULT_COLUMNS: ExportColumn[] = [
  { key: "jobName", label: "Job Name" },
  { key: "status", label: "Status" },
  { key: "startTime", label: "Start Time", format: formatDateTimeOrEmpty },
  { key: "completionTime", label: "Completion Time", format: formatDateTimeOrEmpty },
  { key: "duration", label: "Duration" },
  { key: "exitCode", label: "Exit Code" },
  { key: "reason", label: "Reason" },
];

function escapeCSV(value: string): string {
  // Escape quotes and wrap in quotes if contains comma, quote, or newline
  if (value.includes('"') || value.includes(",") || value.includes("\n")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

export function exportExecutionsToCSV(
  executions: CronJobExecution[],
  cronJobName: string,
  options?: {
    columns?: ExportColumn[];
    filename?: string;
  }
): void {
  const columns = options?.columns || DEFAULT_COLUMNS;
  const filename = options?.filename || `${cronJobName}-executions-${new Date().toISOString().split("T")[0]}.csv`;

  // Build CSV content
  const headers = columns.map((col) => escapeCSV(col.label)).join(",");

  const rows = executions.map((exec) => {
    return columns
      .map((col) => {
        const value = exec[col.key as keyof CronJobExecution];
        const formatted = col.format ? col.format(value) : String(value ?? "");
        return escapeCSV(formatted);
      })
      .join(",");
  });

  const csvContent = [headers, ...rows].join("\n");

  // Download file
  downloadFile(csvContent, filename, "text/csv");
}

export interface SLAReportData {
  cronJobName: string;
  namespace: string;
  targetSLA: number;
  currentSuccessRate: number;
  status: string;
  monitorName: string;
}

export function exportSLAReportToCSV(data: SLAReportData[], filename?: string): void {
  const finalFilename = filename || `sla-report-${new Date().toISOString().split("T")[0]}.csv`;

  const headers = [
    "CronJob Name",
    "Namespace",
    "Monitor",
    "Target SLA (%)",
    "Current Rate (%)",
    "Status",
  ].join(",");

  const rows = data.map((item) =>
    [
      escapeCSV(item.cronJobName),
      escapeCSV(item.namespace),
      escapeCSV(item.monitorName),
      item.targetSLA.toFixed(1),
      item.currentSuccessRate.toFixed(1),
      escapeCSV(item.status),
    ].join(",")
  );

  const csvContent = [headers, ...rows].join("\n");
  downloadFile(csvContent, finalFilename, "text/csv");
}

function downloadFile(content: string, filename: string, mimeType: string): void {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}
