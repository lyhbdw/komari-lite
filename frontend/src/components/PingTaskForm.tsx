import React from "react";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import { Button, Dialog, Select } from "@/components/ui/radix-shim";
import { useTranslation } from "react-i18next";

export type PingTaskType = "icmp" | "tcp" | "http";

export interface PingTaskFormValue {
  name: string;
  type: PingTaskType;
  target: string;
  clients: string[];
  default_on: boolean;
  interval: number;
}

interface PingTaskFormProps {
  value: PingTaskFormValue;
  onChange: React.Dispatch<React.SetStateAction<PingTaskFormValue>>;
  onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
  submitting?: boolean;
  submitLabel: string;
  nameId: string;
  typeId: string;
  intervalId: string;
  targetId: string;
  defaultOnId: string;
  onCancel?: () => void;
}

export const PingTaskForm: React.FC<PingTaskFormProps> = ({
  value,
  onChange,
  onSubmit,
  submitting = false,
  submitLabel,
  nameId,
  typeId,
  intervalId,
  targetId,
  defaultOnId,
  onCancel,
}) => {
  const { t } = useTranslation();
  const setValue = <K extends keyof PingTaskFormValue>(
    key: K,
    nextValue: PingTaskFormValue[K],
  ) => onChange((current) => ({ ...current, [key]: nextValue }));

  return (
    <form onSubmit={onSubmit} className="grid gap-3.5 my-0.5 text-xs">
      <div className="grid gap-1.5">
        <div className="flex items-center h-5">
          <label htmlFor={nameId} className="text-xs font-medium leading-none text-foreground">
            任务名称
          </label>
        </div>
        <input
          id={nameId}
          type="text"
          value={value.name}
          onChange={(event) => setValue("name", event.target.value)}
          placeholder="例如：江苏电信v4、Cloudflare Anycast"
          required
          autoFocus
          className="w-full h-9 px-3 text-sm rounded-lg border border-input bg-transparent text-foreground outline-none focus-visible:ring-1 focus-visible:ring-ring shadow-xs"
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-1.5">
          <div className="flex items-center h-5">
            <label htmlFor={typeId} className="text-xs font-medium leading-none text-foreground">协议类型</label>
          </div>
          <Select.Root value={value.type} onValueChange={(type) => setValue("type", type as PingTaskType)}>
            <Select.Trigger id={typeId} className="w-full h-9 cursor-pointer text-sm" />
            <Select.Content>
              <Select.Item value="tcp">TCP (端口握手)</Select.Item>
              <Select.Item value="icmp">ICMP (Ping 报文)</Select.Item>
              <Select.Item value="http">HTTP (网页探测)</Select.Item>
            </Select.Content>
          </Select.Root>
        </div>
        <div className="grid gap-1.5">
          <div className="flex items-center h-5">
            <label htmlFor={intervalId} className="text-xs font-medium leading-none text-foreground">探测频率 (秒/次)</label>
          </div>
          <input
            id={intervalId}
            type="number"
            value={value.interval}
            onChange={(event) => setValue("interval", Number(event.target.value))}
            min={1}
            required
            className="w-full h-9 px-3 text-sm rounded-lg border border-input bg-transparent text-foreground outline-none focus-visible:ring-1 focus-visible:ring-ring shadow-xs font-mono"
          />
        </div>
      </div>

      <div className="grid gap-1.5">
        <div className="flex items-center h-5">
          <label htmlFor={targetId} className="text-xs font-medium leading-none text-foreground">目标地址 (Target)</label>
        </div>
        <input
          id={targetId}
          type="text"
          value={value.target}
          onChange={(event) => setValue("target", event.target.value)}
          placeholder="1.1.1.1:80 或 example.com:443 或 http://..."
          required
          className="w-full h-9 px-3 text-sm rounded-lg border border-input bg-transparent text-foreground outline-none focus-visible:ring-1 focus-visible:ring-ring shadow-xs font-mono"
        />
        <p className="text-[11px] text-muted-foreground leading-relaxed">
          TCP 需包含端口（如 1.1.1.1:80）；ICMP 填写 IP 或纯域名；HTTP 需带完整 URL。
        </p>
      </div>

      <div className="p-3 rounded-lg border border-border/60 bg-muted/20 space-y-2.5">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-foreground">执行节点范围</span>
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground font-mono">已选 {value.clients.length} 台</span>
            <NodeSelectorDialog value={value.clients} onChange={(clients) => setValue("clients", clients)} />
          </div>
        </div>
        <div className="pt-2 border-t border-border/40 flex items-start gap-2">
          <input
            type="checkbox"
            id={defaultOnId}
            checked={value.default_on}
            onChange={(event) => setValue("default_on", event.target.checked)}
            className="size-4 mt-0.5 cursor-pointer accent-foreground"
          />
          <div className="flex flex-col">
            <label htmlFor={defaultOnId} className="text-xs font-medium text-foreground cursor-pointer">
              {t("ping.default_on", "默认全网开启")}
            </label>
            <span className="text-[11px] text-muted-foreground leading-relaxed">
              {t("ping.default_on_description", "所有现有服务器及新加入节点均会自动执行该探测任务。")}
            </span>
          </div>
        </div>
      </div>

      <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
        {onCancel && (
          <Dialog.Close asChild>
            <Button variant="outline" type="button" onClick={onCancel} className="cursor-pointer">
              {t("common.cancel", "取消")}
            </Button>
          </Dialog.Close>
        )}
        <Button disabled={submitting} type="submit" className="cursor-pointer">
          {submitting ? t("common.saving", "保存中...") : submitLabel}
        </Button>
      </Dialog.Footer>
    </form>
  );
};
