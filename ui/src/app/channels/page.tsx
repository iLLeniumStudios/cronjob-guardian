"use client";

import { useState, useMemo } from "react";
import {
  MessageSquare,
  Bell,
  Webhook,
  Mail,
  CheckCircle2,
  XCircle,
  Send,
  Loader2,
  AlertTriangle,
} from "lucide-react";
import { toast } from "sonner";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { RelativeTime } from "@/components/relative-time";
import { EmptyState } from "@/components/empty-state";
import { SimpleStatCard } from "@/components/stat-card";
import { PageSkeleton } from "@/components/page-skeleton";
import { useFetchData } from "@/hooks/use-fetch-data";
import { listChannels, testChannel, type Channel } from "@/lib/api";

const channelIcons: Record<string, typeof MessageSquare> = {
  slack: MessageSquare,
  pagerduty: Bell,
  webhook: Webhook,
  email: Mail,
};

const channelTypeLabels: Record<string, string> = {
  slack: "Slack",
  pagerduty: "PagerDuty",
  webhook: "Webhook",
  email: "Email",
};

const channelTypeOrder = ["slack", "pagerduty", "webhook", "email"];

export default function ChannelsPage() {
  const { data: channels, isLoading, isRefreshing, refetch } = useFetchData(listChannels);
  const [testingChannel, setTestingChannel] = useState<string | null>(null);

  // Group and sort channels by type
  const groupedChannels = useMemo(() => {
    if (!channels?.items) return {};

    const groups: Record<string, Channel[]> = {};

    // Sort channels by name within each type
    const sortedItems = [...channels.items].sort((a, b) => a.name.localeCompare(b.name));

    for (const channel of sortedItems) {
      if (!groups[channel.type]) {
        groups[channel.type] = [];
      }
      groups[channel.type].push(channel);
    }

    return groups;
  }, [channels?.items]);

  // Get ordered types that have channels
  const orderedTypes = useMemo(() => {
    return channelTypeOrder.filter((type) => groupedChannels[type]?.length > 0);
  }, [groupedChannels]);

  const handleTest = async (name: string) => {
    setTestingChannel(name);
    try {
      const result = await testChannel(name);
      if (result.success) {
        toast.success(`Test alert sent to ${name}`);
      } else {
        toast.error(result.error || "Failed to send test alert");
      }
      refetch();
    } catch {
      toast.error("Failed to send test alert");
    } finally {
      setTestingChannel(null);
    }
  };

  if (isLoading) {
    return <PageSkeleton title="Alert Channels" variant="grid" />;
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Alert Channels"
        description="Manage notification destinations"
        onRefresh={refetch}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-4 md:p-6">
        {/* Stats */}
        <div className="grid gap-4 md:grid-cols-4">
          <SimpleStatCard
            label="Total Channels"
            value={channels?.summary.total ?? 0}
          />
          <SimpleStatCard
            label="Ready"
            value={channels?.summary.ready ?? 0}
            valueClassName="text-emerald-600 dark:text-emerald-400"
          />
          <SimpleStatCard
            label="Not Ready"
            value={channels?.summary.notReady ?? 0}
            valueClassName="text-red-600 dark:text-red-400"
          />
          <SimpleStatCard
            label="Total Failures"
            value={channels?.items.reduce((sum, c) => sum + c.stats.alertsFailedTotal, 0) ?? 0}
            valueClassName={
              (channels?.items.reduce((sum, c) => sum + c.stats.alertsFailedTotal, 0) ?? 0) > 0
                ? "text-red-600 dark:text-red-400"
                : undefined
            }
          />
        </div>

        {/* Channel Cards grouped by type */}
        {channels?.items.length === 0 ? (
          <Card>
            <CardContent className="p-0">
              <EmptyState
                icon={Bell}
                title="No alert channels configured"
                description="Create an AlertChannel resource to start receiving notifications"
                bordered={false}
              />
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-8">
            {orderedTypes.map((type) => {
              const Icon = channelIcons[type] || Webhook;
              const typeChannels = groupedChannels[type] || [];

              return (
                <div key={type}>
                  {/* Section header */}
                  <div className="flex items-center gap-2 mb-4">
                    <div className="rounded bg-primary/10 p-1.5">
                      <Icon className="h-4 w-4 text-primary" />
                    </div>
                    <h2 className="text-lg font-semibold">{channelTypeLabels[type]}</h2>
                    <Badge variant="secondary" className="ml-1">
                      {typeChannels.length}
                    </Badge>
                  </div>

                  {/* Cards grid */}
                  <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                    {typeChannels.map((channel) => (
                      <ChannelCard
                        key={channel.name}
                        channel={channel}
                        onTest={handleTest}
                        isTesting={testingChannel === channel.name}
                      />
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function ChannelCard({
  channel,
  onTest,
  isTesting,
}: {
  channel: Channel;
  onTest: (name: string) => void;
  isTesting: boolean;
}) {
  return (
    <Card className="flex flex-col">
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between">
          <CardTitle className="text-base font-medium">{channel.name}</CardTitle>
          {channel.ready ? (
            <Badge variant="outline" className="text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="mr-1 h-3 w-3" />
              Ready
            </Badge>
          ) : (
            <Badge variant="outline" className="text-red-600 dark:text-red-400">
              <XCircle className="mr-1 h-3 w-3" />
              Not Ready
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col space-y-4">
        {/* Stats - always show, consistent layout */}
        <div className="grid grid-cols-3 gap-2 text-sm">
          <div className="text-center">
            <p className="text-muted-foreground text-xs">Sent</p>
            <p className="font-medium">{channel.stats.alertsSentTotal}</p>
          </div>
          <div className="text-center">
            <p className="text-muted-foreground text-xs">Failed</p>
            <p className={`font-medium ${channel.stats.alertsFailedTotal > 0 ? "text-red-600 dark:text-red-400" : ""}`}>
              {channel.stats.alertsFailedTotal}
            </p>
          </div>
          <div className="text-center">
            <p className="text-muted-foreground text-xs">Consecutive Failures</p>
            <p className={`font-medium ${channel.stats.consecutiveFailures > 0 ? "text-red-600 dark:text-red-400" : ""}`}>
              {channel.stats.consecutiveFailures}
            </p>
          </div>
        </div>

        {/* Failure Warning */}
        {channel.stats.consecutiveFailures > 0 && (
          <div className="flex items-start gap-2 rounded-md bg-red-50 dark:bg-red-950/20 p-3 text-sm">
            <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
            <div>
              <p className="font-medium text-red-700 dark:text-red-400">
                {channel.stats.consecutiveFailures} consecutive failure{channel.stats.consecutiveFailures !== 1 ? "s" : ""}
              </p>
              {channel.stats.lastFailedError && (
                <p className="text-red-600 dark:text-red-400/80 text-xs mt-1 line-clamp-2">
                  {channel.stats.lastFailedError}
                </p>
              )}
              {channel.stats.lastFailedTime && (
                <p className="text-red-500 dark:text-red-400/70 text-xs mt-1">
                  Last failed: <RelativeTime date={channel.stats.lastFailedTime} showTooltip={false} />
                </p>
              )}
            </div>
          </div>
        )}

        {/* Last test */}
        {channel.lastTest && (
          <div className="text-sm">
            <p className="text-muted-foreground text-xs">Last Test</p>
            <p className="flex items-center gap-2">
              {channel.lastTest.result === "success" ? (
                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />
              ) : (
                <XCircle className="h-3.5 w-3.5 text-red-600" />
              )}
              <span className="capitalize">{channel.lastTest.result}</span>
              <span className="text-muted-foreground">
                <RelativeTime date={channel.lastTest.time} showTooltip={false} />
              </span>
            </p>
          </div>
        )}

        {/* Spacer to push button to bottom */}
        <div className="flex-1" />

        {/* Actions - always at bottom */}
        <Button
          variant="outline"
          size="sm"
          className="w-full"
          onClick={() => onTest(channel.name)}
          disabled={isTesting || !channel.ready}
        >
          {isTesting ? (
            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
          ) : (
            <Send className="mr-1.5 h-3.5 w-3.5" />
          )}
          Send Test Alert
        </Button>
      </CardContent>
    </Card>
  );
}
