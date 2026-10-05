import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { usePingTask } from "@/contexts/usePingTask";
import type { PingTask } from "@/contexts/ping-task-context";
import { Button, Dialog, Flex } from "@radix-ui/themes";
import { Server, Settings2 } from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Selector } from "@/components/Selector";

// 服务器视图：按服务器聚合展示其绑定的任务，并可快速增删绑定
export const ServerView = ({ pingTasks }: { pingTasks: PingTask[] }) => {
  const { t } = useTranslation();
  const { nodeDetail } = useNodeDetails();

  const sortedNodes = React.useMemo(
    () =>
      [...nodeDetail].sort((a, b) => {
        const wa = a.weight ?? 0;
        const wb = b.weight ?? 0;
        if (wa !== wb) return wa - wb;
        return a.name.localeCompare(b.name);
      }),
    [nodeDetail]
  );

  return (
    <div className="rounded-xl border border-border/70 bg-card overflow-hidden shadow-2xs">
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/40 border-b border-border/80 text-[11px] text-muted-foreground">
            <TableHead className="w-56">{t("common.server", "服务器节点")}</TableHead>
            <TableHead>{t("ping.task", "启用的探测任务")}</TableHead>
            <TableHead className="w-24 text-right pr-4">{t("common.action", "管理")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sortedNodes.length === 0 ? (
            <TableRow>
              <TableCell colSpan={3} className="h-32 text-center text-xs text-muted-foreground">
                暂无节点数据
              </TableCell>
            </TableRow>
          ) : (
            sortedNodes.map((n) => (
              <ServerRow
                key={n.uuid}
                nodeUuid={n.uuid}
                nodeName={n.name}
                pingTasks={pingTasks}
              />
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
};

const ServerRow: React.FC<{
  nodeUuid: string;
  nodeName: string;
  pingTasks: PingTask[];
}> = ({ nodeUuid, nodeName, pingTasks }) => {
  const { t } = useTranslation();
  const { refresh } = usePingTask();
  const [open, setOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  // 当前服务器拥有的任务集合（包含明确指定以及默认开启的任务）
  const explicitlyOwnedTasks = React.useMemo(
    () => pingTasks.filter((task) => task.clients?.includes(nodeUuid)),
    [pingTasks, nodeUuid]
  );

  // 默认全网开启的任务
  const defaultOnTasks = React.useMemo(
    () => pingTasks.filter((task) => task.default_on),
    [pingTasks]
  );

  // 编辑状态（所选任务 id 集合）
  const [selectedIds, setSelectedIds] = React.useState<string[]>(
    () => explicitlyOwnedTasks.filter((task) => task.id !== undefined).map((task) => String(task.id))
  );

  React.useEffect(() => {
    setSelectedIds(
      explicitlyOwnedTasks.filter((task) => task.id !== undefined).map((task) => String(task.id))
    );
  }, [explicitlyOwnedTasks]);

  const handleSave = () => {
    setSaving(true);
    const toUpdate = pingTasks
      .filter((task) => task.id !== undefined)
      .filter((task) => {
        const hasBefore = !!task.clients?.includes(nodeUuid);
        const hasAfter = selectedIds.includes(String(task.id));
        return hasBefore !== hasAfter;
      })
      .map((task) => {
        const hasAfter = selectedIds.includes(String(task.id));
        const current = new Set(task.clients || []);
        if (hasAfter) current.add(nodeUuid);
        else current.delete(nodeUuid);
        return {
          id: task.id,
          name: task.name,
          type: task.type,
          target: task.target!,
          default_on: task.default_on || false,
          clients: Array.from(current),
          interval: task.interval,
        };
      });

    if (toUpdate.length === 0) {
      setOpen(false);
      setSaving(false);
      return;
    }

    fetch("/api/admin/ping/edit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tasks: toUpdate }),
    })
      .then((res) => {
        if (!res.ok)
          return res.json().then((d) => {
            throw new Error(d?.message || t("common.error"));
          });
        return res.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully", "更新成功"));
        setOpen(false);
        refresh();
      })
      .catch((e) => toast.error(e.message))
      .finally(() => setSaving(false));
  };

  return (
    <TableRow className="hover:bg-muted/40 transition-colors text-xs">
      <TableCell className="font-medium text-foreground py-3">
        <div className="flex items-center gap-2">
          <Server size={14} className="text-muted-foreground shrink-0" />
          <span className="truncate max-w-[200px]" title={nodeName}>
            {nodeName}
          </span>
        </div>
      </TableCell>
      <TableCell className="py-3">
        <div className="flex flex-wrap items-center gap-1.5">
          {defaultOnTasks.map((task) => (
            <span
              key={`def-${task.id}`}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
              title={`默认全网开启: ${task.type?.toUpperCase()} -> ${task.target}`}
            >
              <span>{task.name}</span>
              <span className="text-[10px] opacity-75">全开</span>
            </span>
          ))}

          {explicitlyOwnedTasks
            .filter((task) => !task.default_on)
            .map((task) => (
              <span
                key={`exp-${task.id}`}
                className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono bg-muted text-foreground border border-border"
                title={`指定执行: ${task.type?.toUpperCase()} -> ${task.target}`}
              >
                <span>{task.name}</span>
              </span>
            ))}

          {defaultOnTasks.length === 0 && explicitlyOwnedTasks.length === 0 && (
            <span className="text-muted-foreground/60 text-xs">未启用任何探测</span>
          )}
        </div>
      </TableCell>
      <TableCell className="text-right pr-4 py-3">
        <Dialog.Root open={open} onOpenChange={setOpen}>
          <Dialog.Trigger>
            <button
              type="button"
              className="size-7 rounded-md border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
              title="配置该服务器的探测任务"
            >
              <Settings2 size={13} />
            </button>
          </Dialog.Trigger>
          <Dialog.Content maxWidth="450px" className="km-pingtask-server-form">
            <Dialog.Title>
              <div className="flex items-center gap-2">
                <Server size={16} className="text-muted-foreground" />
                <span>配置探测任务 - {nodeName}</span>
              </div>
            </Dialog.Title>
            <div className="my-3">
              <Selector
                value={selectedIds}
                onChange={setSelectedIds}
                items={[...pingTasks.filter((t) => t.id !== undefined)]}
                getId={(task) => String(task.id)}
                getLabel={(task) => (
                  <span className="text-xs">
                    <span className="font-medium text-foreground">{task.name}</span>
                    {task.default_on && (
                      <span className="ml-2 text-[10px] font-mono px-1 py-0.2 rounded bg-emerald-500/10 text-emerald-600 border border-emerald-500/20">
                        全网开启
                      </span>
                    )}
                    <span className="ml-2 font-mono text-[11px] text-muted-foreground">
                      {task.type?.toUpperCase()}/{task.interval}s
                    </span>
                  </span>
                )}
                headerLabel={t("ping.task", "探测任务列表")}
                searchPlaceholder={t("common.search", { defaultValue: "搜索任务名称..." })}
                filterItem={(item, keyword) =>
                  String(item.name).toLowerCase().includes(keyword.toLowerCase())
                }
              />
            </div>
            <Flex gap="2" justify="end" className="mt-4">
              <Dialog.Close>
                <Button
                  variant="soft"
                  color="gray"
                  type="button"
                  onClick={() => setOpen(false)}
                  className="cursor-pointer"
                >
                  {t("common.cancel", "取消")}
                </Button>
              </Dialog.Close>
              <Button
                onClick={handleSave}
                disabled={saving}
                className="cursor-pointer"
              >
                {saving ? "保存中..." : t("common.save", "保存修改")}
              </Button>
            </Flex>
          </Dialog.Content>
        </Dialog.Root>
      </TableCell>
    </TableRow>
  );
};
