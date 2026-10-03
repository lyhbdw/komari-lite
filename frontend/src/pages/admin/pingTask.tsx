import Loading from "@/components/loading";
import NodeSelectorDialog from "@/components/NodeSelectorDialog";
import { NodeDetailsProvider } from "@/contexts/NodeDetailsContext";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { PingTaskProvider } from "@/contexts/PingTaskContext";
import { usePingTask } from "@/contexts/usePingTask";
import type { PingTask } from "@/contexts/ping-task-context";
import {
  Box,
  Button,
  Checkbox,
  Dialog,
  Flex,
  Select,
  Tabs,
  TextField,
} from "@radix-ui/themes";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Plus } from "lucide-react";
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

  if (isLoading || nodeDetailLoading) {
    return <Loading />;
  }
  if (error || nodeDetailError) {
    return <div>{error || nodeDetailError}</div>;
  }
  return (
    <div className="space-y-4 km-page-admin-pingtask max-w-7xl">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("ping.title", "延迟监测")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              {pingTasks?.length || 0} 个任务
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            配置 ICMP / TCP / HTTP 目标探测任务，监控各节点到全球关键网络端点的实时延迟与丢包率。
          </p>
        </div>
        <AddButton />
      </div>
      <Tabs.Root defaultValue="task" className="km-pingtask-nav">
        <Tabs.List>
          <Tabs.Trigger value="task">{t("ping.task_view")}</Tabs.Trigger>
          <Tabs.Trigger value="server">{t("ping.server_view")}</Tabs.Trigger>
        </Tabs.List>
        <Box pt="3">
          <Tabs.Content value="task" className="km-pingtask-view">
            <TaskView pingTasks={pingTasks ?? []} />
          </Tabs.Content>
          <Tabs.Content value="server" className="km-pingtask-view">
            <ServerView pingTasks={pingTasks ?? []} />
          </Tabs.Content>
        </Box>
      </Tabs.Root>
    </div>
  );
};

const AddButton: React.FC = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<string[]>([]);
  const [defaultOn, setDefaultOn] = React.useState(false);
  const { refresh } = usePingTask();
  const [selectedType, setSelectedType] = React.useState<
    "icmp" | "tcp" | "http"
  >("icmp");
  const [saving, setSaving] = React.useState(false);
  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!defaultOn && selected.length === 0) {
      toast.error(t("ping.default_on_description"));
      return;
    }
    const payload = {
      name: e.currentTarget.ping_name.value,
      type: selectedType,
      target: e.currentTarget.ping_target.value,
      default_on: defaultOn,
      clients: selected,
      interval: parseInt(e.currentTarget.interval.value, 10),
    };
    setSaving(true);
    fetch("/api/admin/ping/add", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    })
      .then((response) => {
        if (response.ok) {
          setIsOpen(false);
          setSelected([]);
          setDefaultOn(false);
          setSelectedType("icmp");
          toast.success(t("common.success"));
        } else {
          response
            .json()
            .then((data) => {
              toast.error(data?.message || t("common.error"));
            })
            .catch((error) => {
              toast.error(error.message);
            });
        }
      })
      .catch((error) => {
        console.error("Error adding ping task:", error);
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
          className="h-9 px-3.5 rounded-lg bg-foreground text-background font-medium text-xs flex items-center gap-1.5 shadow-sm hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shrink-0"
        >
          <Plus size={14} strokeWidth={2.5} />
          <span>{t("common.add")}</span>
        </button>
      </Dialog.Trigger>
      <Dialog.Content>
        <Dialog.Title>{t("common.add")}</Dialog.Title>
        <form onSubmit={handleSubmit} className="flex flex-col gap-3 mt-1">
          <div>
            <label htmlFor="ping_name" className="text-xs font-medium text-muted-foreground block mb-1">
              {t("common.name")}
            </label>
            <TextField.Root id="ping_name" name="ping_name" required autoFocus />
          </div>

          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <label htmlFor="type" className="text-xs font-medium text-muted-foreground block mb-1">
                {t("common.type")}
              </label>
              <Select.Root
                value={selectedType}
                onValueChange={(value) =>
                  setSelectedType(value as "icmp" | "tcp" | "http")
                }
              >
                <Select.Trigger id="type" name="type" className="w-full" />
                <Select.Content>
                  <Select.Item value="icmp">ICMP</Select.Item>
                  <Select.Item value="tcp">TCP</Select.Item>
                  <Select.Item value="http">HTTP</Select.Item>
                </Select.Content>
              </Select.Root>
            </div>
            <div>
              <label htmlFor="interval" className="text-xs font-medium text-muted-foreground block mb-1">
                {t("ping.interval")} ({t("time.second")})
              </label>
              <TextField.Root
                id="interval"
                name="interval"
                defaultValue={60}
                type="number"
                placeholder="60"
                min="1"
                required
              />
            </div>
          </div>

          <div>
            <label htmlFor="ping_target" className="text-xs font-medium text-muted-foreground block mb-1">
              {t("ping.target")}
            </label>
            <TextField.Root
              id="ping_target"
              name="ping_target"
              placeholder="1.1.1.1 | 1.1.1.1:80 | https://1.1.1.1"
              required
            />
          </div>

          <div className="p-2.5 rounded-lg border border-border/60 bg-muted/20 space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-foreground">
                {t("common.server")}
              </label>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">
                  {t("common.selected", { count: selected.length })}
                </span>
                <NodeSelectorDialog value={selected} onChange={setSelected} />
              </div>
            </div>
            <div className="pt-2 border-t border-border/40 flex items-start gap-2">
              <Checkbox
                id="ping_default_on"
                checked={defaultOn}
                onCheckedChange={(checked) => setDefaultOn(!!checked)}
              />
              <div className="flex flex-col">
                <label htmlFor="ping_default_on" className="text-xs font-medium text-foreground cursor-pointer">
                  {t("ping.default_on")}
                </label>
                <span className="text-[11px] text-muted-foreground">
                  {t("ping.default_on_description")}
                </span>
              </div>
            </div>
          </div>

          <Flex justify="end" gap="2" mt="2">
            <Dialog.Close>
              <Button variant="soft" color="gray" type="button">
                {t("common.cancel", "取消")}
              </Button>
            </Dialog.Close>
            <Button disabled={saving} type="submit">
              {t("common.add")}
            </Button>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
};

export default PingTask;
