import React from "react";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { Button, Dialog, Flex } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";
import NumberPicker from "@/components/ui/number-picker";
import Loading from "@/components/loading";

interface Log {
  id: number;
  ip: string;
  uuid: string;
  message: string;
  msg_type: string;
  time: string;
}
const LogPage = () => {
  const [loading, setLoading] = React.useState<boolean>(true);
  const [logs, setLogs] = React.useState<Log[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [page, setPage] = React.useState<number>(1);
  const [total, setTotal] = React.useState<number>(1);
  const [limit, setLimit] = React.useState<number>(10);
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
        setLogs(data.data.logs);
        setTotal(data.data.total);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error");
      } finally {
        setLoading(false);
      }
    };
    fetchLogs();
  }, [page, limit]);

  const totalPages = Math.ceil(total / limit);
  // 计算分页页码，显示当前页及前后1页，两端省略号
  const siblingsCount = 1;
  let pageNumbers: (number | string)[] = [];
  const leftSibling = Math.max(page - siblingsCount, 1);
  const rightSibling = Math.min(page + siblingsCount, totalPages);
  const showLeftDots = leftSibling > 2;
  const showRightDots = rightSibling < totalPages - 1;
  // 始终包含第一页
  pageNumbers.push(1);
  // 左侧省略或中间连续页
  if (showLeftDots) {
    pageNumbers.push("...");
  } else {
    for (let i = 2; i < leftSibling; i++) pageNumbers.push(i);
  }
  // 中间页，仅当不重复首尾页时加入
  for (let i = leftSibling; i <= rightSibling; i++) {
    if (i > 1 && i < totalPages) pageNumbers.push(i);
  }
  // 右侧省略或中间连续页
  if (showRightDots) {
    pageNumbers.push("...");
  } else {
    for (let i = rightSibling + 1; i < totalPages; i++) pageNumbers.push(i);
  }
  // 始终包含最后一页（如果大于1）
  if (totalPages > 1) pageNumbers.push(totalPages);

  if (loading) {
    return <Loading />;
  }
  if (error) {
    return <div>Error: {error}</div>;
  }

  return (
    <div className="space-y-4 km-page-admin-log max-w-7xl">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2 pb-2 border-b border-border/40">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg sm:text-xl font-bold tracking-tight text-foreground">
              {t("logs.title", "系统审计日志")}
            </h1>
            <span className="px-2 py-0.2 text-[11px] font-mono font-medium rounded-full bg-muted text-muted-foreground border border-border">
              共 {total} 条
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5">
            记录面板运行、认证鉴权、节点变更与告警触发的审计历史。
          </p>
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>每页:</span>
          <NumberPicker
            defaultValue={limit}
            onChange={setLimit}
            min={1}
            max={100}
          />
        </div>
      </div>
      <div className="km-log-output rounded-lg border border-border bg-card overflow-hidden shadow-2xs">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/40 border-b border-border/80 text-[11px]">
              <TableHead className="w-16 font-mono">ID</TableHead>
              <TableHead className="w-36 font-mono">IP</TableHead>
              <TableHead className="w-24">Type</TableHead>
              <TableHead>Message</TableHead>
              <TableHead className="w-48 text-right pr-4">Time</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {logs.map((log) => (
              <TableRow key={log.id}>
                <TableCell>
                  <Dialog.Root>
                    <Dialog.Trigger>
                      <label className="hover:underline font-bold">
                        {log.id}
                      </label>
                    </Dialog.Trigger>
                    <Dialog.Content className="max-w-lg">
                      <Dialog.Title>{t("log.title")}</Dialog.Title>
                      <div className="flex flex-col gap-3 my-2 text-xs">
                        <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                          <div className="p-2 rounded-lg border border-border/50 bg-muted/20">
                            <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">ID</span>
                            <span className="font-mono text-foreground font-medium">{log.id}</span>
                          </div>
                          <div className="p-2 rounded-lg border border-border/50 bg-muted/20">
                            <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">IP</span>
                            <span className="font-mono text-foreground">{log.ip || "-"}</span>
                          </div>
                          <div className="p-2 rounded-lg border border-border/50 bg-muted/20">
                            <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">Type</span>
                            <span className="text-foreground capitalize">{log.msg_type || "-"}</span>
                          </div>
                        </div>

                        {log.uuid && (
                          <div className="p-2 rounded-lg border border-border/50 bg-muted/20">
                            <span className="text-[11px] font-medium text-muted-foreground block mb-0.5">UUID</span>
                            <span className="font-mono text-foreground select-all break-all text-[11px]">{log.uuid}</span>
                          </div>
                        )}

                        <div>
                          <span className="text-[11px] font-medium text-muted-foreground block mb-1">Message</span>
                          <div className="p-2.5 rounded-lg border border-border/70 bg-muted/30 font-mono text-[11px] leading-relaxed break-all select-all text-foreground max-h-48 overflow-y-auto">
                            {log.message}
                          </div>
                        </div>

                        <div className="text-[11px] text-muted-foreground pt-1 flex justify-between">
                          <span>Time:</span>
                          <span className="text-foreground font-mono">{new Date(log.time).toLocaleString()}</span>
                        </div>
                      </div>
                      <Flex justify="end" mt="3">
                        <Dialog.Close>
                          <Button variant="soft" color="gray">{t("common.close")}</Button>
                        </Dialog.Close>
                      </Flex>
                    </Dialog.Content>
                  </Dialog.Root>
                </TableCell>
                <TableCell>{log.ip}</TableCell>
                <TableCell>{log.msg_type}</TableCell>
                <TableCell>
                  {log.message.length > 75
                    ? `${log.message.slice(0, 75)}...`
                    : log.message}
                </TableCell>
                <TableCell>{new Date(log.time).toLocaleString()}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {/* 分页数字按钮 */}
      <div className="flex justify-center items-center space-x-2 mt-4 gap-2">
        <Button
          disabled={page === 1}
          onClick={() => setPage((p) => Math.max(1, p - 1))}
          title={t("common.previous_page", "Previous page")}
          aria-label={t("common.previous_page", "Previous page")}
        >
          {"<"}
        </Button>
        {pageNumbers.map((p, i) =>
          typeof p === "number" ? (
            <Button
              key={i}
              variant={p === page ? "solid" : "soft"}
              onClick={() => setPage(p)}
            >
              {p}
            </Button>
          ) : (
            <span key={i} className="px-2">
              ...
            </span>
          )
        )}
        <Button
          disabled={page === totalPages}
          onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          title={t("common.next_page", "Next page")}
          aria-label={t("common.next_page", "Next page")}
        >
          {">"}
        </Button>
      </div>
    </div>
  );
};
export default LogPage;
