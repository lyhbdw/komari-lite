import { quoteShellArgs } from "@/utils/shellQuote";
import { buildAgentInstallArgs } from "@/utils/agentInstallCommand";
import { copyToClipboard } from "@/utils/clipboard";
import React from "react";
import { useState } from "react";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import type { NodeDetail } from "@/contexts/node-details-context";
import { TextField, Button, Checkbox, Dialog, IconButton } from "@/components/ui/radix-shim";
import { Copy, Download, Trash2Icon, AlertTriangle, CalendarCheck, Layers } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { calculateNextRenewalDate } from "../../../../../theme/src/utils/billingCore";

function BatchGroupDialog({
  open,
  onOpenChange,
  selectedCount,
  onConfirm,
  availableGroups,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedCount: number;
  onConfirm: (group: string) => Promise<void>;
  availableGroups: string[];
}) {
  const { t } = useTranslation();
  const [group, setGroup] = useState("");
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    try {
      setSaving(true);
      await onConfirm(group.trim());
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <Layers size={16} className="text-muted-foreground shrink-0" />
            <span>批量设置分组</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            将选中的 <strong className="text-foreground">{selectedCount}</strong> 台服务器统一归入指定分组。
          </Dialog.Description>
        </Dialog.Header>
        <div className="space-y-3 my-0.5">
          <div className="grid gap-1.5">
            <label className="text-xs font-medium leading-none text-foreground">
              分组名称
            </label>
            <TextField.Root
              value={group}
              onChange={(e) => setGroup(e.target.value)}
              placeholder="输入新分组名称或快速选择已有"
              autoFocus
              className="h-9 text-sm"
            />
          </div>
          {availableGroups.length > 0 && (
            <div className="flex flex-wrap gap-1.5 items-center pt-1">
              <span className="text-[11px] text-muted-foreground">快速选择:</span>
              {availableGroups.map((g) => (
                <button
                  key={g}
                  type="button"
                  onClick={() => setGroup(g)}
                  className={`px-2 py-0.5 text-xs rounded-md border transition-colors cursor-pointer ${
                    group === g
                      ? "border-foreground bg-foreground text-background font-medium"
                      : "border-border/60 bg-muted/40 hover:bg-muted text-foreground"
                  }`}
                >
                  {g}
                </button>
              ))}
            </div>
          )}
        </div>
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button" disabled={saving}>
              {t("common.cancel", "取消")}
            </Button>
          </Dialog.Close>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? "保存中..." : t("common.save", "保存")}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
}


