import { useCallback, useEffect, useMemo, useState } from 'react';
import { unwrapError } from '@/core/api';

const TRANSIENT_ERROR_PATTERNS = ['timeout', 'network', 'econn', 'rate', '429', 'temporar'];
const CACHE_TTL_MS = 15_000;
const cache = new Map<string, { at: number; data: unknown }>();

const delay = (ms: number) =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, ms);
  });

const isTransientError = (error: unknown) => {
  const message = unwrapError(error).toLowerCase();
  return TRANSIENT_ERROR_PATTERNS.some((pattern) => message.includes(pattern));
};

const executeWithRetry = async <T,>(fn: () => Promise<T>, retries = 2): Promise<T> => {
  let attempt = 0;
  while (true) {
    try {
      return await fn();
    } catch (error) {
      if (attempt >= retries || !isTransientError(error)) throw error;
      attempt += 1;
      await delay(300 * attempt);
    }
  }
};

export const useCachedQuery = <T,>(
  key: string,
  queryFn: () => Promise<T>,
  options?: { enabled?: boolean; cacheMs?: number }
) => {
  const enabled = options?.enabled ?? true;
  const cacheMs = options?.cacheMs ?? CACHE_TTL_MS;
  const cached = cache.get(key);
  const isCachedFresh = Boolean(cached && Date.now() - cached.at < cacheMs);
  const [data, setData] = useState<T | undefined>(() => (isCachedFresh ? (cached?.data as T) : undefined));
  const [isLoading, setIsLoading] = useState(enabled && !isCachedFresh);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(
    async (background = false) => {
      if (!enabled) return;
      if (background) setIsRefreshing(true);
      else setIsLoading(true);
      try {
        const result = await executeWithRetry(queryFn);
        setData(result);
        cache.set(key, { at: Date.now(), data: result });
        setError(null);
      } catch (err) {
        setError(unwrapError(err));
      } finally {
        setIsLoading(false);
        setIsRefreshing(false);
      }
    },
    [enabled, key, queryFn]
  );

  useEffect(() => {
    if (!enabled) return;
    if (isCachedFresh) return;
    void run();
  }, [enabled, isCachedFresh, run]);

  const refresh = useCallback(async () => {
    await run(true);
  }, [run]);

  return { data, isLoading, isRefreshing, error, refresh };
};

export const useMutationAction = <TInput, TOutput>(mutationFn: (input: TInput) => Promise<TOutput>) => {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TOutput | null>(null);

  const mutate = useCallback(
    async (input: TInput) => {
      setIsLoading(true);
      try {
        const output = await executeWithRetry(() => mutationFn(input));
        setData(output);
        setError(null);
        return output;
      } catch (err) {
        const message = unwrapError(err);
        setError(message);
        throw err;
      } finally {
        setIsLoading(false);
      }
    },
    [mutationFn]
  );

  const reset = useCallback(() => {
    setData(null);
    setError(null);
  }, []);

  return useMemo(() => ({ mutate, reset, isLoading, error, data }), [data, error, isLoading, mutate, reset]);
};
