"use client";

import { useState } from "react";
import {
  MessageSquare,
  Bell,
  Webhook,
  Mail,
  CheckCircle2,
  XCircle,
  Send,
  Loader2,
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

export default function ChannelsPage() {
  const { data: channels, isLoading, isRefreshing, refetch } = useFetchData(listChannels);
  const [testingChannel, setTestingChannel] = useState<string | null>(null);

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
            label="Alerts (24h)"
            value={channels?.items.reduce((sum, c) => sum + c.stats.alertsSent24h, 0) ?? 0}
          />
        </div>

        {/* Channel Cards */}
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
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {channels?.items.map((channel) => (
              <ChannelCard
                key={channel.name}
                channel={channel}
                onTest={handleTest}
                isTesting={testingChannel === channel.name}
              />
            ))}
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
  const Icon = channelIcons[channel.type] || Webhook;

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="rounded bg-primary/10 p-2">
              <Icon className="h-4 w-4 text-primary" />
            </div>
            <div>
              <CardTitle className="text-base font-medium">{channel.name}</CardTitle>
              <p className="text-xs text-muted-foreground capitalize">{channel.type}</p>
            </div>
          </div>
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
      <CardContent className="space-y-4">
        {/* Config summary */}
        {channel.config && Object.keys(channel.config).length > 0 && (
          <div className="rounded bg-muted/50 px-3 py-2 text-sm">
            {Object.entries(channel.config).map(([key, value]) => (
              <p key={key} className="truncate">
                <span className="text-muted-foreground">{key}:</span> {value}
              </p>
            ))}
          </div>
        )}

        {/* Stats */}
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <p className="text-muted-foreground">Alerts (24h)</p>
            <p className="font-medium">{channel.stats.alertsSent24h}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Total Alerts</p>
            <p className="font-medium">{channel.stats.alertsSentTotal}</p>
          </div>
        </div>

        {/* Last test */}
        {channel.lastTest && (
          <div className="text-sm">
            <p className="text-muted-foreground">Last Test</p>
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

        {/* Actions */}
        <div className="flex gap-2 pt-2">
          <Button
            variant="outline"
            size="sm"
            className="flex-1"
            onClick={() => onTest(channel.name)}
            disabled={isTesting || !channel.ready}
          >
            {isTesting ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Send className="mr-1.5 h-3.5 w-3.5" />
            )}
            Test
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
