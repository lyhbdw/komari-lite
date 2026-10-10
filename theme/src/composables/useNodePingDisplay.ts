import type { MaybeRefOrGetter } from 'vue'
import type { NodePingPerTaskStat } from '@/composables/useNodePingStats'
import { computed, toValue } from 'vue'
import { NODE_PING_BAR_COUNT, useNodePingStats } from '@/composables/useNodePingStats'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import { formatDateTime } from '@/utils/helper'

type NodePingMetric = 'latency' | 'loss'

// getRecords 在新版主控中返回的是近期可用样本，不保证覆盖完整 1 小时。
const RECENT_PING_RECORDS_QUERY_HOURS = 1

// 三网延迟固定展示的记录数量
const PING_NETWORK_DISPLAY_COUNT = 3

interface NodePingBar {
  key: string
  className: string
  tooltip: string
}

interface NodePingNetworkDisplay {
  name: string
  shortName: string
  latency: string
  toneClass: string
}

interface UseNodePingDisplayOptions {
  enabled?: MaybeRefOrGetter<boolean>
  loadingDisplayText?: string
  emptyDisplayText?: string
  loadingPanelTooltipText?: Partial<Record<NodePingMetric, string>>
  emptyPanelTooltipText?: Partial<Record<NodePingMetric, string>>
}

function getPingToneClass(value: number): string {
  if (!value)
    return 'text-muted-foreground'
  if (value <= 60)
    return 'text-emerald-600 dark:text-emerald-400 font-medium'
  if (value <= 120)
    return 'text-emerald-600/90 dark:text-emerald-400/90 font-medium'
  if (value <= 180)
    return 'text-amber-600 dark:text-amber-400 font-medium'
  if (value <= 240)
    return 'text-orange-600 dark:text-orange-400 font-medium'
  return 'text-rose-600 dark:text-rose-400 font-medium'
}

function getLatencyToneClass(latency: number): string {
  if (latency <= 60)
    return 'bg-emerald-500/60'
  if (latency <= 120)
    return 'bg-emerald-600/50'
  if (latency <= 180)
    return 'bg-amber-400/60'
  if (latency <= 240)
    return 'bg-orange-400/60'
  return 'bg-rose-500/60'
}

function getLossToneClass(loss: number): string {
  if (loss <= 1)
    return 'bg-emerald-500/60'
  if (loss <= 3)
    return 'bg-emerald-600/50'
  if (loss <= 6)
    return 'bg-amber-400/60'
  if (loss <= 9)
    return 'bg-orange-400/60'
  return 'bg-rose-500/60'
}

function getNetworkShortName(name: string): string {
  if (name.includes('电信') || name.toUpperCase().includes('CT'))
    return '电'
  if (name.includes('联通') || name.toUpperCase().includes('CU'))
    return '联'
  if (name.includes('移动') || name.toUpperCase().includes('CM'))
    return '移'
  return name.slice(0, 2)
}

function toNetworkDisplay(stat: NodePingPerTaskStat): NodePingNetworkDisplay {
  const isBlocked = stat.loss >= 99 || stat.avgLatency < 0 || (stat.avgLatency === 0 && stat.loss > 0)
  return {
    name: stat.name,
    shortName: getNetworkShortName(stat.name),
    latency: isBlocked ? '--' : `${Math.round(stat.avgLatency)}ms`,
    toneClass: isBlocked ? 'text-rose-500/75 font-mono' : getPingToneClass(stat.avgLatency),
  }
}

