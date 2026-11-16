import { copyToClipboard } from "@/utils/clipboard";
import React from "react";
import { useState } from "react";
import type { NodeDetail } from "@/contexts/node-details-context";
import { Checkbox, DropdownMenu } from "@/components/ui/radix-shim";
import { CircleDollarSign, Copy, Download, Trash2Icon, CalendarCheck, ArrowUpDown, ArrowUp, ArrowDown, GripVertical, MoreHorizontal, Pencil } from "lucide-react";
import { DndContext, closestCenter, useSensor, useSensors, TouchSensor, MouseSensor, KeyboardSensor } from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy, useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useIsMobile } from "@/hooks/use-mobile";
import { isNodeExpiringSoon } from "../../../../../theme/src/utils/billingCore";
import PriceTags from "@/components/PriceTags";
import { DetailView } from "./DetailView";
import { EditButton } from "./EditButton";
import { BillingButton } from "./BillingButton";
import { GenerateCommandButton, RenewDialog, DeleteButton } from "./NodeDialogs";

const ServerRow = ({
  node,
  selectedNodes,
  handleSelectNode,
  settings,
}: {
  node: NodeDetail;
  selectedNodes: string[];
  handleSelectNode: (uuid: string, checked: boolean) => void;
  settings: any;
}) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: node.uuid });
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 10 : undefined,
  };
  function copy(text: string) {
    copyToClipboard(text, {
      successMessage: t("copy_success"),
    });
  }
  return (
    <TableRow
      ref={setNodeRef}
      style={style}
      className={`transition-colors h-[52px] ${
        isDragging
          ? "bg-muted/70 shadow-md opacity-90 relative"
          : "hover:bg-muted/40"
      }`}
    >
      <TableCell className="w-12 pl-2.5 pr-1 text-center">
        <div className="flex items-center justify-center gap-1.5">
          <div
            {...attributes}
            {...listeners}
            className={`cursor-default p-0.5 rounded hover:bg-muted text-muted-foreground/30 hover:text-foreground transition-colors inline-flex items-center justify-center ${
              isMobile ? "touch-manipulation select-none" : ""
            }`}
            style={{
              touchAction: "none",
              WebkitUserSelect: "none",
              userSelect: "none",
            }}
            title={t("admin.nodeTable.dragToReorder", "长按拖拽重新排序")}
          >
            <GripVertical size={13} />
          </div>
          <Checkbox
            checked={selectedNodes.includes(node.uuid)}
            onCheckedChange={(checked) => handleSelectNode(node.uuid, !!checked)}
          />
        </div>
      </TableCell>
      <TableCell className="min-w-0 px-2.5">
        <DetailView node={node} />
      </TableCell>
      <TableCell className="w-40 px-2 hidden md:table-cell">
        <div className="flex items-center gap-2 flex-nowrap">
          {node.ipv4 ? (
            <div
              onClick={() => copy(node.ipv4!)}
              className="inline-flex items-center gap-1.5 group cursor-pointer text-foreground/90 hover:text-foreground select-none"
              title={`${node.ipv4} (点击复制)`}
            >
              <span className="font-mono text-xs tracking-tight font-medium">
                {node.ipv4}
              </span>
              <Copy size={11} className="opacity-0 group-hover:opacity-100 text-muted-foreground transition-opacity shrink-0" />
            </div>
          ) : (
            <span className="text-xs text-muted-foreground/40 font-mono">-</span>
          )}
          {node.ipv6 && (
            <button
              type="button"
              onClick={() => copy(node.ipv6!)}
              className="px-1.5 py-0.5 rounded text-[10px] font-mono font-medium bg-muted/60 hover:bg-muted text-muted-foreground hover:text-foreground border border-border/40 transition-colors cursor-pointer shrink-0"
              title={`${node.ipv6} (点击复制)`}
            >
              IPv6
            </button>
          )}
        </div>
      </TableCell>
      <TableCell className="w-20 px-1.5 text-center hidden md:table-cell">
        {node.group ? (
          <span className="inline-block text-xs px-2 py-0.5 rounded-md bg-muted/50 text-foreground/80 font-medium border border-border/40 max-w-[76px] truncate" title={node.group}>
            {node.group}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground/40 font-mono">-</span>
        )}
      </TableCell>
      <TableCell className="w-28 px-2 hidden md:table-cell">
        <PriceTags
          price={node.price}
          billing_cycle={node.billing_cycle}
          expired_at={node.expired_at}
          currency={node.currency}
          tags={node.tags || ""}
        />
      </TableCell>
      <TableCell className="w-44 px-2 text-center whitespace-nowrap">
        <ActionButtons
          node={node}
          settings={settings}
          isExpiring={isNodeExpiringSoon(node.expired_at)}
        />
      </TableCell>
    </TableRow>
  );
};


