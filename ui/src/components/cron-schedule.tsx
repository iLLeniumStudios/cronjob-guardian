"use client";

import cronstrue from "cronstrue";
import { Info } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

interface CronScheduleProps {
  schedule: string;
  className?: string;
  showIcon?: boolean;
}

export function CronSchedule({ schedule, className, showIcon = false }: CronScheduleProps) {
  let description = "Invalid schedule";
  try {
    description = cronstrue.toString(schedule);
  } catch {
    // Keep default
  }

  return (
    <TooltipProvider delayDuration={300}>
      <Tooltip>
        <TooltipTrigger asChild>
          <div className={cn("flex items-center gap-1.5 cursor-help w-fit", className)}>
            <span className="font-mono text-sm truncate">{schedule}</span>
            {showIcon && <Info className="h-3.5 w-3.5 text-muted-foreground/70" />}
          </div>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-[300px] text-center">
          <p className="font-medium">{description}</p>
          <p className="text-xs text-muted-foreground mt-1 font-mono">{schedule}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
