import { quoteShellArgs } from "@/utils/shellQuote";
import { buildAgentInstallArgs } from "@/utils/agentInstallCommand";
import React, { useEffect, useState } from "react";
import { NodeDetailsProvider } from "@/contexts/NodeDetailsContext";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { useAccount } from "@/contexts/useAccount";
import type { NodeDetail } from "@/contexts/node-details-context";
import {
  Flex,
  TextField,
  Button,
  Checkbox,
  Text,
  Dialog,
  IconButton,
  TextArea,
  Select,
  Switch,
} from "@radix-ui/themes";
import {
  Activity,
  CircleDollarSign,
  Copy,
  CornerRightUp,
  Download,
  Folder,
  Globe,
  GripVertical,
  MenuIcon,
  Pencil,
  Plus,
  Search,
  Server,
  Trash2Icon,
  ArrowUpCircle,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  DndContext,
  closestCenter,
  useSensor,
  useSensors,
  TouchSensor,
  MouseSensor,
  KeyboardSensor,
} from "@dnd-kit/core";
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { toast } from "sonner";
import Flag from "@/components/Flag";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer";
import { formatBytes, stringToBytes } from "@/utils/unitHelper";
import PriceTags from "@/components/PriceTags";
import Loading from "@/components/loading";
import Tips from "@/components/ui/tips";

import { useSettings } from "@/lib/api";
import { SelectOrInput } from "@/components/ui/select-or-input";



const NodeDetailsPage = () => {
  return (
    <NodeDetailsProvider>
      <Layout />
    </NodeDetailsProvider>
  );
};

const Layout = () => {
  const { nodeDetail, isLoading, error, refresh } = useNodeDetails();
  const { account } = useAccount();
  const { settings } = useSettings();
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const filteredNodes = Array.isArray(nodeDetail)
    ? nodeDetail
        .filter((node) => {
          const lower = searchTerm.toLowerCase();
          if (lower === "online" || lower === "在线") return !!node.online;
          if (lower === "offline" || lower === "离线") return !node.online;
          return (
            node.name.toLowerCase().includes(lower) ||
            (node.ipv4 && node.ipv4.includes(searchTerm)) ||
            (node.group && node.group.toLowerCase().includes(lower))
          );
        })
        .sort((a, b) => a.weight - b.weight)
    : [];

  useEffect(() => {
    const interval = setInterval(() => {
      refresh();
    }, 5000);
    return () => clearInterval(interval);
  }, [refresh]);

  if (account && !account.logged_in) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[50vh] text-center p-6">
        <div className="w-12 h-12 rounded-2xl bg-foreground text-background flex items-center justify-center font-bold text-xl mb-4 shadow-sm select-none">
          K
        </div>
        <h2 className="text-lg font-semibold tracking-tight text-foreground">
          请先登录管理后台
        </h2>
        <p className="text-xs text-muted-foreground mt-1 max-w-sm">
          认证会话已失效或尚未登录，请在弹出的登录窗口中完成身份验证。
        </p>
      </div>
    );
  }

  if (isLoading && (!nodeDetail || nodeDetail.length === 0)) return <Loading text="" />;
  if (error && (!nodeDetail || nodeDetail.length === 0)) {
    return (
      <div className="p-6 rounded-xl border border-border bg-card text-center my-6 shadow-2xs">
        <p className="text-sm font-medium text-foreground">{error}</p>
        <button
          onClick={() => refresh()}
          className="mt-3 px-3.5 py-1.5 rounded-lg bg-foreground text-background text-xs font-medium cursor-pointer shadow-sm hover:opacity-90 transition-opacity"
        >
          重试连接
        </button>
      </div>
    );
  }

  const isEmpty = Array.isArray(nodeDetail) && nodeDetail.length === 0;

  return (
    <div className="km-page-admin-index space-y-5">
      <Header
        searchTerm={searchTerm}
        setSearchTerm={setSearchTerm}
        selectedNodes={selectedNodes}
        totalNodes={nodeDetail?.length || 0}
      />

      {!isEmpty && <MetricsOverview nodes={nodeDetail || []} />}

      {isEmpty ? (
        <EmptyNodesGuide />
      ) : (
        <NodeTable
          nodes={filteredNodes}
          selectedNodes={selectedNodes}
          setSelectedNodes={setSelectedNodes}
          settings={settings}
        />
      )}
    </div>
  );
};

