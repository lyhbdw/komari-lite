import { quoteShellArgs } from "@/utils/shellQuote";
import { buildAgentInstallArgs } from "@/utils/agentInstallCommand";
import { copyToClipboard } from "@/utils/clipboard";
import type { BatchResult } from "@/utils/batchNodes";
import { remainingSelection, runBatchNodes } from "@/utils/batchNodes";
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
  DropdownMenu,
} from "@/components/ui/radix-shim";
import {
  CircleDollarSign,
  Copy,
  CornerRightUp,
  Download,
  Pencil,
  Plus,
  Search,
  Trash2Icon,
  ArrowUpCircle,
  AlertTriangle,
  CalendarClock,
  CalendarCheck,
  ArrowUpDown,
  ArrowUp,
  ArrowDown,
  CheckSquare,
  X,
  Layers,
  GripVertical,
  ChevronDown,
  Check,
} from "lucide-react";
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
  verticalListSortingStrategy,
  useSortable,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useTranslation } from "react-i18next";
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
import {
  calculateTrafficAndSpeedFromStatusMap,
  formatBytes,
  formatBytesPerSecond,
  stringToBytes,
} from "@/utils/unitHelper";
import * as financeHelper from "@/utils/financeHelper";
import {
  calculateNextRenewalDate,
  isNodeExpiringSoon,
} from "../../../../theme/src/utils/billingCore";
import {
  isNodeInGroup,
  isNodeUngrouped,
  parseNodeGroups,
} from "../../../../theme/src/utils/groupCore";
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
  const [selectedGroup, setSelectedGroup] = useState<string>("all");
  const [selectedStatus, setSelectedStatus] = useState<"all" | "online" | "offline" | "expiring">("all");
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);

  type SortField = "name" | "ip" | "group" | "billing" | "none";
  type SortOrder = "asc" | "desc";
  const [sortField, setSortField] = useState<SortField>("none");
  const [sortOrder, setSortOrder] = useState<SortOrder>("asc");

  // 批量操作弹窗状态
  const [batchGroupOpen, setBatchGroupOpen] = useState(false);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);

  const handleSort = (field: "name" | "ip" | "group" | "billing") => {
    if (sortField !== field) {
      setSortField(field);
      setSortOrder("asc");
    } else if (sortOrder === "asc") {
      setSortOrder("desc");
    } else {
      setSortField("none");
      setSortOrder("asc");
    }
  };

  const handleResetSort = () => {
    setSortField("none");
    setSortOrder("asc");
  };

  // 判定是否为 7 天内即将到期机器
  const isExpiringSoon = React.useCallback(
    (node: NodeDetail) => isNodeExpiringSoon(node.expired_at),
    []
  );

  const expiringCount = React.useMemo(() => {
    if (!Array.isArray(nodeDetail)) return 0;
    return nodeDetail.filter(isExpiringSoon).length;
  }, [nodeDetail, isExpiringSoon]);

  const availableGroups = React.useMemo(() => {
    if (!Array.isArray(nodeDetail)) return [];
    const groups = new Set<string>();
    nodeDetail.forEach((node) => {
      parseNodeGroups(node.group).forEach((group) => groups.add(group));
    });
    return Array.from(groups);
  }, [nodeDetail]);

  const filteredNodes = React.useMemo(() => {
    if (!Array.isArray(nodeDetail)) return [];
    return nodeDetail
      .filter((node) => {
        // 状态筛选：全部 / 在线 / 离线 / 待续费
        if (selectedStatus === "online" && !node.online) return false;
        if (selectedStatus === "offline" && node.online) return false;
        if (selectedStatus === "expiring" && !isExpiringSoon(node)) return false;

        // 分组筛选
        if (selectedGroup === "_ungrouped_") {
          if (!isNodeUngrouped(node.group)) return false;
        } else if (!isNodeInGroup(node.group, selectedGroup)) {
          return false;
        }

        // 文本搜索
        if (!searchTerm) return true;
        const lower = searchTerm.toLowerCase();
        if (lower === "online" || lower === "在线") return !!node.online;
        if (lower === "offline" || lower === "离线") return !node.online;
        if (lower === "expiring" || lower === "临期" || lower === "待续费") return isExpiringSoon(node);
        return (
          node.name.toLowerCase().includes(lower) ||
          (node.ipv4 && node.ipv4.includes(searchTerm)) ||
          (node.ipv6 && node.ipv6.toLowerCase().includes(lower)) ||
          (node.public_remark && node.public_remark.toLowerCase().includes(lower)) ||
          (node.tags && node.tags.toLowerCase().includes(lower)) ||
          (node.group && node.group.toLowerCase().includes(lower))
        );
      })
      .sort((a, b) => {
        // 待续费模式下优先按到期时间排序
        if (selectedStatus === "expiring") {
          const aTime = a.expired_at ? new Date(a.expired_at).getTime() : Infinity;
          const bTime = b.expired_at ? new Date(b.expired_at).getTime() : Infinity;
          return aTime - bTime;
        }

        // 自定义列排序
        if (sortField === "name") {
          return sortOrder === "asc"
            ? a.name.localeCompare(b.name, undefined, { numeric: true })
            : b.name.localeCompare(a.name, undefined, { numeric: true });
        }
        if (sortField === "ip") {
          const aIp = a.ipv4 || a.ipv6 || "";
          const bIp = b.ipv4 || b.ipv6 || "";
          return sortOrder === "asc"
            ? aIp.localeCompare(bIp, undefined, { numeric: true })
            : bIp.localeCompare(aIp, undefined, { numeric: true });
        }
        if (sortField === "group") {
          const aGrp = a.group || "";
          const bGrp = b.group || "";
          return sortOrder === "asc" ? aGrp.localeCompare(bGrp) : bGrp.localeCompare(aGrp);
        }
        if (sortField === "billing") {
          const aTime = a.expired_at ? new Date(a.expired_at).getTime() : Infinity;
          const bTime = b.expired_at ? new Date(b.expired_at).getTime() : Infinity;
          if (aTime !== bTime) {
            return sortOrder === "asc" ? aTime - bTime : bTime - aTime;
          }
          return sortOrder === "asc" ? a.price - b.price : b.price - a.price;
        }

        return a.weight - b.weight;
      });
  }, [nodeDetail, selectedStatus, selectedGroup, searchTerm, isExpiringSoon, sortField, sortOrder]);

  // 批量操作处理器
  const handleBatchCopyIPs = async () => {
    if (!Array.isArray(nodeDetail)) return;
    const ips = selectedNodes
      .map((uuid) => {
        const n = nodeDetail.find((node) => node.uuid === uuid);
        return n?.ipv4 || n?.ipv6 || "";
      })
      .filter(Boolean);
    if (ips.length === 0) {
      toast.error("未找到有效 IP");
      return;
    }
    await copyToClipboard(ips.join("\n"), {
      successMessage: `已复制 ${ips.length} 台服务器 IP 到剪贴板`,
    });
  };

  const applyBatchResult = (result: BatchResult, label: string) => {
    setSelectedNodes(current => remainingSelection(current, result));
    if (result.failed.length) {
      toast.error(`${label}：成功 ${result.succeeded.length} 台，失败 ${result.failed.length} 台；失败项已保留选择。${result.failed.map(item => `${item.uuid}: ${item.message}`).join("；")}`);
    } else if (result.succeeded.length) {
      toast.success(`${label}成功：${result.succeeded.length} 台服务器`);
    }
    if (result.succeeded.length) refresh();
  };

  const handleBatchGroupConfirm = async (newGroup: string) => {
    applyBatchResult(await runBatchNodes(selectedNodes, "group", newGroup), "批量修改分组");
  };

  const handleBatchDeleteConfirm = async () => {
    applyBatchResult(await runBatchNodes(selectedNodes, "delete"), "批量删除");
  };

  useEffect(() => {
    if (account && !account.logged_in)
      return;

    const tick = () => {
      if (typeof document === "undefined" || !document.hidden) {
        refresh();
      }
    };
    const handleVisibilityChange = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        refresh();
      }
    };

    const interval = setInterval(tick, 5000);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      clearInterval(interval);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [refresh, account]);

  if (account && !account.logged_in) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[50vh] text-center p-6">
        <div className="w-12 h-12 rounded-2xl bg-foreground text-background flex items-center justify-center p-2.5 mb-4 shadow-sm select-none">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" fill="none" className="size-full">
            <rect x="36" y="36" width="440" height="440" rx="96" stroke="currentColor" strokeWidth="40" fill="none" />
            <path d="M 148 168 L 246 256 L 148 344" stroke="currentColor" strokeWidth="46" strokeLinecap="round" strokeLinejoin="round" />
            <rect x="276" y="322" width="112" height="44" rx="12" fill="currentColor" />
          </svg>
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
    <div className="km-page-admin-index space-y-4">
      {!isEmpty && (
        <MetricsOverview
          nodes={nodeDetail || []}
          selectedStatus={selectedStatus}
          setSelectedStatus={setSelectedStatus}
          expiringCount={expiringCount}
        />
      )}

      <TableToolbar
        searchTerm={searchTerm}
        setSearchTerm={setSearchTerm}
        selectedGroup={selectedGroup}
        setSelectedGroup={setSelectedGroup}
        availableGroups={availableGroups}
        selectedStatus={selectedStatus}
        setSelectedStatus={setSelectedStatus}
        totalNodes={nodeDetail?.length || 0}
        onlineCount={nodeDetail?.filter((n) => n.online).length || 0}
        offlineCount={nodeDetail?.filter((n) => !n.online).length || 0}
        expiringCount={expiringCount}
        selectedNodes={selectedNodes}
      />

      {/* 批量操作浮动工具栏 */}
      {selectedNodes.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2.5 p-2.5 sm:p-3 rounded-lg border border-primary/25 bg-card shadow-xs text-xs">
          <div className="flex items-center gap-2">
            <CheckSquare size={15} className="text-foreground" />
            <span className="font-semibold text-foreground">
              已选中 {selectedNodes.length} 台服务器
            </span>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <button
              type="button"
              onClick={() => setBatchGroupOpen(true)}
              className="px-2.5 py-1 rounded-md border border-border bg-card text-foreground font-medium hover:bg-muted transition-colors flex items-center gap-1.5 cursor-pointer shadow-2xs"
            >
              <Layers size={13} />
              <span>设置分组</span>
            </button>
            <button
              type="button"
              onClick={handleBatchCopyIPs}
              className="px-2.5 py-1 rounded-md border border-border bg-card text-foreground font-medium hover:bg-muted transition-colors flex items-center gap-1.5 cursor-pointer shadow-2xs"
            >
              <Copy size={13} />
              <span>复制选中 IP</span>
            </button>
            <button
              type="button"
              onClick={() => setBatchDeleteOpen(true)}
              className="px-2.5 py-1 rounded-md border border-destructive/30 bg-destructive/10 text-destructive font-medium hover:bg-destructive/20 transition-colors flex items-center gap-1.5 cursor-pointer"
            >
              <Trash2Icon size={13} />
              <span>批量删除</span>
            </button>
            <button
              type="button"
              onClick={() => setSelectedNodes([])}
              className="px-2 py-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-colors flex items-center gap-1 cursor-pointer"
            >
              <X size={13} />
              <span>取消选择</span>
            </button>
          </div>
        </div>
      )}

      <BatchGroupDialog
        open={batchGroupOpen}
        onOpenChange={setBatchGroupOpen}
        selectedCount={selectedNodes.length}
        onConfirm={handleBatchGroupConfirm}
        availableGroups={availableGroups}
      />

      <BatchDeleteDialog
        open={batchDeleteOpen}
        onOpenChange={setBatchDeleteOpen}
        selectedCount={selectedNodes.length}
        onConfirm={handleBatchDeleteConfirm}
      />

      {isEmpty ? (
        <EmptyNodesGuide />
      ) : (
        <NodeTable
          nodes={filteredNodes}
          selectedNodes={selectedNodes}
          setSelectedNodes={setSelectedNodes}
          settings={settings}
          sortField={sortField}
          sortOrder={sortOrder}
          onSort={handleSort}
          onResetSort={handleResetSort}
        />
      )}
    </div>
  );
};

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