export function useNodePingDisplay(
  uuid: MaybeRefOrGetter<string>,
  options: UseNodePingDisplayOptions = {},
) {
  const appStore = useAppStore()
  // Komari 1.2.6+ uses metric-store retention and keeps the legacy public
  // record fields for compatibility only. They can report records as disabled
  // even when ping metrics are available, so only an explicit caller option
  // should prevent the query.
  const pingStatsEnabled = computed(() => options.enabled === undefined || toValue(options.enabled))

  const pingRecordsQueryHours = computed(() => RECENT_PING_RECORDS_QUERY_HOURS)

  const pingStats = useNodePingStats(uuid, {
    hours: pingRecordsQueryHours,
    enabled: pingStatsEnabled,
  })

  const nodesStore = useNodesStore()
  const realtimePingMap = computed(() => {
    const node = nodesStore.nodesByUuid.get(toValue(uuid))
    return node?.ping
  })

  const realtimePerTaskStats = computed<NodePingPerTaskStat[]>(() => {
    const ping = realtimePingMap.value
    if (!ping)
      return []
    return Object.entries(ping).map(([taskIdStr, stat]) => ({
      taskId: Number(taskIdStr),
      name: stat.name || `Ping ${taskIdStr}`,
      avgLatency: stat.avg,
      loss: stat.loss,
    }))
  })

  const hasRealtimePing = computed(() => realtimePerTaskStats.value.length > 0)

  const effectivePerTaskStats = computed(() => {
    if (hasRealtimePing.value)
      return realtimePerTaskStats.value
    return pingStats.perTaskStats.value
  })

  const effectiveAvgLatency = computed(() => {
    if (hasRealtimePing.value) {
      const valid = realtimePerTaskStats.value.filter(s => s.avgLatency > 0 && s.loss < 99)
      if (valid.length === 0)
        return -1
      return valid.reduce((sum, s) => sum + s.avgLatency, 0) / valid.length
    }
    return pingStats.avgLatency.value
  })

  const effectiveAvgLoss = computed(() => {
    if (hasRealtimePing.value) {
      const stats = realtimePerTaskStats.value
      if (stats.length === 0)
        return 0
      return stats.reduce((sum, s) => sum + s.loss, 0) / stats.length
    }
    return pingStats.avgLoss.value
  })

  const isAllBlocked = computed(() => {
    const stats = effectivePerTaskStats.value
    if (stats.length === 0)
      return false
    return stats.every(s => s.loss >= 99 || (s.avgLatency <= 0 && s.loss > 0))
  })

  function buildPingBars(metric: NodePingMetric): NodePingBar[] {
    if (isAllBlocked.value)
      return []

    const points = pingStats.history.value
    if (!points.length)
      return []

    return points.map((point, index) => {
      const value = point[metric]

      return {
        key: `${point.time}-${index}`,
        className: value === null
          ? 'bg-muted-foreground/15'
          : metric === 'latency'
            ? getLatencyToneClass(value)
            : getLossToneClass(value),
        tooltip: value === null
          ? `${formatDateTime(point.time, 'HH:mm:ss')} N/A`
          : metric === 'latency'
            ? `${formatDateTime(point.time, 'HH:mm:ss')}\n${Math.round(value)} ms`
            : `${formatDateTime(point.time, 'HH:mm:ss')}\n${value.toFixed(1)}%`,
      }
    })
  }

  function buildEmptyPingBars(metric: NodePingMetric): NodePingBar[] {
    const isBlocked = isAllBlocked.value
    const isZh = appStore.lang === 'zh-CN'
    const tooltip = isBlocked
      ? metric === 'loss'
        ? (isZh ? '国内阻断 · 100% 丢包' : 'Blocked · 100% Loss')
        : (isZh ? '国内阻断 · 无法连通' : 'Blocked · Unreachable')
      : pingStats.loading.value
        ? '加载中'
        : pingStats.error.value
          ? '加载失败'
          : !pingStatsEnabled.value
              ? '未启用记录'
              : 'N/A'

    return Array.from({ length: NODE_PING_BAR_COUNT }, (_, index) => ({
      key: `${metric}-empty-${index}`,
      className: 'bg-muted-foreground/10',
      tooltip,
    }))
  }

  const latencyBars = computed(() => buildPingBars('latency'))
  const lossBars = computed(() => buildPingBars('loss'))
  const latencyRenderBars = computed(() => latencyBars.value.length ? latencyBars.value : buildEmptyPingBars('latency'))
  const lossRenderBars = computed(() => lossBars.value.length ? lossBars.value : buildEmptyPingBars('loss'))

  const latencyDisplay = computed(() => {
    if (isAllBlocked.value)
      return '--'
    if (hasRealtimePing.value) {
      const lat = effectiveAvgLatency.value
      return lat >= 0 ? `${Math.round(lat)} ms` : '--'
    }
    if (pingStats.hasData.value)
      return `${Math.round(pingStats.avgLatency.value)} ms`
    if (pingStats.loading.value)
      return options.loadingDisplayText ?? '加载中'
    return options.emptyDisplayText ?? '-'
  })

  const lossDisplay = computed(() => {
    if (isAllBlocked.value)
      return '100%'
    if (hasRealtimePing.value)
      return `${effectiveAvgLoss.value.toFixed(1)}%`
    if (pingStats.hasData.value)
      return `${pingStats.avgLoss.value.toFixed(1)}%`
    if (pingStats.loading.value)
      return options.loadingDisplayText ?? '加载中'
    return options.emptyDisplayText ?? '-'
  })

  const latencyPanelTooltip = computed(() => {
    if (isAllBlocked.value) {
      return appStore.lang === 'zh-CN'
        ? '国内探测点全量丢包（IP 可能已被墙或未开放 ICMP）'
        : '100% packet loss from mainland China probes'
    }
    if (hasRealtimePing.value) {
      const lat = effectiveAvgLatency.value
      return lat >= 0 ? `平均延迟 ${Math.round(lat)} ms` : '暂无延迟数据'
    }
    if (!pingStats.hasData.value) {
      if (pingStats.loading.value)
        return options.loadingPanelTooltipText?.latency ?? ''
      return options.emptyPanelTooltipText?.latency ?? ''
    }
    return `平均延迟 ${Math.round(pingStats.avgLatency.value)} ms`
  })

  const lossPanelTooltip = computed(() => {
    if (isAllBlocked.value) {
      return appStore.lang === 'zh-CN'
        ? '平均丢包 100%（国内探测点全部超时）'
        : 'Average packet loss: 100% (all probe targets timed out)'
    }
    if (hasRealtimePing.value) {
      return `平均丢包 ${effectiveAvgLoss.value.toFixed(1)}%`
    }
    if (!pingStats.hasData.value) {
      if (pingStats.loading.value)
        return options.loadingPanelTooltipText?.loss ?? ''
      return options.emptyPanelTooltipText?.loss ?? ''
    }

    const volatility = pingStats.avgVolatility.value > 0
      ? `，平均波动 ${pingStats.avgVolatility.value.toFixed(2)}`
      : ''
    return `平均丢包 ${pingStats.avgLoss.value.toFixed(1)}%${volatility}`
  })

  const topPingNetworks = computed(() => {
    const perTaskStats = effectivePerTaskStats.value
    const configuredNames = appStore.pingNetworkOrder

    // 未配置自定义顺序时保持默认行为：按 taskId 顺序取前 3 条
    if (!configuredNames.length)
      return perTaskStats.slice(0, PING_NETWORK_DISPLAY_COUNT).map(toNetworkDisplay)

    const statsByName = new Map(perTaskStats.map(stat => [stat.name, stat]))
    const selected: NodePingPerTaskStat[] = []
    const usedTaskIds = new Set<number>()

    // 按配置顺序精确匹配节点名称，最多取 3 条
    for (const name of configuredNames) {
      if (selected.length >= PING_NETWORK_DISPLAY_COUNT)
        break
      const stat = statsByName.get(name)
      if (stat && !usedTaskIds.has(stat.taskId)) {
        selected.push(stat)
        usedTaskIds.add(stat.taskId)
      }
    }

    // 不足 3 条时用剩余任务（taskId 升序）补位
    for (const stat of perTaskStats) {
      if (selected.length >= PING_NETWORK_DISPLAY_COUNT)
        break
      if (!usedTaskIds.has(stat.taskId)) {
        selected.push(stat)
        usedTaskIds.add(stat.taskId)
      }
    }

    return selected.map(toNetworkDisplay)
  })

  return {
    pingStats,
    pingStatsEnabled,
    pingRecordsQueryHours,
    latencyRenderBars,
    lossRenderBars,
    latencyDisplay,
    lossDisplay,
    latencyPanelTooltip,
    lossPanelTooltip,
    perTaskStats: pingStats.perTaskStats,
    topPingNetworks,
    isAllBlocked,
  }
}
