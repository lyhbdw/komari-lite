<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import type { CurrencyCode } from '@/utils/financeHelper'
import { Icon } from '@iconify/vue'
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import NodeEarthGlobe from '@/components/NodeEarthGlobe.vue'
import { CardX } from '@/components/ui/card-x'
import { DataTooltip } from '@/components/ui/data-tooltip'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useBackgroundSurface } from '@/composables/useBackgroundSurface'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import * as financeHelper from '@/utils/financeHelper'
import { formatBytesPerSecondSplit, formatBytesSplit } from '@/utils/helper'

const props = defineProps<{
  nodes?: NodeData[]
  globeNodes?: NodeData[]
  transitionKey?: string
}>()

const NodeEarthMaps = defineAsyncComponent(() => import('@/components/NodeEarthMaps.vue'))

const appStore = useAppStore()
const { pickSurfaceClass } = useBackgroundSurface()
const nodesStore = useNodesStore()
const exchangeRates = ref(financeHelper.DEFAULT_EXCHANGE_RATES)
const exchangeRateBaseCurrency = ref<CurrencyCode>('CNY')
const excludeFreeNodes = ref(true)
const financeRateCurrencies: readonly CurrencyCode[] = financeHelper.DISPLAY_FINANCE_CURRENCIES
const summaryNodes = computed(() => props.nodes ?? nodesStore.nodes)
const summaryTransitionKey = computed(() => props.transitionKey ?? 'all')
const metricSwitchTransitionProps = computed(() => ({
  ...(appStore.disablePageAnimation
    ? { css: false }
    : { name: 'metric-switch', mode: 'out-in' as const }),
}))

const openFinanceDialog = ref(false)

function getMetricSwitchStyle(index: number): Record<string, string> {
  return {
    '--metric-switch-delay': `${index * 35}ms`,
  }
}

function setExchangeRateBaseCurrency(event: Event): void {
  const target = event.target as HTMLSelectElement
  exchangeRateBaseCurrency.value = financeHelper.normalizeCurrency(target.value)
  financeHelper.setStoredFinanceCurrency(exchangeRateBaseCurrency.value)
}

const totalSpeed = computed(() => {
  const onlineNodes = summaryNodes.value.filter(node => node.online)
  const up = onlineNodes.reduce((sum, node) => sum + (node.net_out || 0), 0)
  const down = onlineNodes.reduce((sum, node) => sum + (node.net_in || 0), 0)
  return { up, down }
})

const totalTraffic = computed(() => {
  const up = summaryNodes.value.reduce((sum, node) => sum + (node.net_total_up || 0), 0)
  const down = summaryNodes.value.reduce((sum, node) => sum + (node.net_total_down || 0), 0)
  return { up, down }
})

const formattedTrafficUp = computed(() => formatBytesSplit(totalTraffic.value.up, appStore.byteDecimals))
const formattedTrafficDown = computed(() => formatBytesSplit(totalTraffic.value.down, appStore.byteDecimals))
const totalTrafficTooltip = computed(() => formatBytesSplit(totalTraffic.value.up + totalTraffic.value.down, appStore.byteDecimals))

const formattedSpeedUp = computed(() => formatBytesPerSecondSplit(totalSpeed.value.up, appStore.byteDecimals))
const formattedSpeedDown = computed(() => formatBytesPerSecondSplit(totalSpeed.value.down, appStore.byteDecimals))

// ==================== 内存 / 硬盘 汇总 ====================
// 离线节点的 ram / disk 为 0，不影响 used 求和；mem_total / disk_total 是静态库存信息，按全量统计
const totalMemory = computed(() => {
  let used = 0
  let total = 0
  for (const node of summaryNodes.value) {
    used += node.ram || 0
    total += node.mem_total || 0
  }
  return { used, total }
})

const totalDisk = computed(() => {
  let used = 0
  let total = 0
  for (const node of summaryNodes.value) {
    used += node.disk || 0
    total += node.disk_total || 0
  }
  return { used, total }
})

