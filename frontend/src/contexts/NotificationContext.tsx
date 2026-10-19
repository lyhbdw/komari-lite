import React from "react";
import {
  NotificationContext,
  type OfflineNotification,
} from "./notification-context";
import { useFetchedResource } from "./useFetchedResource";

export const OfflineNotificationProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  const {
    data: offlineNotification,
    loading,
    error,
    refresh,
  } = useFetchedResource<OfflineNotification[]>({
    url: "/api/admin/notification/offline",
    initialData: [],
    errorMessage: "Failed to fetch offline notifications",
    loadingOnlyFirstTime: true,
    transform: (resp) => resp?.data || [],
  });

  return (
    <NotificationContext.Provider
      value={{ offlineNotification, refresh, loading, error }}
    >
      {children}
    </NotificationContext.Provider>
  );
};
