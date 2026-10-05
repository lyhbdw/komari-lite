import type { NodeDetail } from "@/contexts/node-details-context";

const FINANCE_CURRENCY_CONFIG = {
  AUD: { rate: 0.20941, symbol: "A$" },
  BRL: { rate: 0.74734, symbol: "R$" },
  CAD: { rate: 0.20691, symbol: "C$" },
  CHF: { rate: 0.11746, symbol: "CHF" },
  CNY: { rate: 1, symbol: "¥" },
  CZK: { rate: 3.0787, symbol: "Kč" },
  DKK: { rate: 0.95296, symbol: "kr" },
  EUR: { rate: 0.1275, symbol: "€" },
  GBP: { rate: 0.11027, symbol: "£" },
  HKD: { rate: 1.1594, symbol: "$" },
  HUF: { rate: 44.688, symbol: "Ft" },
  IDR: { rate: 2622.37, symbol: "Rp" },
  ILS: { rate: 0.43085, symbol: "₪" },
  INR: { rate: 14.0178, symbol: "₹" },
  ISK: { rate: 18.4626, symbol: "kr" },
  JPY: { rate: 23.707, symbol: "¥" },
  KRW: { rate: 224.11, symbol: "₩" },
  KZT: { rate: 64, symbol: "₸" },
  MXN: { rate: 2.5472, symbol: "Mex$" },
  MYR: { rate: 0.59945, symbol: "RM" },
  NOK: { rate: 1.4096, symbol: "kr" },
  NZD: { rate: 0.2535, symbol: "NZ$" },
  PHP: { rate: 8.9288, symbol: "₱" },
  PLN: { rate: 0.54138, symbol: "zł" },
  RON: { rate: 0.66769, symbol: "lei" },
  RUB: { rate: 11.9, symbol: "₽" },
  SEK: { rate: 1.3895, symbol: "kr" },
  SGD: { rate: 0.18975, symbol: "S$" },
  THB: { rate: 4.8172, symbol: "฿" },
  TRY: { rate: 6.849, symbol: "₺" },
  UAH: { rate: 3.6, symbol: "₴" },
  USD: { rate: 0.14799, symbol: "$" },
  VND: { rate: 3500, symbol: "₫" },
  ZAR: { rate: 2.3995, symbol: "R" },
} as const;

export type CurrencyCode = keyof typeof FINANCE_CURRENCY_CONFIG;

export type ExchangeRates = Record<CurrencyCode, number>;

export const DEFAULT_EXCHANGE_RATES = Object.fromEntries(
  Object.entries(FINANCE_CURRENCY_CONFIG).map(([currency, config]) => [
    currency,
    config.rate,
  ])
) as ExchangeRates;

const CURRENCY_SYMBOLS = Object.fromEntries(
  Object.entries(FINANCE_CURRENCY_CONFIG).map(([currency, config]) => [
    currency,
    config.symbol,
  ])
) as Record<CurrencyCode, string>;

const MS_PER_DAY = 24 * 60 * 60 * 1000;
const LONG_TERM_YEARS = 100;

const EXPLICIT_CURRENCY_ALIASES: Record<string, CurrencyCode> = {
  $: "USD",
  US$: "USD",
  CA$: "CAD",
  "CN¥": "CNY",
  RMB: "CNY",
  "HK$": "HKD",
  "€": "EUR",
  "£": "GBP",
  "¥": "CNY",
  "￥": "CNY",
  "JP¥": "JPY",
};

function normalizeCurrency(
  currency: string | null | undefined
): CurrencyCode {
  const value = String(currency || "CNY")
    .trim()
    .toUpperCase();
  if (value in FINANCE_CURRENCY_CONFIG) {
    return value as CurrencyCode;
  }
  return EXPLICIT_CURRENCY_ALIASES[value] || "CNY";
}

const FREE_NODE_REGEX = /白嫖|免费|free/i;
const NODE_PREMIUM_TAG_REGEX =
  /溢价\s*(\d+(?:\.\d+)?)\s*(?:rmb|cny|usd|eur|gbp|[r元$€£])?/i;

function isFreeNode(node: NodeDetail): boolean {
  const price = Number(node.price);
  if (price === 0 || price === -1) return true;
  const tags = String(node.tags || "");
  return FREE_NODE_REGEX.test(tags);
}

export interface NodePremiumInfo {
  amount: number;
  currency: CurrencyCode;
  symbol: string;
}

function getNodePremiumInfo(node: NodeDetail): NodePremiumInfo {
  let tagAmount: number | null = null;
  let tagCurrency: CurrencyCode | null = null;

  if (node.tags) {
    const match = node.tags.match(NODE_PREMIUM_TAG_REGEX);
    if (match && match[1]) {
      const val = parseFloat(match[1]);
      if (Number.isFinite(val) && val > 0) {
        tagAmount = val;
        const unit = (match[2] || "").toLowerCase();
        if (["r", "元", "rmb", "cny"].includes(unit)) {
          tagCurrency = "CNY";
        } else if (["$", "usd"].includes(unit)) {
          tagCurrency = "USD";
        } else if (["€", "eur"].includes(unit)) {
          tagCurrency = "EUR";
        } else if (["£", "gbp"].includes(unit)) {
          tagCurrency = "GBP";
        }
      }
    }
  }

  const rawPremium = Number(node.premium);
  const hasDbPremium = Number.isFinite(rawPremium) && rawPremium > 0;
  const amount = hasDbPremium ? rawPremium : tagAmount || 0;

  if (amount <= 0) {
    return {
      amount: 0,
      currency: "CNY",
      symbol: CURRENCY_SYMBOLS.CNY,
    };
  }

  let currency: CurrencyCode = "CNY";
  if (node.premium_currency && node.premium_currency.trim() !== "") {
    currency = normalizeCurrency(node.premium_currency);
  } else if (tagCurrency) {
    currency = tagCurrency;
  } else if (node.currency) {
    currency = normalizeCurrency(node.currency);
  }

  return {
    amount,
    currency,
    symbol: CURRENCY_SYMBOLS[currency] || "¥",
  };
}

