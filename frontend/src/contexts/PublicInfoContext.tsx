import React, { useCallback, useEffect, useState } from "react";
import { PublicInfoContext, type PublicInfo } from "./public-info-context";

type Response = {
  data: PublicInfo;
  message: string;
  status: string;
};

export const PublicInfoProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [publicInfo, setPublicInfo] = useState<PublicInfo | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setError(null);
    setIsLoading(true);
    try {
      const response = await fetch("/api/public");
      if (!response.ok) throw new Error("Failed to fetch public info");
      const resp = (await response.json()) as Response;
      setPublicInfo(resp?.data ?? null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <PublicInfoContext.Provider value={{ publicInfo, isLoading, error, refresh }}>
      {children}
    </PublicInfoContext.Provider>
  );
};