const MetricsOverview = ({
  nodes,
  selectedStatus,
  setSelectedStatus,
  expiringCount,
}: {
  nodes: NodeDetail[];
  selectedStatus: "all" | "online" | "offline" | "expiring";
  setSelectedStatus: (status: "all" | "online" | "offline" | "expiring") => void;
  expiringCount: number;
}) => {
  const { t } = useTranslation();

  const [trafficStats, setTrafficStats] = useState<{
    totalUp: number;
    totalDown: number;
    totalBytes: number;
    speedUp: number;
    speedDown: number;
    isLoading: boolean;
  }>({
    totalUp: 0,
    totalDown: 0,
    totalBytes: 0,
    speedUp: 0,
    speedDown: 0,
    isLoading: true,
  });

  const [exchangeRates, setExchangeRates] = useState<financeHelper.ExchangeRates>(
    () => financeHelper.getInitialExchangeRates()
  );
  const [baseCurrency, setBaseCurrency] = useState<financeHelper.CurrencyCode>(
    () => financeHelper.getStoredFinanceCurrency()
  );
  const [excludeFreeNodes, setExcludeFreeNodes] = useState<boolean>(() =>
    financeHelper.shouldExcludeFreeNodes()
  );

  useEffect(() => {
    let active = true;
    const syncFinancePrefs = () => {
      setExcludeFreeNodes(financeHelper.shouldExcludeFreeNodes());
      setBaseCurrency(financeHelper.getStoredFinanceCurrency());
    };
    syncFinancePrefs();
    financeHelper.getDailyExchangeRates().then(({ rates }) => {
      if (active) {
        setExchangeRates(rates);
      }
    });
    window.addEventListener("storage", syncFinancePrefs);
    return () => {
      active = false;
      window.removeEventListener("storage", syncFinancePrefs);
    };
  }, []);

  const targetExchangeRate = exchangeRates[baseCurrency] || 1;

  const remainingStats = React.useMemo(() => {
    if (!Array.isArray(nodes)) {
      const zeroFmt = financeHelper.formatFinanceAmount(0, baseCurrency);
      const zeroStr = `${zeroFmt.symbol}${zeroFmt.value}`;
      return { total: zeroStr, base: zeroStr, premium: zeroStr, hasPremium: false };
    }
    const totalRemaining =
      financeHelper.calculateTotalRemainingValueCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const baseRemaining =
      financeHelper.calculateTotalBaseRemainingValueCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const totalPremium =
      financeHelper.calculateTotalPremiumCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const fmtTotal = financeHelper.formatFinanceAmount(totalRemaining, baseCurrency);
    const fmtBase = financeHelper.formatFinanceAmount(baseRemaining, baseCurrency);
    const fmtPrem = financeHelper.formatFinanceAmount(totalPremium, baseCurrency);
    return {
      total: `${fmtTotal.symbol}${fmtTotal.value}`,
      base: `${fmtBase.symbol}${fmtBase.value}`,
      premium: `${fmtPrem.symbol}${fmtPrem.value}`,
      hasPremium: totalPremium > 0,
    };
  }, [nodes, exchangeRates, excludeFreeNodes, baseCurrency, targetExchangeRate]);

  const monthlyCostStats = React.useMemo(() => {
    if (!Array.isArray(nodes)) {
      const zeroFmt = financeHelper.formatFinanceAmount(0, baseCurrency);
      return {
        totalCNY: `${zeroFmt.symbol}${zeroFmt.value}/月`,
        yearCNY: `年均 ${zeroFmt.symbol}${zeroFmt.value}`,
      };
    }
    const totalAmount =
      financeHelper.calculateTotalMonthlyAverageCostCNY(
        nodes,
        exchangeRates,
        excludeFreeNodes
      ) * targetExchangeRate;
    const fmtTotal = financeHelper.formatFinanceAmount(totalAmount, baseCurrency);
    const fmtYear = financeHelper.formatFinanceAmount(totalAmount * 12, baseCurrency);
    return {
      totalCNY: `${fmtTotal.symbol}${fmtTotal.value}/月`,
      yearCNY: `年均 ${fmtYear.symbol}${fmtYear.value}`,
    };
  }, [nodes, exchangeRates, excludeFreeNodes, baseCurrency, targetExchangeRate]);

  const nodeUuidsRef = React.useRef<string[]>([]);
  useEffect(() => {
    nodeUuidsRef.current = Array.isArray(nodes)
      ? nodes.map((n) => n.uuid).filter(Boolean)
      : [];
  }, [nodes]);

  useEffect(() => {
    let isMounted = true;
    let inFlight = false;
    const fetchTraffic = async () => {
      if (inFlight || (typeof document !== "undefined" && document.hidden))
        return;
      inFlight = true;
      try {
        const res = await fetch("/api/rpc2", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            jsonrpc: "2.0",
            id: 1,
            method: "common:getNodesLatestStatus",
            params: [],
          }),
        });

        if (!res.ok) return;
        const data = await res.json();
        const stats = calculateTrafficAndSpeedFromStatusMap(
          data.result,
          nodeUuidsRef.current
        );

        if (isMounted) {
          setTrafficStats({
            ...stats,
            isLoading: false,
          });
        }
      } catch {
        // ignore
      } finally {
        inFlight = false;
      }
    };

    fetchTraffic();
    const handleVisibility = () => {
      if (typeof document !== "undefined" && !document.hidden) {
        fetchTraffic();
      }
    };
    const timer = setInterval(fetchTraffic, 5000);
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      isMounted = false;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

  const isExpiringActive = selectedStatus === "expiring";

  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
      {/* 1. 资产与预算 */}
      <div className="p-3.5 rounded-xl border border-border/70 bg-card shadow-2xs hover:border-foreground/25 transition-all flex flex-col justify-between h-[84px]">
        <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
          <span className="tracking-tight">{t("admin.overview.financial_overview", "资产与预算")}</span>
          <div className="w-6 h-6 rounded-md bg-muted/60 text-muted-foreground flex items-center justify-center shrink-0">
            <CircleDollarSign size={13} />
          </div>
        </div>
        <div className="flex items-baseline justify-between mt-1">
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-bold font-mono tracking-tight text-foreground">
              {remainingStats.total}
            </span>
            <span className="text-[11px] text-muted-foreground">剩余</span>
          </div>
          <div className="px-2 py-0.5 rounded-md bg-muted/50 border border-border/40 text-[11px] font-mono text-muted-foreground flex items-center gap-1">
            <span>支出</span>
            <span className="text-foreground/90 font-medium">{monthlyCostStats.totalCNY}</span>
          </div>
        </div>
      </div>

      {/* 2. 流量与速率 */}
      <div className="p-3.5 rounded-xl border border-border/70 bg-card shadow-2xs hover:border-foreground/25 transition-all flex flex-col justify-between h-[84px]">
        <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
          <span className="tracking-tight">{t("admin.overview.traffic_total", "累计流量与速率")}</span>
          <div className="w-6 h-6 rounded-md bg-muted/60 text-muted-foreground flex items-center justify-center shrink-0">
            <ArrowUpDown size={13} />
          </div>
        </div>
        <div className="flex items-baseline justify-between mt-1">
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-bold font-mono tracking-tight text-foreground">
              {trafficStats.isLoading && trafficStats.totalBytes === 0
                ? "..."
                : formatBytes(trafficStats.totalBytes)}
            </span>
            <span className="text-[11px] text-muted-foreground">累计</span>
          </div>
          <div
            className="px-2 py-0.5 rounded-md bg-muted/50 border border-border/40 text-[11px] font-mono text-muted-foreground flex items-center gap-1.5 cursor-help"
            title={`累计上行: ${formatBytes(trafficStats.totalUp)} · 累计下行: ${formatBytes(trafficStats.totalDown)}`}
          >
            <span className="flex items-center gap-0.5">
              <span className="opacity-60">↑</span>
              <span className="text-foreground/90 font-medium">
                {trafficStats.isLoading ? "..." : formatBytesPerSecond(trafficStats.speedUp)}
              </span>
            </span>
            <span className="opacity-40">·</span>
            <span className="flex items-center gap-0.5">
              <span className="opacity-60">↓</span>
              <span className="text-foreground/90 font-medium">
                {trafficStats.isLoading ? "..." : formatBytesPerSecond(trafficStats.speedDown)}
              </span>
            </span>
          </div>
        </div>
      </div>

      {/* 3. 待续费机器 */}
      <div
        onClick={() => setSelectedStatus(isExpiringActive ? "all" : "expiring")}
        className={`p-3.5 rounded-xl border bg-card shadow-2xs cursor-pointer transition-all duration-150 group select-none flex flex-col justify-between h-[84px] ${
          isExpiringActive
            ? "border-amber-500/80 ring-2 ring-amber-500/20 bg-amber-500/5 dark:bg-amber-500/10"
            : "border-border/70 hover:border-amber-500/50 hover:shadow-xs"
        }`}
        title={isExpiringActive ? "点击恢复展示全部节点" : "点击在下方列表筛选这批临期节点"}
      >
        <div className="flex items-center justify-between text-muted-foreground text-xs font-medium">
          <span className="tracking-tight group-hover:text-amber-600 dark:group-hover:text-amber-400 transition-colors">
            {t("admin.overview.expiring_soon", "待续费机器")}
          </span>
          <div
            className={`w-6 h-6 rounded-md flex items-center justify-center shrink-0 transition-colors ${
              isExpiringActive
                ? "bg-amber-500/20 text-amber-600 dark:text-amber-400"
                : "bg-muted/60 text-muted-foreground group-hover:bg-amber-500/10 group-hover:text-amber-600 dark:group-hover:text-amber-400"
            }`}
          >
            <CalendarClock size={13} />
          </div>
        </div>
        <div className="flex items-baseline justify-between mt-1">
          <div className="flex items-baseline gap-1">
            <span className="text-xl font-bold font-mono tracking-tight text-foreground">
              {expiringCount} 台
            </span>
            <span className="text-[11px] text-muted-foreground">临期</span>
          </div>
          {expiringCount > 0 ? (
            <div className="px-2 py-0.5 rounded-md bg-amber-500/15 text-amber-700 dark:text-amber-300 border border-amber-500/30 text-[11px] font-medium flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-amber-500 inline-block animate-pulse" />
              <span>{isExpiringActive ? "已筛选" : "需要续费"}</span>
            </div>
          ) : (
            <div className="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 text-[11px] font-medium flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />
              <span>全部正常</span>
            </div>
          )}
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