function getNodePremiumCNY(
  node: NodeDetail,
  exchangeRates: ExchangeRates
): number {
  const { amount, currency } = getNodePremiumInfo(node);
  if (amount <= 0) return 0;
  if (currency === "CNY") return amount;
  const rate = exchangeRates[currency] || 1;
  return amount / rate;
}

function getPriceCNY(
  node: NodeDetail,
  exchangeRates: ExchangeRates
): number {
  const price = Number(node.price);
  if (!Number.isFinite(price) || price <= 0) return 0;
  const currency = normalizeCurrency(node.currency);
  if (currency === "CNY") return price;
  const rate = exchangeRates[currency] || 1;
  return price / rate;
}

function calculateBaseRemainingValueCNY(
  node: NodeDetail,
  exchangeRates: ExchangeRates,
  now = new Date()
): number {
  if (!node.expired_at) return 0;
  const expiredAt = new Date(node.expired_at).getTime();
  if (!Number.isFinite(expiredAt)) return 0;

  const diffMs = expiredAt - now.getTime();
  if (diffMs <= 0) return 0;

  const diffYears = diffMs / (MS_PER_DAY * 365);
  const priceCNY = getPriceCNY(node, exchangeRates);

  if (diffYears > LONG_TERM_YEARS) {
    return priceCNY;
  }

  const billingCycle = Number(node.billing_cycle);
  const billingCycleMs = billingCycle * MS_PER_DAY;
  if (billingCycleMs > 0 && priceCNY > 0) {
    return priceCNY * (diffMs / billingCycleMs);
  }

  return 0;
}

function calculateRemainingValueCNY(
  node: NodeDetail,
  exchangeRates: ExchangeRates,
  now = new Date()
): number {
  if (!node.expired_at) return 0;
  const expiredAt = new Date(node.expired_at).getTime();
  if (!Number.isFinite(expiredAt)) return 0;

  const diffMs = expiredAt - now.getTime();
  if (diffMs <= 0) return 0;

  const baseRemainingCNY = calculateBaseRemainingValueCNY(
    node,
    exchangeRates,
    now
  );
  const premiumCNY = getNodePremiumCNY(node, exchangeRates);

  return baseRemainingCNY + premiumCNY;
}

export function calculateTotalRemainingValueCNY(
  nodes: NodeDetail[],
  exchangeRates: ExchangeRates,
  excludeFreeTags = true,
  now = new Date()
): number {
  return nodes.reduce((sum, node) => {
    if (excludeFreeTags && isFreeNode(node)) return sum;
    return sum + calculateRemainingValueCNY(node, exchangeRates, now);
  }, 0);
}

export function calculateTotalBaseRemainingValueCNY(
  nodes: NodeDetail[],
  exchangeRates: ExchangeRates,
  excludeFreeTags = true,
  now = new Date()
): number {
  return nodes.reduce((sum, node) => {
    if (excludeFreeTags && isFreeNode(node)) return sum;
    return sum + calculateBaseRemainingValueCNY(node, exchangeRates, now);
  }, 0);
}

export function calculateTotalPremiumCNY(
  nodes: NodeDetail[],
  exchangeRates: ExchangeRates,
  excludeFreeTags = true
): number {
  return nodes.reduce((sum, node) => {
    if (excludeFreeTags && isFreeNode(node)) return sum;
    return sum + getNodePremiumCNY(node, exchangeRates);
  }, 0);
}

const MONTH_DAYS = 30;

function calculateMonthlyAverageCostCNY(
  node: NodeDetail,
  exchangeRates: ExchangeRates
): number {
  const priceCNY = getPriceCNY(node, exchangeRates);
  if (priceCNY <= 0) return 0;
  const billingCycle = Number(node.billing_cycle);
  if (!Number.isFinite(billingCycle) || billingCycle <= 0) return 0;
  return (priceCNY / billingCycle) * MONTH_DAYS;
}

export function calculateTotalMonthlyAverageCostCNY(
  nodes: NodeDetail[],
  exchangeRates: ExchangeRates,
  excludeFreeTags = true
): number {
  return nodes.reduce((sum, node) => {
    if (excludeFreeTags && isFreeNode(node)) return sum;
    return sum + calculateMonthlyAverageCostCNY(node, exchangeRates);
  }, 0);
}

export function formatFinanceAmount(
  amount: number,
  currency: CurrencyCode = "CNY"
): {
  currency: CurrencyCode;
  symbol: string;
  value: string;
} {
  const safeAmount = Number.isFinite(amount) ? amount : 0;
  const value = new Intl.NumberFormat("zh-CN", {
    maximumFractionDigits: 2,
    minimumFractionDigits: Math.abs(safeAmount) < 100000 ? 2 : 0,
    notation: Math.abs(safeAmount) >= 100000 ? "compact" : "standard",
  }).format(safeAmount);

  return {
    currency,
    symbol: CURRENCY_SYMBOLS[currency] || "¥",
    value,
  };
}
