"use client";

import { useState } from "react";
import { Lightbulb, Check, Terminal } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface SuggestedFixProps {
  fix: string;
  exitCode?: number;
  reason?: string;
  namespace?: string;
  name?: string;
  className?: string;
  compact?: boolean; // For use in tables/lists
}

// Regex to detect kubectl commands in the suggestion
const KUBECTL_REGEX = /kubectl\s+[^\n]+/g;

export function SuggestedFix({
  fix,
  exitCode,
  reason,
  namespace,
  name,
  className,
  compact = false,
}: SuggestedFixProps) {
  const [copied, setCopied] = useState(false);

  // Extract kubectl commands from the suggestion
  const kubectlCommands = fix.match(KUBECTL_REGEX) || [];
  const hasKubectlCommand = kubectlCommands.length > 0;

  const handleCopyCommand = async () => {
    const firstCommand = kubectlCommands[0];
    if (firstCommand) {
      // Copy the first kubectl command found
      let command = firstCommand;
      // Replace template variables if namespace/name provided
      if (namespace) {
        command = command.replace(/\{\{\.Namespace\}\}/g, namespace);
      }
      if (name) {
        command = command.replace(/\{\{\.Name\}\}/g, name);
      }
      await navigator.clipboard.writeText(command);
      setCopied(true);
      toast.success("Command copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    }
  };

  // Replace template variables in display text
  const displayFix = fix
    .replace(/\{\{\.Namespace\}\}/g, namespace || "<namespace>")
    .replace(/\{\{\.Name\}\}/g, name || "<name>")
    .replace(/\{\{\.JobName\}\}/g, name ? `${name}-*` : "<job-name>")
    .replace(/\{\{\.ExitCode\}\}/g, exitCode?.toString() || "<exit-code>")
    .replace(/\{\{\.Reason\}\}/g, reason || "<reason>");

  if (compact) {
    return (
      <div className={cn("flex items-start gap-2", className)}>
        <Lightbulb className="h-3.5 w-3.5 text-blue-500 mt-0.5 shrink-0" />
        <span className="text-xs text-blue-700 dark:text-blue-400">{displayFix}</span>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "rounded-md bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 p-3",
        className
      )}
    >
      <div className="flex items-start gap-2">
        <Lightbulb className="h-4 w-4 text-blue-600 dark:text-blue-400 mt-0.5 shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <p className="text-sm font-medium text-blue-800 dark:text-blue-300">
              Suggested Fix
            </p>
            {exitCode !== undefined && exitCode !== 0 && (
              <Badge
                variant="outline"
                className="text-xs bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 border-red-200 dark:border-red-800"
              >
                Exit {exitCode}
              </Badge>
            )}
            {reason && (
              <Badge
                variant="outline"
                className="text-xs bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-400 border-orange-200 dark:border-orange-800"
              >
                {reason}
              </Badge>
            )}
          </div>
          <p className="text-sm text-blue-700 dark:text-blue-400 mt-1 break-words">
            {displayFix}
          </p>
          {hasKubectlCommand && (
            <Button
              variant="ghost"
              size="sm"
              onClick={handleCopyCommand}
              className="mt-2 h-7 text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900/50"
            >
              {copied ? (
                <>
                  <Check className="h-3 w-3 mr-1" />
                  Copied
                </>
              ) : (
                <>
                  <Terminal className="h-3 w-3 mr-1" />
                  Copy kubectl command
                </>
              )}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
