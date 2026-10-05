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

  // 安全配置
  const [corsOrigins, setCorsOrigins] = useState("");
  const [wsOrigins, setWsOrigins] = useState("");
  const [ssrfEnabled, setSsrfEnabled] = useState(false);

  const [savingBasic, setSavingBasic] = useState(false);
  const [savingSecurity, setSavingSecurity] = useState(false);

  useEffect(() => {
    if (settings) {
      setSitename(settings.sitename || "");
      setDescription(settings.description || "");
      setScriptDomain(settings.script_domain || "");

      setCorsOrigins(settings.cors_allowed_origins || "");
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
        cors_allowed_origins: corsOrigins,
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
      {/* 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("settings.site.title", "站点设置")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {sitename || "Monitor"}
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            管理站点公开信息、Agent 安装连接地址与跨域访问安全。
          </p>
        </div>
      </div>

      {/* 卡片 1: 基本信息 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <Globe size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("settings.site.basic_info", "基本信息")}
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
            <p className="text-[11px] text-muted-foreground">展示在前台顶栏与浏览器标签页。</p>
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
            <p className="text-[11px] text-muted-foreground">一键安装脚本与探针通信地址，留空使用当前域名。</p>
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
            placeholder="站点的简要介绍，用于前台副标题与公开元信息预览"
            className="w-full p-2.5 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs resize-none"
          />
        </div>
      </div>

      {/* 卡片 2: 安全设置 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div className="flex items-center gap-2">
            <Shield size={16} className="text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">
              {t("settings.site.security_title", "安全设置")}
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
              {t("settings.site.ssrf_protection_enabled", "局域网请求拦截（SSRF 防护）")}
            </span>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              禁止面板向私有局域网及本地回环地址发起远程网络请求。
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

        {/* 访问来源白名单（双列对称网格布局） */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-1">
          {/* 左列: CORS 白名单 */}
          <div className="flex flex-col justify-between p-3.5 rounded-lg border border-border/60 bg-muted/20">
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium text-foreground block">
                  {t("settings.site.cors_origin_check", "API 跨域白名单 (CORS)")}
                </span>
                <span className="px-1.5 py-0.2 text-[10px] font-mono rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 shrink-0">
                  内置严格校验
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                默认仅允许同源调用，额外允许的 API 来源在此填写：
              </p>
            </div>
            <textarea
              value={corsOrigins}
              onChange={(e) => setCorsOrigins(e.target.value)}
              rows={3}
              placeholder={"https://example.com\nhttps://sub.example.com"}
              className="mt-2.5 w-full p-2 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono resize-none"
            />
          </div>

          {/* 右列: WebSocket 白名单 */}
          <div className="flex flex-col justify-between p-3.5 rounded-lg border border-border/60 bg-muted/20">
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium text-foreground block">
                  {t("settings.site.ws_origin_check", "WebSocket 来源白名单")}
                </span>
                <span className="px-1.5 py-0.2 text-[10px] font-mono rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 shrink-0">
                  内置严格校验
                </span>
              </div>
              <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                默认仅允许同源握手，额外允许的长连接来源在此填写：
              </p>
            </div>
            <textarea
              value={wsOrigins}
              onChange={(e) => setWsOrigins(e.target.value)}
              rows={3}
              placeholder={"https://example.com\nhttps://sub.example.com"}
              className="mt-2.5 w-full p-2 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono resize-none"
            />
          </div>
        </div>
      </div>
    </div>
  );
}
