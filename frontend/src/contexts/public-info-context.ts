import React from "react";

export interface PublicInfo {
  description: string;
  disable_password_login: boolean;
  metric_retention_days: number;
  sitename: string;
  theme: string;
  theme_settings: any;
  [property: string]: any;
}

export interface PublicInfoContextType {
  publicInfo: PublicInfo | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

export const PublicInfoContext = React.createContext<
  PublicInfoContextType | undefined
>(undefined);
