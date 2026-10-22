import { formatBytesSplit } from "../../../theme/src/utils/unitCore";

export interface RetentionTier {
  interval_seconds: number;
  retention_seconds: number;
  source: string;
}
export interface MetricRetention {
  metric_name: string;
  definition_retention_days: number;
  enabled: boolean;
  configured_tiers: RetentionTier[];
  effective_tiers: RetentionTier[];
}
export interface RetentionInfo {
  raw_retention_seconds: number;
  raw_source: string;
  metrics: MetricRetention[];
}
export interface MemoryStats {
  store_ready: boolean;
  go: Record<string, number>;
  metric: Record<string, number>;
  pools: Record<string, number>;
  queues: Record<string, number>;
  history_query_slots: { capacity: number; in_use: number };
}
export function formatBytes(value?: number): string {
  if (value === undefined || !Number.isFinite(value) || value < 0) return "—";
  const formatted = formatBytesSplit(value, { B: 1, KB: 1, MB: 1, GB: 1, TB: 1 });
  // The shared core scales by 1024; spell binary units explicitly here.
  const unit = formatted.unit === "B" ? "B" : formatted.unit.replace("B", "iB");
  return `${Number(formatted.value)} ${unit}`;
}
export function formatRatio(value: number, limit: number): string {
  if (!Number.isFinite(limit) || limit <= 0 || limit >= Number.MAX_SAFE_INTEGER) return "未设置";
  return `${(100 * value / limit).toFixed(1)}%`;
}
export function queueTone(depth: number, capacity: number): "normal" | "warning" | "danger" {
  return capacity > 0 && depth >= capacity ? "danger" : capacity > 0 && depth / capacity >= 0.8 ? "warning" : "normal";
}
export function formatDuration(seconds: number): string {
  for (const [unit, label] of [[86400, "天"], [3600, "小时"], [60, "分钟"]] as const) {
    if (seconds >= unit && seconds % unit === 0) return `${seconds / unit} ${label}`;
  }
  return `${seconds} 秒`;
}
export function retentionRows(metric: MetricRetention) {
  return metric.configured_tiers.map(tier => {
    const effective = metric.effective_tiers.find(item => item.interval_seconds === tier.interval_seconds);
    return {
      interval: formatDuration(tier.interval_seconds), configured: formatDuration(tier.retention_seconds),
      effective: !metric.enabled ? "停用（不会主动清理历史）" : effective ? formatDuration(effective.retention_seconds) : "冗余层已省略",
      source: effective?.source === "metric_definition" ? "指标定义优先" : effective ? "存储配置" : "—",
    };
  });
}
export interface BudgetRow { label: string; value: string; tone?: "normal" | "warning" | "danger" }
export function memoryRows(stats: MemoryStats): BudgetRow[] {
  const g = stats.go;
  const rows: BudgetRow[] = [
    { label: "Go HeapAlloc", value: `${formatBytes(g.heap_alloc_bytes)} · ${formatRatio(g.heap_alloc_bytes, g.soft_memory_limit_bytes)} 软预算` },
    ...[["HeapSys", "heap_sys_bytes"], ["HeapReleased", "heap_released_bytes"], ["StackSys", "stack_sys_bytes"], ["Sys", "sys_bytes"], ["NextGC", "next_gc_bytes"]].map(([label, key]) => ({ label: `Go ${label}`, value: `${formatBytes(g[key])} · ${formatRatio(g[key], g.soft_memory_limit_bytes)} 软预算` })),
    { label: "Go 软内存预算", value: formatRatio(0, g.soft_memory_limit_bytes) === "未设置" ? "未设置" : formatBytes(g.soft_memory_limit_bytes) },
    { label: "GC 次数 / CPU 占比", value: `${g.num_gc ?? "—"} / ${((g.gc_cpu_fraction ?? 0) * 100).toFixed(2)}%` },
    { label: "历史查询槽", value: `${stats.history_query_slots.in_use} / ${stats.history_query_slots.capacity}`, tone: queueTone(stats.history_query_slots.in_use, stats.history_query_slots.capacity) },
  ];
  if (!stats.store_ready) return [...rows, { label: "指标存储", value: "指标存储未初始化" }];
  return [...rows,
    ...[["Raw", "estimated_raw_bytes"], ["Hot", "estimated_hot_bytes"], ["Coarse", "estimated_coarse_bytes"]].map(([label, key]) => ({ label: `${label} 结构估算`, value: formatBytes(stats.metric[key]) })),
    { label: "报告队列", value: `${stats.queues.reports_queued} / ${stats.queues.reports_capacity}`, tone: queueTone(stats.queues.reports_queued, stats.queues.reports_capacity) },
    { label: "Ping 队列", value: `${stats.queues.pings_queued} / ${stats.queues.pings_capacity}`, tone: queueTone(stats.queues.pings_queued, stats.queues.pings_capacity) },
    { label: "数据库等待次数", value: String(stats.pools.wait_count ?? "—") },
    { label: "数据库等待耗时", value: `${stats.pools.wait_duration_ms ?? "—"} ms` },
    { label: "写池 使用 / 已开 / 上限", value: `${stats.pools.in_use ?? "—"} / ${stats.pools.open_connections ?? "—"} / ${stats.pools.max_open_connections ?? "—"}` },
    { label: "读池 使用 / 已开 / 上限", value: `${stats.pools.read_pool_in_use ?? "—"} / ${stats.pools.read_pool_open_connections ?? "—"} / ${stats.pools.read_pool_max_open_connections ?? "—"}` },
  ];
}