function BatchDeleteDialog({
  open,
  onOpenChange,
  selectedCount,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedCount: number;
  onConfirm: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [deleting, setDeleting] = useState(false);

  const handleDelete = async () => {
    try {
      setDeleting(true);
      await onConfirm();
      onOpenChange(false);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="text-destructive flex items-center gap-2 text-base font-semibold leading-none tracking-tight">
            <AlertTriangle size={17} className="shrink-0 text-destructive" />
            <span>确认批量删除</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            确定要继续删除选中的 <strong className="text-foreground">{selectedCount}</strong> 台服务器节点吗？
          </Dialog.Description>
        </Dialog.Header>
        <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs leading-relaxed my-0.5">
          警告：此操作不可撤销！将同时永久删除选中的 <strong>{selectedCount}</strong> 台服务器及其监控历史数据。
        </div>
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button" disabled={deleting}>
              {t("common.cancel", "取消")}
            </Button>
          </Dialog.Close>
          <Button onClick={handleDelete} disabled={deleting} color="red">
            {deleting ? "删除中..." : `确认删除 (${selectedCount}台)`}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
}


function RenewDialog({
  node,
  open,
  onOpenChange,
}: {
  node: NodeDetail;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [loading, setLoading] = useState(false);

  const cycleDays = Number(node.billing_cycle) > 0 ? Number(node.billing_cycle) : 30;
  const newExpiry = calculateNextRenewalDate(node.expired_at, cycleDays);
  const currentDateStr = node.expired_at
    ? new Date(node.expired_at).toLocaleDateString()
    : "未设置";
  const newDateStr = newExpiry.toLocaleDateString();

  const handleRenew = async () => {
    try {
      setLoading(true);
      const res = await fetch(`/api/admin/client/${node.uuid}/edit`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expired_at: newExpiry.toISOString(),
        }),
      });
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      toast.success(
        t("admin.nodeTable.renewSuccess", {
          defaultValue: `已成功续费！到期时间顺延至：${newDateStr} (+${cycleDays}天)`,
          date: newDateStr,
          days: cycleDays,
        })
      );
      onOpenChange(false);
      refresh();
    } catch (err) {
      toast.error(
        `${t("common.error", "Error")}: ${
          err instanceof Error ? err.message : String(err)
        }`
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <CalendarCheck size={16} className="text-emerald-600 dark:text-emerald-400 shrink-0" />
            <span>{t("admin.nodeTable.renewTitle", "确认节点已续费")}</span>
            <span className="font-mono text-xs text-muted-foreground font-normal ml-1">
              · {node.name}
            </span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            确认已在服务商处完成该机器的续费？系统将按该节点已配置的计费周期自动顺延时长。
          </Dialog.Description>
        </Dialog.Header>
        <div className="rounded-lg border border-border/80 bg-muted/40 p-3 space-y-2 text-xs my-0.5">
          <div className="flex justify-between items-center">
            <span className="text-muted-foreground">服务器名称:</span>
            <span className="font-semibold text-foreground">{node.name}</span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-muted-foreground">计费周期:</span>
            <span className="font-mono text-foreground">{cycleDays} 天</span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-muted-foreground">当前到期日:</span>
            <span className="font-mono text-foreground">{currentDateStr}</span>
          </div>
          <div className="pt-2 border-t border-border/60 flex justify-between items-center">
            <span className="font-medium text-foreground">续费后新到期日:</span>
            <span className="font-mono font-bold text-emerald-600 dark:text-emerald-400 text-sm">
              {newDateStr}
            </span>
          </div>
        </div>
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button" disabled={loading}>
              {t("common.cancel", "取消")}
            </Button>
          </Dialog.Close>
          <Button onClick={handleRenew} disabled={loading} color="green">
            {loading ? t("common.saving", "更新中...") : t("admin.nodeTable.confirmRenew", "确认续费")}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
}


function DeleteButton({
  node,
  trigger,
  open: controlledOpen,
  onOpenChange,
}: {
  node: NodeDetail;
  trigger?: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [internalOpen, setInternalOpen] = React.useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : internalOpen;
  const setOpen = (o: boolean) => {
    if (isControlled) {
      onOpenChange?.(o);
    } else {
      setInternalOpen(o);
    }
  };
  const [deleting, setDeleting] = React.useState(false);
  const handleDelete = async () => {
    try {
      setDeleting(true);
      await fetch(`/api/admin/client/${node.uuid}/remove`, {
        method: "POST",
      });
      toast.success(`已删除节点：${node.name}`);
      setOpen(false);
      refresh();
    } catch (error) {
      toast.error(
        `Error: ${error instanceof Error ? error.message : String(error)}`
      );
    } finally {
      setDeleting(false);
    }
  };
  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      {trigger !== null && (
        <Dialog.Trigger asChild>
          {trigger !== undefined ? (
            trigger
          ) : (
            <IconButton variant="ghost" color="red" title={t("common.delete")}>
              <Trash2Icon size="18" />
            </IconButton>
          )}
        </Dialog.Trigger>
      )}
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="text-destructive flex items-center gap-2 text-base font-semibold leading-none tracking-tight">
            <AlertTriangle size={17} className="shrink-0 text-destructive" />
            <span>{t("common.delete", "删除服务器节点")}</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            {t("common.confirm_delete", "确认删除节点：")} <strong className="text-foreground font-mono">{node.name}</strong>？
          </Dialog.Description>
        </Dialog.Header>
        <div className="p-3 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive text-xs leading-relaxed my-0.5">
          {t(
            "admin.nodeTable.deleteWarning",
            "警告：此操作不可撤销！删除后该节点将立即失去连接，历史监控数据将被清除。"
          )}
        </div>
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button" disabled={deleting}>
              {t("common.cancel", "取消")}
            </Button>
          </Dialog.Close>
          <Button disabled={deleting} color="red" onClick={handleDelete}>
            {deleting ? t("common.deleting", "删除中...") : t("common.confirm_delete", "确认删除")}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
}

type InstallOptions = {
  includeNics: string;
  excludeNics: string;
  interval: string;
};
function GenerateCommandButton({
  settings,
  nodeToken,
  open,
  onOpenChange,
  trigger,
}: {
  settings: any;
  nodeToken: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  trigger?: React.ReactNode;
}) {
  const [installOptions, setInstallOptions] = React.useState<InstallOptions>({
    includeNics: "",
    excludeNics: "",
    interval: "",
  });

  const [enableIncludeNics, setEnableIncludeNics] = React.useState(false);
  const [enableExcludeNics, setEnableExcludeNics] = React.useState(false);
  const [enableInterval, setEnableInterval] = React.useState(false);
  const generateCommand = () => {
    const host = function () {
      if (!settings.script_domain) {
        return window.location.origin;
      }
      if (settings.script_domain.startsWith("http")) {
        return settings.script_domain.replace(/\/+$/, "");
      }
      return `http://${settings.script_domain.replace(/\/+$/, "")}`;
    }();
    const hasCustomOptions = enableInterval || enableIncludeNics || enableExcludeNics;
    if (!hasCustomOptions) {
      return `curl -sL ${host}/i/${nodeToken} | bash`;
    }
    const extraArgs = buildAgentInstallArgs({
      interval: enableInterval ? installOptions.interval : undefined,
      includeNics: enableIncludeNics ? installOptions.includeNics : undefined,
      excludeNics: enableExcludeNics ? installOptions.excludeNics : undefined,
    });
    return `curl -sL ${host}/i/${nodeToken} | bash -s -- ${quoteShellArgs(extraArgs)}`;
  };

  const copyToClipboardHandler = async (text: string) => {
    await copyToClipboard(text, {
      successMessage: t("copy_success", "已复制到剪贴板"),
    });
  };
  const { t } = useTranslation();
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      {trigger !== null && (
        <Dialog.Trigger asChild>
          {trigger !== undefined ? (
            trigger
          ) : (
            <button
              type="button"
              className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
              title={t("admin.nodeTable.installCommand", "安装指令")}
            >
              <Download size={13} />
            </button>
          )}
        </Dialog.Trigger>
      )}
      <Dialog.Content className="sm:max-w-lg p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <Download size={16} className="text-muted-foreground shrink-0" />
            <span>{t("admin.nodeTable.installCommand", "一键部署指令")}</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            配置客户端采集参数并生成跨平台一键部署脚本。
          </Dialog.Description>
        </Dialog.Header>
        <div className="flex flex-col gap-3.5 my-0.5">
          <div>
            <span className="text-xs font-medium text-foreground block mb-2">
              {t("admin.nodeTable.installOptions", "安装选项")}
            </span>
            <div className="space-y-2.5 rounded-lg border border-border/70 bg-muted/20 p-3">
              <div className="flex flex-col gap-1.5">
                <label className="flex items-center gap-2 cursor-pointer select-none text-xs font-medium text-foreground">
                  <Checkbox
                    checked={enableIncludeNics}
                    onCheckedChange={(checked) => {
                      setEnableIncludeNics(Boolean(checked));
                      if (!checked) {
                        setInstallOptions((prev) => ({
                          ...prev,
                          includeNics: "",
                        }));
                      }
                    }}
                  />
                  <span>{t("admin.nodeTable.includeNics", "只监测特定网卡")}</span>
                </label>
                {enableIncludeNics && (
                  <div className="pl-6">
                    <TextField.Root
                      placeholder="eth0,eth1"
                      value={installOptions.includeNics}
                      onChange={(e) =>
                        setInstallOptions((prev) => ({
                          ...prev,
                          includeNics: e.target.value,
                        }))
                      }
                      className="h-8 text-xs"
                    />
                  </div>
                )}
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="flex items-center gap-2 cursor-pointer select-none text-xs font-medium text-foreground">
                  <Checkbox
                    checked={enableExcludeNics}
                    onCheckedChange={(checked) => {
                      setEnableExcludeNics(Boolean(checked));
                      if (!checked) {
                        setInstallOptions((prev) => ({
                          ...prev,
                          excludeNics: "",
                        }));
                      }
                    }}
                  />
                  <span>{t("admin.nodeTable.excludeNics", "排除特定网卡")}</span>
                </label>
                {enableExcludeNics && (
                  <div className="pl-6">
                    <TextField.Root
                      placeholder="lo"
                      value={installOptions.excludeNics}
                      onChange={(e) =>
                        setInstallOptions((prev) => ({
                          ...prev,
                          excludeNics: e.target.value,
                        }))
                      }
                      className="h-8 text-xs"
                    />
                  </div>
                )}
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="flex items-center gap-2 cursor-pointer select-none text-xs font-medium text-foreground">
                  <Checkbox
                    checked={enableInterval}
                    onCheckedChange={(checked) => {
                      const enabled = Boolean(checked);
                      setEnableInterval(enabled);
                      setInstallOptions((prev) => ({
                        ...prev,
                        interval: !enabled ? "" : prev.interval?.trim() ? prev.interval : "1",
                      }));
                    }}
                  />
                  <span>{t("admin.nodeTable.interval", "采集间隔(秒)")}</span>
                </label>
                {enableInterval && (
                  <div className="pl-6">
                    <TextField.Root
                      placeholder="1"
                      type="number"
                      min="1"
                      step="0.1"
                      value={installOptions.interval}
                      onChange={(e) =>
                        setInstallOptions((prev) => ({
                          ...prev,
                          interval: e.target.value,
                        }))
                      }
                      className="h-8 text-xs"
                    />
                  </div>
                )}
              </div>
            </div>
          </div>

          <div>
            <span className="text-xs font-medium text-foreground block mb-1.5">
              {t("admin.nodeTable.generatedCommand", "生成的指令")}
            </span>
            <div className="rounded-lg border border-border/70 bg-muted/40 p-3 font-mono text-[11px] leading-relaxed break-all select-all text-foreground max-h-36 overflow-y-auto">
              {generateCommand()}
            </div>
          </div>
        </div>
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button">
              {t("common.close", "关闭")}
            </Button>
          </Dialog.Close>
          <Button onClick={() => copyToClipboardHandler(generateCommand())}>
            <Copy size={13} />
            <span>{t("common.copy", "复制指令")}</span>
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
}


export { BatchGroupDialog, BatchDeleteDialog, RenewDialog, DeleteButton, GenerateCommandButton };
