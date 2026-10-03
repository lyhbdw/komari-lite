import { useTranslation } from "react-i18next";
import { Text, Switch } from "@radix-ui/themes";
import { updateSettingsWithToast, useSettings } from "@/lib/api";
import {
  SettingCardCollapse,
  SettingCardLongTextInput,
} from "@/components/admin/SettingCard";
import { toast } from "sonner";
import Loading from "@/components/loading";
import React from "react";
import { renderProviderInputs } from "@/utils/renderProviders";
import { Send } from "lucide-react";

const NotificationSettings = () => {
  const { t } = useTranslation();
  const { settings, loading, error } = useSettings();
  const [messageDefs, setMessageDefs] = React.useState<any>({});
  const [messageList, setMessageList] = React.useState<string[]>([]);
  const [currentMessageSender, setCurrentMessageSender] = React.useState<string>("");
  const [messageValues, setMessageValues] = React.useState<any>({});
  const [messageLoading, setMessageLoading] = React.useState(false);
  const [messageError, setMessageError] = React.useState("");
  const [testing, setTesting] = React.useState(false);

  // 拉取所有 message sender 及字段定义
  React.useEffect(() => {
    if (loading) return;
    setMessageLoading(true);
    fetch("/api/admin/settings/message-sender")
      .then((res) => res.json())
      .then((data) => {
        if (data.status === "success" && data.data) {
          setMessageDefs(data.data);
          const senders = Object.keys(data.data);
          setMessageList(senders);
          const initialSender =
            settings.notification_method && senders.includes(settings.notification_method)
              ? settings.notification_method
              : "";
          setCurrentMessageSender(initialSender);
        } else {
          setMessageError(data.message || t("settings.notification.provider_fetch_failed"));
        }
      })
      .catch(() => setMessageError(t("settings.notification.provider_fetch_failed")))
      .finally(() => setMessageLoading(false));
  }, [loading, settings.notification_method, t]);

  // 拉取当前 message sender 的设置
  React.useEffect(() => {
    if (!currentMessageSender) return;
    setMessageLoading(true);
    fetch(`/api/admin/settings/message-sender?provider=${currentMessageSender}`)
      .then((res) => res.json())
      .then((data) => {
        if (data.status === "success" && data.data) {
          try {
            setMessageValues(JSON.parse(data.data.addition || "{}"));
          } catch {
            setMessageValues({});
          }
        } else {
          setMessageError(data.message || t("settings.notification.provider_settings_fetch_failed"));
        }
      })
      .catch(() => setMessageError(t("settings.notification.provider_settings_fetch_failed")))
      .finally(() => setMessageLoading(false));
  }, [currentMessageSender, t]);

  // 处理保存
  const handleMessageSave = async (values: any) => {
    setMessageLoading(true);
    setMessageError("");
    const body = {
      name: currentMessageSender,
      addition: JSON.stringify(values),
    };
    try {
      const res = await fetch("/api/admin/settings/message-sender", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (data.status !== "success") {
        throw new Error(data.message || t("common.error"));
      } else {
        setMessageValues(values);
      }
      toast.success(t("common.success"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    setMessageLoading(false);
  };

  const handleTestSend = async () => {
    setTesting(true);
    try {
      const res = await fetch("/api/admin/test/sendMessage", {
        method: "POST",
      });
      let data;
      try {
        data = await res.json();
      } catch {
        toast.error(t("common.error"));
        return;
      }
      if (data && data.message && data.code !== 200) {
        toast.error(data.message);
        return;
      }
      toast.success(t("common.success", "测试消息发送成功"));
    } catch (error) {
      toast.error(
        t("common.error") +
          ": " +
          (error instanceof Error ? error.message : String(error))
      );
    } finally {
      setTesting(false);
    }
  };

  if (loading || (!messageLoading && messageList.length === 0 && !messageError)) {
    return <Loading />;
  }
  if (error) {
    return <Text color="red">{error}</Text>;
  }
  if (messageError) {
    return <Text color="red">{messageError}</Text>;
  }

  return (
    <div className="space-y-4 max-w-4xl">
      {/* 1. 页面头部：紧凑标题与副文本 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("settings.notification.title", "通知渠道")}
            </h1>
            {currentMessageSender && (
              <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
                {currentMessageSender.charAt(0).toUpperCase() + currentMessageSender.slice(1)}
              </span>
            )}
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            配置警报通道，在服务器离线或临期时及时推送消息通知。
          </p>
        </div>
      </div>

      {/* 2. 核心控制总卡片：启用总开关 + 渠道选择下拉框（合并为单卡片） */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold text-foreground">
                {t("settings.notification.enable", "开启消息通知")}
              </span>
              <span
                className={`w-2 h-2 rounded-full ${
                  settings.notification_enabled ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                }`}
              />
            </div>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("settings.notification.enable_description", "全局控制所有节点的警报与通知推送。")}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs text-muted-foreground">
              {settings.notification_enabled ? "已开启" : "已停用"}
            </span>
            <Switch
              checked={settings.notification_enabled}
              onCheckedChange={async (checked) => {
                await updateSettingsWithToast({ notification_enabled: checked }, t);
              }}
            />
          </div>
        </div>

        <div className="h-px bg-border/50" />

        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <span className="text-sm font-semibold text-foreground">
              {t("settings.notification.method", "通知渠道")}
            </span>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("settings.notification.method_description", "选择接收警报通知的目标平台。")}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <select
              value={currentMessageSender}
              onChange={async (e) => {
                const val = e.target.value;
                if (val === currentMessageSender) return;
                await updateSettingsWithToast({ notification_method: val }, t);
                setCurrentMessageSender(val);
              }}
              className="h-8 px-3 text-xs rounded-lg border border-border bg-background text-foreground outline-none focus:border-foreground/50 cursor-pointer shadow-2xs font-medium"
            >
              {messageList
                .filter((sender) => sender !== "empty")
                .map((sender) => (
                  <option key={sender} value={sender}>
                    {sender.charAt(0).toUpperCase() + sender.slice(1)}
                  </option>
                ))}
            </select>
          </div>
        </div>
      </div>

      {/* 3. 渠道字段配置卡片 */}
      <div className="rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-4">
        <div className="flex items-center justify-between pb-2 border-b border-border/40">
          <div>
            <h2 className="text-sm font-semibold text-foreground">
              {t("settings.notification.provider_fields", "发送设置")}
            </h2>
            <p className="text-xs text-muted-foreground mt-0.5">
              {t("settings.notification.provider_fields_description", "详细配置当前推送渠道的连接凭证与接收端点。")}
            </p>
          </div>
          <button
            type="button"
            onClick={handleTestSend}
            disabled={testing || !settings.notification_enabled}
            className="h-8 px-3 rounded-lg border border-border bg-card text-foreground font-medium text-xs flex items-center gap-1.5 shadow-2xs hover:bg-muted active:scale-[0.98] transition-all cursor-pointer disabled:opacity-50"
            title="发送一条即时测试消息验证连通性"
          >
            <Send size={13} className={testing ? "animate-spin" : ""} />
            <span>{testing ? "测试中..." : "发送测试消息"}</span>
          </button>
        </div>

        {messageLoading ? (
          <Loading />
        ) : (
          renderProviderInputs({
            currentProvider: currentMessageSender,
            providerDefs: messageDefs,
            providerValues: messageValues,
            translationPrefix: `settings.notification.${currentMessageSender}`,
            title: "",
            description: "",
            setProviderValues: setMessageValues,
            handleSave: handleMessageSave,
            t,
          })
        )}
      </div>

      {/* 4. 消息模板：折叠收纳的高级选项 */}
      <SettingCardCollapse
        title={t("settings.notification.template", "消息通知模板")}
        description={t("settings.notification.template_description", "自定义通知消息的格式与占位符（高级配置）。")}
        defaultOpen={false}
      >
        <div className="pt-2">
          <SettingCardLongTextInput
            bordless
            title=""
            description=""
            defaultValue={settings.notification_template}
            OnSave={async (value) => {
              await updateSettingsWithToast({ notification_template: value }, t);
            }}
          />
        </div>
      </SettingCardCollapse>
    </div>
  );
};

export default NotificationSettings;
