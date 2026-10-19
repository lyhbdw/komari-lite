import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useFetchedResource } from "@/contexts/useFetchedResource";

/**
 * API utility functions for settings management
 */

export interface SettingsResponse {
  sitename: string;
  description: string;
  geo_ip_enabled: boolean;
  geo_ip_provider: string;
  ssrf_protection_enabled: boolean;
  metric_rollup_minute_retention_minutes?: number;
  metric_rollup_five_minute_retention_minutes?: number;
  metric_rollup_hour_retention_hours?: number;
  metric_rollup_day_retention_days?: number;
  CreatedAt: string;
  UpdatedAt: string;
  [key: string]: any;
}


const defaultSettings: SettingsResponse = {
  sitename: "",
  description: "",
  geo_ip_enabled: false,
  geo_ip_provider: "",
  ssrf_protection_enabled: false,
  CreatedAt: "",
  UpdatedAt: "",
};

function sanitizeSettingsResponse(payload: unknown): SettingsResponse {
  const response = payload as { data?: unknown };
  const settingsPayload = response?.data;
  if (
    typeof settingsPayload !== "object" ||
    settingsPayload === null ||
    Array.isArray(settingsPayload)
  ) {
    throw new Error("Invalid settings response payload");
  }

  return Object.fromEntries(
    Object.entries(settingsPayload).filter(
      ([key]) => !["CreatedAt", "UpdatedAt", "id"].includes(key),
    ),
  ) as SettingsResponse;
}

/**
 * Fetch settings from the API
 * @returns Promise containing the settings data
 */
async function getSettings(): Promise<SettingsResponse> {
  const response = await fetch("/api/admin/settings");
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return sanitizeSettingsResponse(await response.json());
}

/**
 * Update settings via the API
 * @param settings - The settings object to update
 * @returns Promise containing the response
 */
async function updateSettings(
  settings: Partial<SettingsResponse>
): Promise<void> {
  const response = await fetch("/api/admin/settings", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(settings),
  });

  if (!response.ok) {
    let message = `HTTP error! status: ${response.status}`;

    try {
      const errorData = await response.json();
      if (errorData?.message) {
        message = String(errorData.message);
      }
    } catch {
      // Keep the fallback HTTP status message.
    }

    console.error("Failed to update settings:", message);
    throw new Error(message);
  }

  try {
    await response.json();
  } catch {
    return;
  }
}

export async function updateSettingsWithToast(
  settings: Partial<SettingsResponse>,
  t: (key: string) => string
): Promise<void> {
  try {
    await updateSettings(settings);
    toast.success(t("settings.settings_saved"));
  } catch (error) {
    toast.error(t("settings.settings_save_failed") + ": " + error);
    throw error;
  }
}

/**
 * Hook for managing settings state and API calls
 */
export function useSettings() {
  const {
    data: settings,
    loading,
    errorString: error,
    refresh,
  } = useFetchedResource<SettingsResponse>({
    url: "/api/admin/settings",
    initialData: defaultSettings,
    errorMessage: "Failed to fetch settings",
    initialLoading: true,
    transform: sanitizeSettingsResponse,
  });
  const [settingsOverride, setSettingsOverride] = useState<SettingsResponse | null>(null);
  useEffect(() => {
    setSettingsOverride(null);
  }, [settings]);
  const activeSettings = settingsOverride ?? settings;
  const settingsRef = useRef(activeSettings);
  settingsRef.current = activeSettings;

  const setSettings = useCallback((updater: React.SetStateAction<SettingsResponse>) => {
    setSettingsOverride((current) => {
      const base = current ?? settingsRef.current;
      return typeof updater === "function"
        ? (updater as (previous: SettingsResponse) => SettingsResponse)(base)
        : updater;
    });
  }, []);

  const updateSetting = async <K extends keyof SettingsResponse>(
    key: K,
    value: SettingsResponse[K]
  ) => {
    try {
      const updatedSettings = { ...settingsRef.current, [key]: value };
      await updateSettings(updatedSettings);
      setSettings(updatedSettings);
    } catch (err) {
      throw err;
    }
  };

  const updateMultipleSettings = async (
    newSettings: Partial<SettingsResponse>
  ) => {
    const updatedSettings = { ...settingsRef.current, ...newSettings };
    await updateSettings(updatedSettings);
    setSettings(updatedSettings);
  };

  return {
    settings: activeSettings,
    loading,
    error,
    setSettings,
    updateSetting,
    updateMultipleSettings,
    refetch: refresh,
  };
}
