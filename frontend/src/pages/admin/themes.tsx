import { Badge, Button, Callout, TextField } from "@radix-ui/themes";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { usePublicInfo } from "@/contexts/usePublicInfo";

interface ThemeInfo {
  name?: string | Record<string, string>;
  short: string;
  description?: string | Record<string, string>;
  version?: string;
  author?: string | Record<string, string>;
  preview?: string;
  url?: string;
}

function text(value: ThemeInfo["name"], language: string) {
  if (typeof value === "string") return value;
  if (!value) return "";
  return value[language] ?? value[language.split(/[-_]/)[0]] ?? Object.values(value)[0] ?? "";
}

export default function ThemesPage() {
  const { t, i18n } = useTranslation();
  const { publicInfo, refresh: refreshPublicInfo } = usePublicInfo();
  const navigate = useNavigate();
  const [themes, setThemes] = useState<ThemeInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [switching, setSwitching] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [remoteUrl, setRemoteUrl] = useState("");
  const [remoteBusy, setRemoteBusy] = useState(false);
  const [marketBusy, setMarketBusy] = useState(false);
  const language = i18n.resolvedLanguage || i18n.language || "en";
  const currentTheme = publicInfo?.theme || "Emerald";

  const loadThemes = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch("/api/admin/theme/list", { cache: "no-store" });
      const payload = await response.json();
      if (!response.ok || payload.status === "error") {
        throw new Error(payload.message || `HTTP ${response.status}`);
      }
      setThemes(payload.data ?? []);
      const first = payload.data?.[0] as ThemeInfo | undefined;
      if (first?.url) setRemoteUrl(first.url);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("theme.load_failed", "加载主题失败"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadThemes();
  }, [loadThemes]);

  const selectTheme = async (short: string) => {
    if (short === currentTheme) return;
    setSwitching(short);
    try {
      const response = await fetch(`/api/admin/theme/set?theme=${encodeURIComponent(short)}`, {
        credentials: "include",
      });
      const payload = await response.json();
      if (!response.ok || payload.status === "error") {
        throw new Error(payload.message || `HTTP ${response.status}`);
      }
      await refreshPublicInfo();
      toast.success(t("theme.set_success", "主题已切换"));
      window.location.reload();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("theme.set_failed", "主题切换失败"));
    } finally {
      setSwitching(null);
    }
  };

  const runRemoteAction = async (path: string, body: Record<string, string>) => {
    setRemoteBusy(true);
    try {
      const response = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(body),
      });
      const payload = await response.json();
      if (!response.ok || payload.status === "error") throw new Error(payload.message || `HTTP ${response.status}`);
      toast.success(t("theme.remote_success", "Emerald 远程操作成功"));
      await loadThemes();
      await refreshPublicInfo();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("theme.remote_failed", "远程主题操作失败"));
    } finally {
      setRemoteBusy(false);
    }
  };

  const installFromMarket = async () => {
    setMarketBusy(true);
    try {
      const response = await fetch("/api/admin/theme/market/catalog?refresh=true", { credentials: "include" });
      const payload = await response.json();
      if (!response.ok || payload.status === "error") throw new Error(payload.message || `HTTP ${response.status}`);
      const emerald = payload.data?.themes?.find((item: { short?: string; installable?: boolean; source_id?: string }) => item.short === "Emerald" && item.installable);
      const source = payload.data?.sources?.find((item: { id?: string; error?: string }) => !item.error && item.id === emerald?.source_id);
      if (!source || !emerald) throw new Error(t("theme.market_unavailable", "官方市场中没有可安装的 Emerald 包"));
      const install = await fetch("/api/admin/theme/market/install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ source_id: source.id, short: "Emerald" }),
      });
      const result = await install.json();
      if (!install.ok || result.status === "error") throw new Error(result.message || `HTTP ${install.status}`);
      toast.success(t("theme.market_success", "已从官方市场更新 Emerald"));
      await loadThemes();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("theme.market_failed", "主题市场操作失败"));
    } finally {
      setMarketBusy(false);
    }
  };

  if (loading) return <Loading />;
  if (error) {
    return (
      <Callout.Root color="red">
        <Callout.Text>{error}</Callout.Text>
      </Callout.Root>
    );
  }

  return (
    <div className="km-page-admin-themes max-w-5xl space-y-6">
      <div className="space-y-1">
        <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">{t("theme.title", "主题")}</h1>
        <p className="text-sm text-muted-foreground">
          {t("theme.local_description", "当前使用 Komari Emerald，可从主题市场导入并在线更新。")}
        </p>
      </div>

      <div className="rounded-xl border border-border/60 bg-card p-5 shadow-2xs space-y-4">
        <div>
          <h2 className="text-sm font-semibold tracking-tight text-foreground">{t("theme.remote_title", "远程主题管理")}</h2>
          <p className="text-xs text-muted-foreground mt-0.5">{t("theme.remote_description", "仅允许下载、校验并覆盖 Komari Emerald。")}</p>
        </div>
        <div className="space-y-3 max-w-xl">
          <TextField.Root
            value={remoteUrl}
            onChange={(event) => setRemoteUrl(event.target.value)}
            placeholder="https://.../theme.zip 或 GitHub 仓库地址"
            className="w-full"
          />
          <div className="flex gap-2 flex-wrap">
            <Button disabled={remoteBusy || !remoteUrl.trim()} onClick={() => void runRemoteAction("/api/admin/theme/import", { url: remoteUrl.trim() })}>
              {t("theme.remote_import", "远程导入")}
            </Button>
            <Button variant="soft" disabled={remoteBusy} onClick={() => void runRemoteAction("/api/admin/theme/update", { short: "Emerald", url: remoteUrl.trim() })}>
              {t("theme.remote_update", "在线更新 Emerald")}
            </Button>
            <Button variant="soft" disabled={marketBusy} onClick={() => void installFromMarket()}>
              {t("theme.market_install", "从官方市场更新")}
            </Button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-5">
        {themes.map((theme) => {
          const active = theme.short === currentTheme;
          const preview = theme.preview ? `/themes/${theme.short}/${theme.preview}` : undefined;
          return (
            <div
              key={theme.short}
              className={`rounded-xl border transition-all duration-200 overflow-hidden bg-card flex flex-col justify-between shadow-2xs ${
                active ? "border-primary ring-1 ring-primary/30" : "border-border/60 hover:border-border"
              }`}
            >
              <div>
                {preview ? (
                  <div className="aspect-video w-full overflow-hidden bg-muted/40 border-b border-border/40">
                    <img
                      src={preview}
                      alt={text(theme.name, language)}
                      className="w-full h-full object-cover"
                    />
                  </div>
                ) : (
                  <div className="aspect-video w-full bg-muted/30 border-b border-border/40 flex items-center justify-center text-muted-foreground/40 text-xs">
                    No Preview
                  </div>
                )}
                <div className="p-4 space-y-2">
                  <div className="flex justify-between items-start gap-2">
                    <h3 className="font-semibold text-base tracking-tight text-foreground">{text(theme.name, language) || theme.short}</h3>
                    {active && <Badge color="green" variant="soft">{t("theme.active", "当前使用")}</Badge>}
                  </div>
                  <p className="text-xs text-muted-foreground line-clamp-2">
                    {text(theme.description, language)}
                  </p>
                  <p className="text-[11px] text-muted-foreground/75">
                    {theme.author ? `${t("theme.author", "作者")}: ${text(theme.author, language)}` : theme.short}
                    {theme.version ? ` · v${theme.version}` : ""}
                  </p>
                </div>
              </div>

              <div className="p-4 pt-0 flex gap-2">
                <Button
                  className="flex-1 cursor-pointer"
                  disabled={active || switching !== null}
                  onClick={() => void selectTheme(theme.short)}
                  variant={active ? "soft" : "solid"}
                >
                  {switching === theme.short
                    ? t("common.loading", "切换中…")
                    : active
                      ? t("theme.active", "当前使用")
                      : t("theme.use", "使用此主题")}
                </Button>
                <Button variant="soft" className="cursor-pointer" onClick={() => navigate(`/admin/themes/settings?theme=${encodeURIComponent(theme.short)}`)}>
                  {t("theme.settings", "配置")}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
