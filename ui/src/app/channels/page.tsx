"use client";

import { useCallback, useEffect, useState } from "react";
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
import { Skeleton } from "@/components/ui/skeleton";
import { RelativeTime } from "@/components/relative-time";
import { listChannels, testChannel, type ChannelsResponse, type Channel } from "@/lib/api";

const channelIcons: Record<string, typeof MessageSquare> = {
  slack: MessageSquare,
  pagerduty: Bell,
  webhook: Webhook,
  email: Mail,
};

export default function ChannelsPage() {
  const [channels, setChannels] = useState<ChannelsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [testingChannel, setTestingChannel] = useState<string | null>(null);

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) setIsRefreshing(true);
    try {
      const data = await listChannels();
      setChannels(data);
    } catch (error) {
      console.error("Failed to fetch channels:", error);
      toast.error("Failed to load channels");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const interval = setInterval(() => fetchData(), 5000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const handleTest = async (name: string) => {
    setTestingChannel(name);
    try {
      const result = await testChannel(name);
      if (result.success) {
        toast.success(`Test alert sent to ${name}`);
      } else {
        toast.error(result.error || "Failed to send test alert");
      }
      fetchData();
    } catch {
      toast.error("Failed to send test alert");
    } finally {
      setTestingChannel(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <Header title="Alert Channels" />
        <div className="flex-1 space-y-6 overflow-auto p-6">
          <div className="grid gap-4 md:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-48" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <Header
        title="Alert Channels"
        description="Manage notification destinations"
        onRefresh={() => fetchData(true)}
        isRefreshing={isRefreshing}
      />
      <div className="flex-1 space-y-6 overflow-auto p-6">
        {/* Stats */}
        <div className="grid gap-4 md:grid-cols-4">
          <Card>
            <CardContent className="p-4">
              <p className="text-sm text-muted-foreground">Total Channels</p>
              <p className="mt-1 text-2xl font-semibold">
                {channels?.summary.total ?? 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <p className="text-sm text-muted-foreground">Ready</p>
              <p className="mt-1 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
                {channels?.summary.ready ?? 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <p className="text-sm text-muted-foreground">Not Ready</p>
              <p className="mt-1 text-2xl font-semibold text-red-600 dark:text-red-400">
                {channels?.summary.notReady ?? 0}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <p className="text-sm text-muted-foreground">Alerts (24h)</p>
              <p className="mt-1 text-2xl font-semibold">
                {channels?.items.reduce((sum, c) => sum + c.stats.alertsSent24h, 0) ?? 0}
              </p>
            </CardContent>
          </Card>
        </div>

        {/* Channel Cards */}
        {channels?.items.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Bell className="mb-4 h-12 w-12 text-muted-foreground/50" />
              <p className="text-lg font-medium">No alert channels configured</p>
              <p className="text-sm text-muted-foreground">
                Create an AlertChannel resource to start receiving notifications
              </p>
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
