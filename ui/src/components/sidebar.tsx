"use client";

import {
  LayoutDashboard,
  Timer,
  Bell,
  Radio,
  Settings,
  Shield,
  Target,
} from "lucide-react";
import { NavLink } from "./nav-link";

export function Sidebar() {
  return (
    <aside className="flex h-full w-52 flex-col border-r border-border bg-sidebar">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2.5 border-b border-border px-4">
        <Shield className="h-5 w-5" />
        <span className="font-semibold tracking-tight">Guardian</span>
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

      {/* Bottom section */}
      <div className="border-t border-border p-2">
        <NavLink href="/settings" icon={Settings}>
          Settings
        </NavLink>
      </div>
    </aside>
  );
}
