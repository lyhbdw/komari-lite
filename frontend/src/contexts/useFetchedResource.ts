import { useCallback, useEffect, useRef, useState } from "react";

import { notifyUnauthorized, SESSION_EXPIRED_MESSAGE } from "@/lib/auth-events";

interface UseFetchedResourceOptions<T> {
  url: string;
  initialData: T;
  errorMessage: string;
  initialLoading?: boolean;
  loadingOnlyFirstTime?: boolean;
  transform?: (json: any) => T;
}

export function useFetchedResource<T>({
  url,
  initialData,
  errorMessage,
  initialLoading = false,
  loadingOnlyFirstTime = false,
  transform,
}: UseFetchedResourceOptions<T>) {
  const [data, setData] = useState<T>(initialData);
  const [loading, setLoading] = useState<boolean>(initialLoading);
  const [error, setError] = useState<Error | null>(null);
  const firstLoadRef = useRef(true);
  const transformRef = useRef(transform);
  transformRef.current = transform;

  const refresh = useCallback(async () => {
    if (!loadingOnlyFirstTime || firstLoadRef.current) {
      setLoading(true);
    }
    setError(null);
    try {
      const response = await fetch(url);
      if (response.status === 401) {
        // 会话过期：通知全局翻转为未登录态（弹出登录框），并给出可读的错误信息
        notifyUnauthorized();
        throw new Error(SESSION_EXPIRED_MESSAGE);
      }
      if (!response.ok) throw new Error(errorMessage);
      const json = await response.json();
      setData(transformRef.current ? transformRef.current(json) : (json as T));
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (!loadingOnlyFirstTime || firstLoadRef.current) {
        setLoading(false);
        firstLoadRef.current = false;
      }
    }
  }, [url, errorMessage, loadingOnlyFirstTime]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return {
    data,
    loading,
    error,
    errorString: error ? error.message : null,
    refresh,
  };
}