const formattedMemoryUsed = computed(() => formatBytesSplit(totalMemory.value.used, appStore.byteDecimals))
const formattedMemoryTotal = computed(() => formatBytesSplit(totalMemory.value.total, appStore.byteDecimals))
const formattedDiskUsed = computed(() => formatBytesSplit(totalDisk.value.used, appStore.byteDecimals))
const formattedDiskTotal = computed(() => formatBytesSplit(totalDisk.value.total, appStore.byteDecimals))

const remainingValueCNY = computed(() => {
  return financeHelper.calculateTotalRemainingValueCNY(summaryNodes.value, exchangeRates.value, excludeFreeNodes.value)
})
const targetExchangeRate = computed(() => exchangeRates.value[exchangeRateBaseCurrency.value] || 1)
const remainingValue = computed(() => {
  return remainingValueCNY.value * targetExchangeRate.value
})
const formattedRemainingValue = computed(() => {
  return financeHelper.formatFinanceAmount(remainingValue.value, exchangeRateBaseCurrency.value)
})
const baseRemainingValueCNY = computed(() => {
  return financeHelper.calculateTotalBaseRemainingValueCNY(summaryNodes.value, exchangeRates.value, excludeFreeNodes.value)
})
const baseRemainingValue = computed(() => {
  return baseRemainingValueCNY.value * targetExchangeRate.value
})
const formattedBaseRemainingValue = computed(() => {
  return financeHelper.formatFinanceAmount(baseRemainingValue.value, exchangeRateBaseCurrency.value)
})
const totalPremiumCNY = computed(() => {
  return financeHelper.calculateTotalPremiumCNY(summaryNodes.value, exchangeRates.value, excludeFreeNodes.value)
})
const totalPremium = computed(() => {
  return totalPremiumCNY.value * targetExchangeRate.value
})
const formattedTotalPremium = computed(() => {
  return financeHelper.formatFinanceAmount(totalPremium.value, exchangeRateBaseCurrency.value)
})
const monthlyAverageCostCNY = computed(() => {
  return financeHelper.calculateTotalMonthlyAverageCostCNY(summaryNodes.value, exchangeRates.value, excludeFreeNodes.value)
})
const monthlyAverageCost = computed(() => {
  return monthlyAverageCostCNY.value * targetExchangeRate.value
})
const formattedMonthlyAverageCost = computed(() => {
  return financeHelper.formatFinanceAmount(monthlyAverageCost.value, exchangeRateBaseCurrency.value)
})
const financeSummaryItems = computed(() => [
  {
    label: '剩余价值',
    icon: 'tabler:coins',
    value: formattedBaseRemainingValue.value.value,
    symbol: formattedBaseRemainingValue.value.symbol,
    currency: formattedBaseRemainingValue.value.currency,
  },
  {
    label: '总溢价投入',
    icon: 'tabler:coins-plus',
    value: formattedTotalPremium.value.value,
    symbol: formattedTotalPremium.value.symbol,
    currency: formattedTotalPremium.value.currency,
  },
  {
    label: '月均支出',
    icon: 'tabler:receipt-2',
    value: formattedMonthlyAverageCost.value.value,
    symbol: formattedMonthlyAverageCost.value.symbol,
    currency: `${formattedMonthlyAverageCost.value.currency}/月`,
  },
  {
    label: '剩余总价值',
    icon: 'tabler:wallet',
    value: formattedRemainingValue.value.value,
    symbol: formattedRemainingValue.value.symbol,
    currency: formattedRemainingValue.value.currency,
  },
])
const exchangeRateRows = computed(() => financeRateCurrencies.map((currency) => {
  const baseRate = exchangeRates.value[exchangeRateBaseCurrency.value] || 1
  const targetRate = exchangeRates.value[currency] || 1
  const rate = targetRate / baseRate

  return {
    currency,
    baseCurrency: exchangeRateBaseCurrency.value,
    baseSymbol: financeHelper.CURRENCY_SYMBOLS[exchangeRateBaseCurrency.value],
    targetSymbol: financeHelper.CURRENCY_SYMBOLS[currency],
    rate: new Intl.NumberFormat('zh-CN', {
      maximumFractionDigits: 6,
      minimumFractionDigits: 6,
    }).format(rate),
  }
}))
const showEarth = computed(() => appStore.earthViewMode === 'earth' || appStore.earthViewMode === 'earth-stop')
const showMaps = computed(() => appStore.earthViewMode === 'maps')
const showVisualPanel = computed(() => showEarth.value || showMaps.value)
const wrapperClass = computed(() => showVisualPanel.value
  ? 'p-4 grid grid-cols-12 grid-rows-1 gap-2 h-auto md:h-58'
  : 'p-4 grid grid-cols-1 gap-2 h-auto')
