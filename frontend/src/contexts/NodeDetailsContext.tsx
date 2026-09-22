import React, { useCallback, useEffect, useRef, useState } from "react";
import { NodeDetailsContext, type NodeDetail } from "./node-details-context";

export const NodeDetailsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [nodeDetail, setNodeDetail] = useState<NodeDetail[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);
  const hasLoadedRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const refresh = useCallback(() => {
    if (!hasLoadedRef.current) {
      setIsLoading(true);
    }
    setError(null);
    fetch("/api/admin/client/list")
      .then((response) => {
        if (!response.ok) throw new Error("Failed to fetch node details");
        return response.json();
      })
      .then((data: NodeDetail[]) => {
        if (!mountedRef.current) return;
        setNodeDetail(data);
        hasLoadedRef.current = true;
        setIsLoading(false);
      })
      .catch((err) => {
        if (!mountedRef.current) return;
        setError(err instanceof Error ? err.message : String(err));
        setIsLoading(false);
      });
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <NodeDetailsContext.Provider value={{ nodeDetail, isLoading, error, refresh }}>
      {children}
    </NodeDetailsContext.Provider>
  );
};
