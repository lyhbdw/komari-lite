import React, { useCallback, useEffect, useRef, useState } from "react";
import {
  NotificationContext,
  type OfflineNotification,
} from "./notification-context";

export const OfflineNotificationProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  const [offlineNotification, setOfflineNotification] = useState<OfflineNotification[]>([]);
  const [loading, setLoading] = useState(false);
  const firstLoad = useRef(true);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    if (firstLoad.current) setLoading(true);
    try {
      const response = await fetch("/api/admin/notification/offline");
      if (!response.ok) throw new Error("Failed to fetch offline notifications");
      const data = await response.json();
      setOfflineNotification(data.data || []);
      setError(null);
    } catch (err) {
      console.error("Error fetching offline notifications:", err);
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (firstLoad.current) {
        setLoading(false);
        firstLoad.current = false;
      }
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <NotificationContext.Provider value={{ offlineNotification, refresh, loading, error }}>
      {children}
    </NotificationContext.Provider>
  );
};
