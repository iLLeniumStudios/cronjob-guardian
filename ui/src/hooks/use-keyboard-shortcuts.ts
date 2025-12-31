"use client";

import { useEffect, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";

interface KeyboardShortcutsOptions {
  /** Callback to trigger refresh */
  onRefresh?: () => void;
  /** Callback to show keyboard shortcuts dialog */
  onShowHelp?: () => void;
  /** Whether shortcuts are enabled (default: true) */
  enabled?: boolean;
}

/**
 * A hook that provides keyboard shortcuts for navigation and actions.
 *
 * Shortcuts:
 * - `r` - Refresh current page
 * - `g then d` - Go to Dashboard
 * - `g then m` - Go to Monitors
 * - `g then a` - Go to Alerts
 * - `g then c` - Go to Channels
 * - `g then l` - Go to SLA
 * - `g then s` - Go to Settings
 * - `?` - Show keyboard shortcuts help
 *
 * @example
 * useKeyboardShortcuts({
 *   onRefresh: refetch,
 *   onShowHelp: () => setHelpOpen(true),
 * });
 */
export function useKeyboardShortcuts({
  onRefresh,
  onShowHelp,
  enabled = true,
}: KeyboardShortcutsOptions = {}) {
  const router = useRouter();
  const pendingGotoRef = useRef(false);
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (!enabled) return;

      // Ignore if user is typing in an input, textarea, or contenteditable
      const target = event.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      // Ignore if any modifier keys are pressed (except for ?)
      if (event.metaKey || event.ctrlKey || event.altKey) {
        return;
      }

      const key = event.key.toLowerCase();

      // Handle "g then X" navigation
      if (pendingGotoRef.current) {
        pendingGotoRef.current = false;
        if (timeoutRef.current) {
          clearTimeout(timeoutRef.current);
          timeoutRef.current = null;
        }

        switch (key) {
          case "d":
            event.preventDefault();
            router.push("/");
            return;
          case "m":
            event.preventDefault();
            router.push("/monitors");
            return;
          case "a":
            event.preventDefault();
            router.push("/alerts");
            return;
          case "c":
            event.preventDefault();
            router.push("/channels");
            return;
          case "l":
            event.preventDefault();
            router.push("/sla");
            return;
          case "s":
            event.preventDefault();
            router.push("/settings");
            return;
        }
        return;
      }

      // Start "g then X" sequence
      if (key === "g") {
        event.preventDefault();
        pendingGotoRef.current = true;
        // Cancel after 1 second if no second key pressed
        timeoutRef.current = setTimeout(() => {
          pendingGotoRef.current = false;
        }, 1000);
        return;
      }

      // Refresh shortcut
      if (key === "r" && onRefresh) {
        event.preventDefault();
        onRefresh();
        return;
      }

      // Help shortcut
      if (key === "?" && onShowHelp) {
        event.preventDefault();
        onShowHelp();
        return;
      }
    },
    [enabled, onRefresh, onShowHelp, router]
  );

  useEffect(() => {
    if (!enabled) return;

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, [enabled, handleKeyDown]);
}

/**
 * List of all keyboard shortcuts for display in help dialog.
 */
export const KEYBOARD_SHORTCUTS = [
  { keys: ["r"], description: "Refresh current page" },
  { keys: ["g", "d"], description: "Go to Dashboard" },
  { keys: ["g", "m"], description: "Go to Monitors" },
  { keys: ["g", "a"], description: "Go to Alerts" },
  { keys: ["g", "c"], description: "Go to Channels" },
  { keys: ["g", "l"], description: "Go to SLA" },
  { keys: ["g", "s"], description: "Go to Settings" },
  { keys: ["?"], description: "Show keyboard shortcuts" },
] as const;
