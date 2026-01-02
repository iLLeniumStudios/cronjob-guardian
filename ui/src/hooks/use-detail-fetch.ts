"use client";

import { useState, useEffect, useCallback, useRef } from "react";

interface UseDetailFetchOptions<T> {
  /** Path prefix to match (e.g., "cronjob" for /cronjob/ns/name) */
  pathPrefix: string;
  /** Fetch function that takes namespace and name */
  fetchFn: (namespace: string, name: string) => Promise<T>;
  /** Auto-refresh interval in milliseconds. Default: 5000 */
  refreshInterval?: number;
  /** Callback when an error occurs */
  onError?: (error: Error) => void;
}

interface UseDetailFetchResult<T> {
  /** Parsed namespace from URL */
  namespace: string;
  /** Parsed name from URL */
  name: string;
  /** The fetched data, or null if not yet loaded */
  data: T | null;
  /** True during initial load (before first successful fetch) */
  isLoading: boolean;
  /** True during manual refresh (after initial load) */
  isRefreshing: boolean;
  /** The last error that occurred, or null */
  error: Error | null;
  /** Manually trigger a refresh */
  refetch: () => Promise<void>;
}

/**
 * A hook for detail pages that need to parse namespace/name from URL and fetch data.
 *
 * This hook combines:
 * 1. URL parsing for /prefix/namespace/name patterns
 * 2. Data fetching with loading/error states
 * 3. Auto-refresh support
 *
 * @example
 * const { namespace, name, data: cronJob, isLoading, isRefreshing, refetch } = useDetailFetch({
 *   pathPrefix: "cronjob",
 *   fetchFn: getCronJob,
 * });
 *
 * @example
 * const { namespace, name, data: monitor, isLoading } = useDetailFetch({
 *   pathPrefix: "monitors",
 *   fetchFn: getMonitor,
 *   refreshInterval: 10000,
 *   onError: (err) => toast.error(err.message),
 * });
 */
export function useDetailFetch<T>({
  pathPrefix,
  fetchFn,
  refreshInterval = 5000,
  onError,
}: UseDetailFetchOptions<T>): UseDetailFetchResult<T> {
  // URL parsing state
  const [namespace, setNamespace] = useState("");
  const [name, setName] = useState("");

  // Data fetching state
  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Use refs to keep latest callbacks without causing effect re-runs
  const fetchFnRef = useRef(fetchFn);
  fetchFnRef.current = fetchFn;

  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;

  // Parse URL on mount
  useEffect(() => {
    const path = window.location.pathname;
    const parts = path.split("/").filter(Boolean);
    if (parts.length >= 3 && parts[0] === pathPrefix) {
      setNamespace(decodeURIComponent(parts[1]));
      setName(decodeURIComponent(parts[2]));
    }
  }, [pathPrefix]);

  // Fetch data function
  const fetchData = useCallback(async (showRefreshing = false) => {
    if (!namespace || !name) {
      setIsLoading(false);
      return;
    }

    if (showRefreshing) {
      setIsRefreshing(true);
    }

    try {
      const result = await fetchFnRef.current(namespace, name);
      setData(result);
      setError(null);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      setError(error);
      console.error(`Failed to fetch ${pathPrefix} data:`, error);
      onErrorRef.current?.(error);
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, [namespace, name, pathPrefix]);

  // Refetch function for manual refresh
  const refetch = useCallback(async () => {
    await fetchData(true);
  }, [fetchData]);

  // Auto-fetch and refresh interval
  useEffect(() => {
    if (namespace && name) {
      fetchData();

      if (refreshInterval > 0) {
        const interval = setInterval(() => fetchData(), refreshInterval);
        return () => clearInterval(interval);
      }
    }
  }, [fetchData, namespace, name, refreshInterval]);

  return {
    namespace,
    name,
    data,
    isLoading,
    isRefreshing,
    error,
    refetch,
  };
}
