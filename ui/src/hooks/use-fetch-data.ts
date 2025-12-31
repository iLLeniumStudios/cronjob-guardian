"use client";

import { useCallback, useEffect, useState, useRef } from "react";

interface UseFetchDataOptions {
  /** Auto-refresh interval in milliseconds. Set to 0 to disable. Default: 5000 */
  refreshInterval?: number;
  /** Whether to fetch immediately on mount. Default: true */
  fetchOnMount?: boolean;
  /** Callback when an error occurs */
  onError?: (error: Error) => void;
}

interface UseFetchDataResult<T> {
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
 * A hook for fetching data with auto-refresh support.
 *
 * @example
 * // Simple usage
 * const { data, isLoading, refetch } = useFetchData(
 *   () => getStats(),
 *   { refreshInterval: 5000 }
 * );
 *
 * @example
 * // Multiple parallel fetches
 * const { data, isLoading, refetch } = useFetchData(
 *   async () => {
 *     const [stats, cronJobs, alerts] = await Promise.all([
 *       getStats(),
 *       listCronJobs(),
 *       listAlerts(),
 *     ]);
 *     return { stats, cronJobs, alerts };
 *   }
 * );
 */
export function useFetchData<T>(
  fetchFn: () => Promise<T>,
  options: UseFetchDataOptions = {}
): UseFetchDataResult<T> {
  const {
    refreshInterval = 5000,
    fetchOnMount = true,
    onError,
  } = options;

  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Use ref to keep the latest fetchFn without causing effect re-runs
  const fetchFnRef = useRef(fetchFn);
  fetchFnRef.current = fetchFn;

  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;

  const fetchData = useCallback(async (showRefreshing = false) => {
    if (showRefreshing) {
      setIsRefreshing(true);
    }

    try {
      const result = await fetchFnRef.current();
      setData(result);
      setError(null);
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      setError(error);
      console.error("Fetch error:", error);
      onErrorRef.current?.(error);
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  const refetch = useCallback(async () => {
    await fetchData(true);
  }, [fetchData]);

  useEffect(() => {
    if (fetchOnMount) {
      fetchData();
    } else {
      setIsLoading(false);
    }

    if (refreshInterval > 0) {
      const interval = setInterval(() => fetchData(), refreshInterval);
      return () => clearInterval(interval);
    }
  }, [fetchData, fetchOnMount, refreshInterval]);

  return {
    data,
    isLoading,
    isRefreshing,
    error,
    refetch,
  };
}
