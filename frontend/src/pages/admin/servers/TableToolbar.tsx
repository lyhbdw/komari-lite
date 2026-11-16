import React from "react";
import { useState } from "react";
import { useNodeDetails } from "@/contexts/useNodeDetails";
import { Button, Dialog, DropdownMenu } from "@/components/ui/radix-shim";
import { Plus, Search, Layers, ChevronDown, Check } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { UpgradeAgentButton } from "./UpgradeAgentButton";

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


export { TableToolbar };
