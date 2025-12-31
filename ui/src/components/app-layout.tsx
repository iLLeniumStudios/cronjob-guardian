"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Timer,
  Bell,
  Radio,
  Shield,
  Target,
  Menu,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { cn } from "@/lib/utils";

const navItems = [
  { href: "/", icon: LayoutDashboard, label: "Dashboard", exact: true },
  { href: "/monitors", icon: Timer, label: "Monitors" },
  { href: "/sla", icon: Target, label: "SLA Compliance" },
  { href: "/channels", icon: Radio, label: "Channels" },
  { href: "/alerts", icon: Bell, label: "Alerts" },
];

function NavItem({
  href,
  icon: Icon,
  label,
  exact,
  onClick,
}: {
  href: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  exact?: boolean;
  onClick?: () => void;
}) {
  const pathname = usePathname();
  const isActive = exact ? pathname === href : pathname.startsWith(href);

  return (
    <Link
      href={href}
      onClick={onClick}
      className={cn(
        "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
        isActive
          ? "bg-primary text-primary-foreground"
          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
      )}
    >
      <Icon className="h-4 w-4" />
      {label}
    </Link>
  );
}

export function AppLayout({ children }: { children: React.ReactNode }) {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  return (
    <div className="flex h-screen flex-col md:flex-row">
      {/* Desktop Sidebar */}
      <aside className="hidden md:flex h-full w-52 flex-col border-r border-border bg-sidebar">
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
          {navItems.map((item) => (
            <NavItem key={item.href} {...item} />
          ))}
        </nav>
      </aside>

      {/* Mobile Header */}
      <div className="flex md:hidden h-14 items-center justify-between border-b border-border px-4 bg-background">
        <div className="flex items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Shield className="h-3.5 w-3.5" />
          </div>
          <span className="font-semibold tracking-tight">Guardian</span>
        </div>
        <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" className="h-9 w-9 cursor-pointer">
              <Menu className="h-5 w-5" />
              <span className="sr-only">Toggle menu</span>
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-64 p-0">
            <SheetHeader className="border-b border-border p-4">
              <SheetTitle className="flex items-center gap-2">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                  <Shield className="h-4 w-4" />
                </div>
                <div className="flex flex-col items-start">
                  <span className="font-semibold tracking-tight">Guardian</span>
                  <span className="text-[10px] text-muted-foreground font-normal">CronJob Monitoring</span>
                </div>
              </SheetTitle>
            </SheetHeader>
            <nav className="space-y-0.5 p-2">
              {navItems.map((item) => (
                <NavItem
                  key={item.href}
                  {...item}
                  onClick={() => setMobileMenuOpen(false)}
                />
              ))}
            </nav>
          </SheetContent>
        </Sheet>
      </div>

      {/* Main Content */}
      <main className="flex-1 overflow-auto min-w-0">{children}</main>
    </div>
  );
}
