import { useCallback, useEffect, useState } from 'react';
import { ApiError } from '@/api/client';

interface AsyncState<T> {
  data: T | null;
  isLoading: boolean;
  error: string | null;
}

interface UseAsyncResourceOptions {
  /** Skip the request entirely (e.g. when the user lacks permissions). */
  enabled?: boolean;
}

function toMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error) return error.message;
  return 'Неизвестная ошибка';
}

/**
 * Small data-fetching hook: handles loading, error and refresh states for a
 * single GET endpoint.
 */
export function useAsyncResource<T>(
  loader: () => Promise<T>,
  deps: readonly unknown[],
  options: UseAsyncResourceOptions = {},
): AsyncState<T> & { refresh: () => void; setData: (value: T) => void } {
  const { enabled = true } = options;
  const [state, setState] = useState<AsyncState<T>>({
    data: null,
    isLoading: enabled,
    error: null,
  });
  const [reloadToken, setReloadToken] = useState(0);

  useEffect(() => {
    if (!enabled) {
      setState({ data: null, isLoading: false, error: null });
      return undefined;
    }

    let cancelled = false;
    setState((prev) => ({ ...prev, isLoading: true, error: null }));

    loader()
      .then((data) => {
        if (!cancelled) setState({ data, isLoading: false, error: null });
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setState({ data: null, isLoading: false, error: toMessage(error) });
        }
      });

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, reloadToken, enabled]);

  const refresh = useCallback(() => setReloadToken((token) => token + 1), []);
  const setData = useCallback((value: T) => {
    setState((prev) => ({ ...prev, data: value }));
  }, []);

  return { ...state, refresh, setData };
}
