import Loading from "@/components/loading";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import { NodeDetailsProvider } from "@/contexts/NodeDetailsContext";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { PingTaskProvider } from "@/contexts/PingTaskContext";
import { usePingTask } from "@/contexts/usePingTask";
import {
  Button,
  Dialog,
  Flex,
  Select,
} from "@/components/ui/radix-shim";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Activity, Plus, Radio, Server } from "lucide-react";
import { TaskView } from "./pingTask_Task";
import { ServerView } from "./pingTask_Server";

const PingTask = () => {
  return (
    <PingTaskProvider>
      <NodeDetailsProvider>
        <InnerLayout />
      </NodeDetailsProvider>
    </PingTaskProvider>
  );
};

const InnerLayout = () => {
  const { pingTasks, isLoading, error } = usePingTask();
  const { isLoading: nodeDetailLoading, error: nodeDetailError } =
    useNodeDetails();
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = React.useState<"task" | "server">("task");

  if (isLoading || nodeDetailLoading) {
    return <Loading />;
  }
  if (error || nodeDetailError) {
    return <div className="p-4 text-xs text-rose-500">{error || nodeDetailError}</div>;
  }

  return (
    <div className="space-y-4 km-page-admin-pingtask max-w-7xl">
      {/* 1. 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("ping.title", "网络探测")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {pingTasks?.length || 0} 个任务
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            配置 ICMP / TCP / HTTP 目标探测任务，监控各节点到网络端点的实时延迟与丢包率。
          </p>
        </div>
        <AddButton />
      </div>

      {/* 2. 视图切换 Pill Tabs */}
      <div className="flex items-center justify-between gap-3">
        <div className="inline-flex p-1 rounded-lg bg-muted/60 border border-border/60">
          <button
            type="button"
            onClick={() => setActiveTab("task")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all cursor-pointer ${
              activeTab === "task"
                ? "bg-card text-foreground shadow-2xs"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Radio size={13} />
            <span>{t("ping.task_view", "按任务视图")}</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("server")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all cursor-pointer ${
              activeTab === "server"
                ? "bg-card text-foreground shadow-2xs"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Server size={13} />
            <span>{t("ping.server_view", "按服务器视图")}</span>
          </button>
        </div>
      </div>

      {/* 3. 视图内容渲染 */}
      {activeTab === "task" ? (
        <TaskView pingTasks={pingTasks ?? []} />
      ) : (
        <ServerView pingTasks={pingTasks ?? []} />
      )}
    </div>
  );
};

const AddButton: React.FC = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);
  const [name, setName] = React.useState("");
  const [target, setTarget] = React.useState("");
  const [interval, setInterval] = React.useState(60);
  const [selectedType, setSelectedType] = React.useState<"icmp" | "tcp" | "http">("tcp");
  const [selected, setSelected] = React.useState<string[]>([]);
  const [defaultOn, setDefaultOn] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const { refresh } = usePingTask();

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!name.trim()) {
      toast.error("请输入任务名称");
      return;
    }
    if (!target.trim()) {
      toast.error("请输入探测目标");
      return;
    }
    if (!defaultOn && selected.length === 0) {
      toast.error(t("ping.default_on_description", "请至少勾选默认全网开启或指定执行节点"));
      return;
    }
    const payload = {
      name: name.trim(),
      type: selectedType,
      target: target.trim(),
      default_on: defaultOn,
      clients: selected,
      interval: interval || 60,
    };
    setSaving(true);
    fetch("/api/admin/ping/add", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    })
      .then(async (response) => {
        if (response.ok) {
          setIsOpen(false);
          setName("");
          setTarget("");
          setInterval(60);
          setSelected([]);
          setDefaultOn(true);
          setSelectedType("tcp");
          toast.success(t("common.success", "添加成功"));
        } else {
          const data = await response.json().catch(() => ({}));
          toast.error(data?.message || t("common.error", "添加失败"));
        }
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setSaving(false);
        refresh();
      });
  };

  return (
    <Dialog.Root open={isOpen} onOpenChange={setIsOpen}>
      <Dialog.Trigger>
        <button
          onClick={() => setIsOpen(true)}
          className="h-8 px-3.5 rounded-lg bg-foreground text-background font-medium text-xs flex items-center gap-1.5 shadow-2xs hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shrink-0"
        >
          <Plus size={14} strokeWidth={2.5} />
          <span>{t("common.add", "添加任务")}</span>
        </button>
      </Dialog.Trigger>
      <Dialog.Content className="max-w-md">
        <Dialog.Title>
          <div className="flex items-center gap-2">
            <Activity size={16} className="text-muted-foreground" />
            <span>{t("common.add", "添加网络探测任务")}</span>
          </div>
        </Dialog.Title>
        <form onSubmit={handleSubmit} className="flex flex-col gap-3.5 my-2 text-xs">
          {/* 任务名称 */}
          <div className="space-y-1">
            <label htmlFor="ping_name" className="text-xs font-medium text-foreground block">
              任务名称
            </label>
            <input
              id="ping_name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如：江苏电信v4、Cloudflare Anycast"
              required
              autoFocus
              className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs"
            />
          </div>

          {/* 协议与间隔 */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <label htmlFor="ping_type" className="text-xs font-medium text-foreground block">
                协议类型
              </label>
              <Select.Root
                value={selectedType}
                onValueChange={(value) => setSelectedType(value as "icmp" | "tcp" | "http")}
              >
                <Select.Trigger id="ping_type" className="w-full h-8 cursor-pointer text-xs" />
                <Select.Content>
                  <Select.Item value="tcp">TCP (端口握手)</Select.Item>
                  <Select.Item value="icmp">ICMP (Ping 报文)</Select.Item>
                  <Select.Item value="http">HTTP (网页探测)</Select.Item>
                </Select.Content>
              </Select.Root>
            </div>
            <div className="space-y-1">
              <label htmlFor="ping_interval" className="text-xs font-medium text-foreground block">
                探测频率 (秒/次)
              </label>
              <input
                id="ping_interval"
                type="number"
                value={interval}
                onChange={(e) => setInterval(Number(e.target.value))}
                min={1}
                required
                className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
              />
            </div>
          </div>

          {/* 目标端点 */}
          <div className="space-y-1">
            <label htmlFor="ping_target" className="text-xs font-medium text-foreground block">
              目标地址 (Target)
            </label>
            <input
              id="ping_target"
              type="text"
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              placeholder="1.1.1.1:80 或 example.com:443 或 http://..."
              required
              className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
            />
            <p className="text-[11px] text-muted-foreground">
              TCP 需包含端口（如 1.1.1.1:80）；ICMP 填写 IP 或纯域名；HTTP 需带完整 URL。
            </p>
          </div>

          {/* 执行节点范围 */}
          <div className="p-3 rounded-lg border border-border/60 bg-muted/20 space-y-2.5">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-foreground">
                执行节点范围
              </span>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground font-mono">
                  已选 {selected.length} 台
                </span>
                <NodeSelectorDialog value={selected} onChange={setSelected} />
              </div>
            </div>

            <div className="pt-2 border-t border-border/40 flex items-start gap-2">
              <input
                type="checkbox"
                id="ping_default_on"
                checked={defaultOn}
                onChange={(e) => setDefaultOn(e.target.checked)}
                className="size-4 mt-0.5 cursor-pointer accent-foreground"
              />
              <div className="flex flex-col">
                <label htmlFor="ping_default_on" className="text-xs font-medium text-foreground cursor-pointer">
                  {t("ping.default_on", "默认全网开启")}
                </label>
                <span className="text-[11px] text-muted-foreground leading-relaxed">
                  {t("ping.default_on_description", "所有现有服务器及新加入节点均会自动执行该探测任务。")}
                </span>
              </div>
            </div>
          </div>

          <Flex justify="end" gap="2" mt="2">
            <Dialog.Close>
              <Button variant="soft" color="gray" type="button" className="cursor-pointer">
                {t("common.cancel", "取消")}
              </Button>
            </Dialog.Close>
            <Button disabled={saving} type="submit" className="cursor-pointer">
              {saving ? "添加中..." : t("common.add", "添加任务")}
            </Button>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
};

export default PingTask;