const MetricsOverview = ({ nodes }: { nodes: NodeDetail[] }) => {
  const { t } = useTranslation();
  const total = nodes.length;
  const onlineCount = nodes.filter((n) => n.online).length;
  const uniqueRegions = Array.from(
    new Set(nodes.map((n) => n.region).filter(Boolean))
  );
  const uniqueGroups = Array.from(
    new Set(nodes.map((n) => n.group).filter(Boolean))
  );

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
      <div className="p-3.5 rounded-lg border border-border bg-card shadow-2xs hover:border-foreground/30 transition-all">
        <div className="flex items-center justify-between text-muted-foreground mb-1">
          <span className="text-[11px] font-medium uppercase tracking-wider">
            {t("admin.overview.total_nodes", "接入节点")}
          </span>
          <Server size={14} className="opacity-70" />
        </div>
        <div className="text-xl font-bold font-mono tracking-tight text-foreground">
          {total}
        </div>
        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground mt-1">
          <span
            className={`inline-block w-1.5 h-1.5 rounded-full ${
              onlineCount > 0 ? "bg-emerald-500 animate-pulse" : "bg-rose-500"
            }`}
          />
          <span>
            {total > 0
              ? `${onlineCount} 在线 · ${total - onlineCount} 离线`
              : t("admin.overview.no_nodes", "等待节点接入")}
          </span>
        </div>
      </div>

      <div className="p-3.5 rounded-lg border border-border bg-card shadow-2xs hover:border-foreground/30 transition-all">
        <div className="flex items-center justify-between text-muted-foreground mb-1">
          <span className="text-[11px] font-medium uppercase tracking-wider">
            {t("admin.overview.regions", "地区覆盖")}
          </span>
          <Globe size={14} className="opacity-70" />
        </div>
        <div className="text-xl font-bold font-mono tracking-tight text-foreground">
          {uniqueRegions.length || (total > 0 ? 1 : 0)}
        </div>
        <div className="text-[11px] text-muted-foreground mt-1 truncate">
          {uniqueRegions.length > 0
            ? uniqueRegions.map((r) => r.toUpperCase()).join(", ")
            : t("common.global", "全球网络")}
        </div>
      </div>

      <div className="p-3.5 rounded-lg border border-border bg-card shadow-2xs hover:border-foreground/30 transition-all">
        <div className="flex items-center justify-between text-muted-foreground mb-1">
          <span className="text-[11px] font-medium uppercase tracking-wider">
            {t("admin.overview.groups", "分组数")}
          </span>
          <Folder size={14} className="opacity-70" />
        </div>
        <div className="text-xl font-bold font-mono tracking-tight text-foreground">
          {uniqueGroups.length || (total > 0 ? 1 : 0)}
        </div>
        <div className="text-[11px] text-muted-foreground mt-1">
          {t("admin.overview.groups_desc", "节点逻辑业务编组")}
        </div>
      </div>

      <div className="p-3.5 rounded-lg border border-border bg-card shadow-2xs hover:border-foreground/30 transition-all">
        <div className="flex items-center justify-between text-muted-foreground mb-1">
          <span className="text-[11px] font-medium uppercase tracking-wider">
            {t("admin.overview.system", "引擎架构")}
          </span>
          <Activity size={14} className="opacity-70" />
        </div>
        <div className="text-xl font-bold font-mono tracking-tight text-foreground">
          Lite Core
        </div>
        <div className="text-[11px] text-muted-foreground mt-1 font-mono">
          v1.1.0 · 高吞吐微核
        </div>
      </div>
    </div>
  );
};

const EmptyNodesGuide = () => {
  const { t } = useTranslation();
  return (
    <Flex
      direction="column"
      align="end"
      justify="start"
      style={{ minHeight: "60vh" }}
      pr="2"
      pt="1"
    >
      {/* 回转箭头指向右上角的“添加节点”按钮 */}
      <CornerRightUp
        size={72}
        strokeWidth={1.25}
        className="text-muted-foreground animate-bounce"
        style={{ marginRight: "1.5rem" }}
      />
      <Flex direction="column" align="end" gap="1" mt="2" mr="2">
        <Text size="4" weight="bold">
          {t("admin.nodeTable.emptyGuide.title", "还没有任何服务器")}
        </Text>
        <Text size="2" color="gray" align="right" style={{ maxWidth: "20rem" }}>
          {t(
            "admin.nodeTable.emptyGuide.description",
            "点击右上角的“添加节点”开始监控服务器。"
          )}
        </Text>
      </Flex>
    </Flex>
  );
};



