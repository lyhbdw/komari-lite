import { useTranslation } from "react-i18next";
import { Text } from "@radix-ui/themes";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import {
  SettingCardIconButton,
  SettingCardLabel,
  SettingCardLongTextInput,
  SettingCardShortTextInput,
  SettingCardSwitch,
} from "@/components/admin/SettingCard";
import Loading from "@/components/loading";
import { DownloadIcon } from "lucide-react";

export default function SiteSettings() {
  const { t } = useTranslation();
  const { settings, loading, error } = useSettings();

  if (loading) {
    return <Loading />;
  }

  if (error) {
    return <Text color="red">{error}</Text>;
  }

  return (
    <>
      <SettingCardLabel>{t("settings.site.title")}</SettingCardLabel>
      <SettingCardShortTextInput
        title={t("settings.site.name")}
        description={t("settings.site.name_description")}
        defaultValue={settings.sitename || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ sitename: data }, t);
        }}
      />
      <SettingCardLongTextInput
        title={t("settings.site.description")}
        description={t("settings.site.description_description")}
        defaultValue={settings.description || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ description: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.cors_origin_check_enabled")}
        description={t("settings.site.cors_origin_check_enabled_description")}
        defaultChecked={settings.cors_origin_check_enabled ?? true}
        onChange={async (checked) => {
          await updateSettingsWithToast({ cors_origin_check_enabled: checked }, t);
        }}
        className="km-page-admin-settings-site km-setting-card"
      />
      <SettingCardLongTextInput
        title={t("settings.site.cors_allowed_origins", "API CORS 允许列表")}
        description={t("settings.site.origins_list_description",
          "每行或用逗号分隔一个 Origin，例如 https://example.com",
        )}
        defaultValue={settings.cors_allowed_origins || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ cors_allowed_origins: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.ws_origin_check_enabled", "WebSocket Origin 校验")}
        description={t(
          "settings.site.ws_origin_check_enabled_description",
          "开启后 WebSocket 请求只允许同源或允许列表中的 Origin",
        )}
        defaultChecked={settings.ws_origin_check_enabled ?? true}
        onChange={async (checked) => {
          await updateSettingsWithToast(
            { ws_origin_check_enabled: checked },
            t,
          );
        }}
        className="km-setting-card"
      />
      <SettingCardLongTextInput
        title={t("settings.site.ws_allowed_origins", "WebSocket Origin 允许列表")}
        description={t("settings.site.origins_list_description",
          "每行或用逗号分隔一个 Origin，例如 https://example.com",
        )}
        defaultValue={settings.ws_allowed_origins || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ ws_allowed_origins: data }, t);
        }}
      />
      <SettingCardSwitch
        title={t("settings.site.ssrf_protection_enabled")}
        description={t("settings.site.ssrf_protection_enabled_description")}
        defaultChecked={settings.ssrf_protection_enabled ?? false}
        onChange={async (checked) => {
          await updateSettingsWithToast(
            { ssrf_protection_enabled: checked },
            t,
          );
        }}
        className="km-setting-card"
      />
      <SettingCardSwitch
        title={t("settings.site.send_ip_addr_to_guest")}
        description={t("settings.site.send_ip_addr_to_guest_description")}
        defaultChecked={settings.send_ip_addr_to_guest}
        onChange={async (checked) => {
          await updateSettingsWithToast({ send_ip_addr_to_guest: checked }, t);
        }}
        className="km-setting-card"
      />
      <SettingCardShortTextInput
        title={t("settings.site.script_domain")}
        description={t("settings.site.script_domain_description")}
        placeholder={`${window.location.origin}`}
        defaultValue={settings.script_domain || ""}
        OnSave={async (data) => {
          await updateSettingsWithToast({ script_domain: data }, t);
        }}
      />

      <SettingCardLabel>{t("settings.site.backup")}</SettingCardLabel>
      <SettingCardIconButton
        title={t("settings.site.backup_download")}
        description={t("settings.site.backup_download_description")}
        onClick={() => {
          window.open("/api/admin/download/backup", "_blank");
        }}
        className="km-setting-card"
      >
        <DownloadIcon size={16} />
      </SettingCardIconButton>

    </>
  );
}
