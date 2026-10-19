import { CURRENCY_SYMBOLS, normalizeCurrency } from './financeCore'

/** 计费周期类型 */
export type BillingCycleType =
  | 'monthly'
  | 'quarterly'
  | 'semi_annual'
  | 'annual'
  | 'biennial'
  | 'triennial'
  | 'quinquennial'
  | 'once'
  | 'custom'

/** 过期状态类型 */
export type ExpireStatus = 'expired' | 'critical' | 'warning' | 'normal' | 'long_term'

/** 计费周期范围配置（天） */
export const BILLING_CYCLE_RANGES: ReadonlyArray<{ type: BillingCycleType, min: number, max: number }> = [
  { type: 'monthly', min: 27, max: 32 },
  { type: 'quarterly', min: 87, max: 95 },
  { type: 'semi_annual', min: 175, max: 185 },
  { type: 'annual', min: 360, max: 370 },
  { type: 'biennial', min: 720, max: 750 },
  { type: 'triennial', min: 1080, max: 1150 },
  { type: 'quinquennial', min: 1800, max: 1850 },
]

/** 过期状态阈值配置（天） */
export const EXPIRE_THRESHOLDS = {
  critical: 7,
  warning: 30,
  theme_warning: 15,
  long_term: 36500,
} as const

const MS_PER_DAY = 24 * 60 * 60 * 1000

/**
 * 解析计费周期类型
 */
export function parseBillingCycleType(billingCycle: number): BillingCycleType {
  if (billingCycle === -1)
    return 'once'

  for (const range of BILLING_CYCLE_RANGES) {
    if (billingCycle >= range.min && billingCycle <= range.max) {
      return range.type
    }
  }

  return 'custom'
}

/**
 * 获取计费周期的显示文本
 */
export function getBillingCycleText(billingCycle: number, lang: 'zh-CN' | 'en-US' = 'zh-CN'): string {
  const type = parseBillingCycleType(billingCycle)

  const texts: Record<BillingCycleType, Record<'zh-CN' | 'en-US', string>> = {
    monthly: { 'zh-CN': '月', 'en-US': 'Month' },
    quarterly: { 'zh-CN': '季', 'en-US': 'Quarter' },
    semi_annual: { 'zh-CN': '半年', 'en-US': 'Semi-Annual' },
    annual: { 'zh-CN': '年', 'en-US': 'Year' },
    biennial: { 'zh-CN': '两年', 'en-US': 'Biennial' },
    triennial: { 'zh-CN': '三年', 'en-US': 'Triennial' },
    quinquennial: { 'zh-CN': '五年', 'en-US': 'Quinquennial' },
    once: { 'zh-CN': '一次性', 'en-US': 'Once' },
    custom: { 'zh-CN': `${billingCycle} 天`, 'en-US': `${billingCycle} Days` },
  }

  return texts[type][lang]
}

/**
 * 获取计费周期对应的 i18n key（用于管理后台 PriceTags）
 */
export function getBillingCycleI18nKey(billingCycle: number): string | null {
  const type = parseBillingCycleType(billingCycle)
  if (type === 'custom')
    return null
  return `common.${type}`
}

/**
 * 计算距离过期的天数（正数表示剩余天数，负数/0表示已过期）
 */
export function getDaysUntilExpired(
  expiredAt: string | number | Date | null | undefined,
  now = new Date(),
): number {
  if (!expiredAt)
    return 0

  const expMs = new Date(expiredAt).getTime()
  const nowMs = now.getTime()
  if (!Number.isFinite(expMs))
    return 0

  const diffDays = (expMs - nowMs) / MS_PER_DAY
  if (expMs < nowMs)
    return Math.floor(diffDays)

  return Math.max(1, Math.ceil(diffDays))
}

/**
 * 判定节点是否在指定天数（默认 7 天）内即将到期
 */
export function isNodeExpiringSoon(
  expiredAt: string | number | Date | null | undefined,
  withinDays: number = EXPIRE_THRESHOLDS.critical,
  now = new Date(),
): boolean {
  if (!expiredAt)
    return false

  const expMs = new Date(expiredAt).getTime()
  const nowMs = now.getTime()
  if (!Number.isFinite(expMs))
    return false

  return expMs > nowMs && expMs <= nowMs + withinDays * MS_PER_DAY
}

/**
 * 获取过期状态
 */
