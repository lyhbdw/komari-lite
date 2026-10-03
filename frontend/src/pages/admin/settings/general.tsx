import { useTranslation } from "react-i18next";
import { Text, Switch } from "@radix-ui/themes";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import { SettingCardCollapse } from "@/components/admin/SettingCard";
import { useState } from "react";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { MapPin, RefreshCw, Play } from "lucide-react";

export default function GeneralSettings() {
  const { t } = useTranslation();
  const { settings, loading, error, setSettings } = useSettings();
  const [geoipTestResult, setGeoipTestResult] = useState<string | null>(null);
  const [updatingDb, setUpdatingDb] = useState(false);
  const [testingIp, setTestingIp] = useState(false);
  const [ipInput, setIpInput] = useState("");

  if (loading) {
    return <Loading text="" />;
  }
  if (error) {
    return <Text color="red">{error}</Text>;
  }

  const handleUpdateMmdb = async () => {
    setUpdatingDb(true);
    try {
      const result = await fetch("/api/admin/update/mmdb", {
        method: "POST",
      });
      const data = await result.json();
      if (data.status === "success") {
        toast.success(t("settings.geoip.update_success", "GeoIP 数据库更新成功"));
      } else {
        toast.error(data.message || t("settings.geoip.update_error", "更新失败"));
      }
    } catch {
      toast.error(t("settings.geoip.update_error", "更新失败"));
    } finally {
      setUpdatingDb(false);
    }
  };

  const handleTestGeoip = async () => {
    if (!ipInput.trim()) {
      toast.error("请输入要测试的 IP 地址");
      return;
    }
    setTestingIp(true);
    try {
      const result = await fetch(`/api/admin/test/geoip?ip=${encodeURIComponent(ipInput.trim())}`);
      const data = await result.json();
      setGeoipTestResult(
        JSON.stringify(data.data, null, 2) || t("common.no_results")
      );
    } catch {
      toast.error(t("common.error"));
    } finally {
      setTestingIp(false);
    }
  };

  return (
    <div className="space-y-4 max-w-4xl km-page-admin-settings-general">
      {/* 1. 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("settings.geoip.title", "地理位置与通用配置")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {settings.geo_ip_enabled ? "GeoIP 已启用" : "GeoIP 已关闭"}
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            配置服务器归属地解析服务商（如 ipinfo.io、MaxMind），并支持一键更新本地 IP 库与解析测试。
          </p>
        </div>
      </div>

      {/* 核心卡片: GeoIP 服务配置 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        {/* 开关行 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <MapPin size={16} className="text-muted-foreground" />
              <span className="text-sm font-semibold text-foreground">
                {t("settings.geoip.enable_title", "启用地理位置信息解析")}
              </span>
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  settings.geo_ip_enabled ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                }`}
              />
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("settings.geoip.enable_description", "开启后，面板自动识别节点 IP 所在的国家与地区旗帜。")}
            </p>
          </div>
          <div className="flex items-center gap-3 shrink-0">
            <span className="text-xs text-muted-foreground">
              {settings.geo_ip_enabled ? "已启用" : "已关闭"}
            </span>
            <Switch
              checked={settings.geo_ip_enabled}
              onCheckedChange={async (checked) => {
                await updateSettingsWithToast({ geo_ip_enabled: checked }, t);
                setSettings((prev: any) => ({ ...prev, geo_ip_enabled: checked }));
              }}
            />
          </div>
        </div>

        <div className="h-px bg-border/40" />

        {/* 提供商选择与数据库更新 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <span className="text-xs font-medium text-foreground block">
              {t("settings.geoip.provider_title", "地理位置数据提供商")}
            </span>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              选择用于解析 IP 归属地的第三方接口或本地数据库。
            </p>
          </div>
          <div className="flex items-center gap-2">
            <select
              value={settings.geo_ip_provider || "ipinfo"}
              onChange={async (e) => {
                const val = e.target.value;
                await updateSettingsWithToast({ geo_ip_provider: val }, t);
                setSettings((prev: any) => ({ ...prev, geo_ip_provider: val }));
              }}
              className="h-8 px-3 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 cursor-pointer shadow-2xs font-medium"
            >
              <option value="empty">{t("common.none", "不使用")}</option>
              <option value="ipinfo">ipinfo.io (推荐)</option>
              <option value="ip-api">ip-api.com</option>
              <option value="geojs">geojs.io</option>
              <option value="mmdb">MaxMind (本地数据库)</option>
            </select>
            <button
              type="button"
              onClick={handleUpdateMmdb}
              disabled={updatingDb}
              className="h-8 px-3 rounded-lg border border-border bg-card text-foreground font-medium text-xs flex items-center gap-1.5 shadow-2xs hover:bg-muted active:scale-[0.98] transition-all cursor-pointer shrink-0 disabled:opacity-50"
              title="下载并更新本地 MaxMind GeoIP 离线数据库"
            >
              <RefreshCw size={12} className={updatingDb ? "animate-spin" : ""} />
              <span>{updatingDb ? "更新中..." : "更新数据库"}</span>
            </button>
          </div>
        </div>
      </div>

      {/* 折叠卡片: GeoIP 快速测试 */}
      <SettingCardCollapse
        title={t("settings.geoip.test_title", "测试 GeoIP 数据库解析")}
        description={t("settings.geoip.test_description", "输入任意 IP 地址，验证当前配置的服务商能否正确返回地理位置与国别。")}
        defaultOpen={false}
      >
        <div className="space-y-3 pt-2">
          <div className="flex items-center gap-2">
            <input
              type="text"
              placeholder="1.1.1.1 或 2606:4700:4700::1111"
              value={ipInput}
              onChange={(e) => setIpInput(e.target.value)}
              className="flex-1 h-8 px-3 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
            />
            <button
              type="button"
              onClick={handleTestGeoip}
              disabled={testingIp}
              className="h-8 px-3.5 rounded-lg bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs shrink-0 flex items-center gap-1.5"
            >
              <Play size={12} />
              <span>{testingIp ? "查询中..." : t("settings.geoip.test_button", "执行测试")}</span>
            </button>
          </div>

          {geoipTestResult && (
            <div className="p-3 rounded-lg border border-border/70 bg-muted/30 font-mono text-xs leading-relaxed text-foreground overflow-auto max-h-60 select-all">
              <pre>{geoipTestResult}</pre>
            </div>
          )}
        </div>
      </SettingCardCollapse>
    </div>
  );
}
