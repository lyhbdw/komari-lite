<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import { Icon } from '@iconify/vue'
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { CardX } from '@/components/ui/card-x'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { ProgressThin } from '@/components/ui/progress-thin'
import { useBackgroundSurface } from '@/composables/useBackgroundSurface'
import { useNodeFormatters } from '@/composables/useNodeFormatters'
import { useNodePingDisplay } from '@/composables/useNodePingDisplay'
import { useAppStore } from '@/stores/app'
import { formatDateTime, getStatus, getStatusTextClass } from '@/utils/helper'
import { getCustomTags, getDiskPercentage, getMemPercentage, getPriceTags, getRemainingTimeTagClass, getTrafficUsed, getTrafficUsedPercentage, hasRegion, showTrafficProgress } from '@/utils/nodeHelpers'
import { getOSImage, getOSName } from '@/utils/osImageHelper'
import { getFlagSrc, getRegionDisplayName } from '@/utils/regionHelper'
import * as financeHelper from '@/utils/financeHelper'
import { sharedExchangeRates, sharedFinanceCurrency } from '@/utils/financeHelper'

const props = defineProps<{ node: NodeData }>()

const emit = defineEmits<{
  click: []
  pingClick: [node: NodeData]
}>()

const appStore = useAppStore()
const { pickSurfaceClass } = useBackgroundSurface()
const { formatBytes, formatBytesPerSecond, formatUptime } = useNodeFormatters()

const offlineTime = computed(() => formatDateTime(props.node.time))
const expiredDate = computed(() => formatDateTime(props.node.expired_at, 'YYYY-MM-DD'))

const cpuStatus = computed(() => getStatus(props.node.cpu ?? 0))
const cpuTextClass = computed(() => getStatusTextClass(props.node.cpu ?? 0))
const memPercentage = computed(() => getMemPercentage(props.node))
const memStatus = computed(() => getStatus(memPercentage.value))
const memTextClass = computed(() => getStatusTextClass(memPercentage.value))
const diskPercentage = computed(() => getDiskPercentage(props.node))
const diskStatus = computed(() => getStatus(diskPercentage.value))
const diskTextClass = computed(() => getStatusTextClass(diskPercentage.value))

const isHighLoad = computed(() => {
  const cpu = props.node.cpu ?? 0
  const mem = memPercentage.value
  return cpu >= 85 || mem >= 85
})

const {
  latencyRenderBars,
  lossRenderBars,
  latencyDisplay,
  lossDisplay,
  latencyPanelTooltip,
  lossPanelTooltip,
  topPingNetworks,
} = useNodePingDisplay(() => props.node.uuid)

const trafficUsedPercentage = computed(() => getTrafficUsedPercentage(props.node))
const trafficUsed = computed(() => getTrafficUsed(props.node))
const priceTags = computed(() => getPriceTags(props.node, appStore.lang))
const remainingTimeTagClass = computed(() => getRemainingTimeTagClass(props.node))
const customTags = computed(() => getCustomTags(props.node))

const monthlyCostInfo = computed(() => {
  const node = props.node
  const price = Number(node.price)
  const billingCycle = Number(node.billing_cycle)
  const isZh = appStore.lang === 'zh-CN'
  const unitSuffix = isZh ? '/月' : '/mo'

  if (price === 0 || price === -1) {
    return {
      text: isZh ? '免费' : 'Free',
      tooltip: isZh ? '免费节点' : 'Free node',
      isFree: true,
    }
  }

  if (!Number.isFinite(price) || price <= 0 || !Number.isFinite(billingCycle) || billingCycle <= 0) {
    return {
      text: '-',
      tooltip: isZh ? '未设置价格或计费周期' : 'No pricing or billing cycle set',
      isFree: false,
    }
  }

  const rawCurrency = financeHelper.normalizeCurrency(node.currency)
  const rawMonthlyAmount = (price / billingCycle) * 30
  const rawFormatted = financeHelper.formatFinanceAmount(rawMonthlyAmount, rawCurrency)

  const baseCurrency = sharedFinanceCurrency.value
  const monthlyCostCNY = financeHelper.calculateMonthlyAverageCostCNY(node, sharedExchangeRates.value)
  const baseRate = sharedExchangeRates.value[baseCurrency] || 1
  const baseMonthlyAmount = monthlyCostCNY * baseRate
  const baseFormatted = financeHelper.formatFinanceAmount(baseMonthlyAmount, baseCurrency)

  if (rawCurrency === baseCurrency) {
    return {
      text: `${baseFormatted.symbol}${baseFormatted.value} ${unitSuffix}`,
      tooltip: isZh
        ? `计费周期: ${billingCycle}天 · 周期原价: ${node.price} ${node.currency}`
        : `Billing: ${billingCycle}d · Price: ${node.price} ${node.currency}`,
      isFree: false,
    }
  }

  return {
    text: `${baseFormatted.symbol}${baseFormatted.value} ${unitSuffix}`,
    tooltip: isZh
      ? `原币约: ${rawFormatted.symbol}${rawFormatted.value} ${unitSuffix} (周期: ${billingCycle}天 · 原价: ${node.price} ${node.currency})`
      : `Original: ${rawFormatted.symbol}${rawFormatted.value} ${unitSuffix} (Cycle: ${billingCycle}d · Price: ${node.price} ${node.currency})`,
    isFree: false,
  }
})

