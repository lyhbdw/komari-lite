import { copyToClipboard } from "@/utils/clipboard";
import React from "react";
import { useState } from "react";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import type { NodeDetail } from "@/contexts/node-details-context";
import { TextField, Button, Dialog, TextArea, Select } from "@/components/ui/radix-shim";
import { Copy, Pencil } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { formatBytes, stringToBytes } from "@/utils/unitHelper";
import Tips from "@/components/ui/tips";

function EditButton({ node, open: controlledOpen, onOpenChange }: { node: NodeDetail; open?: boolean; onOpenChange?: (open: boolean) => void }) {
  const { t } = useTranslation();
  const [internalOpen, setInternalOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const setOpen = onOpenChange ?? setInternalOpen;
  const { refresh } = useNodeDetails();
  const nameRef = React.useRef<HTMLInputElement>(null);
  const groupRef = React.useRef<HTMLInputElement>(null);
  const tagsRef = React.useRef<HTMLInputElement>(null);
  const publicRemarkRef = React.useRef<HTMLTextAreaElement>(null);
  const [saving, setSaving] = useState(false);
  const [traffic_limit, setTrafficLimit] = useState(0);
  const [traffic_limit_type, setTrafficLimitType] = useState("sum");

  React.useEffect(() => {
    if (open) {
      setTrafficLimit(node.traffic_limit || 0);
      setTrafficLimitType(node.traffic_limit_type || "sum");
    }
  }, [node.traffic_limit, node.traffic_limit_type, open]);

  const save = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    try {
      setSaving(true);
      await fetch(`/api/admin/client/${node.uuid}/edit`, {
        method: "POST",
        body: JSON.stringify({
          name: nameRef.current?.value,
          public_remark: publicRemarkRef.current?.value,
          group: groupRef.current?.value,
          tags: tagsRef.current?.value,
          traffic_limit,
          traffic_limit_type,
        }),
        headers: {
          "Content-Type": "application/json",
        },
      });
      refresh();
      setOpen(false);
      toast.success(t("admin.nodeEdit.saveSuccess", "保存成功"));
    } catch (error) {
      console.error("Error updating client:", error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      {controlledOpen === undefined && (
        <Dialog.Trigger asChild>
          <button
            type="button"
            className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
            title={t("admin.nodeEdit.editInfo", "编辑信息")}
          >
            <Pencil size={13} />
          </button>
        </Dialog.Trigger>
      )}
      <Dialog.Content className="sm:max-w-[460px] p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <Pencil size={15} className="text-muted-foreground shrink-0" />
            <span>{t("admin.nodeEdit.editInfo", "编辑信息")}</span>
            <span className="font-mono text-xs text-muted-foreground font-normal ml-1">
              · {node.name}
            </span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            {t(
              "admin.nodeEdit.editDescription",
              "修改服务器名称、Token、分组标签与流量统计规则。"
            )}
          </Dialog.Description>
        </Dialog.Header>

        <form onSubmit={save} className="grid gap-3.5 my-0.5">
          {/* 1. 服务器名称 */}
          <div className="grid gap-1.5">
            <div className="flex items-center justify-between h-5">
              <label
                htmlFor={`edit-name-${node.uuid}`}
                className="text-xs font-medium leading-none text-foreground"
              >
                {t("admin.nodeEdit.name", "名称")}
              </label>
            </div>
            <TextField.Root
              id={`edit-name-${node.uuid}`}
              defaultValue={node.name}
              placeholder={t("admin.nodeEdit.namePlaceholder", "请输入名称")}
              ref={nameRef}
              className="h-9 text-sm"
            />
          </div>

          {/* 2. Token 令牌 */}
          <div className="grid gap-1.5">
            <div className="flex items-center justify-between h-5">
              <label
                htmlFor={`edit-token-${node.uuid}`}
                className="text-xs font-medium leading-none text-foreground"
              >
                {t("admin.nodeEdit.token", "Token 令牌")}
              </label>
              <span className="text-[11px] text-muted-foreground leading-none">
                {t("admin.nodeEdit.tokenReadOnly", "只读")}
              </span>
            </div>
            <TextField.Root
              id={`edit-token-${node.uuid}`}
              value={node.token}
              placeholder={t("admin.nodeEdit.tokenPlaceholder", "请输入 Token")}
              readOnly
              className="h-9 font-mono text-xs bg-muted/40 text-muted-foreground pr-1 selection:bg-primary/20"
            >
              <TextField.Slot side="right">
                <button
                  type="button"
                  onClick={() =>
                    copyToClipboard(node.token, {
                      successMessage: t("admin.nodeEdit.tokenCopied", "Token 已复制到剪贴板"),
                    })
                  }
                  className="h-6 px-2 rounded-md border border-border/60 bg-background hover:bg-muted text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-[11px] font-sans transition-colors cursor-pointer shadow-2xs"
                  title={t("common.copy", "复制")}
                >
                  <Copy size={11} />
                  <span>{t("common.copy", "复制")}</span>
                </button>
              </TextField.Slot>
            </TextField.Root>
          </div>

          {/* 3. 标签与分组 (水平基线完全对齐) */}
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label
                  htmlFor={`edit-tags-${node.uuid}`}
                  className="text-xs font-medium leading-none text-foreground"
                >
                  {t("common.tags", "标签")}
                </label>
                <Tips size="13">
                  <span
                    dangerouslySetInnerHTML={{ __html: t("common.tagsTips") }}
                  />
                </Tips>
              </div>
              <TextField.Root
                id={`edit-tags-${node.uuid}`}
                defaultValue={node.tags}
                ref={tagsRef}
                placeholder="tag1,tag2"
                className="h-9 text-sm"
              />
            </div>
            <div className="grid gap-1.5">
              <div className="flex items-center h-5">
                <label
                  htmlFor={`edit-group-${node.uuid}`}
                  className="text-xs font-medium leading-none text-foreground"
                >
                  {t("common.group", "分组")}
                </label>
              </div>
              <TextField.Root
                id={`edit-group-${node.uuid}`}
                defaultValue={node.group}
                ref={groupRef}
                placeholder="default"
                className="h-9 text-sm"
              />
            </div>
          </div>

          {/* 4. 公开备注 */}
          <div className="grid gap-1.5">
            <div className="flex items-center justify-between h-5">
              <label
                htmlFor={`edit-remark-${node.uuid}`}
                className="text-xs font-medium leading-none text-foreground"
              >
                {t("admin.nodeEdit.publicRemark", "公开备注")}
              </label>
            </div>
            <TextArea
              id={`edit-remark-${node.uuid}`}
              defaultValue={node.public_remark}
              className="min-h-[68px] max-h-32 text-xs leading-relaxed resize-y"
              placeholder={t(
                "admin.nodeEdit.publicRemarkPlaceholder",
                "请输入公开备注"
              )}
              ref={publicRemarkRef}
            />
          </div>

          {/* 5. 流量阈值与统计方式 */}
          <div className="pt-3 border-t border-border/60 grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center h-5">
                <label
                  htmlFor={`edit-traffic-type-${node.uuid}`}
                  className="text-xs font-medium leading-none text-foreground"
                >
                  {t("admin.nodeEdit.trafficLimitType", "统计方式")}
                </label>
              </div>
              <Select.Root
                value={traffic_limit_type}
                onValueChange={setTrafficLimitType}
              >
                <Select.Trigger
                  id={`edit-traffic-type-${node.uuid}`}
                  className="w-full h-9 text-sm"
                />
                <Select.Content>
                  <Select.Item value="sum">
                    {t("admin.nodeEdit.trafficLimitType_sum", "总和")}
                  </Select.Item>
                  <Select.Item value="max">
                    {t("admin.nodeEdit.trafficLimitType_max", "取最大")}
                  </Select.Item>
                  <Select.Item value="min">
                    {t("admin.nodeEdit.trafficLimitType_min", "取最小")}
                  </Select.Item>
                  <Select.Item value="up">
                    {t("admin.nodeEdit.trafficLimitType_up", "仅上传")}
                  </Select.Item>
                  <Select.Item value="down">
                    {t("admin.nodeEdit.trafficLimitType_down", "仅下载")}
                  </Select.Item>
                </Select.Content>
              </Select.Root>
            </div>
            <div className="grid gap-1.5">
              <div className="flex items-center justify-between h-5">
                <label
                  htmlFor={`edit-traffic-limit-${node.uuid}`}
                  className="text-xs font-medium leading-none text-foreground"
                >
                  {t("admin.nodeEdit.trafficLimit", "流量阈值")}
                </label>
                <span className="font-mono text-[11px] text-muted-foreground leading-none">
                  {traffic_limit > 0 ? formatBytes(traffic_limit) : "0 B"}
                </span>
              </div>
              <TextField.Root
                id={`edit-traffic-limit-${node.uuid}`}
                defaultValue={traffic_limit > 0 ? formatBytes(traffic_limit) : ""}
                placeholder="1 TB (0 为不限制)"
                className="h-9 text-sm"
                onChange={(e) => {
                  setTrafficLimit(stringToBytes(e.currentTarget.value));
                }}
                onBlur={(e) => {
                  if (traffic_limit > 0) {
                    e.currentTarget.value = formatBytes(traffic_limit);
                  } else if (!e.currentTarget.value.trim() || e.currentTarget.value === "0") {
                    e.currentTarget.value = "";
                  }
                }}
              />
            </div>
          </div>

          <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
            <Dialog.Close asChild>
              <Button variant="outline" type="button" disabled={saving}>
                {t("common.cancel", "取消")}
              </Button>
            </Dialog.Close>
            <Button type="submit" disabled={saving}>
              {saving
                ? t("admin.nodeEdit.waiting", "等待...")
                : t("common.save", "保存")}
            </Button>
          </Dialog.Footer>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}


export { EditButton };
