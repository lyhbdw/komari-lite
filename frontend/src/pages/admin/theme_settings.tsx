import { ArrowLeft } from "lucide-react";
import {
  Button,
  Callout,
  Checkbox,
  Flex,
  Heading,
  Select,
  Text,
  TextArea,
  TextField,
} from "@radix-ui/themes";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import Loading from "@/components/loading";
import { usePublicInfo } from "@/contexts/usePublicInfo";

type LocalizedText = string | Record<string, string>;

type ThemeField = {
  key?: string;
  name?: LocalizedText;
  help?: LocalizedText;
  type?: string;
  default?: unknown;
  options?: string;
};

function resolveText(value: LocalizedText | undefined, language: string) {
  if (typeof value === "string") return value;
  if (!value) return "";
  return value[language] ?? value[language.split(/[-_]/)[0]] ?? Object.values(value)[0] ?? "";
}

export default function ThemeSettingsPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { publicInfo, refresh } = usePublicInfo();
  const requestedTheme = new URLSearchParams(location.search).get("theme");
  const theme = requestedTheme || publicInfo?.theme || "Emerald";
  const language = i18n.resolvedLanguage || i18n.language || "en";
  const [fields, setFields] = useState<ThemeField[]>([]);
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const response = await fetch(`/themes/${encodeURIComponent(theme)}/komari-theme.json`, {
          cache: "no-store",
        });
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const manifest = await response.json();
        const nextFields = Array.isArray(manifest.configuration?.data)
          ? (manifest.configuration.data as ThemeField[])
          : [];
        const saved = theme === publicInfo?.theme ? publicInfo?.theme_settings ?? {} : {};
        const nextValues: Record<string, unknown> = {};
        for (const field of nextFields) {
          if (!field.key || field.type === "title") continue;
          nextValues[field.key] = saved[field.key] ?? field.default ?? "";
        }
        if (!cancelled) {
          setFields(nextFields);
          setValues(nextValues);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : t("theme.load_config_failed", "加载主题配置失败"));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [theme, publicInfo?.theme, publicInfo?.theme_settings, t]);

  const title = useMemo(() => {
    const selected = fields.find((field) => field.type === "title");
    return selected ? resolveText(selected.name, language) : theme;
  }, [fields, language, theme]);

  const updateValue = (key: string, value: unknown) => {
    setValues((current) => ({ ...current, [key]: value }));
  };

  const save = async () => {
    setSaving(true);
    try {
      const response = await fetch(`/api/admin/theme/settings?theme=${encodeURIComponent(theme)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(values),
      });
      const payload = await response.json();
      if (!response.ok || payload.status === "error") {
        throw new Error(payload.message || `HTTP ${response.status}`);
      }
      await refresh();
      toast.success(t("settings.settings_saved", "主题设置已保存"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.settings_save_failed", "主题设置保存失败"));
    } finally {
      setSaving(false);
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
    <Flex direction="column" gap="4" className="km-page-admin-theme-settings max-w-4xl">
      <Flex align="center" gap="3">
        <Button variant="soft" onClick={() => navigate("/admin/themes")}>
          <ArrowLeft size={16} />
          {t("common.back", "返回")}
        </Button>
        <Flex direction="column" gap="1">
          <Heading size="5">{t("theme.settings", "主题配置")}</Heading>
          <Text color="gray">{title || theme}</Text>
        </Flex>
      </Flex>

      {fields.length === 0 ? (
        <Callout.Root>
          <Callout.Text>{t("theme.no_config", "当前主题没有可配置项")}</Callout.Text>
        </Callout.Root>
      ) : (
        <Flex direction="column" gap="4" className="max-w-3xl">
          {fields.map((field, index) => {
            if (field.type === "title") {
              return <Heading key={`${field.key ?? "title"}-${index}`} size="3">{resolveText(field.name, language)}</Heading>;
            }
            if (!field.key) return null;
            const label = resolveText(field.name, language) || field.key;
            const help = resolveText(field.help, language);
            const value = values[field.key];
            if (field.type === "switch") {
              return (
                <Flex key={field.key} direction="column" gap="1">
                  <Flex align="center" gap="2">
                    <Checkbox checked={Boolean(value)} onCheckedChange={(checked) => updateValue(field.key!, checked)} />
                    <Text weight="medium">{label}</Text>
                  </Flex>
                  {help && <Text size="2" color="gray">{help}</Text>}
                </Flex>
              );
            }
            if (field.type === "select") {
              const options = (field.options ?? "").split(",").map((option) => option.trim()).filter(Boolean);
              return (
                <Flex key={field.key} direction="column" gap="1">
                  <Text weight="medium">{label}</Text>
                  <Select.Root value={String(value ?? "")} onValueChange={(next) => updateValue(field.key!, next)}>
                    <Select.Trigger />
                    <Select.Content>
                      {options.map((option) => <Select.Item key={option} value={option}>{option}</Select.Item>)}
                    </Select.Content>
                  </Select.Root>
                  {help && <Text size="2" color="gray">{help}</Text>}
                </Flex>
              );
            }
            if (field.type === "richtext") {
              return (
                <Flex key={field.key} direction="column" gap="1">
                  <Text weight="medium">{label}</Text>
                  <TextArea value={String(value ?? "")} onChange={(event) => updateValue(field.key!, event.target.value)} />
                  {help && <Text size="2" color="gray">{help}</Text>}
                </Flex>
              );
            }
            return (
              <Flex key={field.key} direction="column" gap="1">
                <Text weight="medium">{label}</Text>
                <TextField.Root
                  type={field.type === "number" ? "number" : "text"}
                  value={String(value ?? "")}
                  onChange={(event) => updateValue(field.key!, field.type === "number" ? Number(event.target.value) : event.target.value)}
                />
                {help && <Text size="2" color="gray">{help}</Text>}
              </Flex>
            );
          })}
          <Flex>
            <Button onClick={() => void save()} disabled={saving}>
              {saving ? t("common.saving", "保存中…") : t("common.save", "保存")}
            </Button>
          </Flex>
        </Flex>
      )}
    </Flex>
  );
}
