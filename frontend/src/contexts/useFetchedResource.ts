import { useCallback, useEffect, useRef, useState } from "react";

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
