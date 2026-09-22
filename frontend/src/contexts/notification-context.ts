import React from "react";

export type OfflineNotification = {
  client: string;
  enable: boolean;
  cooldown: number;
  grace_period: number;
  last_notified: string;
};

export interface OfflineNotificationContextType {
  offlineNotification: OfflineNotification[];
  loading?: boolean;
  error?: Error | null;
  refresh: () => Promise<void>;
}

export const NotificationContext = React.createContext<
  OfflineNotificationContextType | undefined
>(undefined);