const Header = ({
  searchTerm,
  setSearchTerm,
  selectedNodes,
  totalNodes,
}: {
  searchTerm: string;
  setSearchTerm: (term: string) => void;
  selectedNodes: string[];
  totalNodes: number;
}) => {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [loading, setLoading] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeVersion, setUpgradeVersion] = useState<string>("");
  const [upgrading, setUpgrading] = useState(false);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const handleAddNode = async (name: string | undefined) => {
    setDialogOpen(true);
    setLoading(true);
    try {
      await fetch("/api/admin/client/add", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: name || "" }),
      });
      refresh();
    } catch (error) {
      toast.error(
        `${t("common.error", "Error")}: ${
          error instanceof Error ? error.message : String(error)
        }`
      );
    } finally {
      setLoading(false);
      setDialogOpen(false);
    }
  };

  const openUpgradeDialog = async () => {
    setUpgradeOpen(true);
    setUpgradeVersion("");
    try {
      const res = await fetch("/api/admin/agent-asset-version");
      const data = await res.json();
      // REST 别名经 renderStandard 包装: {status, message, data: {version}}
      const ver = data?.data?.version || data?.result?.version || data?.version;
      if (typeof ver === "string" && ver) setUpgradeVersion(ver);
    } catch {
      // 面板版本查询失败时仍允许手动触发（服务端会用默认版本）
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
      // REST 别名经 renderStandard 包装: {status, message, data: {version, dispatched}}
      const dispatched =
        data?.data?.dispatched ?? data?.result?.dispatched ?? data?.dispatched;
      const version =
        data?.data?.version ??
        data?.result?.version ??
        data?.version ??
        upgradeVersion;
      toast.success(
        t("admin.nodeTable.upgradeSuccess", {
          defaultValue: `升级事件已下发（v${version}，${dispatched} 个节点）`,
          version,
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
    <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 pb-2 border-b border-border/40">
      <div>
        <div className="flex items-center gap-2.5">
          <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">
            {t("admin.nodeTable.nodeList")}
          </h1>
          <span className="px-2 py-0.5 text-xs font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
            {totalNodes}
          </span>
          {selectedNodes.length > 0 && (
            <span className="px-2 py-0.5 text-xs font-mono font-medium rounded-full bg-foreground text-background">
              {selectedNodes.length} selected
            </span>
          )}
        </div>
        <p className="text-xs text-muted-foreground mt-1">
          {t("admin.nodeTable.subtitle", "实时管理与监控所有已接入的主机探针与网络资产")}
        </p>
      </div>
      <div className="flex items-center gap-2.5 w-full sm:w-auto">
        <div className="relative flex-1 sm:w-64 flex items-center">
          <Search size={14} className="absolute left-3 text-muted-foreground pointer-events-none" />
          <input
            type="text"
            placeholder={t("admin.nodeTable.searchByName")}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full h-9 pl-9 pr-3 text-xs rounded-lg border border-border bg-card text-foreground placeholder:text-muted-foreground/60 outline-none focus:border-foreground/50 focus:ring-1 focus:ring-foreground/20 transition-all shadow-2xs"
          />
        </div>
        <Dialog.Root open={upgradeOpen} onOpenChange={setUpgradeOpen}>
          <Dialog.Trigger>
            <button
              onClick={() => openUpgradeDialog()}
              className="h-9 px-3.5 rounded-lg border border-border bg-card text-foreground font-medium text-xs flex items-center gap-1.5 shadow-sm hover:bg-muted active:scale-[0.98] transition-all cursor-pointer shrink-0"
            >
              <ArrowUpCircle size={14} strokeWidth={2.5} />
              <span>{t("admin.nodeTable.upgradeAgents")}</span>
            </button>
          </Dialog.Trigger>
          <Dialog.Content className="max-w-md">
            <Dialog.Title>{t("admin.nodeTable.upgradeAgents")}</Dialog.Title>
            <div className="mt-2 text-xs text-muted-foreground leading-relaxed">
              {t("admin.nodeTable.upgradeDescription", {
                defaultValue:
                  "将向全部节点下发升级事件。Agent 会从面板下载新版本并自动完成替换与重启。",
                version: upgradeVersion,
              })}
              {upgradeVersion ? (
                <div className="mt-2 font-mono text-foreground">
                  {t("admin.nodeTable.upgradeTargetVersion")}: v{upgradeVersion}
                </div>
              ) : null}
            </div>
            <Flex justify="end" gap="2" mt="4">
              <Dialog.Close>
                <Button variant="soft" color="gray" disabled={upgrading}>
                  {t("common.cancel", "Cancel")}
                </Button>
              </Dialog.Close>
              <Button onClick={() => handleUpgradeAgents()} disabled={upgrading}>
                {upgrading
                  ? t("admin.nodeTable.upgrading", "下发中...")
                  : t("admin.nodeTable.upgradeConfirm", "确认升级")}
              </Button>
            </Flex>
          </Dialog.Content>
        </Dialog.Root>
        <Dialog.Root open={dialogOpen} onOpenChange={setDialogOpen}>
          <Dialog.Trigger>
            <button
              onClick={() => setDialogOpen(true)}
              className="h-9 px-3.5 rounded-lg bg-foreground text-background font-medium text-xs flex items-center gap-1.5 shadow-sm hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shrink-0"
            >
              <Plus size={14} strokeWidth={2.5} />
              <span>{t("admin.nodeTable.addNode")}</span>
            </button>
          </Dialog.Trigger>
          <Dialog.Content className="max-w-md">
            <Dialog.Title>{t("admin.nodeTable.addNode")}</Dialog.Title>
            <div className="mt-2">
              <label className="text-xs font-medium text-muted-foreground block mb-1.5">
                {t("admin.nodeTable.nameOptional")}
              </label>
              <TextField.Root
                ref={inputRef}
                placeholder={t("admin.nodeTable.nameOptional")}
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleAddNode(inputRef.current?.value);
                  }
                }}
              />
            </div>
            <Flex justify="end" gap="2" mt="4">
              <Dialog.Close>
                <Button variant="soft" color="gray" disabled={loading}>
                  {t("common.cancel", "Cancel")}
                </Button>
              </Dialog.Close>
              <Button
                onClick={() => handleAddNode(inputRef.current?.value)}
                disabled={loading}
              >
                {t("admin.nodeTable.addNode")}
              </Button>
            </Flex>
          </Dialog.Content>
        </Dialog.Root>
      </div>
    </div>
  );
};

