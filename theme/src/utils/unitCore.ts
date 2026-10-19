/** 字节单位常量 */
const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const
const LAST_BYTE_UNIT = BYTE_UNITS[BYTE_UNITS.length - 1]

/** 时间单位配置（秒为单位） */
const TIME_UNITS = [
  { value: 86400, label: '天' },
  { value: 3600, label: '小时' },
  { value: 60, label: '分钟' },
  { value: 1, label: '秒' },
] as const

/** 运行时间格式化精度类型 */
export type UptimeFormat = 'day' | 'hour' | 'minute' | 'second'

/** 字节格式化精度配置 */
export interface ByteDecimalsConfig {
  /** B 精确位数，-1 为不显示此单位 */
  B?: number
  /** KB 精确位数，-1 为不显示此单位 */
  KB?: number
  /** MB 精确位数，-1 为不显示此单位 */
  MB?: number
  /** GB 精确位数，-1 为不显示此单位 */
  GB?: number
  /** TB 及以上精确位数，-1 为不显示此单位 */
  TB?: number
}

/** 前后台统一默认字节精度配置 */
export const DEFAULT_BYTE_DECIMALS: ByteDecimalsConfig = {
  B: 0,
  KB: 0,
  MB: 1,
  GB: 1,
  TB: 2,
}

export interface TrafficTelemetryLike {
  online?: boolean | null
  net_in?: number | string | null
  net_out?: number | string | null
  net_total_up?: number | string | null
  net_total_down?: number | string | null
}

export interface TrafficAndSpeedStats {
  totalUp: number
  totalDown: number
  totalBytes: number
  speedUp: number
  speedDown: number
}

/**
 * 统一汇总节点列表的累计上下行流量与实时在线上下行速率
 */
export function calculateTrafficAndSpeedStats(
  items: readonly (TrafficTelemetryLike | null | undefined)[],
): TrafficAndSpeedStats {
  let totalUp = 0
  let totalDown = 0
  let speedUp = 0
  let speedDown = 0

  for (const item of items) {
    if (!item)
      continue

    const up = Number(item.net_total_up || 0)
    const down = Number(item.net_total_down || 0)
    if (Number.isFinite(up) && up > 0)
      totalUp += up
    if (Number.isFinite(down) && down > 0)
      totalDown += down

    if (item.online) {
      const sUp = Number(item.net_out || 0)
      const sDown = Number(item.net_in || 0)
      if (Number.isFinite(sUp) && sUp > 0)
        speedUp += sUp
      if (Number.isFinite(sDown) && sDown > 0)
        speedDown += sDown
    }
  }

  return {
    totalUp,
    totalDown,
    totalBytes: totalUp + totalDown,
    speedUp,
    speedDown,
  }
}

/**
 * 从 RPC common:getNodesLatestStatus 返回的字典汇总流量与速率
 * 若传入 allowedUuids，则仅统计有效节点列表内的节点，与前台 Pinia 节点池行为严格一致
 */
export function calculateTrafficAndSpeedFromStatusMap(
  statusMap: Record<string, TrafficTelemetryLike | null | undefined> | null | undefined,
  allowedUuids?: Iterable<string>,
): TrafficAndSpeedStats {
  if (!statusMap || typeof statusMap !== 'object') {
    return {
      totalUp: 0,
      totalDown: 0,
      totalBytes: 0,
      speedUp: 0,
      speedDown: 0,
    }
  }

  if (allowedUuids) {
    const uuidSet = new Set(allowedUuids)
    if (uuidSet.size > 0) {
      const items: (TrafficTelemetryLike | null | undefined)[] = []
      uuidSet.forEach((uuid) => {
        if (uuid in statusMap)
          items.push(statusMap[uuid])
      })
      return calculateTrafficAndSpeedStats(items)
    }
  }

  return calculateTrafficAndSpeedStats(Object.values(statusMap))
}

/**
 * 格式化字节数为分离的数值和单位（支持自定义精度配置）
 */
export function formatBytesSplit(bytes: number, config?: ByteDecimalsConfig): { value: string, unit: string } {
  const mergedConfig = { ...DEFAULT_BYTE_DECIMALS, ...config }

  if (!Number.isFinite(bytes) || bytes <= 0) {
    if (mergedConfig.B === -1)
      return { value: '0', unit: 'KB' }
    return { value: '0', unit: 'B' }
  }

  const k = 1024
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), BYTE_UNITS.length - 1)

  const unitKey = BYTE_UNITS[i]
  const decimals = (unitKey === 'TB' || unitKey === 'PB') ? mergedConfig.TB : mergedConfig[unitKey as keyof ByteDecimalsConfig]

  if (decimals === -1) {
    for (let j = i + 1; j < BYTE_UNITS.length; j++) {
      const nextUnitKey = BYTE_UNITS[j]
      const nextDecimals = (nextUnitKey === 'TB' || nextUnitKey === 'PB') ? mergedConfig.TB : mergedConfig[nextUnitKey as keyof ByteDecimalsConfig]
      if (nextDecimals !== -1) {
        const unit = BYTE_UNITS[j]
        return { value: (bytes / k ** j).toFixed(nextDecimals), unit: `${unit}` }
      }
    }
    const unit = BYTE_UNITS[i] ?? LAST_BYTE_UNIT
    return { value: (bytes / k ** i).toFixed(1), unit: `${unit}` }
  }

  const unit = BYTE_UNITS[i] ?? LAST_BYTE_UNIT
  return { value: (bytes / k ** i).toFixed(decimals), unit: `${unit}` }
}