const UpgradeAgentButton = () => {
  const { t } = useTranslation();
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeVersion, setUpgradeVersion] = useState<string>("");
  const [upgrading, setUpgrading] = useState(false);

  const openUpgradeDialog = async () => {
    setUpgradeOpen(true);
    setUpgradeVersion("");
    try {
      const res = await fetch("/api/admin/agent-asset-version");
      const data = await res.json();
      const ver = data?.data?.version || data?.result?.version || data?.version;
      if (typeof ver === "string" && ver) setUpgradeVersion(ver);
    } catch {
      // ignore
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
      const dispatched =
        data?.data?.dispatched ?? data?.result?.dispatched ?? data?.dispatched;
      const rawVersion =
        data?.data?.version ??
        data?.result?.version ??
        data?.version ??
        upgradeVersion;
      const formattedVersion = rawVersion
        ? rawVersion.startsWith("v")
          ? rawVersion
          : `v${rawVersion}`
        : "";
      toast.success(
        t("admin.nodeTable.upgradeSuccess", {
          defaultValue: `升级事件已下发（${formattedVersion}，${dispatched} 个节点）`,
          version: formattedVersion,
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
    <Dialog.Root open={upgradeOpen} onOpenChange={setUpgradeOpen}>
      <Dialog.Trigger asChild>
        <button
          type="button"
          onClick={() => openUpgradeDialog()}
          className="h-8 px-2.5 rounded-lg border border-border/70 bg-card hover:bg-muted/40 text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1.5 text-xs font-medium cursor-pointer shadow-2xs shrink-0"
          title={t("admin.nodeTable.upgradeAgents", "批量升级 Agent")}
        >
          <ArrowUpCircle size={13} />
          <span className="hidden sm:inline">升级 Agent</span>
        </button>
      </Dialog.Trigger>
      <Dialog.Content className="sm:max-w-md p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <ArrowUpCircle size={16} className="text-muted-foreground shrink-0" />
            <span>{t("admin.nodeTable.upgradeAgents", "批量升级 Agent")}</span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            {t("admin.nodeTable.upgradeDescription", {
              defaultValue:
                "将向全部节点下发升级事件。Agent 会从面板下载新版本并自动完成替换与重启。",
              version: upgradeVersion,
            })}
          </Dialog.Description>
        </Dialog.Header>
        {upgradeVersion ? (
          <div className="p-3 rounded-lg border border-border/70 bg-muted/30 flex items-center justify-between text-xs">
            <span className="text-muted-foreground">{t("admin.nodeTable.upgradeTargetVersion", "目标版本")}</span>
            <span className="font-mono font-medium text-foreground">
              {upgradeVersion.startsWith("v") ? upgradeVersion : `v${upgradeVersion}`}
            </span>
          </div>
        ) : null}
        <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
          <Dialog.Close asChild>
            <Button variant="outline" type="button" disabled={upgrading}>
              {t("common.cancel", "Cancel")}
            </Button>
          </Dialog.Close>
          <Button onClick={() => handleUpgradeAgents()} disabled={upgrading}>
            {upgrading
              ? t("admin.nodeTable.upgrading", "下发中...")
              : t("admin.nodeTable.upgradeConfirm", "确认升级")}
          </Button>
        </Dialog.Footer>
      </Dialog.Content>
    </Dialog.Root>
  );
};

const TableToolbar = ({
  searchTerm,
  setSearchTerm,
  selectedGroup,
  setSelectedGroup,
  availableGroups,
  selectedStatus,
  setSelectedStatus,
  totalNodes,
  onlineCount,
  offlineCount,
  expiringCount,
  selectedNodes = [],
}: {
  searchTerm: string;
  setSearchTerm: (term: string) => void;
  selectedGroup: string;
  setSelectedGroup: (group: string) => void;
  availableGroups: string[];
  selectedStatus: "all" | "online" | "offline" | "expiring";
  setSelectedStatus: (status: "all" | "online" | "offline" | "expiring") => void;
  totalNodes: number;
  onlineCount: number;
  offlineCount: number;
  expiringCount: number;
  selectedNodes?: string[];
}) => {
  const { t } = useTranslation();
  const { refresh } = useNodeDetails();
  const [loading, setLoading] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
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

  return (
    <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3 pt-0.5">
      {/* 左侧：页面标题 + 状态筛选药丸栏 */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-2 shrink-0">
          <h1 className="text-lg font-bold tracking-tight text-foreground">
            {t("admin.nodeTable.nodeList", "节点列表")}
          </h1>
          {selectedNodes.length > 0 && (
            <span className="px-2 py-0.5 text-xs font-mono font-medium rounded-full bg-foreground text-background">
              {selectedNodes.length} 已选
            </span>
          )}
        </div>

        <div className="inline-flex items-center p-0.5 bg-muted/50 rounded-lg border border-border/50 text-xs shrink-0">
          <button
            type="button"
            onClick={() => setSelectedStatus("all")}
            className={`h-7 px-3 rounded-md transition-all cursor-pointer font-medium text-xs ${
              selectedStatus === "all"
                ? "bg-card text-foreground shadow-2xs font-semibold"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            全部 ({totalNodes})
          </button>
          <button
            type="button"
            onClick={() => setSelectedStatus("online")}
            className={`h-7 px-3 rounded-md flex items-center gap-1.5 transition-all cursor-pointer font-medium text-xs ${
              selectedStatus === "online"
                ? "bg-card text-foreground shadow-2xs font-semibold"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />
            在线 ({onlineCount})
          </button>
          <button
            type="button"
            onClick={() => setSelectedStatus("offline")}
            className={`h-7 px-3 rounded-md flex items-center gap-1.5 transition-all cursor-pointer font-medium text-xs ${
              selectedStatus === "offline"
                ? "bg-card text-foreground shadow-2xs font-semibold"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <span className="w-1.5 h-1.5 rounded-full bg-rose-500 inline-block" />
            离线 ({offlineCount})
          </button>
          {expiringCount > 0 && (
            <button
              type="button"
              onClick={() => setSelectedStatus(selectedStatus === "expiring" ? "all" : "expiring")}
              className={`h-7 px-3 rounded-md flex items-center gap-1.5 transition-all cursor-pointer font-medium text-xs ${
                selectedStatus === "expiring"
                  ? "bg-amber-500/15 text-amber-700 dark:text-amber-300 shadow-2xs font-semibold border border-amber-500/30"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-amber-500 inline-block" />
              待续费 ({expiringCount})
            </button>
          )}
        </div>
      </div>

      {/* 右侧：升级 Agent + 搜索框 + 分组下拉 + 添加节点 */}
      <div className="flex items-center gap-2 w-full lg:w-auto flex-wrap sm:flex-nowrap">
        <UpgradeAgentButton />

        <div className="relative flex-1 sm:w-52 flex items-center">
          <Search size={13} className="absolute left-2.5 text-muted-foreground pointer-events-none" />
          <input
            type="text"
            placeholder={t("admin.nodeTable.searchByName", "搜索节点、IP...")}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full h-8 pl-8 pr-7 text-xs rounded-lg border border-border/70 bg-card text-foreground placeholder:text-muted-foreground/60 outline-none focus:border-foreground/40 focus:ring-1 focus:ring-foreground/15 transition-all shadow-2xs"
          />
          {searchTerm && (
            <button
              type="button"
              onClick={() => setSearchTerm("")}
              className="absolute right-2 text-muted-foreground/60 hover:text-foreground text-xs p-0.5 rounded cursor-pointer"
              title="清空搜索"
            >
              ×
            </button>
          )}
        </div>

        {availableGroups.length > 0 && (
          <DropdownMenu.Root>
            <DropdownMenu.Trigger>
              <button
                type="button"
                className="h-8 px-2.5 rounded-lg border border-border/70 bg-card hover:bg-muted/40 text-foreground text-xs font-medium inline-flex items-center gap-1.5 shadow-2xs cursor-pointer transition-colors shrink-0"
              >
                <Layers size={13} className="text-muted-foreground/70" />
                <span>{selectedGroup === "all" ? `分组 (${availableGroups.length})` : selectedGroup}</span>
                <ChevronDown size={11} className="text-muted-foreground/60 ml-0.5" />
              </button>
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="end" className="min-w-[150px]">
              <DropdownMenu.Item
                onClick={() => setSelectedGroup("all")}
                className={`flex items-center justify-between cursor-pointer ${
                  selectedGroup === "all" ? "bg-muted/70 font-semibold" : ""
                }`}
              >
                <span>全部分组 ({totalNodes})</span>
                {selectedGroup === "all" && <Check size={13} className="text-primary ml-2" />}
              </DropdownMenu.Item>
              <DropdownMenu.Separator />
              {availableGroups.map((g) => (
                <DropdownMenu.Item
                  key={g}
                  onClick={() => setSelectedGroup(g)}
                  className={`flex items-center justify-between cursor-pointer ${
                    selectedGroup === g ? "bg-muted/70 font-semibold" : ""
                  }`}
                >
                  <span>{g}</span>
                  {selectedGroup === g && <Check size={13} className="text-primary ml-2" />}
                </DropdownMenu.Item>
              ))}
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        )}

        <Dialog.Root open={dialogOpen} onOpenChange={setDialogOpen}>
          <Dialog.Trigger asChild>
            <button
              onClick={() => setDialogOpen(true)}
              className="h-8 px-3 rounded-lg bg-foreground text-background font-medium text-xs flex items-center gap-1.5 shadow-2xs hover:opacity-90 active:scale-[0.98] transition-all cursor-pointer shrink-0"
            >
              <Plus size={13} strokeWidth={2.5} />
              <span>{t("admin.nodeTable.addNode", "添加节点")}</span>
            </button>
          </Dialog.Trigger>
          <Dialog.Content className="sm:max-w-md p-6 gap-5">
            <Dialog.Header className="space-y-1.5 text-left">
              <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
                <Plus size={16} className="text-muted-foreground shrink-0" />
                <span>{t("admin.nodeTable.addNode", "添加新服务器节点")}</span>
              </Dialog.Title>
              <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
                创建新节点记录。保存后将自动生成专属 Token 令牌，供一键部署脚本接入使用。
              </Dialog.Description>
            </Dialog.Header>
            <div className="space-y-1.5 my-0.5">
              <label htmlFor="new-node-name-input" className="text-xs font-medium text-foreground block">
                {t("admin.nodeTable.nameOptional", "服务器名称（选填）")}
              </label>
              <input
                id="new-node-name-input"
                ref={inputRef}
                placeholder="例如：香港 BGP、东京 CN2..."
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleAddNode(inputRef.current?.value);
                  }
                }}
                className="w-full h-9 px-3 text-sm rounded-md border border-border bg-background text-foreground outline-none focus:border-foreground/50 shadow-2xs"
              />
            </div>
            <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
              <Dialog.Close asChild>
                <Button variant="outline" type="button" disabled={loading} className="cursor-pointer">
                  {t("common.cancel", "取消")}
                </Button>
              </Dialog.Close>
              <Button
                onClick={() => handleAddNode(inputRef.current?.value)}
                disabled={loading}
                className="cursor-pointer"
              >
                {loading ? "添加中..." : t("common.confirm", "确认添加")}
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Root>
      </div>
    </div>
  );
};

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
      <TableCell className="w-40 px-2">
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
      <TableCell className="w-20 px-1.5 text-center">
        {node.group ? (
          <span className="inline-block text-xs px-2 py-0.5 rounded-md bg-muted/50 text-foreground/80 font-medium border border-border/40 max-w-[76px] truncate" title={node.group}>
            {node.group}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground/40 font-mono">-</span>
        )}
      </TableCell>
      <TableCell className="w-28 px-2">
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
                className="w-40 px-2 text-left cursor-pointer select-none group hover:text-foreground transition-colors"
              >
                <span className="inline-flex items-center">
                  <span>{t("admin.nodeDetail.ipAddress")}</span>
                  {renderSortIcon("ip")}
                </span>
              </TableHead>
              <TableHead
                onClick={() => onSort("group")}
                className="w-20 px-1.5 text-center cursor-pointer select-none group hover:text-foreground transition-colors"
              >
                <span className="inline-flex items-center justify-center">
                  <span>{t("common.group")}</span>
                  {renderSortIcon("group")}
                </span>
              </TableHead>
              <TableHead
                onClick={() => onSort("billing")}
                className="w-28 px-2 text-left cursor-pointer select-none group hover:text-foreground transition-colors"
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
  const [billingOpen, setBillingOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [renewOpen, setRenewOpen] = useState(false);
  const [installOpen, setInstallOpen] = useState(false);

  const cycleDays = Number(node.billing_cycle) > 0 ? Number(node.billing_cycle) : 30;

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

      {/* 弹窗部分 */}
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
    </div>
  );
};

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

function EditButton({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
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
      <Dialog.Trigger asChild>
        <button
          type="button"
          className="size-7 rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center justify-center transition-colors cursor-pointer shadow-2xs"
          title={t("admin.nodeEdit.editInfo", "编辑信息")}
        >
          <Pencil size={13} />
        </button>
      </Dialog.Trigger>
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

function DetailView({ node }: { node: NodeDetail }) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();

  return (
    <Drawer direction={isMobile ? "bottom" : "right"}>
      <DrawerTrigger asChild>
        <div className="flex items-center gap-2.5 py-1 hover:underline cursor-pointer group">
          <div className="relative shrink-0 w-6 h-5 flex items-center justify-center">
            <Flag flag={node.region} size="5" className="m-0 shrink-0 rounded-xs" />
            <span
              className={`absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full ring-1.5 ring-card ${
                node.online ? "bg-emerald-500" : "bg-rose-500"
              }`}
              title={node.online ? t("nodeCard.online", "在线") : t("nodeCard.offline", "离线")}
            />
          </div>
          <div className="flex flex-col min-w-0">
            <div className="flex items-center gap-1.5 min-w-0">
              <span className="font-semibold text-sm text-foreground group-hover:text-primary transition-colors truncate max-w-[380px] 2xl:max-w-[500px]" title={node.name}>
                {node.name}
              </span>
              {node.public_remark && (
                <span
                  className="text-[10px] px-1.5 py-0.2 rounded bg-muted/70 text-muted-foreground border border-border/60 font-normal truncate max-w-[120px] shrink-0"
                  title={`备注: ${node.public_remark}`}
                >
                  {node.public_remark}
                </span>
              )}
              {node.tags && (
                <span
                  className="text-[10px] px-1.5 py-0.2 rounded bg-muted/70 text-muted-foreground border border-border/60 font-normal truncate max-w-[120px] shrink-0"
                  title={`标签: ${node.tags}`}
                >
                  {node.tags}
                </span>
              )}
            </div>
            <span className="text-[11px] text-muted-foreground/75 font-mono truncate max-w-[380px]">
              {node.os || "Linux"} {node.arch ? `· ${node.arch}` : ""} {node.version ? `· v${node.version}` : ""}
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
                          copyToClipboard(node.ipv4!, {
                            successMessage: `已复制 IPv4: ${node.ipv4}`,
                          });
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
                          copyToClipboard(node.ipv6!, {
                            successMessage: `已复制 IPv6: ${node.ipv6}`,
                          });
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

function BillingButton({
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
  const [internalOpen, setInternalOpen] = useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : internalOpen;
  const setOpen = (o: boolean) => {
    if (isControlled) {
      onOpenChange?.(o);
    } else {
      setInternalOpen(o);
    }
  };
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
      {trigger !== null && (
        <Dialog.Trigger asChild>
          {trigger !== undefined ? (
            trigger
          ) : (
            <IconButton
              variant="ghost"
              title={t("admin.nodeTable.billing", "账单")}
            >
              <CircleDollarSign size="18" />
            </IconButton>
          )}
        </Dialog.Trigger>
      )}
      <Dialog.Content className="sm:max-w-[460px] p-6 gap-5">
        <Dialog.Header className="space-y-1.5 text-left">
          <Dialog.Title className="flex items-center gap-2 text-base font-semibold leading-none tracking-tight text-foreground">
            <CircleDollarSign size={16} className="text-muted-foreground shrink-0" />
            <span>{t("admin.nodeTable.billing", "账单管理")}</span>
            <span className="font-mono text-xs text-muted-foreground font-normal ml-1">
              · {node.name}
            </span>
          </Dialog.Title>
          <Dialog.Description className="text-xs text-muted-foreground leading-relaxed">
            {t("admin.nodeTable.billingDescription", "配置该节点的采购价格、结算货币、计费周期与续费规则。")}
          </Dialog.Description>
        </Dialog.Header>
        <form onSubmit={handleSave} className="grid gap-3.5 my-0.5">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.price")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.priceTips")}</span></Tips>
              </div>
              <TextField.Root name="price" defaultValue={node.price} placeholder="0.00" className="h-9 text-sm" />
            </div>

            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.currency", "价格货币")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.currencyTips")}</span></Tips>
              </div>
              <Select.Root value={currency} onValueChange={setCurrency}>
                <Select.Trigger className="w-full h-9 text-sm" />
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

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.premium")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.premiumTips")}</span></Tips>
              </div>
              <TextField.Root name="premium" defaultValue={node.premium} placeholder="0.00" className="h-9 text-sm" />
            </div>

            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.premiumCurrency", "溢价货币")}
                </label>
                <Tips size="13"><span>{t("admin.nodeTable.premiumCurrencyTips", "溢价独立结算币种，默认为 ¥")}</span></Tips>
              </div>
              <Select.Root value={premiumCurrency} onValueChange={setPremiumCurrency}>
                <Select.Trigger className="w-full h-9 text-sm" />
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

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-1.5">
              <div className="flex items-center gap-1.5 h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.billingCycle")}
                </label>
                <Tips size="13"><span dangerouslySetInnerHTML={{ __html: t("admin.nodeTable.billingCycleTips") }}></span></Tips>
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
                className="h-9 text-sm"
              />
            </div>

            <div className="grid gap-1.5">
              <div className="flex items-center h-5">
                <label className="text-xs font-medium leading-none text-foreground">
                  {t("admin.nodeTable.expiredAt")}
                </label>
              </div>
              <TextField.Root
                name="expiredAt"
                defaultValue={
                  node.expired_at
                    ? new Date(node.expired_at).toISOString().slice(0, 10)
                    : "0001-01-01"
                }
                type="date"
                className="h-9 text-sm"
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

          <Dialog.Footer className="pt-2 border-t border-border/50 flex flex-row justify-end gap-2">
            <Dialog.Close asChild>
              <Button variant="outline" type="button">
                {t("common.cancel")}
              </Button>
            </Dialog.Close>
            <Button type="submit" disabled={saving}>
              {t("common.save")}
            </Button>
          </Dialog.Footer>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}

export default NodeDetailsPage;
