import NodeSelectorDialog from "@/components/NodeSelectorDialog";
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
import {
  DndContext,
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  Button,
  Dialog,
  Flex,
  Select,
} from "@radix-ui/themes";
import { GripVertical, Pencil, Settings2, Trash2 } from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

const getTaskSortableId = (task: { id?: number; name?: string; target?: string }) =>
  task.id !== undefined
    ? `id-${task.id}`
    : `tmp-${task.name ?? ""}-${task.target ?? ""}`;

function ProtocolBadge({ type }: { type: string }) {
  const norm = (type || "").toLowerCase();
  if (norm === "tcp") {
    return (
      <span className="px-1.5 py-0.2 text-[10px] font-mono font-medium rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 uppercase">
        TCP
      </span>
    );
  }
  if (norm === "http" || norm === "https") {
    return (
      <span className="px-1.5 py-0.2 text-[10px] font-mono font-medium rounded bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20 uppercase">
        HTTP
      </span>
    );
  }
  return (
    <span className="px-1.5 py-0.2 text-[10px] font-mono font-medium rounded bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20 uppercase">
      ICMP
    </span>
  );
}

export const TaskView = ({ pingTasks }: { pingTasks: PingTask[] }) => {
  const { t } = useTranslation();
  const { refresh } = usePingTask();
  const { nodeDetail } = useNodeDetails();
  const sensors = useSensors(
    useSensor(MouseSensor, {
      activationConstraint: {
        distance: 10,
      },
    }),
    useSensor(TouchSensor, {
      activationConstraint: {
        delay: 200,
        tolerance: 5,
      },
    }),
    useSensor(KeyboardSensor, {})
  );

  const processedTasks = React.useMemo(() => {
    if (!pingTasks) return [];
    const nodeUuidSet = new Set(nodeDetail.map((n) => n.uuid));
    return pingTasks.map((task) => {
      const original = task.clients || [];
      const existing = original.filter((uuid) => nodeUuidSet.has(uuid));
      const allDeleted = original.length > 0 && existing.length === 0;
      return {
        ...task,
        clients: existing,
        __allClientsDeleted: allDeleted,
        __originalCount: original.length,
      };
    });
  }, [pingTasks, nodeDetail]);

  const [localTasks, setLocalTasks] = React.useState(processedTasks);

  React.useEffect(() => {
    setLocalTasks(processedTasks);
  }, [processedTasks]);

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = localTasks.findIndex(
      (task) => getTaskSortableId(task) === String(active.id)
    );
    const newIndex = localTasks.findIndex(
      (task) => getTaskSortableId(task) === String(over.id)
    );
    if (oldIndex < 0 || newIndex < 0) return;

    const previousTasks = Array.from(localTasks);
    const reorderedTasks = Array.from(localTasks);
    const [reorderedItem] = reorderedTasks.splice(oldIndex, 1);
    reorderedTasks.splice(newIndex, 0, reorderedItem);

    setLocalTasks(reorderedTasks);

    const orderData = reorderedTasks.reduce((acc, task, index) => {
      if (task.id !== undefined) {
        acc[String(task.id)] = index;
      }
      return acc;
    }, {} as Record<string, number>);

    try {
      const response = await fetch("/api/admin/ping/order", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(orderData),
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data?.message || t("common.error"));
      }
    } catch (error: any) {
      setLocalTasks(previousTasks);
      toast.error(error?.message || t("common.error"));
      refresh();
    }
  };

  return (
    <div className="rounded-xl border border-border/70 bg-card overflow-hidden shadow-2xs">
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/40 border-b border-border/80 text-[11px] text-muted-foreground">
            <TableHead className="w-10 text-center"></TableHead>
            <TableHead className="w-36">{t("ping.col_name", "任务名称")}</TableHead>
            <TableHead className="w-56 font-mono">{t("ping.col_target", "探测目标")}</TableHead>
            <TableHead className="w-20 text-center">{t("ping.col_type", "协议")}</TableHead>
            <TableHead className="w-24 font-mono">{t("ping.col_interval", "探测间隔")}</TableHead>
            <TableHead>{t("ping.col_server", "执行节点")}</TableHead>
            <TableHead className="w-24 text-right pr-4">{t("ping.col_action", "操作")}</TableHead>
          </TableRow>
        </TableHeader>
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext
            items={localTasks.map((task) => getTaskSortableId(task))}
            strategy={verticalListSortingStrategy}
          >
            <TableBody>
              {localTasks.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-32 text-center text-xs text-muted-foreground">
                    {t("ping.empty", "暂无网络探测任务，点击右上角添加。")}
                  </TableCell>
                </TableRow>
              ) : (
                localTasks.map((task) => (
                  <Row key={getTaskSortableId(task)} task={task} />
                ))
              )}
            </TableBody>
          </SortableContext>
        </DndContext>
      </Table>
    </div>
  );
};