/**
 * 格式化字节数为可读单位（支持自定义精度配置）
 */
export function formatBytesWithConfig(bytes: number, config?: ByteDecimalsConfig): string {
  const { value, unit } = formatBytesSplit(bytes, config)
  return `${value} ${unit}`
}

/**
 * 格式化字节数为可读单位
 * 未传 decimals 时默认使用前后台统一的 DEFAULT_BYTE_DECIMALS 精度规则
 */
export function formatBytes(bytes: number, decimals?: number): string {
  if (decimals === undefined)
    return formatBytesWithConfig(bytes, DEFAULT_BYTE_DECIMALS)

  if (!Number.isFinite(bytes) || bytes <= 0)
    return '0 B'

  const k = 1024
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), BYTE_UNITS.length - 1)
  const unit = BYTE_UNITS[i] ?? LAST_BYTE_UNIT
  return `${(bytes / k ** i).toFixed(decimals)} ${unit}`
}

/**
 * 格式化字节速率为分离的数值和单位（支持自定义精度配置）
 */
export function formatBytesPerSecondSplit(bytes: number, config?: ByteDecimalsConfig): { value: string, unit: string } {
  const result = formatBytesSplit(bytes, config)
  return { value: result.value, unit: `${result.unit}/s` }
}

/**
 * 格式化字节速率为可读单位（支持自定义精度配置）
 */
export function formatBytesPerSecondWithConfig(bytes: number, config?: ByteDecimalsConfig): string {
  return `${formatBytesWithConfig(bytes, config)}/s`
}

/**
 * 格式化字节速率为可读单位（使用前后台统一精度配置）
 */
export function formatBytesPerSecond(bytesPerSec: number): string {
  return formatBytesPerSecondWithConfig(bytesPerSec, DEFAULT_BYTE_DECIMALS)
}

/**
 * 将表示数据大小的字符串（如 '1.5MB', '128*1024gb'）转换为字节数
 */
export function stringToBytes(str: string): number {
  if (typeof str !== 'string' || str.length === 0) {
    return 0
  }
  const units: Record<string, number> = {
    b: 1,
    byte: 1,
    bytes: 1,
    k: 1024,
    kb: 1024,
    kib: 1024,
    kilobyte: 1024,
    m: 1024 ** 2,
    mb: 1024 ** 2,
    mib: 1024 ** 2,
    megabyte: 1024 ** 2,
    g: 1024 ** 3,
    gb: 1024 ** 3,
    gib: 1024 ** 3,
    gigabyte: 1024 ** 3,
    t: 1024 ** 4,
    tb: 1024 ** 4,
    tib: 1024 ** 4,
    terabyte: 1024 ** 4,
    p: 1024 ** 5,
    pb: 1024 ** 5,
    pib: 1024 ** 5,
    petabyte: 1024 ** 5,
  }

  const cleanStr = str.toLowerCase().replace(/,/g, '').replace(/\s/g, '')
  const unitKeys = Object.keys(units).sort((a, b) => b.length - a.length)
  const unitRegex = new RegExp(`(${unitKeys.join('|')})$`)

  let unit = 'b'
  let numericPart = cleanStr

  const match = cleanStr.match(unitRegex)
  if (match && match[1]) {
    unit = match[1]
    numericPart = cleanStr.slice(0, cleanStr.length - unit.length)
  }

  if (numericPart === '') {
    numericPart = '1'
  }

  try {
    // eslint-disable-next-line no-new-func
    const value = new Function(`return ${numericPart}`)()
    if (Number.isNaN(value)) {
      return 0
    }
    const multiplier = units[unit] || 1
    return Math.round(value * multiplier)
  }
  catch (error) {
    console.error(`Error parsing string "${str}":`, error)
    return 0
  }
}

/**
 * 格式化运行时间（支持自定义精度）
 */
export function formatUptimeWithFormat(seconds: number, format: UptimeFormat = 'day'): string {
  if (!seconds || seconds <= 0)
    return '0 秒'

  const formatMaxUnitIndexMap: Record<UptimeFormat, number> = {
    day: 0,
    hour: 1,
    minute: 2,
    second: 3,
  }

  const maxUnitIndex = formatMaxUnitIndexMap[format]
  const parts: string[] = []
  let remaining = seconds

  for (let i = 0; i < TIME_UNITS.length; i++) {
    const unit = TIME_UNITS[i]
    if (!unit)
      continue
    const { value, label } = unit
    const amount = Math.floor(remaining / value)
    if (amount > 0) {
      parts.push(`${amount} ${label}`)
      remaining %= value
    }
    if (i >= maxUnitIndex) {
      break
    }
  }

  if (parts.length === 0) {
    const fallbackUnit = TIME_UNITS[maxUnitIndex]
    const fallbackLabel = fallbackUnit?.label ?? '秒'
    return `不足 1 ${fallbackLabel}`
  }

  return parts.join(' ')
}

const STATUS_THRESHOLDS = {
  success: 60,
  warning: 80,
} as const

export function getStatus(percentage: number): 'success' | 'warning' | 'error' {
  if (percentage < STATUS_THRESHOLDS.success)
    return 'success'
  if (percentage < STATUS_THRESHOLDS.warning)
    return 'warning'
  return 'error'
}

export function getStatusTextClass(percentage: number): string {
  if (percentage >= STATUS_THRESHOLDS.warning)
    return 'text-rose-600 dark:text-rose-400 font-bold'
  if (percentage >= STATUS_THRESHOLDS.success)
    return 'text-amber-600 dark:text-amber-400 font-semibold'
  return 'text-foreground/90 font-medium'
}
