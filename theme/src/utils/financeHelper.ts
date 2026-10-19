import type { CurrencyCode, ExchangeRates, ExchangeRateSource } from './financeCore'
import { ref } from 'vue'
import {
  fetchDailyExchangeRates,
  getInitialExchangeRates,
  getStoredFinanceCurrency,
  writeStoredFinanceCurrency,
} from './financeCore'

export * from './financeCore'

export const sharedExchangeRates = ref<ExchangeRates>(getInitialExchangeRates())
export const sharedFinanceCurrency = ref<CurrencyCode>(getStoredFinanceCurrency())

export function setStoredFinanceCurrency(currency: CurrencyCode): void {
  writeStoredFinanceCurrency(currency)
  sharedFinanceCurrency.value = currency
}

export async function getDailyExchangeRates(): Promise<{
  rates: ExchangeRates
  source: ExchangeRateSource
}> {
  const res = await fetchDailyExchangeRates()
  sharedExchangeRates.value = res.rates
  return res
}
