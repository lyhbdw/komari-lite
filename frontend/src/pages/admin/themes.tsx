import { Badge, Button, Callout, Card, Flex, Heading, Text } from "@radix-ui/themes";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { usePublicInfo } from "@/contexts/PublicInfoContext";

interface ThemeInfo {
  name?: string | Record<string, string>;
  short: string;
  description?: string | Record<string, string>;
  version?: string;
  author?: string | Record<string, string>;
  preview?: string;
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
  const language = i18n.resolvedLanguage || i18n.language || "en";
  const currentTheme = publicInfo?.theme || "default";

  const loadThemes = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch("/api/admin/theme/list", { cache: "no-store" });
      const payload = await response.json();
      if (!response.ok || payload.status === "error") {
        throw new Error(payload.message || `HTTP ${response.status}`);
      }
      setThemes(payload.data ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("theme.load_failed", "加载主题失败"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadThemes();
  }, []);

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

  if (loading) return <Loading />;
  if (error) {
    return (
      <Callout.Root color="red">
        <Callout.Text>{error}</Callout.Text>
      </Callout.Root>
    );
  }

  return (
    <Flex direction="column" gap="4" className="km-page-admin-themes p-2 md:p-4">
      <Flex direction="column" gap="1">
        <Heading size="5">{t("theme.title", "主题")}</Heading>
        <Text color="gray">
          {t("theme.local_description", "选择已安装的本地主题。主题市场、远程下载和在线更新已关闭。")}
        </Text>
      </Flex>
      <Flex wrap="wrap" gap="4">
        {themes.map((theme) => {
          const active = theme.short === currentTheme;
          const preview = theme.preview ? `/themes/${theme.short}/${theme.preview}` : undefined;
          return (
            <Card key={theme.short} size="2" style={{ width: 320 }}>
              <Flex direction="column" gap="3">
                {preview && (
                  <img
                    src={preview}
                    alt={text(theme.name, language)}
                    style={{ width: "100%", height: 150, objectFit: "cover", borderRadius: 6 }}
                  />
                )}
                <Flex justify="between" align="start" gap="2">
                  <Flex direction="column" gap="1">
                    <Heading size="3">{text(theme.name, language) || theme.short}</Heading>
                    <Text size="2" color="gray">
                      {text(theme.description, language)}
                    </Text>
                  </Flex>
                  {active && <Badge color="green">{t("theme.active", "当前使用")}</Badge>}
                </Flex>
                <Text size="1" color="gray">
                  {theme.author ? `${t("theme.author", "作者")}: ${text(theme.author, language)}` : theme.short}
                  {theme.version ? ` · v${theme.version}` : ""}
                </Text>
                <Flex gap="2">
                  <Button disabled={active || switching !== null} onClick={() => void selectTheme(theme.short)}>
                    {switching === theme.short
                      ? t("common.loading", "切换中…")
                      : active
                        ? t("theme.active", "当前使用")
                        : t("theme.use", "使用此主题")}
                  </Button>
                  <Button variant="soft" onClick={() => navigate(`/admin/themes/settings?theme=${encodeURIComponent(theme.short)}`)}>
                    {t("theme.settings", "配置")}
                  </Button>
                </Flex>
              </Flex>
            </Card>
          );
        })}
      </Flex>
    </Flex>
  );
}