const NodeTable = ({
  nodes,
  selectedNodes,
  setSelectedNodes,
  settings,
  sortField,
  sortOrder,
  onSort,
  onResetSort,
}: {
  nodes: NodeDetail[];
  selectedNodes: string[];
  setSelectedNodes: (nodes: string[]) => void;
  settings: any;
  sortField: "name" | "ip" | "group" | "billing" | "none";
  sortOrder: "asc" | "desc";
  onSort: (field: "name" | "ip" | "group" | "billing") => void;
  onResetSort?: () => void;
}) => {
  const { t } = useTranslation();
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
  const [localNodes, setLocalNodes] = useState<NodeDetail[]>(nodes);
  const [isDragging, setIsDragging] = useState(false);
  React.useEffect(() => {
    setLocalNodes(nodes);
  }, [nodes]);

  const handleDragStart = () => {
    setIsDragging(true);
    if ("vibrate" in navigator) {
      navigator.vibrate(50);
    }
  };

  const handleDragEnd = async (event: any) => {
    setIsDragging(false);
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = localNodes.findIndex((node) => node.uuid === active.id);
    const newIndex = localNodes.findIndex((node) => node.uuid === over.id);
    if (oldIndex === -1 || newIndex === -1) return;

    const reorderedNodes = Array.from(localNodes);
    const [reorderedItem] = reorderedNodes.splice(oldIndex, 1);
    reorderedNodes.splice(newIndex, 0, reorderedItem);

    setLocalNodes(reorderedNodes);
    if (onResetSort) {
      onResetSort();
    }

    if ("vibrate" in navigator) {
      navigator.vibrate([30, 10, 30]);
    }

    try {
      const orderData = reorderedNodes.reduce((acc, node, index) => {
        acc[node.uuid] = index;
        return acc;
      }, {} as Record<string, number>);

      await fetch("/api/admin/client/order", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(orderData),
      });
    } catch {
      toast.error(t("admin.nodeTable.errorRefreshNodeList"));
    }
  };

  const handleSelectAll = (checked: boolean) => {
    setSelectedNodes(checked ? localNodes.map((node) => node.uuid) : []);
  };

  const handleSelectNode = (uuid: string, checked: boolean) => {
    setSelectedNodes(
      checked
        ? [...selectedNodes, uuid]
        : selectedNodes.filter((id) => id !== uuid)
    );
  };

  const renderSortIcon = (field: "name" | "ip" | "group" | "billing") => {
    if (sortField !== field) {
      return (
        <ArrowUpDown
          size={11}
          className="opacity-25 group-hover:opacity-70 transition-opacity ml-1 inline-block shrink-0"
        />
      );
    }
    return sortOrder === "asc" ? (
      <ArrowUp size={11} className="text-foreground ml-1 inline-block shrink-0" />
    ) : (
      <ArrowDown size={11} className="text-foreground ml-1 inline-block shrink-0" />
    );
  };

  return (
    <div
      className={`rounded-xl border border-border/70 bg-card overflow-hidden shadow-2xs ${
        isDragging ? "select-none [&_*]:!cursor-default" : ""
      }`}
    >
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
      >
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40 border-b border-border/80 text-[11px]">
              <TableHead className="w-12 pl-2.5 pr-1 text-center">
                <Checkbox
                  checked={
                    selectedNodes.length === localNodes.length &&
                    localNodes.length > 0
                  }
                  onCheckedChange={handleSelectAll}
                />
              </TableHead>
              <TableHead
                onClick={() => onSort("name")}
                className="min-w-0 px-2.5 text-left cursor-pointer select-none group hover:text-foreground transition-colors"
              >
                <span className="inline-flex items-center">
                  <span>{t("admin.nodeTable.name")}</span>
                  {renderSortIcon("name")}
                </span>
              </TableHead>
              <TableHead
                onClick={() => onSort("ip")}
                className="w-40 px-2 text-left cursor-pointer select-none group hover:text-foreground transition-colors hidden md:table-cell"
              >
                <span className="inline-flex items-center">
                  <span>{t("admin.nodeDetail.ipAddress")}</span>
                  {renderSortIcon("ip")}
                </span>
              </TableHead>
              <TableHead
                onClick={() => onSort("group")}
                className="w-20 px-1.5 text-center cursor-pointer select-none group hover:text-foreground transition-colors hidden md:table-cell"
              >
                <span className="inline-flex items-center justify-center">
                  <span>{t("common.group")}</span>
                  {renderSortIcon("group")}
                </span>
              </TableHead>
              <TableHead
                onClick={() => onSort("billing")}
                className="w-28 px-2 text-left cursor-pointer select-none group hover:text-foreground transition-colors hidden md:table-cell"
              >
                <span className="inline-flex items-center">
                  <span>{t("admin.nodeTable.billing_col", "账单")}</span>
                  {renderSortIcon("billing")}
                </span>
              </TableHead>
              <TableHead className="w-44 px-2 text-center">
                {t("common.actions", "操作")}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <SortableContext
              items={localNodes.map((node) => node.uuid)}
              strategy={verticalListSortingStrategy}
            >
              {localNodes.map((node) => (
                <ServerRow
                  key={node.uuid}
                  node={node}
                  selectedNodes={selectedNodes}
                  handleSelectNode={handleSelectNode}
                  settings={settings}
                />
              ))}
            </SortableContext>
          </TableBody>
        </Table>
      </DndContext>
    </div>
  );
};