const SortableRow = ({
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
  const { attributes, listeners, setNodeRef, transform, transition } =
    useSortable({ id: node.uuid });
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };
  function copy(text: string) {
    navigator.clipboard.writeText(text);
    toast.success(t("copy_success"));
  }
  return (
    <TableRow ref={setNodeRef} style={style} className="hover:bg-muted/40 transition-colors h-14">
      <TableCell className="w-9 pl-3 pr-0 text-center">
        <div
          {...attributes}
          {...listeners}
          className={`cursor-grab p-1 rounded hover:bg-muted text-muted-foreground/40 hover:text-foreground transition-colors inline-flex items-center justify-center ${
            isMobile ? "touch-manipulation select-none" : ""
          }`}
          style={{
            touchAction: "none", // 禁用移动端的默认手势
            WebkitUserSelect: "none",
            userSelect: "none",
          }}
          title={
            isMobile
              ? t("admin.nodeTable.dragToReorder", "长按拖拽重新排序")
              : undefined
          }
        >
          <MenuIcon size={14} />
        </div>
      </TableCell>
      <TableCell className="w-9 px-1 text-center">
        <Checkbox
          checked={selectedNodes.includes(node.uuid)}
          onCheckedChange={(checked) => handleSelectNode(node.uuid, !!checked)}
        />
      </TableCell>
      <TableCell className="min-w-[190px] px-3">
        <DetailView node={node} />
      </TableCell>
      <TableCell className="w-48 px-3">
        <div className="flex flex-col gap-1">
          {node.ipv4 && (
            <div className="flex items-center gap-1.5 group">
              <span className="font-mono text-xs text-foreground/80 bg-muted/60 px-1.5 py-0.5 rounded border border-border/50 select-all">
                {node.ipv4}
              </span>
              <button
                type="button"
                onClick={() => copy(node.ipv4)}
                className="opacity-0 group-hover:opacity-100 p-0.5 text-muted-foreground hover:text-foreground transition-opacity cursor-pointer"
                title={t("common.copy")}
              >
                <Copy size={12} />
              </button>
            </div>
          )}
          {node.ipv6 && (
            <div className="flex items-center gap-1.5 group">
              <span
                className="font-mono text-[11px] text-muted-foreground/80 bg-muted/30 px-1.5 py-0.5 rounded border border-border/30 select-all truncate max-w-[180px]"
                title={node.ipv6}
              >
                {node.ipv6.length > 20
                  ? (() => {
                      const segments = node.ipv6.split(":");
                      return segments.length > 3
                        ? `${segments.slice(0, 2).join(":")}:...:${segments[segments.length - 1]}`
                        : node.ipv6;
                    })()
                  : node.ipv6}
              </span>
              <button
                type="button"
                onClick={() => copy(node.ipv6)}
                className="opacity-0 group-hover:opacity-100 p-0.5 text-muted-foreground hover:text-foreground transition-opacity cursor-pointer"
                title={t("common.copy")}
              >
                <Copy size={12} />
              </button>
            </div>
          )}
        </div>
      </TableCell>
      <TableCell className="w-20 px-2 text-center">
        <span className="inline-block font-mono text-[11px] px-2 py-0.5 rounded-full bg-muted text-muted-foreground border border-border/60">
          v{node.version || "1.0.6"}
        </span>
      </TableCell>
      <TableCell className="w-20 px-2 text-center">
        {node.group ? (
          <span className="inline-block text-xs px-2 py-0.5 rounded-md bg-muted/50 text-foreground/80 font-medium border border-border/40">
            {node.group}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground/40 font-mono">-</span>
        )}
      </TableCell>
      <TableCell className="w-24 px-2 text-center">
        {node.remark ? (
          <span className="inline-block text-xs text-muted-foreground truncate max-w-[80px]" title={node.remark}>
            {node.remark}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground/40 font-mono">-</span>
        )}
      </TableCell>
      <TableCell className="w-48 px-3">
        <PriceTags
          price={node.price}
          billing_cycle={node.billing_cycle}
          expired_at={node.expired_at}
          currency={node.currency}
          tags={node.tags || ""}
        />
      </TableCell>
      <TableCell className="w-32 px-2 text-center">
        <ActionButtons
          node={node}
          settings={settings}
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
}: {
  nodes: NodeDetail[];
  selectedNodes: string[];
  setSelectedNodes: (nodes: string[]) => void;
  settings: any;
}) => {
  const { t } = useTranslation();
  const sensors = useSensors(
    useSensor(MouseSensor, {
      // 需要按住 10px 距离才开始拖拽，避免与点击冲突
      activationConstraint: {
        distance: 10,
      },
    }),
    useSensor(TouchSensor, {
      // 移动端需要按住 5px 距离才开始拖拽，并且延迟 200ms，避免与滚动冲突
      activationConstraint: {
        delay: 200,
        tolerance: 5,
      },
    }),
    useSensor(KeyboardSensor, {})
  );
  // 添加 localNodes 状态，实现即时 UI 更新
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
    const reorderedNodes = Array.from(localNodes);
    const [reorderedItem] = reorderedNodes.splice(oldIndex, 1);
    reorderedNodes.splice(newIndex, 0, reorderedItem);

    // 立即更新 UI
    setLocalNodes(reorderedNodes);

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
      // 不再调用 refresh，以免覆盖本地排序
    } catch {
      toast.error(t("admin.nodeTable.errorRefreshNodeList"));
    }
  };

  // 更新全选逻辑，使用 localNodes
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
  return (
    <div
      className={`rounded-lg border border-border bg-card overflow-hidden ${
        isDragging ? "select-none" : ""
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
              <TableHead className="w-9 pl-3 pr-0 text-center" title={t("admin.nodeTable.dragToReorder", "长按拖拽重新排序")}>
                <GripVertical size={13} className="text-muted-foreground/30 mx-auto" />
              </TableHead>
              <TableHead className="w-9 px-1 text-center">
                <Checkbox
                  checked={
                    selectedNodes.length === localNodes.length &&
                    localNodes.length > 0
                  }
                  onCheckedChange={handleSelectAll}
                />
              </TableHead>
              <TableHead className="min-w-[190px] px-3 text-left">
                <span className="pl-[34px]">{t("admin.nodeTable.name")}</span>
              </TableHead>
              <TableHead className="w-48 px-3 text-left">{t("admin.nodeDetail.ipAddress")}</TableHead>
              <TableHead className="w-20 px-2 text-center">{t("admin.nodeDetail.clientVersion", "版本")}</TableHead>
              <TableHead className="w-20 px-2 text-center">{t("common.group")}</TableHead>
              <TableHead className="w-24 px-2 text-center">{t("admin.nodeEdit.remark", "备注")}</TableHead>
              <TableHead className="w-48 px-3 text-left">{t("admin.nodeTable.billing")}</TableHead>
              <TableHead className="w-32 px-2 text-center">{t("common.actions", "操作")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <SortableContext
              items={localNodes.map((node) => node.uuid)}
              strategy={verticalListSortingStrategy}
            >
              {localNodes.map((node) => (
                <SortableRow
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
}: {
  node: NodeDetail;
  settings: any;
}) => {
  return (
    <div className="flex items-center justify-center gap-1">
      <GenerateCommandButton
        settings={settings}
        nodeToken={node.token}
      />

      <EditButton node={node} />
      <BillingButton node={node} />
      <DeleteButton node={node} />
    </div>
  );
};

export default NodeDetailsPage;
function DeleteButton({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [open, setOpen] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const handleDelete = async () => {
    try {
      setDeleting(true);
      await fetch(`/api/admin/client/${node.uuid}/remove`, {
        method: "POST",
      });
      toast.success(`Delete ${node.name}`);
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
      <Dialog.Trigger>
        <IconButton variant="ghost" color="red" title={t("common.delete")}>
          <Trash2Icon size="18" />
        </IconButton>
      </Dialog.Trigger>
      <Dialog.Content>
        <Dialog.Title>{t("common.delete")}</Dialog.Title>
        <Dialog.Description className="text-xs text-muted-foreground mt-1">
          {t("common.confirm_delete")}
        </Dialog.Description>
        <Flex justify="end" gap="2" mt="4">
          <Dialog.Close>
            <Button variant="soft" color="gray">{t("common.cancel")}</Button>
          </Dialog.Close>
          <Button disabled={deleting} color="red" onClick={handleDelete}>
            {t("common.confirm_delete")}
          </Button>
        </Flex>
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
}: {
  settings: any;
  nodeToken: string;
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
    const args = buildAgentInstallArgs({
      endpoint: host,
      token: nodeToken,
      interval: enableInterval ? installOptions.interval : undefined,
      includeNics: enableIncludeNics ? installOptions.includeNics : undefined,
      excludeNics: enableExcludeNics ? installOptions.excludeNics : undefined,
    });
    const scriptUrl = `${host}/download/agent-install.sh`;
    return `curl -fsSL ${JSON.stringify(scriptUrl)} | bash -s -- ${quoteShellArgs(args)}`;
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(t("copy_success", "已复制到剪贴板"));
    } catch (err) {
      console.error("Failed to copy text: ", err);
    }
  };
  const { t } = useTranslation();
  return (
    <Dialog.Root>
      <Dialog.Trigger>
        <IconButton variant="ghost" title={t("admin.nodeTable.installCommand")}>
          <Download size="18" />
        </IconButton>
      </Dialog.Trigger>
      <Dialog.Content className="max-w-lg">
        <Dialog.Title>
          {t("admin.nodeTable.installCommand", "一键部署指令")}
        </Dialog.Title>
        <div className="flex flex-col gap-3.5 my-1">
          <div>
            <span className="text-xs font-medium text-muted-foreground block mb-2">
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
                    />
                  </div>
                )}
              </div>
            </div>
          </div>

          <div>
            <span className="text-xs font-medium text-muted-foreground block mb-1.5">
              {t("admin.nodeTable.generatedCommand", "生成的指令")}
            </span>
            <div className="rounded-lg border border-border/70 bg-muted/40 p-3 font-mono text-[11px] leading-relaxed break-all select-all text-foreground max-h-36 overflow-y-auto">
              {generateCommand()}
            </div>
          </div>

          <Flex justify="end" gap="2" mt="2">
            <Dialog.Close>
              <Button variant="soft" color="gray" type="button">
                {t("common.close", "关闭")}
              </Button>
            </Dialog.Close>
            <Button onClick={() => copyToClipboard(generateCommand())}>
              <Copy size={14} />
              {t("common.copy")}
            </Button>
          </Flex>
        </div>
      </Dialog.Content>
    </Dialog.Root>
  );
}

function EditButton({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const { refresh } = useNodeDetails();
  const nameRef = React.useRef<HTMLInputElement>(null);
  const groupRef = React.useRef<HTMLInputElement>(null);
  const tagsRef = React.useRef<HTMLInputElement>(null);
  const publicRemarkRef = React.useRef<HTMLTextAreaElement>(null);
  const privateRemarkRef = React.useRef<HTMLTextAreaElement>(null);
  const [hidden, setHidden] = useState(false);
  const [saving, setSaving] = useState(false);
  const [traffic_limit, setTrafficLimit] = useState(0);
  const [traffic_limit_type, setTrafficLimitType] = useState("sum");

  React.useEffect(() => {
    setHidden(node.hidden);
    setTrafficLimit(node.traffic_limit || 0);
    setTrafficLimitType(node.traffic_limit_type || "sum");
  }, [node.hidden, node.traffic_limit, node.traffic_limit_type]);

  const save = async () => {
    try {
      setSaving(true);
      await fetch(`/api/admin/client/${node.uuid}/edit`, {
        method: "POST",
        body: JSON.stringify({
          name: nameRef.current?.value,
          remark: privateRemarkRef.current?.value,
          public_remark: publicRemarkRef.current?.value,
          group: groupRef.current?.value,
          tags: tagsRef.current?.value,
          hidden,
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
      <Dialog.Trigger>
        <IconButton
          variant="ghost"
          title={t("admin.nodeEdit.editInfo", "编辑信息")}
        >
          <Pencil size="18" />
        </IconButton>
      </Dialog.Trigger>
      <Dialog.Content className="max-w-md">
        <Dialog.Title>{t("admin.nodeEdit.editInfo", "编辑信息")}</Dialog.Title>
        <div className="flex flex-col gap-3 my-1">
          <div>
            <label className="block mb-1 text-xs font-medium text-muted-foreground">
              {t("admin.nodeEdit.name", "名称")}
            </label>
            <TextField.Root
              defaultValue={node.name}
              placeholder={t("admin.nodeEdit.namePlaceholder", "请输入名称")}
              ref={nameRef}
            />
          </div>
          <div>
            <label className="block mb-1 text-xs font-medium text-muted-foreground">
              {t("admin.nodeEdit.token", "Token 令牌")}
            </label>
            <TextField.Root
              value={node.token}
              placeholder={t("admin.nodeEdit.tokenPlaceholder", "请输入 Token")}
              readOnly
            />
          </div>
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <div className="flex items-center gap-1 mb-1">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("common.tags")}
                </label>
                <Tips>
                  <span
                    dangerouslySetInnerHTML={{ __html: t("common.tagsTips") }}
                  />
                </Tips>
              </div>
              <TextField.Root defaultValue={node.tags} ref={tagsRef} placeholder="tag1,tag2" />
            </div>
            <div>
              <label className="block mb-1 text-xs font-medium text-muted-foreground">
                {t("common.group")}
              </label>
              <TextField.Root defaultValue={node.group} ref={groupRef} placeholder="default" />
            </div>
          </div>
          <div>
            <label className="block mb-1 text-xs font-medium text-muted-foreground">
              {t("admin.nodeEdit.remark", "私有备注")}
            </label>
            <TextArea
              defaultValue={node.remark}
              ref={privateRemarkRef}
              resize={"vertical"}
              style={{ minHeight: "60px" }}
              placeholder={t(
                "admin.nodeEdit.remarkPlaceholder",
                "请输入私有备注"
              )}
            />
          </div>
          <div>
            <label className="block mb-1 text-xs font-medium text-muted-foreground">
              {t("admin.nodeEdit.publicRemark", "公开备注")}
            </label>
            <TextArea
              defaultValue={node.public_remark}
              resize={"vertical"}
              style={{ minHeight: "60px" }}
              placeholder={t(
                "admin.nodeEdit.publicRemarkPlaceholder",
                "请输入公开备注"
              )}
              ref={publicRemarkRef}
            />
          </div>

          <div className="pt-2 border-t border-border/60">
            <div className="flex items-center justify-between py-1">
              <div className="flex flex-col">
                <span className="text-xs font-medium text-foreground">
                  {t("admin.nodeEdit.hidden")}
                </span>
                <span className="text-[11px] text-muted-foreground">
                  {t("admin.nodeEdit.hidden_description")}
                </span>
              </div>
              <Switch checked={hidden} onCheckedChange={setHidden} />
            </div>
          </div>

          <div className="pt-2 border-t border-border/60 space-y-2">
            <div className="text-xs font-medium text-foreground">
              {t("admin.nodeEdit.trafficLimit")}
            </div>
            <div className="grid grid-cols-2 gap-2.5">
              <div>
                <label className="block mb-1 text-[11px] font-medium text-muted-foreground">
                  {t("admin.nodeEdit.trafficLimitType")}
                </label>
                <Select.Root
                  value={traffic_limit_type}
                  onValueChange={setTrafficLimitType}
                >
                  <Select.Trigger className="w-full" />
                  <Select.Content>
                    <Select.Item value="sum">{t("admin.nodeEdit.trafficLimitType_sum")}</Select.Item>
                    <Select.Item value="max">{t("admin.nodeEdit.trafficLimitType_max")}</Select.Item>
                    <Select.Item value="min">{t("admin.nodeEdit.trafficLimitType_min")}</Select.Item>
                    <Select.Item value="up">{t("admin.nodeEdit.trafficLimitType_up")}</Select.Item>
                    <Select.Item value="down">{t("admin.nodeEdit.trafficLimitType_down")}</Select.Item>
                  </Select.Content>
                </Select.Root>
              </div>
              <div>
                <label className="block mb-1 text-[11px] font-medium text-muted-foreground">
                  {t("admin.nodeEdit.trafficLimit")}
                </label>
                <TextField.Root
                  defaultValue={formatBytes(traffic_limit || 0)}
                  placeholder="1 TB"
                  onChange={(e) => {
                    setTrafficLimit(stringToBytes(e.currentTarget.value));
                  }}
                  onBlur={(e) => {
                    e.currentTarget.value = formatBytes(traffic_limit);
                  }}
                />
              </div>
            </div>
          </div>
        </div>
        <Flex gap="2" justify="end" mt="4">
          <Dialog.Close>
            <Button variant="soft" color="gray" type="button">
              {t("common.cancel")}
            </Button>
          </Dialog.Close>
          <Button
            type="submit"
            disabled={saving}
            onClick={save}
          >
            {saving
              ? t("admin.nodeEdit.waiting", "等待...")
              : t("common.save", "保存")}
          </Button>
        </Flex>
      </Dialog.Content>
    </Dialog.Root>
  );
}

function DetailView({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  return (
    <Drawer direction={isMobile ? "bottom" : "right"}>
      <DrawerTrigger asChild>
        <div className="flex items-center gap-2.5 py-1 hover:underline cursor-pointer group">
          <div className="relative shrink-0 flex items-center justify-center">
            <Flag flag={node.region} size="6" />
            <span
              className={`absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full ring-2 ring-card ${
                node.online ? "bg-emerald-500" : "bg-rose-500"
              }`}
              title={node.online ? t("nodeCard.online", "在线") : t("nodeCard.offline", "离线")}
            />
          </div>
          <div className="flex flex-col min-w-0">
            <span className="font-medium text-sm text-foreground group-hover:text-primary transition-colors truncate max-w-[200px]" title={node.name}>
              {node.name}
            </span>
            <span className="text-[11px] text-muted-foreground/70 font-mono truncate max-w-[180px]">
              {node.os || "Linux"} {node.arch ? `· ${node.arch}` : ""}
            </span>
          </div>
        </div>
      </DrawerTrigger>
      <DrawerContent>
        <DrawerHeader className="gap-1">
          <div className="flex items-center gap-2">
            <DrawerTitle>{node.name}</DrawerTitle>
            <span
              className={`px-2 py-0.5 text-xs font-medium rounded-full ${
                node.online
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
                  : "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20"
              }`}
            >
              {node.online ? t("nodeCard.online", "在线") : t("nodeCard.offline", "离线")}
            </span>
          </div>
          <DrawerDescription>
            {t("admin.nodeDetail.machineDetail", "机器详细信息")}
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex flex-col gap-4 overflow-y-auto px-4 text-sm">
          <form className="flex flex-col gap-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-ip">
                  {t("admin.nodeDetail.ipAddress", "IP 地址")}
                </label>
                <div className="flex flex-col gap-1">
                  {node.ipv4 && (
                    <div className="flex items-center gap-1">
                      <span
                        id="detail-ipv4"
                        className="bg-muted px-3 py-2 rounded border flex-1 min-w-0 select-text"
                      >
                        {node.ipv4}
                      </span>
                      <IconButton
                        variant="ghost"
                        className="size-5"
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(node.ipv4!);
                        }}
                      >
                        <Copy size={16} />
                      </IconButton>
                    </div>
                  )}
                  {node.ipv6 && (
                    <div className="flex items-center gap-1">
                      <span
                        id="detail-ipv6"
                        className="bg-muted px-3 py-2 rounded border flex-1 min-w-0 select-text"
                      >
                        {node.ipv6}
                      </span>
                      <IconButton
                        variant="ghost"
                        className="size-5"
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(node.ipv6!);
                        }}
                      >
                        <Copy size={16} />
                      </IconButton>
                    </div>
                  )}
                </div>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-version">
                  {t("admin.nodeDetail.clientVersion", "客户端版本")}
                </label>
                <span
                  id="detail-version"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.version || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-os">
                  {t("admin.nodeDetail.os", "操作系统")}
                </label>
                <span
                  id="detail-os"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.os || <span className="text-muted-foreground">-</span>}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-arch">
                  {t("admin.nodeDetail.arch", "架构")}
                </label>
                <span
                  id="detail-arch"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.arch || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-cpu_name">
                  {t("admin.nodeDetail.cpu", "CPU")}
                </label>
                <span
                  id="detail-cpu_name"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.cpu_name || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-cpu_cores">
                  {t("admin.nodeDetail.cpuCores", "CPU 核心数")}
                </label>
                <span
                  id="detail-cpu_cores"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.cpu_cores?.toString() || (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-mem_total">
                  {t("admin.nodeDetail.memTotal", "总内存 (Bytes)")}
                </label>
                <span
                  id="detail-mem_total"
                  className="bg-muted px-3 py-2 rounded border select-text"
                  title={
                    node.mem_total ? String(node.mem_total) + " Bytes" : "-"
                  }
                >
                  {formatBytes(node.mem_total)}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-disk_total">
                  {t("admin.nodeDetail.diskTotal", "总磁盘空间 (Bytes)")}
                </label>
                <span
                  id="detail-disk_total"
                  className="bg-muted px-3 py-2 rounded border select-text"
                  title={
                    node.disk_total ? String(node.disk_total) + " Bytes" : "-"
                  }
                >
                  {formatBytes(node.disk_total)}
                </span>
              </div>
            </div>
            <div className="flex flex-col gap-3">
              <label htmlFor="detail-gpu_name">
                {t("admin.nodeDetail.gpu", "GPU")}
              </label>
              <span
                id="detail-gpu_name"
                className="bg-muted px-3 py-2 rounded border select-text"
              >
                {node.gpu_name || (
                  <span className="text-muted-foreground">-</span>
                )}
              </span>
            </div>
            <div className="flex flex-col gap-3">
              <label htmlFor="detail-uuid">
                {t("admin.nodeDetail.uuid", "UUID")}
              </label>
              <span
                id="detail-uuid"
                className="bg-muted px-3 py-2 rounded border select-text"
              >
                {node.uuid || <span className="text-muted-foreground">-</span>}
              </span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-createdAt">
                  {t("admin.nodeDetail.createdAt", "创建时间")}
                </label>
                <span
                  id="detail-createdAt"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.created_at ? (
                    new Date(node.created_at).toLocaleString()
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
              <div className="flex flex-col gap-3">
                <label htmlFor="detail-updatedAt">
                  {t("admin.nodeDetail.updatedAt", "更新时间")}
                </label>
                <span
                  id="detail-updatedAt"
                  className="bg-muted px-3 py-2 rounded border select-text"
                >
                  {node.updated_at ? (
                    new Date(node.updated_at).toLocaleString()
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              </div>
            </div>
          </form>
        </div>
        <DrawerFooter>
          <DrawerClose asChild>
            <Button>{t("admin.nodeDetail.done", "完成")}</Button>
          </DrawerClose>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  );
}

const CURRENCY_SELECT_OPTIONS = [
  { label: "¥ 人民币 (CNY / RMB)", value: "¥" },
  { label: "$ 美元 (USD)", value: "$" },
  { label: "€ 欧元 (EUR)", value: "€" },
  { label: "£ 英镑 (GBP)", value: "£" },
  { label: "HK$ 港币 (HKD)", value: "HK$" },
  { label: "JP¥ 日元 (JPY)", value: "JP¥" },
  { label: "₩ 韩元 (KRW)", value: "₩" },
  { label: "S$ 新加坡元 (SGD)", value: "S$" },
  { label: "A$ 澳大利亚元 (AUD)", value: "A$" },
  { label: "C$ 加拿大元 (CAD)", value: "C$" },
  { label: "CHF 瑞士法郎 (CHF)", value: "CHF" },
  { label: "RM 马来西亚林吉特 (MYR)", value: "RM" },
  { label: "฿ 泰铢 (THB)", value: "฿" },
  { label: "₽ 俄罗斯卢布 (RUB)", value: "₽" },
  { label: "NT$ 新台币 (TWD)", value: "NT$" },
];

const getCurrencySelectOptions = (val: string) => {
  if (val && !CURRENCY_SELECT_OPTIONS.some((o) => o.value === val)) {
    return [{ label: `${val} (自定义)`, value: val }, ...CURRENCY_SELECT_OPTIONS];
  }
  return CURRENCY_SELECT_OPTIONS;
};

function BillingButton({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [billingCycle, setBillingCycle] = React.useState<string>(
    node.billing_cycle.toString()
  );
  const [autoRenewal, setAutoRenewal] = React.useState<boolean>(
    node.auto_renewal || false
  );
  const [currency, setCurrency] = React.useState<string>(node.currency || "$");
  const [premiumCurrency, setPremiumCurrency] = React.useState<string>(
    node.premium_currency || "¥"
  );

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setSaving(true);
      const formData = new FormData(e.target as HTMLFormElement);
      const priceValue = (formData.get("price") as string) || "0";
      const premiumValue = (formData.get("premium") as string) || "0";

      const price = parseFloat(priceValue);
      const premium = premiumValue.trim() === "" ? 0 : parseFloat(premiumValue);

      if (isNaN(price) || (price < 0 && price !== -1)) {
        toast.error(t("admin.nodeTable.invalidPrice"));
        return;
      }
      if (isNaN(premium) || premium < 0) {
        toast.error(t("admin.nodeTable.invalidPremium"));
        return;
      }
      const billingCycleValue = parseInt(
        (formData.get("billingCycle") as string) || "30"
      );
      const expiredAtValue = (formData.get("expiredAt") as string) || "";
      const expiredAt = expiredAtValue
        ? new Date(`${expiredAtValue}T00:00:00Z`).toISOString()
        : null;
      const currencyValue = currency || "$";
      const premiumCurrencyValue = premiumCurrency || "¥";

      await fetch(`/api/admin/client/${node.uuid}/edit`, {
        method: "POST",
        body: JSON.stringify({
          price,
          premium,
          premium_currency: premiumCurrencyValue,
          billing_cycle: billingCycleValue,
          expired_at: expiredAt,
          currency: currencyValue,
          auto_renewal: autoRenewal,
        }),
        headers: {
          "Content-Type": "application/json",
        },
      });
      refresh();
      setOpen(false);
    } catch (error) {
      toast.error("Failed to save billing information:" + error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger>
        <IconButton
          variant="ghost"
          title={t("admin.nodeTable.billing", "账单")}
        >
          <CircleDollarSign size="18" />
        </IconButton>
      </Dialog.Trigger>
      <Dialog.Content className="max-w-md">
        <Dialog.Title>{t("admin.nodeTable.billing", "账单")}</Dialog.Title>
        <form onSubmit={handleSave} className="flex flex-col gap-3 my-1">
          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <div className="flex items-center gap-1 mb-1">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.nodeTable.price")}
                </label>
                <Tips><span>{t("admin.nodeTable.priceTips")}</span></Tips>
              </div>
              <TextField.Root name="price" defaultValue={node.price} placeholder="0.00" />
            </div>

            <div>
              <div className="flex items-center gap-1 mb-1">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.nodeTable.currency", "价格货币")}
                </label>
                <Tips><span>{t("admin.nodeTable.currencyTips")}</span></Tips>
              </div>
              <Select.Root value={currency} onValueChange={setCurrency}>
                <Select.Trigger className="w-full" />
                <Select.Content>
                  {getCurrencySelectOptions(currency).map((c) => (
                    <Select.Item key={c.value} value={c.value}>
                      {c.label}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <div className="flex items-center gap-1 mb-1">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.nodeTable.premium")}
                </label>
                <Tips><span>{t("admin.nodeTable.premiumTips")}</span></Tips>
              </div>
              <TextField.Root name="premium" defaultValue={node.premium} placeholder="0.00" />
            </div>

            <div>
              <div className="flex items-center gap-1 mb-1">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.nodeTable.premiumCurrency", "溢价货币")}
                </label>
                <Tips><span>{t("admin.nodeTable.premiumCurrencyTips", "溢价独立结算币种，默认为 ¥")}</span></Tips>
              </div>
              <Select.Root value={premiumCurrency} onValueChange={setPremiumCurrency}>
                <Select.Trigger className="w-full" />
                <Select.Content>
                  {getCurrencySelectOptions(premiumCurrency).map((c) => (
                    <Select.Item key={c.value} value={c.value}>
                      {c.label}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select.Root>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-2.5">
            <div>
              <div className="flex items-center gap-1 mb-1">
                <label className="text-xs font-medium text-muted-foreground">
                  {t("admin.nodeTable.billingCycle")}
                </label>
                <Tips><span dangerouslySetInnerHTML={{ __html: t("admin.nodeTable.billingCycleTips") }}></span></Tips>
              </div>
              <SelectOrInput
                options={[
                  { label: t("common.monthly"), value: "30" },
                  { label: t("common.quarterly"), value: "92" },
                  { label: t("common.semi_annual"), value: "184" },
                  { label: t("common.annual"), value: "365" },
                  { label: t("common.biennial"), value: "730" },
                  { label: t("common.triennial"), value: "1095" },
                  { label: t("common.quinquennial"), value: "1825" },
                  { label: t("common.once"), value: "-1" },
                ]}
                type="number"
                name="billingCycle"
                value={billingCycle === "0" ? "" : billingCycle}
                onChange={setBillingCycle}
              />
            </div>

            <div>
              <label className="block mb-1 text-xs font-medium text-muted-foreground">
                {t("admin.nodeTable.expiredAt")}
              </label>
              <TextField.Root
                name="expiredAt"
                defaultValue={
                  node.expired_at
                    ? new Date(node.expired_at).toISOString().slice(0, 10)
                    : "0001-01-01"
                }
                type="date"
              >
                <TextField.Slot side="right">
                  <Button
                    type="button"
                    variant="ghost"
                    size="1"
                    onClick={() => {
                      const dateInput = document.querySelector(
                        'input[name="expiredAt"]'
                      ) as HTMLInputElement;
                      if (dateInput) {
                        const futureDate = new Date();
                        futureDate.setFullYear(futureDate.getFullYear() + 200);
                        dateInput.value = futureDate.toISOString().slice(0, 10);
                      }
                    }}
                  >
                    {t("admin.nodeTable.setToLongTerm", "长期")}
                  </Button>
                </TextField.Slot>
              </TextField.Root>
            </div>
          </div>

          <div className="pt-2 border-t border-border/60">
            <div className="flex items-center justify-between py-1">
              <div className="flex flex-col">
                <span className="text-xs font-medium text-foreground">
                  {t("admin.nodeTable.autoRenewal")}
                </span>
                <span className="text-[11px] text-muted-foreground">
                  {t("admin.nodeTable.autoRenewalDescription")}
                </span>
              </div>
              <Switch checked={autoRenewal} onCheckedChange={setAutoRenewal} />
            </div>
          </div>

          <Flex justify="end" gap="2" mt="4">
            <Dialog.Close>
              <Button variant="soft" color="gray" type="button">
                {t("common.cancel")}
              </Button>
            </Dialog.Close>
            <Button type="submit" disabled={saving}>
              {t("common.save")}
            </Button>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}
