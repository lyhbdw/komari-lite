import { useTranslation } from "react-i18next";
import { Text, Switch } from "@radix-ui/themes";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import Loading from "@/components/loading";
import { Globe, Shield } from "lucide-react";
import { useState, useEffect } from "react";

export default function SiteSettings() {
  const { t } = useTranslation();
  const { settings, loading, error, setSettings } = useSettings();

  // 本地表单状态
  const [sitename, setSitename] = useState("");
  const [description, setDescription] = useState("");
  const [scriptDomain, setScriptDomain] = useState("");
  const [sendIpToGuest, setSendIpToGuest] = useState(false);

  // 安全配置
  const [corsEnabled, setCorsEnabled] = useState(true);
  const [corsOrigins, setCorsOrigins] = useState("");
  const [wsEnabled, setWsEnabled] = useState(true);
  const [wsOrigins, setWsOrigins] = useState("");
  const [ssrfEnabled, setSsrfEnabled] = useState(false);

  const [savingBasic, setSavingBasic] = useState(false);
  const [savingSecurity, setSavingSecurity] = useState(false);

  useEffect(() => {
    if (settings) {
      setSitename(settings.sitename || "");
      setDescription(settings.description || "");
      setScriptDomain(settings.script_domain || "");
      setSendIpToGuest(!!settings.send_ip_addr_to_guest);

      setCorsEnabled(settings.cors_origin_check_enabled ?? true);
      setCorsOrigins(settings.cors_allowed_origins || "");
      setWsEnabled(settings.ws_origin_check_enabled ?? true);
      setWsOrigins(settings.ws_allowed_origins || "");
      setSsrfEnabled(settings.ssrf_protection_enabled ?? false);
    }
  }, [settings]);

  if (loading) return <Loading />;
  if (error) return <Text color="red">{error}</Text>;

  const handleSaveBasic = async () => {
    setSavingBasic(true);
    try {
      const payload = {
        sitename,
        description,
        script_domain: scriptDomain,
        send_ip_addr_to_guest: sendIpToGuest,
      };
      await updateSettingsWithToast(payload, t);
      setSettings((prev: any) => ({ ...prev, ...payload }));
    } finally {
      setSavingBasic(false);
    }
  };

  const handleSaveSecurity = async () => {
    setSavingSecurity(true);
    try {
      const payload = {
        cors_origin_check_enabled: corsEnabled,
        cors_allowed_origins: corsOrigins,
        ws_origin_check_enabled: wsEnabled,
        ws_allowed_origins: wsOrigins,
        ssrf_protection_enabled: ssrfEnabled,
      };
      await updateSettingsWithToast(payload, t);
      setSettings((prev: any) => ({ ...prev, ...payload }));
    } finally {
      setSavingSecurity(false);
    }
  };

  return (
    <div className="space-y-4 max-w-4xl km-page-admin-settings-site">
      {/* 1. 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("settings.site.title", "站点与系统设置")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {sitename || "Komari"}
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            配置探针站点的公开品牌信息、安装脚本连接地址与跨域安全校验。
          </p>
        </div>
      </div>

      {/* 卡片 1: 站点基本信息与公开显示 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <Globe size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("settings.site.basic_info", "基本信息与展示")}
            </h2>
          </div>
          <button
            type="button"
            onClick={handleSaveBasic}
            disabled={savingBasic}
            className="h-7 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-50"
          >
            {savingBasic ? t("common.saving", "保存中...") : t("common.save", "保存修改")}
          </button>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
          {/* 站点名称 */}
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-foreground block">
              {t("settings.site.name", "站点名称")}
            </label>
            <input
              type="text"
              value={sitename}
              onChange={(e) => setSitename(e.target.value)}
              placeholder="例如：海狸の探针"
              className="w-full h-8 px-3 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs"
            />
            <p className="text-[11px] text-muted-foreground">显示在浏览器标签页与前台顶栏。</p>
          </div>

          {/* Agent 连接地址 */}
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-foreground block">
              {t("settings.site.script_domain", "Agent 连接地址")}
            </label>
            <input
              type="text"
              value={scriptDomain}
              onChange={(e) => setScriptDomain(e.target.value)}
              placeholder={window.location.origin}
              className="w-full h-8 px-3 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
            />
            <p className="text-[11px] text-muted-foreground">一键安装脚本所使用的面板域名，留空使用当前域名。</p>
          </div>
        </div>

        {/* 站点描述 */}
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-foreground block">
            {t("settings.site.description", "站点描述")}
          </label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
            placeholder="设置站点描述，用于 SEO 元信息及社交媒体卡片预览"
            className="w-full p-2.5 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs resize-none"
          />
        </div>

        <div className="h-px bg-border/40" />

        {/* 访客显示部分IP开关 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <span className="text-xs font-medium text-foreground block">
              {t("settings.site.send_ip_addr_to_guest", "向访客展示部分 IP 地址")}
            </span>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              未登录访客可通过 API 查看脱敏 IP（形如 9.*.*.*），用于部分前台主题展示机器归属。
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <span className="text-xs text-muted-foreground">
              {sendIpToGuest ? "已公开" : "已隐藏"}
            </span>
            <Switch checked={sendIpToGuest} onCheckedChange={setSendIpToGuest} />
          </div>
        </div>
      </div>

      {/* 卡片 2: 网络与访问安全防护（CORS / WS / SSRF） */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <Shield size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("settings.site.security_title", "网络与访问安全防护")}
            </h2>
          </div>
          <button
            type="button"
            onClick={handleSaveSecurity}
            disabled={savingSecurity}
            className="h-7 px-3 rounded-md bg-foreground text-background text-xs font-medium hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shadow-2xs disabled:opacity-50"
          >
            {savingSecurity ? t("common.saving", "保存中...") : t("common.save", "保存修改")}
          </button>
        </div>

        {/* SSRF 防护开关 */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-1">
          <div>
            <span className="text-xs font-medium text-foreground block">
              {t("settings.site.ssrf_protection_enabled", "启用 SSRF 拦截防护")}
            </span>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              拦截任何针对私有局域网、本地回环等非公网地址的远程下载调用。
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <span className="text-xs text-muted-foreground">
              {ssrfEnabled ? "已启用" : "已停用"}
            </span>
            <Switch checked={ssrfEnabled} onCheckedChange={setSsrfEnabled} />
          </div>
        </div>

        <div className="h-px bg-border/40" />

        {/* CORS 校验与允许列表 */}
        <div className="space-y-2">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div>
              <span className="text-xs font-medium text-foreground block">
                {t("settings.site.cors_origin_check_enabled", "CORS 跨域请求校验")}
              </span>
              <p className="text-[11px] text-muted-foreground mt-0.5">
                开启后，API 接口只允许同源或下方白名单中的域名跨域调用。
              </p>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <span className="text-xs text-muted-foreground">
                {corsEnabled ? "已开启" : "已停用"}
              </span>
              <Switch checked={corsEnabled} onCheckedChange={setCorsEnabled} />
            </div>
          </div>

          {corsEnabled && (
            <div className="pt-1">
              <label className="text-[11px] text-muted-foreground block mb-1 font-mono">
                API CORS 允许列表（每行或逗号分隔）：
              </label>
              <textarea
                value={corsOrigins}
                onChange={(e) => setCorsOrigins(e.target.value)}
                rows={2}
                placeholder="https://example.com"
                className="w-full p-2 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono resize-none"
              />
            </div>
          )}
        </div>

        <div className="h-px bg-border/40" />

        {/* WebSocket Origin 校验与允许列表 */}
        <div className="space-y-2">
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <div>
              <span className="text-xs font-medium text-foreground block">
                {t("settings.site.ws_origin_check_enabled", "WebSocket Origin 握手校验")}
              </span>
              <p className="text-[11px] text-muted-foreground mt-0.5">
                开启后，实时数据 WebSocket 连接仅允许白名单来源建立长连接。
              </p>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <span className="text-xs text-muted-foreground">
                {wsEnabled ? "已开启" : "已停用"}
              </span>
              <Switch checked={wsEnabled} onCheckedChange={setWsEnabled} />
            </div>
          </div>

          {wsEnabled && (
            <div className="pt-1">
              <label className="text-[11px] text-muted-foreground block mb-1 font-mono">
                WebSocket 允许列表（每行或逗号分隔）：
              </label>
              <textarea
                value={wsOrigins}
                onChange={(e) => setWsOrigins(e.target.value)}
                rows={2}
                placeholder="https://example.com"
                className="w-full p-2 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono resize-none"
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