const cardGridClass = computed(() => showVisualPanel.value
  ? 'h-42 -mt-42 md:mt-0 col-span-12 row-start-3 z-9 md:h-auto md:col-span-6 md:row-start-1 grid grid-cols-12 grid-rows-2 gap-2'
  : 'col-span-1 grid grid-cols-3 md:grid-cols-6 gap-2')

onMounted(async () => {
  exchangeRateBaseCurrency.value = financeHelper.getStoredFinanceCurrency()
  excludeFreeNodes.value = financeHelper.shouldExcludeFreeNodes()

  const { rates } = await financeHelper.getDailyExchangeRates()
  exchangeRates.value = rates
})
</script>

<template>
  <div :class="wrapperClass">
    <NodeEarthGlobe v-if="showEarth" :nodes="globeNodes" class="col-span-12 col-start-1 md:col-span-6 md:col-start-7" />
    <NodeEarthMaps v-else-if="showMaps" :nodes="globeNodes" class="col-span-12 col-start-1 md:col-span-6 md:col-start-7" />

    <div :class="cardGridClass">
      <CardX
        hoverable
        class="group h-full border border-border bg-card hover:border-foreground/35 hover:shadow-xs rounded-md transition-all shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
        :class="[
          showVisualPanel ? 'col-span-4 row-span-1 col-start-1 row-start-1' : 'col-span-1 row-start-1 col-start-1 min-h-18 md:min-h-24 md:row-start-1 md:col-start-1',
        ]"
        content-class="h-full !p-3"
      >
        <div class="flex h-full flex-col justify-between gap-1">
          <div class="flex items-start justify-between">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">内存用量</span>
            <Icon
              icon="tabler:cash" :width="20" :height="20"
              class="text-slate-500/20 group-hover:text-slate-500 transition-colors"
            />
          </div>
          <Transition v-bind="metricSwitchTransitionProps">
            <div
              :key="`memory-${summaryTransitionKey}`" class="flex items-baseline gap-1 min-w-0"
              :style="getMetricSwitchStyle(0)"
            >
              <span class="text-md md:text-2xl font-bold leading-none tracking-tight font-mono tabular-nums">
                {{ formattedMemoryUsed.value }}
              </span>
              <span class="text-[11px] md:text-xs font-medium text-muted-foreground truncate font-mono tabular-nums">
                {{ formattedMemoryUsed.unit }} / {{ formattedMemoryTotal.value }} {{ formattedMemoryTotal.unit }}
              </span>
            </div>
          </Transition>
        </div>
      </CardX>
      <CardX
        hoverable
        class="group h-full border border-border bg-card hover:border-foreground/35 hover:shadow-xs rounded-md transition-all shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
        :class="[
          showVisualPanel ? 'col-span-4 row-span-1 col-start-1 row-start-2' : 'col-span-1 row-start-2 col-start-1 min-h-18 md:min-h-24 md:row-start-1 md:col-start-2',
        ]"
        content-class="h-full !p-3"
      >
        <div class="flex h-full flex-col justify-between gap-1">
          <div class="flex items-start justify-between">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">硬盘用量</span>
            <Icon
              icon="tabler:server-2" :width="20" :height="20"
              class="text-slate-500/20 group-hover:text-slate-500 transition-colors"
            />
          </div>
          <Transition v-bind="metricSwitchTransitionProps">
            <div
              :key="`disk-${summaryTransitionKey}`" class="flex items-baseline gap-1 min-w-0"
              :style="getMetricSwitchStyle(1)"
            >
              <span class="text-md md:text-2xl font-bold leading-none tracking-tight font-mono tabular-nums">{{ formattedDiskUsed.value
              }}</span>
              <span class="text-[11px] md:text-xs font-medium text-muted-foreground truncate font-mono tabular-nums">
                {{ formattedDiskUsed.unit }} / {{ formattedDiskTotal.value }} {{ formattedDiskTotal.unit }}
              </span>
            </div>
          </Transition>
        </div>
      </CardX>
      <div
        class="relative w-full h-full"
        :class="showVisualPanel ? 'col-span-4 row-span-1 col-start-5 row-start-1' : 'col-span-1 row-start-1 col-start-2 min-h-18 md:min-h-24 md:row-start-1 md:col-start-3'"
      >
        <CardX
          hoverable
          class="group h-full border border-border bg-card hover:border-foreground/35 hover:shadow-xs rounded-md transition-all cursor-pointer shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
          content-class="h-full !p-3" @click="openFinanceDialog = true"
        >
          <div class="flex h-full flex-col justify-between gap-1">
            <div class="flex items-start justify-between">
              <span class="text-xs font-medium tracking-wider text-muted-foreground flex items-center gap-1">
                剩余总价值
                <Icon
                  icon="tabler:arrow-up-right" :width="13" :height="13"
                  class="text-muted-foreground/40 group-hover:text-foreground transition-colors"
                />
              </span>
              <Icon
                icon="tabler:cash" :width="20" :height="20"
                class="text-slate-500/20 group-hover:text-slate-500 transition-colors"
              />
            </div>
            <Transition v-bind="metricSwitchTransitionProps">
              <div
                :key="`remaining-value-${summaryTransitionKey}`" class="flex items-baseline gap-1 min-w-0"
                :style="getMetricSwitchStyle(2)"
              >
                <span class="text-md md:text-2xl font-bold leading-none tracking-tight font-mono tabular-nums">
                  {{ formattedRemainingValue.symbol }}{{ formattedRemainingValue.value }}
                </span>
                <span class="block truncate text-[11px] md:text-xs font-medium text-muted-foreground">
                  {{ formattedRemainingValue.currency }}
                </span>
              </div>
            </Transition>
          </div>
        </CardX>
      </div>
      <CardX
        hoverable
        class="group h-full border border-border bg-card hover:border-foreground/35 hover:shadow-xs rounded-md transition-all shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
        :class="[
          showVisualPanel ? 'col-span-4 row-span-1 col-start-5 row-start-2' : 'col-span-1 row-start-2 col-start-2 min-h-18 md:min-h-24 md:row-start-1 md:col-start-4',
        ]"
        content-class="h-full !p-3"
      >
        <div class="flex h-full flex-col justify-between gap-1">
          <div class="flex items-start justify-between">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">累计流量</span>
            <Icon
              icon="tabler:download" :width="20" :height="20"
              class="text-slate-500/20 group-hover:text-slate-500 transition-colors"
            />
          </div>
          <DataTooltip
            as="span" placement="top"
            :content="`↑ ${formattedTrafficUp.value} ${formattedTrafficUp.unit}\n↓ ${formattedTrafficDown.value} ${formattedTrafficDown.unit}`"
            class="min-w-0" content-class="whitespace-pre px-2 py-1 left-0 -translate-x-0 leading-normal"
          >
            <Transition v-bind="metricSwitchTransitionProps">
              <div
                :key="`traffic-${summaryTransitionKey}`" class="flex items-baseline gap-1"
                :style="getMetricSwitchStyle(3)"
              >
                <span class="inline-block text-md md:text-2xl font-bold leading-none tracking-tight font-mono tabular-nums">
                  {{ totalTrafficTooltip.value }}
                </span>
                <span class="inline-block text-[11px] md:text-xs font-medium text-muted-foreground">
                  {{ totalTrafficTooltip.unit }}
                </span>
              </div>
            </Transition>
          </DataTooltip>
        </div>
      </CardX>

      <CardX
        hoverable
        class="group h-full border border-border bg-card hover:border-foreground/35 hover:shadow-xs rounded-md transition-all shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
        :class="[
          showVisualPanel ? 'col-span-4 row-span-1 col-start-9 row-start-1' : 'col-span-1 row-start-1 col-start-3 min-h-18 md:min-h-24 md:row-start-1 md:col-start-5',
        ]"
        content-class="h-full !p-3"
      >
        <div class="flex h-full flex-col justify-between gap-1">
          <div class="flex items-start justify-between">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">实时上行</span>
            <Icon
              icon="tabler:chevrons-up" :width="20" :height="20"
              class="text-slate-500/20 group-hover:text-slate-500 transition-colors"
            />
          </div>
          <Transition v-bind="metricSwitchTransitionProps">
            <div
              :key="`speed-up-${summaryTransitionKey}`" class="flex items-baseline gap-1"
              :style="getMetricSwitchStyle(4)"
            >
              <span class="text-md md:text-2xl font-bold leading-none tracking-tight font-mono tabular-nums">{{ formattedSpeedUp.value
              }}</span>
              <span class="text-[11px] md:text-xs font-medium text-muted-foreground">{{ formattedSpeedUp.unit }}</span>
            </div>
          </Transition>
        </div>
      </CardX>
      <CardX
        hoverable
        class="group h-full border border-border bg-card hover:border-foreground/35 hover:shadow-xs rounded-md transition-all shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
        :class="[
          showVisualPanel ? 'col-span-4 row-span-1 col-start-9 row-start-2' : 'col-span-1 row-start-2 col-start-3 min-h-18 md:min-h-24 md:row-start-1 md:col-start-6',
        ]"
        content-class="h-full !p-3"
      >
        <div class="flex h-full flex-col justify-between gap-1">
          <div class="flex items-start justify-between">
            <span class="text-xs font-medium tracking-wider text-muted-foreground">实时下行</span>
            <Icon
              icon="tabler:chevrons-down" :width="20" :height="20"
              class="text-slate-500/20 group-hover:text-slate-500 transition-colors"
            />
          </div>
          <Transition v-bind="metricSwitchTransitionProps">
            <div
              :key="`speed-down-${summaryTransitionKey}`" class="flex items-baseline gap-1"
              :style="getMetricSwitchStyle(5)"
            >
              <span class="text-md md:text-2xl font-bold leading-none tracking-tight font-mono tabular-nums">
                {{ formattedSpeedDown.value }}
              </span>
              <span class="text-[11px] md:text-xs font-medium text-muted-foreground">{{ formattedSpeedDown.unit
              }}</span>
            </div>
          </Transition>
        </div>
      </CardX>
    </div>

    <!-- 剩余价值与汇率详情模态弹窗 (Vercel 极简无彩色设计) -->
    <Dialog v-model:open="openFinanceDialog">
      <DialogContent class="sm:max-w-xl border-border/80 bg-background/95 backdrop-blur-md shadow-2xl p-0 overflow-hidden">
        <DialogHeader class="p-5 border-b border-border/70 bg-card/60">
          <div class="flex items-center justify-between pr-6">
            <div class="flex items-center gap-2.5">
              <div class="p-2 rounded-md bg-muted text-foreground border border-border/60">
                <Icon icon="tabler:cash-banknote" :width="18" :height="18" />
              </div>
              <div>
                <DialogTitle class="text-base font-semibold tracking-tight text-foreground">
                  资产与汇率详情
                </DialogTitle>
                <DialogDescription class="text-xs text-muted-foreground mt-0.5">
                  所有节点的剩余价值与溢价汇总及今日实时汇率
                </DialogDescription>
              </div>
            </div>
          </div>
        </DialogHeader>

        <div class="p-5 space-y-4">
          <!-- 核心财务统计指标卡片 -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
            <div
              v-for="(item, index) in financeSummaryItems"
              :key="item.label"
              class="flex flex-col p-3 rounded-md border border-border/70 bg-card transition-colors hover:border-foreground/30 group"
            >
              <span class="text-[11px] font-medium text-muted-foreground flex items-center gap-1 mb-1">
                {{ item.label }}
              </span>
              <Transition v-bind="metricSwitchTransitionProps">
                <div
                  :key="`remaining-modal-${summaryTransitionKey}-${exchangeRateBaseCurrency}`"
                  class="flex items-baseline truncate mt-auto"
                  :style="getMetricSwitchStyle(index)"
                >
                  <span class="shrink-0 text-xs font-semibold text-muted-foreground mr-0.5">
                    {{ item.symbol }}
                  </span>
                  <span class="text-base sm:text-lg font-bold tracking-tight text-foreground tabular-nums">
                    {{ item.value }}
                  </span>
                </div>
              </Transition>
            </div>
          </div>

          <!-- 实时汇率区域 -->
          <div class="rounded-md border border-border/70 bg-card p-4 space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-1.5 text-xs font-semibold tracking-tight text-foreground">
                <Icon icon="tabler:chart-arrows" :width="15" :height="15" class="text-muted-foreground" />
                <span>实时汇率参考</span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="text-[11px] text-muted-foreground hidden sm:inline">基准货币</span>
                <select
                  :value="exchangeRateBaseCurrency"
                  class="rounded-md border border-border bg-background px-2 py-1 text-xs font-medium text-foreground outline-none transition-colors hover:border-foreground/40 focus:border-foreground cursor-pointer shadow-2xs"
                  aria-label="切换汇率基准币种"
                  @change.stop="setExchangeRateBaseCurrency"
                >
                  <option v-for="currency in financeRateCurrencies" :key="currency" :value="currency">
                    {{ currency }}
                  </option>
                </select>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-2 max-h-48 overflow-y-auto pr-0.5">
              <div
                v-for="(row, index) in exchangeRateRows"
                :key="row.currency"
                class="flex items-center justify-between px-3 py-2 rounded-md bg-muted/30 border border-border/50 text-xs"
              >
                <span class="font-medium text-muted-foreground flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-foreground/40" />
                  {{ row.currency }}
                </span>
                <Transition v-bind="metricSwitchTransitionProps">
                  <span
                    :key="`modal-rate-${exchangeRateBaseCurrency}-${row.currency}`"
                    class="font-mono font-medium text-foreground"
                    :style="getMetricSwitchStyle(index)"
                  >
                    {{ row.targetSymbol }}{{ row.rate }}
                  </span>
                </Transition>
              </div>
            </div>
          </div>
        </div>

        <div class="px-5 py-3 border-t border-border/60 bg-muted/20 flex items-center justify-between text-[11px] text-muted-foreground">
          <span class="flex items-center gap-1">
            <Icon icon="tabler:info-circle" :width="13" :height="13" class="text-muted-foreground/70" />
            汇率数据定时刷新，实际以各运营商结算为准
          </span>
          <button
            type="button"
            class="px-3.5 py-1.5 rounded-md text-xs font-medium bg-foreground text-background hover:bg-foreground/90 transition-colors cursor-pointer"
            @click="openFinanceDialog = false"
          >
            关闭
          </button>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.metric-switch-enter-active,
.metric-switch-leave-active {
  transition:
    opacity 160ms ease,
    transform 180ms cubic-bezier(0.22, 1, 0.36, 1),
    filter 180ms ease;
}

.metric-switch-enter-active {
  transition-delay: var(--metric-switch-delay, 0ms);
}

.metric-switch-enter-from {
  opacity: 0;
  transform: translateY(6px);
  filter: blur(3px);
}

.metric-switch-leave-to {
  opacity: 0;
  transform: translateY(-4px);
  filter: blur(2px);
}

@media (prefers-reduced-motion: reduce) {
  .metric-switch-enter-active,
  .metric-switch-leave-active {
    transition: none;
    transition-delay: 0ms;
  }

  .metric-switch-enter-from,
  .metric-switch-leave-to {
    opacity: 1;
    transform: none;
    filter: none;
  }
}
</style>
