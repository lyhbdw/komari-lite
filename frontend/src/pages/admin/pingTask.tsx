import Loading from "@/components/loading";
import { PingTaskForm, type PingTaskFormValue } from "@/components/PingTaskForm";
import { NodeDetailsProvider } from "@/contexts/NodeDetailsContext";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { PingTaskProvider } from "@/contexts/PingTaskContext";
import { usePingTask } from "@/contexts/usePingTask";
import { Dialog } from "@/components/ui/radix-shim";
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
  const [form, setForm] = React.useState<PingTaskFormValue>({
    name: "",
    target: "",
    interval: 60,
    type: "tcp",
    clients: [],
    default_on: true,
  });
  const [saving, setSaving] = React.useState(false);
  const { refresh } = usePingTask();

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!form.name.trim()) {
      toast.error("请输入任务名称");
      return;
    }
    if (!form.target.trim()) {
      toast.error("请输入探测目标");
      return;
    }
    if (!form.default_on && form.clients.length === 0) {
      toast.error(t("ping.default_on_description", "请至少勾选默认全网开启或指定执行节点"));
      return;
    }
    const payload = {
      ...form,
      name: form.name.trim(),
      target: form.target.trim(),
      interval: form.interval || 60,
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
          setForm({
            name: "",
            target: "",
            interval: 60,
            type: "tcp",
            clients: [],
            default_on: true,
          });
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
      <Dialog.Trigger asChild>
        <button
          onClick={() => setIsOpen(true)}
          className="h-8 px-3.5 rounded-lg bg-foreground text-background font-medium text-xs flex items-center gap-1.5 shadow-2xs hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shrink-0"
        >
          <Plus size={14} strokeWidth={2.5} />
          <span>{t("common.add", "添加任务")}</span>
        </button>
      </Dialog.Trigger>
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <Activity size={16} className="text-muted-foreground shrink-0" />
            <span>{t("common.add", "添加网络探测任务")}</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            创建 ICMP、TCP 或 HTTP 延迟监测任务，并下发给指定服务器节点。
          </Dialog.Description>
        </Dialog.Header>
        <PingTaskForm
          value={form}
          onChange={setForm}
          onSubmit={handleSubmit}
          submitting={saving}
          submitLabel={t("common.add", "添加任务")}
          nameId="ping_name"
          typeId="ping_type"
          intervalId="ping_interval"
          targetId="ping_target"
          defaultOnId="ping_default_on"
        />
      </Dialog.Content>
    </Dialog.Root>
  );
};

export default PingTask;
