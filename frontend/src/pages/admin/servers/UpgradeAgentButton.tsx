import { useState } from "react";
import { Button, Dialog } from "@/components/ui/radix-shim";
import { ArrowUpCircle } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

const UpgradeAgentButton = () => {
  const { t } = useTranslation();
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeVersion, setUpgradeVersion] = useState<string>("");
  const [upgrading, setUpgrading] = useState(false);

  const openUpgradeDialog = async () => {
    setUpgradeOpen(true);
    setUpgradeVersion("");
    try {
      const res = await fetch("/api/admin/agent-asset-version");
      const data = await res.json();
      const ver = data?.data?.version || data?.result?.version || data?.version;
      if (typeof ver === "string" && ver) setUpgradeVersion(ver);
    } catch {
      // ignore
    }
  };

  const handleUpgradeAgents = async () => {
    setUpgrading(true);
    try {
      const res = await fetch("/api/admin/upgrade-agents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      });
      const data = await res.json().catch(() => null);
      if (!res.ok || data?.status === "error") {
        throw new Error(data?.message || `HTTP ${res.status}`);
      }
      const dispatched =
        data?.data?.dispatched ?? data?.result?.dispatched ?? data?.dispatched;
      const rawVersion =
        data?.data?.version ??
        data?.result?.version ??
        data?.version ??
        upgradeVersion;
      const formattedVersion = rawVersion
        ? rawVersion.startsWith("v")
          ? rawVersion
          : `v${rawVersion}`
        : "";
      toast.success(
        t("admin.nodeTable.upgradeSuccess", {
          defaultValue: `升级事件已下发（${formattedVersion}，${dispatched} 个节点）`,
          version: formattedVersion,
          dispatched,
        })
      );
      setUpgradeOpen(false);
    } catch (error) {
      toast.error(
        `${t("common.error", "Error")}: ${
          error instanceof Error ? error.message : String(error)
        }`
      );
    } finally {
      setUpgrading(false);
    }
  };

  return (
    <Dialog.Root open={upgradeOpen} onOpenChange={setUpgradeOpen}>
      <Dialog.Trigger asChild>
        <button
          type="button"
          onClick={() => openUpgradeDialog()}
          className="h-8 px-2.5 rounded-lg border border-border/70 bg-card hover:bg-muted/40 text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1.5 text-xs font-medium cursor-pointer shadow-2xs shrink-0"
          title={t("admin.nodeTable.upgradeAgents", "批量升级 Agent")}
        >
          <ArrowUpCircle size={13} />
          <span className="hidden sm:inline">升级 Agent</span>
        </button>
      </Dialog.Trigger>
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <ArrowUpCircle size={16} className="text-muted-foreground shrink-0" />
            <span>{t("admin.nodeTable.upgradeAgents", "批量升级 Agent")}</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            {t("admin.nodeTable.upgradeDescription", {
              defaultValue:
                "将向全部节点下发升级事件。Agent 会从面板下载新版本并自动完成替换与重启。",
              version: upgradeVersion,
            })}
          </Dialog.Description>
        </Dialog.Header>
        {upgradeVersion ? (
          <div className="p-3 rounded-lg border border-border/70 bg-muted/30 flex items-center justify-between text-xs">
            <span className="text-muted-foreground">{t("admin.nodeTable.upgradeTargetVersion", "目标版本")}</span>
            <span className="font-mono font-medium text-foreground">
              {upgradeVersion.startsWith("v") ? upgradeVersion : `v${upgradeVersion}`}
            </span>
          </div>
        ) : null}
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button" disabled={upgrading}>
              {t("common.cancel", "Cancel")}
            </Button>
          </Dialog.Close>
          <Button onClick={() => handleUpgradeAgents()} disabled={upgrading}>
            {upgrading
              ? t("admin.nodeTable.upgrading", "下发中...")
              : t("admin.nodeTable.upgradeConfirm", "确认升级")}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
};


export { UpgradeAgentButton };