const ActionButtons = ({
  node,
  settings,
  isExpiring = false,
}: {
  node: NodeDetail;
  settings: any;
  isExpiring?: boolean;
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [billingOpen, setBillingOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [renewOpen, setRenewOpen] = useState(false);
  const [installOpen, setInstallOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);

  const cycleDays = Number(node.billing_cycle) > 0 ? Number(node.billing_cycle) : 30;

  const dialogs = (
    <>
      <RenewDialog
        node={node}
        open={renewOpen}
        onOpenChange={setRenewOpen}
      />
      <BillingButton
        node={node}
        open={billingOpen}
        onOpenChange={setBillingOpen}
        trigger={null}
      />
      <GenerateCommandButton
        settings={settings}
        nodeToken={node.token}
        open={installOpen}
        onOpenChange={setInstallOpen}
        trigger={null}
      />
      <DeleteButton
        node={node}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        trigger={null}
      />
      <EditButton node={node} open={editOpen} onOpenChange={setEditOpen} />
    </>
  );

  // 触屏没有 hover，title 提示不可见：移动端收敛为带文字标签的溢出菜单，
  // 避免 5 个无标签图标按钮难以识别。
  if (isMobile) {
    return (
      <div className="flex items-center justify-center">
        <DropdownMenu.Root>
          <DropdownMenu.Trigger>
            <button
              type="button"
              className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
              title={t("common.actions", "操作")}
              aria-label={t("common.actions", "操作")}
            >
              <MoreHorizontal size={14} />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Content align="end" className="min-w-[168px]">
            <DropdownMenu.Item onClick={() => setEditOpen(true)} className="cursor-pointer">
              <Pencil size={13} className="mr-2 shrink-0" />
              <span className="truncate">{t("admin.nodeEdit.editInfo", "编辑信息")}</span>
            </DropdownMenu.Item>
            <DropdownMenu.Item onClick={() => setRenewOpen(true)} className="cursor-pointer">
              <CalendarCheck size={13} className="mr-2 shrink-0" />
              <span className="truncate">{t("admin.nodeTable.renewOneCycle", `快捷续费 (+${cycleDays}天)`)}</span>
            </DropdownMenu.Item>
            <DropdownMenu.Item onClick={() => setBillingOpen(true)} className="cursor-pointer">
              <CircleDollarSign size={13} className="mr-2 shrink-0" />
              <span className="truncate">{t("admin.nodeTable.billing", "账单管理")}</span>
            </DropdownMenu.Item>
            <DropdownMenu.Item onClick={() => setInstallOpen(true)} className="cursor-pointer">
              <Download size={13} className="mr-2 shrink-0" />
              <span className="truncate">{t("admin.nodeTable.installCommand", "安装指令")}</span>
            </DropdownMenu.Item>
            <DropdownMenu.Separator />
            <DropdownMenu.Item onClick={() => setDeleteOpen(true)} className="cursor-pointer text-rose-600 dark:text-rose-400">
              <Trash2Icon size={13} className="mr-2 shrink-0" />
              <span className="truncate">{t("common.delete", "删除节点")}</span>
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Root>
        {dialogs}
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center gap-1">
      {/* 1. 编辑信息 */}
      <EditButton node={node} />

      {/* 2. 续费按钮 */}
      <button
        type="button"
        onClick={() => setRenewOpen(true)}
        className={`size-7 rounded-md border inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs ${
          isExpiring
            ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300 border-emerald-500/30 hover:bg-emerald-500/25"
            : "border-border/60 text-muted-foreground hover:text-emerald-600 dark:hover:text-emerald-400 hover:bg-muted"
        }`}
        title={t("admin.nodeTable.renewOneCycle", `快捷续费 (+${cycleDays}天)`)}
      >
        <CalendarCheck size={13} />
      </button>

      {/* 3. 账单管理 */}
      <button
        type="button"
        onClick={() => setBillingOpen(true)}
        className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
        title={t("admin.nodeTable.billing", "账单管理")}
      >
        <CircleDollarSign size={13} />
      </button>

      {/* 4. 安装指令 */}
      <button
        type="button"
        onClick={() => setInstallOpen(true)}
        className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
        title={t("admin.nodeTable.installCommand", "安装指令")}
      >
        <Download size={13} />
      </button>

      {/* 5. 删除节点 */}
      <button
        type="button"
        onClick={() => setDeleteOpen(true)}
        className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-rose-600 dark:hover:text-rose-400 hover:bg-rose-500/10 inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
        title={t("common.delete", "删除节点")}
      >
        <Trash2Icon size={13} />
      </button>

      {dialogs}
    </div>
  );
};


export { ServerRow, NodeTable, ActionButtons };
