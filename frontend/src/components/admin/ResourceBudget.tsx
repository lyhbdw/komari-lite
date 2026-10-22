import { useEffect, useState } from "react";
import { useRPC2Call } from "@/contexts/useRPC2";
import type { MemoryStats, RetentionInfo } from "@/utils/resourceBudget";
import { formatDuration, memoryRows, retentionRows } from "@/utils/resourceBudget";

interface Props {
  retention?: RetentionInfo;
  memory?: MemoryStats;
  retentionError?: string;
  memoryError?: string;
}
const cardClass = "rounded-xl border border-border/70 bg-card p-4 sm:p-5 shadow-2xs space-y-3";

export function ResourceBudgetPanels({ retention, memory, retentionError, memoryError }: Props) {
  return <div className="space-y-4">
    <section className={cardClass} aria-label="保留策略">
      <h2 className="text-sm font-semibold">保留策略 · 只读</h2>
      <p className="text-xs text-muted-foreground">配置来自当前存储实例；指标定义决定末层保留期限，并可缩短前层、略去冗余层。此处不会应用策略或清理历史。</p>
      {retentionError ? <p role="alert" className="text-sm text-destructive">{retentionError}</p> : !retention ? <p className="text-xs text-muted-foreground">正在读取保留策略…</p> : <>
        <p className="text-xs text-muted-foreground">原始样本：{formatDuration(retention.raw_retention_seconds)}，来源：固定内存窗口</p>
        {retention.metrics.length === 0 && <p className="text-xs text-muted-foreground">尚无指标定义</p>}
        {retention.metrics.map(metric => <details key={metric.metric_name} className="border-t border-border/50 pt-2">
          <summary className="cursor-pointer text-xs font-mono">{metric.metric_name} · 指标定义 {metric.definition_retention_days} 天{!metric.enabled && " · 已停用"}</summary>
          <div className="overflow-x-auto mt-2"><table className="w-full text-xs text-left">
            <thead className="text-muted-foreground"><tr>{["分辨率", "配置保留", "生效保留", "来源"].map(label => <th className="py-1 pr-3 font-medium" key={label}>{label}</th>)}</tr></thead>
            <tbody>{retentionRows(metric).map(row => <tr key={row.interval} className="border-t border-border/30"><td className="py-1.5 pr-3">{row.interval}</td><td className="pr-3">{row.configured}</td><td className="pr-3">{row.effective}</td><td>{row.source}</td></tr>)}</tbody>
          </table></div>
        </details>)}
      </>}
    </section>
    <section className={cardClass} aria-label="内存预算">
      <h2 className="text-sm font-semibold">内存预算 · 只读</h2>
      <p className="text-xs text-muted-foreground">指标各层为结构体大小估算，不是进程 RSS；Go 软预算也不等同于容器硬限制。队列达到 80% 时提示，满容量时告警。</p>
      {memoryError ? <p role="alert" className="text-sm text-destructive">{memoryError}</p> : !memory ? <p className="text-xs text-muted-foreground">正在读取内存预算…</p> : <dl className="grid gap-x-6 gap-y-2 sm:grid-cols-2">
        {memoryRows(memory).map(row => <div key={row.label} className="flex items-center justify-between gap-2 text-xs"><dt className="text-muted-foreground">{row.label}</dt><dd className={`font-mono tabular-nums text-right ${row.tone === "danger" ? "text-destructive" : row.tone === "warning" ? "text-amber-600 dark:text-amber-400" : ""}`}>{row.value}</dd></div>)}
      </dl>}
    </section>
  </div>;
}

export default function ResourceBudget() {
  const { call } = useRPC2Call();
  const [data, setData] = useState<Props>({});
  const [refreshId, setRefreshId] = useState(0);
  useEffect(() => {
    let active = true;
    setData({});
    void call<undefined, RetentionInfo>("admin:getEffectiveRetention").then(
      retention => { if (active) setData(previous => ({ ...previous, retention })); },
      error => { if (active) setData(previous => ({ ...previous, retentionError: String(error) })); },
    );
    void call<undefined, MemoryStats>("admin:getMemoryStats").then(
      memory => { if (active) setData(previous => ({ ...previous, memory })); },
      error => { if (active) setData(previous => ({ ...previous, memoryError: String(error) })); },
    );
    return () => { active = false; };
  }, [call, refreshId]);
  return <div className="space-y-2">
    <div className="flex justify-end"><button type="button" className="text-xs text-muted-foreground hover:text-foreground cursor-pointer disabled:opacity-50" disabled={!data.retention && !data.retentionError || !data.memory && !data.memoryError} onClick={() => setRefreshId(value => value + 1)}>刷新只读状态</button></div>
    <ResourceBudgetPanels {...data} />
  </div>;
}
