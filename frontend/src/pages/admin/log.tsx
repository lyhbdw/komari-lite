import React from "react";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight, Copy, FileText } from "lucide-react";
import Loading from "@/components/loading";
import { copyToClipboard } from "@/utils/clipboard";

interface Log {
  id: number;
  ip: string;
  uuid: string;
  message: string;
  msg_type: string;
  time: string;
}

function formatLogTime(timeStr: string) {
  if (!timeStr) return "-";
  const d = new Date(timeStr);
  if (isNaN(d.getTime())) return timeStr;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function LogTypeBadge({ type }: { type: string }) {
  const normalized = (type || "").toLowerCase();
  if (normalized === "error" || normalized === "fatal") {
    return (
      <span className="px-1.5 py-0.2 text-[10px] font-mono font-medium rounded bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20 uppercase">
        {type}
      </span>
    );
  }
  if (normalized === "warn" || normalized === "warning") {
    return (
      <span className="px-1.5 py-0.2 text-[10px] font-mono font-medium rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 uppercase">
        {type}
      </span>
    );
  }
  return (
    <span className="px-1.5 py-0.2 text-[10px] font-mono font-medium rounded bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20 uppercase">
      {type || "INFO"}
    </span>
  );
}

const LogPage = () => {
  const [loading, setLoading] = React.useState<boolean>(true);
  const [logs, setLogs] = React.useState<Log[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [page, setPage] = React.useState<number>(1);
  const [total, setTotal] = React.useState<number>(0);
  const [limit, setLimit] = React.useState<number>(15);
  const [activeLog, setActiveLog] = React.useState<Log | null>(null);
  const [t] = useTranslation();

  React.useEffect(() => {
    const fetchLogs = async () => {
      setLoading(true);
      try {
        const response = await fetch(
          `/api/admin/logs?limit=${limit}&page=${page}`
        );
        if (!response.ok) {
          throw new Error("Failed to fetch logs");
        }
        const data = await response.json();
        setLogs(data.data?.logs || []);
        setTotal(data.data?.total || 0);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error");
      } finally {
        setLoading(false);
      }
    };
    fetchLogs();
  }, [page, limit]);

  const totalPages = Math.max(1, Math.ceil(total / limit));
  const siblingsCount = 1;
  const leftSibling = Math.max(page - siblingsCount, 1);
  const rightSibling = Math.min(page + siblingsCount, totalPages);
  const showLeftDots = leftSibling > 2;
  const showRightDots = rightSibling < totalPages - 1;

  const pageNumbers: (number | string)[] = [1];
  if (showLeftDots) {
    pageNumbers.push("...");
  } else {
    for (let i = 2; i < leftSibling; i++) pageNumbers.push(i);
  }
  for (let i = leftSibling; i <= rightSibling; i++) {
    if (i > 1 && i < totalPages) pageNumbers.push(i);
  }
  if (showRightDots) {
    pageNumbers.push("...");
  } else {
    for (let i = rightSibling + 1; i < totalPages; i++) pageNumbers.push(i);
  }
  if (totalPages > 1) pageNumbers.push(totalPages);

  if (loading && logs.length === 0) {
    return <Loading />;
  }
  if (error) {
    return <div className="p-4 text-xs text-rose-500">Error: {error}</div>;
  }

  const startItem = total === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, total);

  return (
    <div className="space-y-4 km-page-admin-log max-w-7xl">
      {/* 1. 统一精致页头 */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("logs.title", "审计日志")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              共 {total} 条
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            {t("logs.description", "记录系统运行、认证鉴权与节点变动历史。")}
          </p>
        </div>

        {/* 每页条数下拉选择 */}
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>{t("logs.per_page", "每页显示")}:</span>
          <select
            value={limit}
            onChange={(e) => {
              setLimit(Number(e.target.value));
              setPage(1);
            }}
            className="h-7 px-2 text-xs rounded-md border border-border bg-card text-foreground outline-none focus:border-foreground/50 cursor-pointer shadow-2xs font-mono"
          >
            {[10, 15, 20, 50, 100].map((n) => (
              <option key={n} value={n}>
                {n} 条
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* 2. 现代日志数据表格 */}
      <div className="rounded-xl border border-border/70 bg-card overflow-hidden shadow-2xs">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40 border-b border-border/80 text-[11px] text-muted-foreground">
              <TableHead className="w-16 font-mono text-center">ID</TableHead>
              <TableHead className="w-36 font-mono">{t("logs.col_ip", "来源 IP")}</TableHead>
              <TableHead className="w-24 text-center">{t("logs.col_type", "类型")}</TableHead>
              <TableHead>{t("logs.col_message", "日志内容")}</TableHead>
              <TableHead className="w-44 text-right pr-4 font-mono">{t("logs.col_time", "时间")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-32 text-center text-xs text-muted-foreground">
                  {t("logs.empty", "暂无审计日志记录")}
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow
                  key={log.id}
                  onClick={() => setActiveLog(log)}
                  className="hover:bg-muted/40 cursor-pointer transition-colors text-xs"
                >
                  <TableCell className="font-mono text-center text-muted-foreground text-[11px]">
                    {log.id}
                  </TableCell>
                  <TableCell className="font-mono text-[11px]">
                    {log.ip ? (
                      <span className="text-foreground">{log.ip}</span>
                    ) : (
                      <span className="text-muted-foreground/60">-</span>
                    )}
                  </TableCell>
                  <TableCell className="text-center">
                    <LogTypeBadge type={log.msg_type} />
                  </TableCell>
                  <TableCell className="font-mono text-[11px] text-foreground max-w-xl truncate">
                    {log.message}
                  </TableCell>
                  <TableCell className="text-right pr-4 font-mono text-[11px] text-muted-foreground">
                    {formatLogTime(log.time)}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* 3. 分页导航条 */}
      <div className="flex flex-col sm:flex-row justify-between items-center gap-3 pt-1">
        <div className="text-xs text-muted-foreground font-mono">
          显示第 {startItem} - {endItem} 条 · 共 {total} 条
        </div>

        <div className="flex items-center gap-1.5">
          <Button
            size="icon"
            variant="outline"
            disabled={page === 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            className="cursor-pointer size-7 p-0 flex items-center justify-center disabled:opacity-40"
            title="上一页"
          >
            <ChevronLeft size={14} />
          </Button>

          {pageNumbers.map((p, i) =>
            typeof p === "number" ? (
              <Button
                key={i}
                size="sm"
                variant={p === page ? "default" : "outline"}
                onClick={() => setPage(p)}
                className="cursor-pointer min-w-7 h-7 text-xs font-mono p-0"
              >
                {p}
              </Button>
            ) : (
              <span key={i} className="px-1 text-xs text-muted-foreground">
                ...
              </span>
            )
          )}

          <Button
            size="icon"
            variant="outline"
            disabled={page === totalPages || total === 0}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            className="cursor-pointer size-7 p-0 flex items-center justify-center disabled:opacity-40"
            title="下一页"
          >
            <ChevronRight size={14} />
          </Button>
        </div>
      </div>

      {/* 4. 日志详情弹窗 */}
      <Dialog.Root open={!!activeLog} onOpenChange={(open) => !open && setActiveLog(null)}>
        <Dialog.Content className="max-w-xl">
          <Dialog.Title>
            <div className="flex items-center gap-2">
              <FileText size={16} className="text-muted-foreground" />
              <span>{t("logs.detail_title", "日志详情")} #{activeLog?.id}</span>
            </div>
          </Dialog.Title>

          {activeLog && (
            <div className="flex flex-col gap-3 my-3 text-xs">
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                <div className="p-2.5 rounded-lg border border-border/50 bg-muted/20">
                  <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">ID</span>
                  <span className="font-mono text-foreground font-semibold">{activeLog.id}</span>
                </div>
                <div className="p-2.5 rounded-lg border border-border/50 bg-muted/20">
                  <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">来源 IP</span>
                  <span className="font-mono text-foreground">{activeLog.ip || "系统内置"}</span>
                </div>
                <div className="p-2.5 rounded-lg border border-border/50 bg-muted/20">
                  <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">日志类型</span>
                  <LogTypeBadge type={activeLog.msg_type} />
                </div>
                <div className="p-2.5 rounded-lg border border-border/50 bg-muted/20">
                  <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">时间</span>
                  <span className="font-mono text-[11px] text-foreground">{formatLogTime(activeLog.time)}</span>
                </div>
              </div>

              {activeLog.uuid && (
                <div className="p-2.5 rounded-lg border border-border/50 bg-muted/20">
                  <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">关联节点 UUID</span>
                  <span className="font-mono text-foreground select-all break-all text-[11px]">{activeLog.uuid}</span>
                </div>
              )}

              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <span className="text-[11px] font-medium text-muted-foreground">完整内容 (Message)</span>
                  <button
                    type="button"
                    onClick={() => copyToClipboard(activeLog.message, { successMessage: "已复制日志内容" })}
                    className="text-[11px] text-muted-foreground hover:text-foreground flex items-center gap-1 cursor-pointer transition-colors"
                  >
                    <Copy size={12} />
                    <span>复制</span>
                  </button>
                </div>
                <div className="p-3 rounded-lg border border-border/70 bg-muted/30 font-mono text-[11px] leading-relaxed break-all select-all text-foreground max-h-56 overflow-y-auto">
                  {activeLog.message}
                </div>
              </div>
            </div>
          )}

          <div className="flex justify-end mt-4">
            <Dialog.Close asChild>
              <Button variant="outline" className="cursor-pointer">
                {t("common.close", "关闭")}
              </Button>
            </Dialog.Close>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  );
};

export default LogPage;