function openPingDialog() {
  emit('pingClick', props.node)
}
</script>

<template>
  <CardX
    hoverable
    class="node-card h-full w-full cursor-pointer border transition-all duration-150 rounded-md bg-card hover:border-foreground/35 hover:shadow-xs hover:z-1 shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
    :class="[
      !props.node.online ? '!border-destructive/40' : isHighLoad ? 'border-amber-500/50 bg-amber-500/[0.02] dark:border-amber-500/40' : 'border-border',
    ]"
    @click="emit('click')"
  >
    <template #header>
      <div class="flex gap-2 min-w-0 items-center">
        <div class="size-2 rounded-full relative shrink-0" :class="[props.node.online ? 'bg-emerald-500' : 'bg-rose-500']">
          <div
            class="absolute -inset-0.5 rounded-full opacity-25"
            :class="[props.node.online ? 'bg-emerald-400' : 'bg-rose-400']"
          />
        </div>
        <div class="text-md font-bold dark:font-semibold flex-1 min-w-0 truncate" :title="props.node.name">
          {{ props.node.name }}
        </div>
        <div v-if="customTags.length > 0" class="flex shrink-0 gap-1 items-center">
          <Badge
            v-for="(tag, index) in customTags" :key="index" variant="outline"
            class="!text-[10px] rounded font-medium text-foreground/85 dark:text-muted-foreground border-border/80 bg-muted/40 px-1.5 py-0.2"
          >
            {{ tag }}
          </Badge>
        </div>
      </div>
    </template>

    <template #header-extra>
      <div class="flex gap-2 items-center">
        <img :src="getOSImage(props.node.os)" :alt="getOSName(props.node.os)" class="size-4">
        <img
          v-if="hasRegion(props.node.region)" :src="getFlagSrc(props.node.region)"
          :alt="getRegionDisplayName(props.node.region)" class="size-5 shrink-0"
        >
      </div>
    </template>

    <template #default>
      <div class="flex flex-col gap-3">
        <div class="gap-x-3 gap-y-1 grid grid-cols-2">
          <!-- CPU -->
          <div class="flex flex-col gap-1">
            <div class="w-full text-xs flex flex-row justify-between">
              <span class="text-muted-foreground tracking-wide">
                CPU
              </span>
              <span class="font-mono tabular-nums" :class="cpuTextClass">{{ (props.node.cpu ?? 0).toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="props.node.cpu ?? 0" :status="cpuStatus" :height="4" />
            <div class="text-[11px] text-foreground/85 dark:text-muted-foreground truncate font-mono font-medium tabular-nums tracking-normal">
              {{ props.node.load.toFixed(2) ?? 0 }}, {{ props.node.load5.toFixed(2) ?? 0 }}, {{
                props.node.load15.toFixed(2) ?? 0 }}
            </div>
          </div>

          <!-- 内存 -->
          <div class="flex flex-col gap-1">
            <div class="w-full text-xs flex flex-row justify-between">
              <span class="text-muted-foreground tracking-wide">
                内存
              </span>
              <span class="font-mono tabular-nums" :class="memTextClass">{{ memPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="memPercentage" :status="memStatus" :height="4" />
            <DataTooltip placement="top" class="block" :content-class="[!props.node.swap && '!hidden']">
              <div class="text-[11px] text-muted-foreground truncate font-mono tabular-nums">
                {{ formatBytes(props.node.ram ?? 0) }} / {{ formatBytes(props.node.mem_total ?? 0) }}
              </div>
              <template #content>
                <div class="flex items-center justify-between gap-3 whitespace-nowrap font-mono tabular-nums">
                  <span class="text-background/70">Swap</span>
                  <span>{{ formatBytes(props.node.swap ?? 0) }}</span>
                </div>
              </template>
            </DataTooltip>
          </div>

          <!-- 硬盘 -->
          <div class="flex flex-col gap-1">
            <div class="w-full text-xs flex flex-row justify-between">
              <span class="text-muted-foreground tracking-wide">
                硬盘
              </span>
              <span class="font-mono tabular-nums" :class="diskTextClass">{{ diskPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="diskPercentage" :status="diskStatus" :height="4" />
            <div class="text-[11px] text-muted-foreground truncate font-mono tabular-nums">
              {{ formatBytes(props.node.disk ?? 0) }} / {{ formatBytes(props.node.disk_total ?? 0) }}
            </div>
          </div>

          <!-- 流量进度条 -->
          <div class="flex flex-col gap-1">
            <div class="w-full text-xs flex flex-row justify-between">
              <span class="text-muted-foreground tracking-wide">
                流量
              </span>
              <span class="font-mono tabular-nums">{{ trafficUsedPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="trafficUsedPercentage" status="success" :height="4" />
            <DataTooltip placement="top" class="block">
              <div class="whitespace-pre-wrap text-[11px] text-muted-foreground truncate font-mono tabular-nums">
                {{ formatBytes(trafficUsed) }} /
                <template v-if="showTrafficProgress(props.node)">
                  {{ formatBytes(props.node.traffic_limit) }}
                </template>
                <template v-else>
                  ∞
                </template>
              </div>
              <template #content>
                <div class="flex items-center justify-between gap-3 whitespace-nowrap font-mono tabular-nums">
                  <div class="text-[11px] flex flex-col">
                    <div class="flex flex-row items-center gap-1">
                      <Icon icon="tabler:chevron-up" width="12" height="12" />
                      {{ formatBytes(props.node.net_total_up ?? 0) }}
                    </div>
                    <div class="flex flex-row items-center gap-1">
                      <Icon icon="tabler:chevron-down" width="12" height="12" />
                      {{ formatBytes(props.node.net_total_down ?? 0) }}
                    </div>
                  </div>
                </div>
              </template>
            </DataTooltip>
          </div>
        </div>
        <div class="relative text-[11px] text-muted-foreground">
          <div
            v-if="!props.node.online"
            class="absolute inset-0 z-10 flex flex-col items-center justify-center space-y-1"
          >
            <span class="text-sm text-red-600">离线</span>
            <div class="font-mono tabular-nums">{{ offlineTime }}</div>
          </div>
          <div class="flex flex-col gap-y-2" :class="[!props.node.online && 'blur-xs opacity-60 pointer-events-none']">
            <div class="flex items-center">
              <span class="truncate tracking-wide">
                速率
              </span>
              <div class="border-t-2 border-dotted border-gray-500/10 mx-2 flex-1" />
              <div class="truncate flex flex-row gap-1.5 font-mono tabular-nums">
                <div class="text-emerald-600 dark:text-emerald-400 font-medium flex flex-row items-center gap-0.5">
                  <Icon icon="tabler:chevron-up" width="12" height="12" class="text-emerald-500 shrink-0" />
                  {{ formatBytesPerSecond(props.node.net_out ?? 0) }}
                </div>
                <div class="text-sky-600 dark:text-sky-400 font-medium flex flex-row items-center gap-0.5">
                  <Icon icon="tabler:chevron-down" width="12" height="12" class="text-sky-500 shrink-0" />
                  {{ formatBytesPerSecond(props.node.net_in ?? 0) }}
                </div>
              </div>
            </div>
            <div class="flex items-center justify-between">
              <span class="truncate tracking-wide">
                在线
              </span>
              <div class="border-t-2 border-dotted border-gray-500/10 mx-2 flex-1" />
              <span class="truncate">
                {{ props.node.uptime > 0 ? formatUptime(props.node.uptime) : '' }}
              </span>
            </div>
            <div class="flex items-center justify-between">
              <span class="truncate tracking-wide">
                费用
              </span>
              <div class="border-t-2 border-dotted border-gray-500/10 mx-2 flex-1" />
              <DataTooltip placement="left" :content="expiredDate" content-class="whitespace-nowrap right-0 mr-0">
                <span class="truncate flex flex-row gap-1">
                  <template v-for="(tag, index) in priceTags" :key="tag.text">
                    <span class="inline-flex flex-row gap-1 items-center">
                      <span :class="tag.highlight ? remainingTimeTagClass : ''">{{ tag.text }}</span>
                    </span>
                    <span v-if="index < priceTags.length - 1">·</span>
                  </template>
                </span>
              </DataTooltip>
            </div>
            <div class="flex items-center justify-between">
              <span class="truncate tracking-wide">
                三网
              </span>
              <div class="border-t-2 border-dotted border-gray-500/10 mx-2 flex-1" />
              <div v-if="topPingNetworks.length > 0" class="flex flex-row items-center gap-1.5">
                <DataTooltip
                  v-for="net in topPingNetworks" :key="net.name" placement="top"
                  :content="`${net.name}\n${net.latency}`" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]"
                >
                  <div class="truncate flex items-center gap-0.5">
                    <span class="text-[10px] text-muted-foreground/75 font-normal">{{ net.shortName }}</span>
                    <span :class="net.toneClass">{{ net.latency }}</span>
                  </div>
                </DataTooltip>
              </div>
              <div v-else class="truncate">
                N/A
              </div>
            </div>
            <div class="grid grid-cols-6 gap-x-3">
              <!-- 延迟 -->
              <div
                role="button" tabindex="0"
                class="group/panel relative col-span-3 flex h-6 cursor-pointer flex-col gap-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                :title="latencyPanelTooltip" :aria-label="`${props.node.name} 延迟`" @click.stop="openPingDialog"
                @keydown.enter.stop.prevent="openPingDialog" @keydown.space.stop.prevent="openPingDialog"
              >
                <div class="flex items-center justify-between text-[11px] leading-none relative">
                  <span class="text-muted-foreground">延迟</span>
                  <div class="border-t-2 border-dotted border-gray-500/10 mx-2 flex-1" />
                  <span class="font-medium text-foreground/85">{{ latencyDisplay }}</span>
                </div>
                <div
                  class="grid h-full items-end gap-[1px]"
                  :style="{ gridTemplateColumns: `repeat(${latencyRenderBars.length}, minmax(0, 1fr))` }"
                >
                  <DataTooltip
                    v-for="bar in latencyRenderBars" :key="bar.key" placement="top" :content="bar.tooltip"
                    class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]"
                  >
                    <span
                      class="block h-full w-full rounded-[1px] transition-transform duration-150 group-hover/data-tooltip:scale-y-200"
                      :class="bar.className"
                    />
                  </DataTooltip>
                </div>
              </div>
              <!-- 丢包 -->
              <div
                role="button" tabindex="0"
                class="group/panel relative col-span-3 flex h-6 cursor-pointer flex-col gap-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                :title="lossPanelTooltip" :aria-label="`${props.node.name} 丢包`" @click.stop="openPingDialog"
                @keydown.enter.stop.prevent="openPingDialog" @keydown.space.stop.prevent="openPingDialog"
              >
                <div class="flex items-center justify-between text-[11px] leading-none">
                  <span class="text-muted-foreground">丢包</span>
                  <div class="border-t-2 border-dotted border-gray-500/10 mx-2 flex-1" />
                  <span class="font-medium text-foreground/85">{{ lossDisplay }}</span>
                </div>
                <div
                  class="grid h-full items-end gap-[1px]"
                  :style="{ gridTemplateColumns: `repeat(${lossRenderBars.length}, minmax(0, 1fr))` }"
                >
                  <DataTooltip
                    v-for="bar in lossRenderBars" :key="bar.key" placement="top" :content="bar.tooltip"
                    class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px] font-mono tabular-nums"
                  >
                    <span
                      class="block h-full w-full rounded-[1px] transition-transform duration-150 group-hover/data-tooltip:scale-y-200"
                      :class="bar.className"
                    />
                  </DataTooltip>
                </div>
              </div>
            </div>

            <!-- 月均费用（最底部） -->
            <div class="pt-2 mt-1 border-t border-border/50 flex items-center justify-between text-[11px] leading-none select-none">
              <span class="text-muted-foreground flex items-center gap-1 font-medium">
                <Icon icon="tabler:receipt-2" :width="13" :height="13" class="text-slate-500/40" />
                <span>{{ appStore.lang === 'zh-CN' ? '月均费用' : 'Monthly Cost' }}</span>
              </span>
              <DataTooltip v-if="monthlyCostInfo.tooltip" placement="top" :content="monthlyCostInfo.tooltip">
                <span
                  class="font-mono font-bold dark:font-semibold tabular-nums"
                  :class="[monthlyCostInfo.isFree ? 'text-muted-foreground font-normal' : 'text-foreground']"
                >
                  {{ monthlyCostInfo.text }}
                </span>
              </DataTooltip>
              <span
                v-else
                class="font-mono font-bold dark:font-semibold tabular-nums"
                :class="[monthlyCostInfo.isFree ? 'text-muted-foreground font-normal' : 'text-foreground']"
              >
                {{ monthlyCostInfo.text }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </CardX>
</template>

<style scoped>
.node-card {
  position: relative;
}
</style>