export function getExpireStatus(
  expiredAt: string | number | Date | null | undefined,
  warningDays: number = EXPIRE_THRESHOLDS.theme_warning,
  now = new Date(),
): ExpireStatus {
  if (!expiredAt)
    return 'normal'

  const expMs = new Date(expiredAt).getTime()
  const nowMs = now.getTime()
  if (!Number.isFinite(expMs))
    return 'normal'

  if (expMs < nowMs)
    return 'expired'

  const days = Math.ceil((expMs - nowMs) / MS_PER_DAY)
  if (days < EXPIRE_THRESHOLDS.critical)
    return 'critical'
  if (days < warningDays)
    return 'warning'
  if (days > EXPIRE_THRESHOLDS.long_term)
    return 'long_term'
  return 'normal'
}

/**
 * 获取过期时间的文本颜色类
 */
export function getExpireTextClass(expiredAt: string | number | Date | null | undefined): string {
  const status = getExpireStatus(expiredAt)

  if (status === 'expired' || status === 'critical')
    return 'text-rose-600 dark:text-rose-400 font-bold'
  if (status === 'warning')
    return 'text-amber-600 dark:text-amber-400 font-medium'
  if (status === 'long_term')
    return 'text-muted-foreground'
  return 'text-foreground'
}

/**
 * 获取过期时间的显示文本
 */
export function getExpireText(
  expiredAt: string | number | Date | null | undefined,
  lang: 'zh-CN' | 'en-US' = 'zh-CN',
): string {
  const days = getDaysUntilExpired(expiredAt)
  const status = getExpireStatus(expiredAt)

  if (status === 'expired') {
    return lang === 'zh-CN' ? '已过期' : 'Expired'
  }

  if (status === 'long_term') {
    return lang === 'zh-CN' ? '长期' : 'Long-term'
  }

  if (lang === 'zh-CN') {
    return `${days} 天`
  }
  return `${days} days`
}

/**
 * 格式化价格显示
 */
export function formatPrice(price: number, currency: string = 'CNY', lang: 'zh-CN' | 'en-US' = 'zh-CN'): string {
  if (price === 0 || price === -1)
    return lang === 'zh-CN' ? '免费' : 'Free'
  const code = normalizeCurrency(currency)
  const symbol = CURRENCY_SYMBOLS[code]
  return `${symbol}${price}`
}

/**
 * 格式化价格和计费周期
 */
export function formatPriceWithCycle(
  price: number,
  billingCycle: number,
  currency: string = 'CNY',
  lang: 'zh-CN' | 'en-US' = 'zh-CN',
): string {
  const priceText = formatPrice(price, currency, lang)
  const cycleText = getBillingCycleText(billingCycle, lang)
  return price > 0 ? `${priceText}/${cycleText}` : priceText
}

/**
 * 计算续费一个周期后的新到期日（与后端 utils/renewal/renewal.go 自然月/自然年规则严格一致）
 */
export function calculateNextRenewalDate(
  expiredAt: string | number | Date | null | undefined,
  billingCycle: number,
  now = new Date(),
): Date {
  const cycleDays = Number(billingCycle) > 0 ? Number(billingCycle) : 30
  let baseDate = new Date(now.getTime())

  if (expiredAt) {
    const expDate = new Date(expiredAt)
    const expMs = expDate.getTime()
    // 与后端一致：若未过期，或过期不足 30 天，则从原到期日顺延；若已过期超过 30 天，则从当前时间顺延
    if (Number.isFinite(expMs) && expMs >= now.getTime() - 30 * MS_PER_DAY) {
      baseDate = expDate
    }
  }

  const next = new Date(baseDate.getTime())
  const cycleType = parseBillingCycleType(cycleDays)

  switch (cycleType) {
    case 'monthly':
      next.setMonth(next.getMonth() + 1)
      break
    case 'quarterly':
      next.setMonth(next.getMonth() + 3)
      break
    case 'semi_annual':
      next.setMonth(next.getMonth() + 6)
      break
    case 'annual':
      next.setFullYear(next.getFullYear() + 1)
      break
    case 'biennial':
      next.setFullYear(next.getFullYear() + 2)
      break
    case 'triennial':
      next.setFullYear(next.getFullYear() + 3)
      break
    case 'quinquennial':
      next.setFullYear(next.getFullYear() + 5)
      break
    default:
      return new Date(baseDate.getTime() + cycleDays * MS_PER_DAY)
  }

  return next
}
