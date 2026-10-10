<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import { Icon } from '@iconify/vue'
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { CardX } from '@/components/ui/card-x'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { ProgressThin } from '@/components/ui/progress-thin'
import { useNodeFormatters } from '@/composables/useNodeFormatters'
import { useNodePingDisplay } from '@/composables/useNodePingDisplay'
import { useAppStore } from '@/stores/app'
import * as financeHelper from '@/utils/financeHelper'
import { sharedExchangeRates, sharedFinanceCurrency } from '@/utils/financeHelper'
import { formatDateTime, getStatus, getStatusTextClass } from '@/utils/helper'
import { getCustomTags, getDiskPercentage, getMemPercentage, getPriceTags, getRemainingTimeTagClass, getTrafficUsed, getTrafficUsedPercentage, hasRegion, showTrafficProgress } from '@/utils/nodeHelpers'
import { getOSImage, getOSName } from '@/utils/osImageHelper'
import { getFlagSrc, getRegionDisplayName } from '@/utils/regionHelper'

const props = defineProps<{ node: NodeData }>()

const emit = defineEmits<{
  click: []
  pingClick: [node: NodeData]
}>()

const appStore = useAppStore()
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
  isAllBlocked,
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
    size="small"
    hoverable
    class="node-card h-full w-full cursor-pointer border transition-all duration-150 rounded-lg bg-card hover:border-foreground/35 hover:shadow-xs hover:z-1 shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
    :class="[
      !props.node.online ? '!border-destructive/40' : isHighLoad ? 'border-amber-500/50 bg-amber-500/[0.02] dark:border-amber-500/40' : 'border-border',
    ]"
    @click="emit('click')"
  >
    <template #header>
      <div class="flex gap-2 min-w-0 items-center">
        <!-- 紧凑状态点 -->
        <div class="relative flex size-2 items-center justify-center shrink-0">
          <span
            class="inline-flex size-2 rounded-full"
            :class="[
              props.node.online
                ? 'bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]'
                : 'bg-rose-500 shadow-[0_0_6px_rgba(244,63,94,0.6)]'
            ]"
          />
        </div>
        <div class="text-[13px] font-semibold tracking-tight text-foreground flex-1 min-w-0 truncate" :title="props.node.name">
          {{ props.node.name }}
        </div>
        <div v-if="customTags.length > 0" class="flex shrink-0 gap-1 items-center">
          <Badge
            v-for="(tag, index) in customTags" :key="index" variant="outline"
            class="!text-[9.5px] rounded-full font-mono font-medium text-foreground/80 dark:text-muted-foreground border-border/70 bg-muted/30 px-1.5 py-0"
          >
            {{ tag }}
          </Badge>
        </div>
      </div>
    </template>

    <template #header-extra>
      <div class="flex gap-1.5 items-center">
        <img :src="getOSImage(props.node.os)" :alt="getOSName(props.node.os)" class="size-3.5 shrink-0 transition-opacity hover:opacity-80">
        <img
          v-if="hasRegion(props.node.region)" :src="getFlagSrc(props.node.region)"
          :alt="getRegionDisplayName(props.node.region)" class="size-4 rounded-[2px] shrink-0 object-cover"
        >
      </div>
    </template>

    <template #default>
      <div class="flex flex-col gap-2">
        <!-- 四大核心指标紧凑展示 -->
        <div class="gap-x-3 gap-y-1.5 grid grid-cols-2">
          <!-- CPU -->
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[10.5px] font-medium tracking-wide">
                CPU
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold" :class="cpuTextClass">{{ (props.node.cpu ?? 0).toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="props.node.cpu ?? 0" :status="cpuStatus" :height="3" class="my-0.5" />
            <DataTooltip placement="top" :content="`负载均值 (1/5/15m): ${props.node.load?.toFixed(2) ?? 0}, ${props.node.load5?.toFixed(2) ?? 0}, ${props.node.load15?.toFixed(2) ?? 0}`">
              <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums cursor-pointer leading-tight">
                {{ props.node.cpu_cores ? `${props.node.cpu_cores} 核心` : '单核' }}
              </div>
            </DataTooltip>
          </div>

          <!-- 内存 -->
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[10.5px] font-medium tracking-wide">
                内存
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold" :class="memTextClass">{{ memPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="memPercentage" :status="memStatus" :height="3" class="my-0.5" />
            <DataTooltip placement="top" class="block cursor-pointer" :content-class="[!props.node.swap && '!hidden']">
              <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums leading-tight">
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
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[10.5px] font-medium tracking-wide">
                硬盘
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold" :class="diskTextClass">{{ diskPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="diskPercentage" :status="diskStatus" :height="3" class="my-0.5" />
            <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums leading-tight">
              {{ formatBytes(props.node.disk ?? 0) }} / {{ formatBytes(props.node.disk_total ?? 0) }}
            </div>
          </div>

          <!-- 流量进度条 -->
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[10.5px] font-medium tracking-wide">
                流量
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold">{{ trafficUsedPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="trafficUsedPercentage" status="success" :height="3" class="my-0.5" />
            <DataTooltip placement="top" class="block cursor-pointer">
              <div class="whitespace-pre-wrap text-[10px] text-muted-foreground truncate font-mono tabular-nums leading-tight">
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

        <!-- 下半区（紧凑精简） -->
        <div class="relative text-[10.5px] text-muted-foreground pt-2 border-t border-border/40">
          <div
            v-if="!props.node.online"
            class="absolute inset-0 z-10 flex flex-col items-center justify-center space-y-1"
          >
            <span class="text-xs font-semibold text-rose-500 bg-background/90 px-2 py-0.5 rounded-full border border-rose-500/30 shadow-xs">离线</span>
            <div class="font-mono tabular-nums text-[10px]">
              {{ offlineTime }}
            </div>
          </div>
          <div class="flex flex-col gap-y-1.5" :class="[!props.node.online && 'blur-xs opacity-60 pointer-events-none']">
            <div class="flex items-center justify-between">
              <span class="truncate tracking-wide text-muted-foreground">
                速率
              </span>
              <div class="truncate flex flex-row gap-2 font-mono tabular-nums text-[10.5px]">
                <div class="text-emerald-600 dark:text-emerald-400 font-medium flex flex-row items-center gap-0.5">
                  <Icon icon="tabler:chevron-up" width="11" height="11" class="text-emerald-500 shrink-0" />
                  {{ formatBytesPerSecond(props.node.net_out ?? 0) }}
                </div>
                <div class="text-sky-600 dark:text-sky-400 font-medium flex flex-row items-center gap-0.5">
                  <Icon icon="tabler:chevron-down" width="11" height="11" class="text-sky-500 shrink-0" />
                  {{ formatBytesPerSecond(props.node.net_in ?? 0) }}
                </div>
              </div>
            </div>
            <div class="flex items-center justify-between">
              <span class="truncate tracking-wide text-muted-foreground">
                在线
              </span>
              <span class="truncate font-mono tabular-nums text-foreground/85">
                {{ props.node.uptime > 0 ? formatUptime(props.node.uptime) : '-' }}
              </span>
            </div>
            <div class="flex items-center justify-between">
              <span class="truncate tracking-wide text-muted-foreground">
                费用
              </span>
              <DataTooltip placement="left" :content="`${expiredDate} 到期${monthlyCostInfo.text && !monthlyCostInfo.isFree ? ` · 月均: ${monthlyCostInfo.text}` : ''}`" content-class="whitespace-nowrap right-0 mr-0">
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
              <span class="truncate tracking-wide text-muted-foreground">
                三网
              </span>
              <div v-if="isAllBlocked" class="flex items-center">
                <DataTooltip
                  placement="top"
                  :content="appStore.lang === 'zh-CN' ? '国内探测点全量丢包（IP 可能已被墙或未响应 ICMP）' : '100% loss from mainland probes (IP may be blocked)'"
                  content-class="whitespace-pre-wrap w-max px-2 py-1 text-[10.5px]"
                >
                  <Badge
                    variant="outline"
                    class="!h-4.5 !px-1.5 !py-0 !text-[10px] font-mono border-rose-500/30 bg-rose-500/8 text-rose-600 dark:text-rose-400 gap-1 inline-flex items-center cursor-default rounded-md hover:bg-rose-500/12 transition-colors"
                  >
                    <span class="size-1 rounded-full bg-rose-500 inline-block animate-pulse" />
                    <span>{{ appStore.lang === 'zh-CN' ? '国内阻断' : 'Blocked' }}</span>
                  </Badge>
                </DataTooltip>
              </div>
              <div v-else-if="topPingNetworks.length > 0" class="flex flex-row items-center gap-1.5">
                <DataTooltip
                  v-for="net in topPingNetworks" :key="net.name" placement="top"
                  :content="`${net.name}\n${net.latency}`" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[10.5px]"
                >
                  <div class="truncate flex items-center gap-0.5">
                    <span class="text-[9.5px] text-muted-foreground/75 font-normal">{{ net.shortName }}</span>
                    <span :class="net.toneClass">{{ net.latency }}</span>
                  </div>
                </DataTooltip>
              </div>
              <div v-else class="truncate text-muted-foreground/60">
                -
              </div>
            </div>
            <div class="grid grid-cols-6 gap-x-3 pt-0.5">
              <!-- 延迟 -->
              <div
                role="button" tabindex="0"
                class="group/panel relative col-span-3 flex h-5 cursor-pointer flex-col gap-1 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                :title="latencyPanelTooltip" :aria-label="`${props.node.name} 延迟`" @click.stop="openPingDialog"
                @keydown.enter.stop.prevent="openPingDialog" @keydown.space.stop.prevent="openPingDialog"
              >
                <div class="flex items-center justify-between text-[10px] leading-none relative">
                  <span class="text-muted-foreground">延迟</span>
                  <span class="font-medium font-mono tabular-nums" :class="isAllBlocked ? 'text-muted-foreground/60' : 'text-foreground/85'">{{ latencyDisplay }}</span>
                </div>
                <div
                  class="grid h-full items-end gap-[1px]"
                  :style="{ gridTemplateColumns: `repeat(${latencyRenderBars.length}, minmax(0, 1fr))` }"
                >
                  <DataTooltip
                    v-for="bar in latencyRenderBars" :key="bar.key" placement="top" :content="bar.tooltip"
                    class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[10.5px]"
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
                class="group/panel relative col-span-3 flex h-5 cursor-pointer flex-col gap-1 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                :title="lossPanelTooltip" :aria-label="`${props.node.name} 丢包`" @click.stop="openPingDialog"
                @keydown.enter.stop.prevent="openPingDialog" @keydown.space.stop.prevent="openPingDialog"
              >
                <div class="flex items-center justify-between text-[10px] leading-none">
                  <span class="text-muted-foreground">丢包</span>
                  <span class="font-medium font-mono tabular-nums" :class="isAllBlocked ? 'text-rose-600/80 dark:text-rose-400/80' : 'text-foreground/85'">{{ lossDisplay }}</span>
                </div>
                <div
                  class="grid h-full items-end gap-[1px]"
                  :style="{ gridTemplateColumns: `repeat(${lossRenderBars.length}, minmax(0, 1fr))` }"
                >
                  <DataTooltip
                    v-for="bar in lossRenderBars" :key="bar.key" placement="top" :content="bar.tooltip"
                    class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[10.5px] font-mono tabular-nums"
                  >
                    <span
                      class="block h-full w-full rounded-[1px] transition-transform duration-150 group-hover/data-tooltip:scale-y-200"
                      :class="bar.className"
                    />
                  </DataTooltip>
                </div>
              </div>
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
