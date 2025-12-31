"use client";

import {
  LayoutDashboard,
  Timer,
  Bell,
  Radio,
  Shield,
  Target,
} from "lucide-react";
import { NavLink } from "./nav-link";

export function Sidebar() {
  return (
    <aside className="flex h-full w-52 flex-col border-r border-border bg-sidebar">
      {/* Logo */}
      <div className="flex h-16 items-center gap-2.5 border-b border-border px-4">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
          <Shield className="h-4 w-4" />
        </div>
        <div className="flex flex-col">
          <span className="font-semibold tracking-tight">Guardian</span>
          <span className="text-[10px] text-muted-foreground">CronJob Monitoring</span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-0.5 p-2">
        <NavLink href="/" icon={LayoutDashboard} exact>
          Dashboard
        </NavLink>
        <NavLink href="/monitors" icon={Timer}>
          Monitors
        </NavLink>
        <NavLink href="/sla" icon={Target}>
          SLA Compliance
        </NavLink>
        <NavLink href="/channels" icon={Radio}>
          Channels
        </NavLink>
        <NavLink href="/alerts" icon={Bell}>
          Alerts
        </NavLink>
      </nav>
    </aside>
  );
}
