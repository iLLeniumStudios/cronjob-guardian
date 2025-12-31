"use client";

import { useState } from "react";
import { Play, Check, AlertCircle, HelpCircle } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { SuggestedFix } from "@/components/suggested-fix";
import type { SuggestedFixPattern, PatternTestResponse } from "@/lib/api/types";

interface PatternTesterProps {
  onPatternValid?: (pattern: SuggestedFixPattern) => void;
}

export function PatternTester({ onPatternValid }: PatternTesterProps) {
  // Pattern configuration
  const [patternName, setPatternName] = useState("");
  const [exitCode, setExitCode] = useState<string>("");
  const [exitCodeMin, setExitCodeMin] = useState<string>("");
  const [exitCodeMax, setExitCodeMax] = useState<string>("");
  const [reason, setReason] = useState("");
  const [reasonPattern, setReasonPattern] = useState("");
  const [logPattern, setLogPattern] = useState("");
  const [eventPattern, setEventPattern] = useState("");
  const [suggestion, setSuggestion] = useState("");
  const [priority, setPriority] = useState<string>("100");

  // Test data
  const [testExitCode, setTestExitCode] = useState<string>("1");
  const [testReason, setTestReason] = useState("");
  const [testLogs, setTestLogs] = useState("");
  const [testEvents, setTestEvents] = useState("");

  // Results
  const [testResult, setTestResult] = useState<PatternTestResponse | null>(null);
  const [isTesting, setIsTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const buildPattern = (): SuggestedFixPattern | null => {
    if (!patternName || !suggestion) {
      setError("Pattern name and suggestion are required");
      return null;
    }

    const match: SuggestedFixPattern["match"] = {};

    // Exact exit code
    if (exitCode) {
      match.exitCode = parseInt(exitCode, 10);
      if (isNaN(match.exitCode)) {
        setError("Exit code must be a number");
        return null;
      }
    }

    // Exit code range
    if (exitCodeMin || exitCodeMax) {
      const min = exitCodeMin ? parseInt(exitCodeMin, 10) : 0;
      const max = exitCodeMax ? parseInt(exitCodeMax, 10) : 255;
      if (isNaN(min) || isNaN(max)) {
        setError("Exit code range must be numbers");
        return null;
      }
      match.exitCodeRange = { min, max };
    }

    // Reason (exact match)
    if (reason) {
      match.reason = reason;
    }

    // Reason pattern (regex)
    if (reasonPattern) {
      try {
        new RegExp(reasonPattern);
        match.reasonPattern = reasonPattern;
      } catch {
        setError("Invalid reason pattern regex");
        return null;
      }
    }

    // Log pattern (regex)
    if (logPattern) {
      try {
        new RegExp(logPattern);
        match.logPattern = logPattern;
      } catch {
        setError("Invalid log pattern regex");
        return null;
      }
    }

    // Event pattern (regex)
    if (eventPattern) {
      try {
        new RegExp(eventPattern);
        match.eventPattern = eventPattern;
      } catch {
        setError("Invalid event pattern regex");
        return null;
      }
    }

    // Must have at least one match criterion
    if (Object.keys(match).length === 0) {
      setError("At least one match criterion is required");
      return null;
    }

    return {
      name: patternName,
      match,
      suggestion,
      priority: priority ? parseInt(priority, 10) : undefined,
    };
  };

  const handleTest = async () => {
    setError(null);
    setTestResult(null);

    const pattern = buildPattern();
    if (!pattern) return;

    setIsTesting(true);
    try {
      const response = await fetch("/api/v1/patterns/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          pattern,
          testData: {
            exitCode: parseInt(testExitCode, 10) || 0,
            reason: testReason,
            logs: testLogs,
            events: testEvents.split("\n").filter(Boolean),
            namespace: "test-namespace",
            name: "test-cronjob",
            jobName: "test-cronjob-12345",
          },
        }),
      });

      if (!response.ok) {
        throw new Error("Failed to test pattern");
      }

      const result: PatternTestResponse = await response.json();
      setTestResult(result);

      if (result.error) {
        setError(result.error);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Test failed");
      toast.error("Failed to test pattern");
    } finally {
      setIsTesting(false);
    }
  };

  const handleAddPattern = () => {
    const pattern = buildPattern();
    if (pattern && testResult?.matched && onPatternValid) {
      onPatternValid(pattern);
      toast.success("Pattern added");
    }
  };

  return (
    <div className="space-y-6">
      {/* Pattern Configuration */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Pattern Configuration</CardTitle>
          <CardDescription>
            Define what conditions this pattern should match
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="patternName">Pattern Name *</Label>
              <Input
                id="patternName"
                placeholder="e.g., db-connection-error"
                value={patternName}
                onChange={(e) => setPatternName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="priority">Priority (higher = checked first)</Label>
              <Input
                id="priority"
                type="number"
                placeholder="100"
                value={priority}
                onChange={(e) => setPriority(e.target.value)}
              />
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="exitCode">Exit Code (exact)</Label>
              <Input
                id="exitCode"
                type="number"
                placeholder="e.g., 137"
                value={exitCode}
                onChange={(e) => setExitCode(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="exitCodeMin">Exit Code Range Min</Label>
              <Input
                id="exitCodeMin"
                type="number"
                placeholder="e.g., 1"
                value={exitCodeMin}
                onChange={(e) => setExitCodeMin(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="exitCodeMax">Exit Code Range Max</Label>
              <Input
                id="exitCodeMax"
                type="number"
                placeholder="e.g., 125"
                value={exitCodeMax}
                onChange={(e) => setExitCodeMax(e.target.value)}
              />
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="reason">Reason (exact match)</Label>
              <Input
                id="reason"
                placeholder="e.g., OOMKilled"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <div className="flex items-center gap-1">
                <Label htmlFor="reasonPattern">Reason Pattern (regex)</Label>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger>
                      <HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>Regular expression to match against the reason</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <Input
                id="reasonPattern"
                placeholder="e.g., Error.*timeout"
                value={reasonPattern}
                onChange={(e) => setReasonPattern(e.target.value)}
              />
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="logPattern">Log Pattern (regex)</Label>
              <Input
                id="logPattern"
                placeholder="e.g., connection refused.*:5432"
                value={logPattern}
                onChange={(e) => setLogPattern(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="eventPattern">Event Pattern (regex)</Label>
              <Input
                id="eventPattern"
                placeholder="e.g., FailedScheduling"
                value={eventPattern}
                onChange={(e) => setEventPattern(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center gap-1">
              <Label htmlFor="suggestion">Suggestion Text *</Label>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
                  </TooltipTrigger>
                  <TooltipContent className="max-w-xs">
                    <p>Available variables:</p>
                    <code className="text-xs">
                      {"{{.Namespace}}, {{.Name}}, {{.JobName}}, {{.ExitCode}}, {{.Reason}}"}
                    </code>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            <Textarea
              id="suggestion"
              placeholder="e.g., PostgreSQL connection failed. Check: kubectl get pods -n {{.Namespace}} -l app=postgres"
              value={suggestion}
              onChange={(e) => setSuggestion(e.target.value)}
              rows={3}
            />
          </div>
        </CardContent>
      </Card>

      {/* Test Data */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Test Data</CardTitle>
          <CardDescription>
            Simulate a failure scenario to test your pattern
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="testExitCode">Exit Code</Label>
              <Input
                id="testExitCode"
                type="number"
                value={testExitCode}
                onChange={(e) => setTestExitCode(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="testReason">Reason</Label>
              <Input
                id="testReason"
                placeholder="e.g., OOMKilled"
                value={testReason}
                onChange={(e) => setTestReason(e.target.value)}
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="testLogs">Log Content</Label>
            <Textarea
              id="testLogs"
              placeholder="Paste sample log output here..."
              value={testLogs}
              onChange={(e) => setTestLogs(e.target.value)}
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="testEvents">Events (one per line)</Label>
            <Textarea
              id="testEvents"
              placeholder="e.g., FailedScheduling: 0/3 nodes available"
              value={testEvents}
              onChange={(e) => setTestEvents(e.target.value)}
              rows={2}
            />
          </div>

          <div className="flex gap-2">
            <Button onClick={handleTest} disabled={isTesting}>
              <Play className="h-4 w-4 mr-1" />
              {isTesting ? "Testing..." : "Test Pattern"}
            </Button>
            {testResult?.matched && onPatternValid && (
              <Button variant="outline" onClick={handleAddPattern}>
                <Check className="h-4 w-4 mr-1" />
                Add Pattern
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Results */}
      {(error || testResult) && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              Test Result
              {testResult?.matched ? (
                <Badge className="bg-emerald-500">Matched</Badge>
              ) : testResult ? (
                <Badge variant="secondary">No Match</Badge>
              ) : null}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {error && (
              <div className="flex items-start gap-2 text-red-600 dark:text-red-400">
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                <p className="text-sm">{error}</p>
              </div>
            )}
            {testResult?.matched && testResult.renderedSuggestion && (
              <SuggestedFix
                fix={testResult.renderedSuggestion}
                exitCode={parseInt(testExitCode, 10)}
                reason={testReason}
                namespace="test-namespace"
                name="test-cronjob"
              />
            )}
            {testResult && !testResult.matched && !error && (
              <p className="text-sm text-muted-foreground">
                The pattern did not match the test data. Adjust your match criteria.
              </p>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
