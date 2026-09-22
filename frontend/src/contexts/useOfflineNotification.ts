import React from "react";
import { NotificationContext } from "./notification-context";

export const useOfflineNotification = () => {
  const context = React.useContext(NotificationContext);
  if (!context) {
    throw new Error(
      "useOfflineNotification must be used within a OfflineNotificationProvider",
    );
  }
  return context;
};
