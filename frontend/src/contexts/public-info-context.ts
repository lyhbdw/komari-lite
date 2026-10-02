import React from "react";
import defaultTheme from "../../komari-theme.json";

type ThemeField = {
  key?: string;
  default?: unknown;
};

export interface PublicInfo {
  cors_origin_check_enabled: boolean;
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

const defaultThemeSettings = Object.fromEntries(
  ((defaultTheme.configuration?.data ?? []) as ThemeField[])
    .filter(
      (field) =>
        typeof field.key === "string" &&
        Object.prototype.hasOwnProperty.call(field, "default"),
    )
    .map((field) => [field.key, field.default]),
);

export const withThemeDefaults = (publicInfo: PublicInfo): PublicInfo => {
  if (publicInfo.theme !== "Emerald") return publicInfo;
  return {
    ...publicInfo,
    theme_settings: {
      ...defaultThemeSettings,
      ...(publicInfo.theme_settings ?? {}),
    },
  };
};

export const PublicInfoContext = React.createContext<
  PublicInfoContextType | undefined
>(undefined);
