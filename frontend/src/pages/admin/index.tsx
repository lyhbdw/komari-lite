import { copyToClipboard } from "@/utils/clipboard";
import type { BatchResult } from "@/utils/batchNodes";
import { remainingSelection, runBatchNodes } from "@/utils/batchNodes";
import React from "react";
import { useEffect, useState } from "react";
import { NodeDetailsProvider } from "@/contexts/NodeDetailsContext";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { useAccount } from "@/contexts/useAccount";
import type { NodeDetail } from "@/contexts/node-details-context";
import { Copy, Trash2Icon, CheckSquare, X, Layers } from "lucide-react";
import { toast } from "sonner";
import { isNodeExpiringSoon } from "../../../../theme/src/utils/billingCore";
import { isNodeInGroup, isNodeUngrouped, parseNodeGroups } from "../../../../theme/src/utils/groupCore";
import Loading from "@/components/loading";
import { useSettings } from "@/lib/api";
import { MetricsOverview } from "./servers/MetricsOverview";
import { TableToolbar } from "./servers/TableToolbar";
import { NodeTable } from "./servers/NodeTable";
import { EmptyNodesGuide } from "./servers/EmptyNodesGuide";
import { BatchGroupDialog, BatchDeleteDialog } from "./servers/NodeDialogs";

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
          认证会话已失效或尚未登录，登录后即可管理节点。
        </p>
        {/* 关闭登录弹窗后的明确回流入口：重载页面会再次自动弹出登录窗口 */}
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="mt-4 px-3.5 py-1.5 rounded-lg bg-foreground text-background text-xs font-medium cursor-pointer shadow-sm hover:opacity-90 transition-opacity"
        >
          打开登录窗口
        </button>
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


export default NodeDetailsPage;