const Row = ({
  task,
}: {
  task: PingTask & { __allClientsDeleted?: boolean; __originalCount?: number };
}) => {
  const { t } = useTranslation();
  const { refresh } = usePingTask();
  const sortableId = getTaskSortableId(task);
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: sortableId });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 10 : undefined,
  };
  const [editOpen, setEditOpen] = React.useState(false);
  const [editSaving, setEditSaving] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteLoading, setDeleteLoading] = React.useState(false);
  const [form, setForm] = React.useState({
    name: task.name || "",
    type: task.type || "icmp",
    target: task.target || "",
    clients: task.clients || [],
    default_on: task.default_on || false,
    interval: task.interval || 60,
  });

  const submitEdit = (newForm: typeof form) => {
    if (!newForm.default_on && newForm.clients.length === 0) {
      toast.error(t("ping.default_on_description", "请至少勾选默认全网开启或指定执行节点"));
      return;
    }
    setEditSaving(true);
    fetch("/api/admin/ping/edit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        tasks: [
          {
            id: task.id,
            name: newForm.name,
            type: newForm.type,
            target: newForm.target,
            default_on: newForm.default_on,
            clients: newForm.clients,
            interval: newForm.interval,
          },
        ],
      }),
    })
      .then(async (res) => {
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data?.message || t("common.error"));
        }
        setEditOpen(false);
        toast.success(t("common.updated_successfully", "更新成功"));
        refresh();
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => setEditSaving(false));
  };

  const handleEdit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    submitEdit(form);
  };

  const handleDelete = () => {
    setDeleteLoading(true);
    fetch("/api/admin/ping/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id: [task.id] }),
    })
      .then(async (res) => {
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data?.message || t("common.error"));
        }
        setDeleteOpen(false);
        toast.success(t("common.deleted_successfully", "已删除"));
        refresh();
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => setDeleteLoading(false));
  };

  const clientCount = task.clients?.length || 0;

  return (
    <TableRow
      ref={setNodeRef}
      style={style}
      className={`transition-colors text-xs ${
        isDragging
          ? "bg-muted/70 shadow-md opacity-90 relative"
          : "hover:bg-muted/40"
      }`}
    >
      <TableCell className="text-center p-2">
        <div
          {...attributes}
          {...listeners}
          className="p-1.5 rounded hover:bg-muted text-muted-foreground/60 hover:text-foreground transition-colors inline-flex items-center justify-center cursor-default select-none"
          title="拖拽上下排序"
        >
          <GripVertical size={14} />
        </div>
      </TableCell>
      <TableCell className="font-medium text-foreground">
        {task.name}
      </TableCell>
      <TableCell className="font-mono text-[11px] text-foreground">
        {task.target}
      </TableCell>
      <TableCell className="text-center">
        <ProtocolBadge type={task.type || "icmp"} />
      </TableCell>
      <TableCell className="font-mono text-[11px] text-muted-foreground">
        {task.interval}s
      </TableCell>
      <TableCell>
        <div className="flex items-center gap-1.5">
          {task.default_on ? (
            <span className="px-1.5 py-0.2 text-[10px] font-mono rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
              {t("ping.default_on_short", "默认全开")}
            </span>
          ) : (
            <span className="px-1.5 py-0.2 text-[10px] font-mono rounded bg-muted text-muted-foreground border border-border">
              已指定 {clientCount} 台
            </span>
          )}

          {/* 节点分配弹窗触发 */}
          <NodeSelectorDialog
            value={form.clients ?? []}
            onChange={(uuids) => {
              const nextForm = { ...form, clients: uuids };
              setForm(nextForm);
              submitEdit(nextForm);
            }}
          >
            <button
              type="button"
              className="p-1 rounded hover:bg-muted text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              title="调整执行节点"
            >
              <Settings2 size={13} />
            </button>
          </NodeSelectorDialog>
        </div>
      </TableCell>
      <TableCell className="text-right pr-4">
        <div className="flex items-center justify-end gap-1">
          {/* 编辑按钮 */}
          <Dialog.Root open={editOpen} onOpenChange={setEditOpen}>
            <Dialog.Trigger>
              <button
                type="button"
                className="size-7 rounded-md border border-border/60 hover:bg-muted text-muted-foreground hover:text-foreground inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
                title="编辑探测任务"
              >
                <Pencil size={13} />
              </button>
            </Dialog.Trigger>
            <Dialog.Content className="max-w-md">
              <Dialog.Title>
                <div className="flex items-center gap-2">
                  <Pencil size={15} className="text-muted-foreground" />
                  <span>编辑探测任务</span>
                </div>
              </Dialog.Title>
              <form onSubmit={handleEdit} className="flex flex-col gap-3.5 my-2 text-xs">
                <div className="space-y-1">
                  <label htmlFor={`edit-task-name-${task.id}`} className="text-xs font-medium text-foreground block">
                    任务名称
                  </label>
                  <input
                    id={`edit-task-name-${task.id}`}
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                    required
                    className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs"
                  />
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1">
                    <label htmlFor={`edit-task-type-${task.id}`} className="text-xs font-medium text-foreground block">
                      协议类型
                    </label>
                    <Select.Root
                      value={form.type}
                      onValueChange={(v) => setForm((f) => ({ ...f, type: v as any }))}
                    >
                      <Select.Trigger id={`edit-task-type-${task.id}`} className="w-full h-8 cursor-pointer text-xs" />
                      <Select.Content>
                        <Select.Item value="tcp">TCP (端口握手)</Select.Item>
                        <Select.Item value="icmp">ICMP (Ping 报文)</Select.Item>
                        <Select.Item value="http">HTTP (网页探测)</Select.Item>
                      </Select.Content>
                    </Select.Root>
                  </div>
                  <div className="space-y-1">
                    <label htmlFor={`edit-task-interval-${task.id}`} className="text-xs font-medium text-foreground block">
                      探测频率 (秒/次)
                    </label>
                    <input
                      id={`edit-task-interval-${task.id}`}
                      type="number"
                      min={1}
                      value={form.interval}
                      onChange={(e) => setForm((f) => ({ ...f, interval: Number(e.target.value) }))}
                      required
                      className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
                    />
                  </div>
                </div>

                <div className="space-y-1">
                  <label htmlFor={`edit-task-target-${task.id}`} className="text-xs font-medium text-foreground block">
                    目标地址 (Target)
                  </label>
                  <input
                    id={`edit-task-target-${task.id}`}
                    value={form.target}
                    onChange={(e) => setForm((f) => ({ ...f, target: e.target.value }))}
                    required
                    className="w-full h-8 px-2.5 text-xs rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs font-mono"
                  />
                </div>

                <div className="p-3 rounded-lg border border-border/60 bg-muted/20 space-y-2.5">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-medium text-foreground">
                      执行节点范围
                    </span>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-muted-foreground font-mono">
                        已选 {form.clients.length} 台
                      </span>
                      <NodeSelectorDialog
                        value={form.clients}
                        onChange={(v) => setForm((f) => ({ ...f, clients: v }))}
                      />
                    </div>
                  </div>
                  <div className="pt-2 border-t border-border/40 flex items-start gap-2">
                    <input
                      type="checkbox"
                      id={`edit_ping_default_on_${task.id}`}
                      checked={form.default_on}
                      onChange={(e) => setForm((f) => ({ ...f, default_on: e.target.checked }))}
                      className="size-4 mt-0.5 cursor-pointer accent-foreground"
                    />
                    <div className="flex flex-col">
                      <label htmlFor={`edit_ping_default_on_${task.id}`} className="text-xs font-medium text-foreground cursor-pointer">
                        {t("ping.default_on", "默认全网开启")}
                      </label>
                      <span className="text-[11px] text-muted-foreground leading-relaxed">
                        {t("ping.default_on_description", "所有现有服务器及新加入节点均会自动执行该探测任务。")}
                      </span>
                    </div>
                  </div>
                </div>

                <Flex gap="2" justify="end" mt="2">
                  <Dialog.Close>
                    <Button variant="soft" color="gray" type="button" className="cursor-pointer">
                      {t("common.cancel", "取消")}
                    </Button>
                  </Dialog.Close>
                  <Button type="submit" disabled={editSaving} className="cursor-pointer">
                    {editSaving ? "保存中..." : t("common.save", "保存修改")}
                  </Button>
                </Flex>
              </form>
            </Dialog.Content>
          </Dialog.Root>

          {/* 删除按钮 */}
          <Dialog.Root open={deleteOpen} onOpenChange={setDeleteOpen}>
            <Dialog.Trigger>
              <button
                type="button"
                className="size-7 rounded-md border border-border/60 hover:bg-rose-500/10 text-muted-foreground hover:text-rose-600 dark:hover:text-rose-400 inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
                title="删除探测任务"
              >
                <Trash2 size={13} />
              </button>
            </Dialog.Trigger>
            <Dialog.Content className="max-w-sm">
              <Dialog.Title>
                <div className="flex items-center gap-2 text-rose-600 dark:text-rose-400">
                  <Trash2 size={16} />
                  <span>确认删除探测任务</span>
                </div>
              </Dialog.Title>
              <p className="text-xs text-muted-foreground my-2 leading-relaxed">
                删除任务 <strong className="text-foreground">{task.name}</strong> 后，所有节点将停止探测且历史延迟数据将被移除。此操作不可恢复。
              </p>
              <Flex gap="2" justify="end" mt="4">
                <Dialog.Close>
                  <Button variant="soft" color="gray" type="button" className="cursor-pointer">
                    {t("common.cancel", "取消")}
                  </Button>
                </Dialog.Close>
                <Button
                  color="red"
                  onClick={handleDelete}
                  disabled={deleteLoading}
                  className="cursor-pointer"
                >
                  {deleteLoading ? "删除中..." : t("common.delete", "确认删除")}
                </Button>
              </Flex>
            </Dialog.Content>
          </Dialog.Root>
        </div>
      </TableCell>
    </TableRow>
  );
};
