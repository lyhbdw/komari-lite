<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import { Icon } from '@iconify/vue'
import { computed } from 'vue'
import NodePingSummary from '@/components/NodePingSummary.vue'
import { CardX } from '@/components/ui/card-x'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { ProgressThin } from '@/components/ui/progress-thin'
import { useNodeFormatters } from '@/composables/useNodeFormatters'
import { useAppStore } from '@/stores/app'
import * as financeHelper from '@/utils/financeHelper'
import { sharedExchangeRates, sharedFinanceCurrency } from '@/utils/financeHelper'
import { formatDateTime, getStatus, getStatusTextClass } from '@/utils/helper'
import { getCustomTags, getDiskPercentage, getMemPercentage, getTrafficUsed, getTrafficUsedPercentage, hasRegion, showTrafficProgress } from '@/utils/nodeHelpers'
import { getOSImage, getOSName } from '@/utils/osImageHelper'
import { getFlagSrc, getRegionDisplayName } from '@/utils/regionHelper'
import { getDaysUntilExpired, getExpireStatus, getExpireTextClass } from '@/utils/tagHelper'

const props = defineProps<{ node: NodeData }>()

const emit = defineEmits<{
  click: []
  pingClick: [node: NodeData]
}>()

const appStore = useAppStore()
const { formatBytes, formatBytesPerSecond, formatUptime } = useNodeFormatters()

const offlineTime = computed(() => formatDateTime(props.node.time))

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

const trafficUsedPercentage = computed(() => getTrafficUsedPercentage(props.node))
const trafficUsed = computed(() => getTrafficUsed(props.node))
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

  const cycleDays = `${billingCycle}${isZh ? '天' : 'd'}`
  const rawPriceText = `${node.price} ${node.currency}`

  if (rawCurrency === baseCurrency) {
    return {
      text: `${baseFormatted.symbol}${baseFormatted.value}${unitSuffix}`,
      tooltip: isZh
        ? `计费周期: ${cycleDays} · 周期原价: ${rawPriceText} · 折合月均: ${baseFormatted.symbol}${baseFormatted.value}/月`
        : `Billing: ${cycleDays} · Price: ${rawPriceText} · Monthly: ${baseFormatted.symbol}${baseFormatted.value}/mo`,
      isFree: false,
    }
  }

  return {
    text: `${baseFormatted.symbol}${baseFormatted.value}${unitSuffix}`,
    tooltip: isZh
      ? `月均折合: ${baseFormatted.symbol}${baseFormatted.value}/月 (原币约: ${rawFormatted.symbol}${rawFormatted.value}/月 · 周期: ${cycleDays} · 原价: ${rawPriceText})`
      : `Monthly: ${baseFormatted.symbol}${baseFormatted.value}/mo (Original: ${rawFormatted.symbol}${rawFormatted.value}/mo · Cycle: ${cycleDays} · Price: ${rawPriceText})`,
    isFree: false,
  }
})

const remainingTimeInfo = computed(() => {
  const node = props.node
  const isZh = appStore.lang === 'zh-CN'
  const price = Number(node.price)

  if (price === 0 || price === -1) {
    return {
      text: isZh ? '长期' : 'Long-term',
      tooltip: isZh ? '免费长期有效' : 'Free long-term node',
      colorClass: 'text-muted-foreground',
    }
  }

  if (!node.expired_at) {
    return {
      text: isZh ? '未设置' : '-',
      tooltip: isZh ? '未设置到期时间' : 'No expiration date set',
      colorClass: 'text-muted-foreground/60',
    }
  }

  const days = getDaysUntilExpired(node.expired_at)
  const status = getExpireStatus(node.expired_at)
  const formattedDate = formatDateTime(node.expired_at, 'YYYY-MM-DD')

  if (status === 'long_term') {
    return {
      text: isZh ? '长期' : 'Long-term',
      tooltip: isZh ? `到期时间: ${formattedDate} (长期有效)` : `Expires: ${formattedDate} (Long-term)`,
      colorClass: 'text-muted-foreground',
    }
  }

  if (status === 'expired') {
    return {
      text: isZh ? '已过期' : 'Expired',
      tooltip: isZh ? `已于 ${formattedDate} 到期` : `Expired on ${formattedDate}`,
      colorClass: 'text-rose-600 dark:text-rose-400 font-bold',
    }
  }

  const daysText = isZh ? `${days}天` : `${days}d`
  const colorClass = getExpireTextClass(node.expired_at)

  return {
    text: daysText,
    tooltip: isZh
      ? `到期时间: ${formattedDate} (剩余 ${days} 天)`
      : `Expires: ${formattedDate} (${days} days left)`,
    colorClass,
  }
})

const premiumInfo = computed(() => {
  const node = props.node
  const { amount, currency } = financeHelper.getNodePremiumInfo(node)
  const isZh = appStore.lang === 'zh-CN'

  if (amount <= 0) {
    return {
      hasPremium: false,
      text: isZh ? '无' : '-',
      tooltip: isZh ? '未设置购入溢价' : 'No purchase premium',
    }
  }

  const baseCurrency = sharedFinanceCurrency.value
  const premiumCNY = financeHelper.getNodePremiumCNY(node, sharedExchangeRates.value)
  const baseRate = sharedExchangeRates.value[baseCurrency] || 1
  const baseAmount = premiumCNY * baseRate
  const baseFormatted = financeHelper.formatFinanceAmount(baseAmount, baseCurrency)
  const rawFormatted = financeHelper.formatFinanceAmount(amount, currency)

  if (currency === baseCurrency) {
    return {
      hasPremium: true,
      text: `${baseFormatted.symbol}${baseFormatted.value}`,
      tooltip: isZh
        ? `购入溢价: ${baseFormatted.symbol}${baseFormatted.value} ${baseCurrency}`
        : `Purchase premium: ${baseFormatted.symbol}${baseFormatted.value} ${baseCurrency}`,
    }
  }

  return {
    hasPremium: true,
    text: `${baseFormatted.symbol}${baseFormatted.value}`,
    tooltip: isZh
      ? `折合溢价: ${baseFormatted.symbol}${baseFormatted.value} ${baseCurrency} (原币: ${rawFormatted.symbol}${rawFormatted.value} ${currency})`
      : `Approx premium: ${baseFormatted.symbol}${baseFormatted.value} ${baseCurrency} (Original: ${rawFormatted.symbol}${rawFormatted.value} ${currency})`,
  }
})

const billingTooltip = computed(() => {
  const parts: string[] = []

  if (monthlyCostInfo.value.tooltip) {
    parts.push(monthlyCostInfo.value.tooltip)
  }

  if (remainingTimeInfo.value.tooltip) {
    parts.push(remainingTimeInfo.value.tooltip)
  }

  if (premiumInfo.value.hasPremium && premiumInfo.value.tooltip) {
    parts.push(premiumInfo.value.tooltip)
  }

  return parts.join('\n')
})

function openPingDialog() {
  emit('pingClick', props.node)
}
</script>

<template>
  <CardX
    size="small"
    hoverable
    class="node-card group/card h-full w-full cursor-pointer border transition-all duration-200 rounded-xl bg-card hover:border-foreground/30 hover:shadow-xs shadow-[0_1px_2px_rgba(0,0,0,0.03)]"
    :class="[
      !props.node.online ? '!border-destructive/40 bg-destructive/[0.01]' : isHighLoad ? 'border-amber-500/40 bg-amber-500/[0.02]' : 'border-border/70',
    ]"
    @click="emit('click')"
  >
    <template #header>
      <div class="flex gap-2 min-w-0 items-center">
        <!-- 紧凑状态点（带圆环柔光） -->
        <div class="relative flex size-2 items-center justify-center shrink-0">
          <span
            class="inline-flex size-2 rounded-full"
            :class="[
              props.node.online
                ? 'bg-emerald-500 ring-2 ring-emerald-500/20'
                : 'bg-rose-500 ring-2 ring-rose-500/20',
            ]"
          />
        </div>
        <div class="text-sm font-semibold tracking-tight text-foreground flex-1 min-w-0 truncate" :title="props.node.name">
          {{ props.node.name }}
        </div>
        <div v-if="customTags.length > 0" class="flex shrink-0 gap-1 items-center">
          <Badge
            v-for="(tag, index) in customTags" :key="index" variant="outline"
            class="h-4.5 px-1.5 py-0 text-[10px] font-mono font-medium rounded-md border-border/70 bg-muted/40 text-muted-foreground hover:bg-muted/60 transition-colors"
          >
            {{ tag }}
          </Badge>
        </div>
      </div>
    </template>

    <template #header-extra>
      <div class="flex gap-1.5 items-center shrink-0">
        <img :src="getOSImage(props.node.os)" :alt="getOSName(props.node.os)" class="size-3.5 shrink-0 transition-opacity hover:opacity-80">
        <img
          v-if="hasRegion(props.node.region)" :src="getFlagSrc(props.node.region)"
          :alt="getRegionDisplayName(props.node.region)" class="size-3.5 rounded-[2px] ring-1 ring-border/50 shrink-0 object-cover"
        >
      </div>
    </template>

    <template #default>
      <div class="flex flex-col gap-2">
        <!-- 四大核心指标紧凑展示 -->
        <div class="gap-x-3 gap-y-2 grid grid-cols-2">
          <!-- CPU -->
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[11px] font-medium tracking-wide">
                CPU
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold" :class="cpuTextClass">{{ (props.node.cpu ?? 0).toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="props.node.cpu ?? 0" :status="cpuStatus" :height="3.5" class="my-0.5" />
            <DataTooltip placement="top" :content="`负载均值 (1/5/15m): ${props.node.load?.toFixed(2) ?? 0}, ${props.node.load5?.toFixed(2) ?? 0}, ${props.node.load15?.toFixed(2) ?? 0}`">
              <div class="text-[10px] text-muted-foreground/80 truncate font-mono tabular-nums cursor-pointer leading-tight">
                {{ props.node.cpu_cores ? (appStore.lang === 'zh-CN' ? `${props.node.cpu_cores} 核心` : `${props.node.cpu_cores} Cores`) : (appStore.lang === 'zh-CN' ? '单核' : '1 Core') }}
              </div>
            </DataTooltip>
          </div>

          <!-- 内存 -->
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[11px] font-medium tracking-wide">
                {{ appStore.lang === 'zh-CN' ? '内存' : 'RAM' }}
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold" :class="memTextClass">{{ memPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="memPercentage" :status="memStatus" :height="3.5" class="my-0.5" />
            <DataTooltip placement="top" class="block cursor-pointer" :content-class="[!props.node.swap && '!hidden']">
              <div class="text-[10px] text-muted-foreground/80 truncate font-mono tabular-nums leading-tight">
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
              <span class="text-muted-foreground text-[11px] font-medium tracking-wide">
                {{ appStore.lang === 'zh-CN' ? '硬盘' : 'Disk' }}
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold" :class="diskTextClass">{{ diskPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="diskPercentage" :status="diskStatus" :height="3.5" class="my-0.5" />
            <div class="text-[10px] text-muted-foreground/80 truncate font-mono tabular-nums leading-tight">
              {{ formatBytes(props.node.disk ?? 0) }} / {{ formatBytes(props.node.disk_total ?? 0) }}
            </div>
          </div>

          <!-- 流量进度条 -->
          <div class="flex flex-col gap-0.5">
            <div class="w-full text-xs flex flex-row justify-between items-baseline leading-none">
              <span class="text-muted-foreground text-[11px] font-medium tracking-wide">
                {{ appStore.lang === 'zh-CN' ? '流量' : 'Traffic' }}
              </span>
              <span class="font-mono tabular-nums text-[11px] font-semibold">{{ trafficUsedPercentage.toFixed(1) }}%</span>
            </div>
            <ProgressThin :percentage="trafficUsedPercentage" status="success" :height="3.5" class="my-0.5" />
            <DataTooltip placement="top" class="block cursor-pointer">
              <div class="whitespace-pre-wrap text-[10px] text-muted-foreground/80 truncate font-mono tabular-nums leading-tight">
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
                      <Icon icon="tabler:arrow-narrow-up" width="12" height="12" />
                      {{ formatBytes(props.node.net_total_up ?? 0) }}
                    </div>
                    <div class="flex flex-row items-center gap-1">
                      <Icon icon="tabler:arrow-narrow-down" width="12" height="12" />
                      {{ formatBytes(props.node.net_total_down ?? 0) }}
                    </div>
                  </div>
                </div>
              </template>
            </DataTooltip>
          </div>
        </div>

        <!-- 下半区（紧凑精简） -->
        <div class="relative pt-2.5 border-t border-border/50">
          <div
            v-if="!props.node.online"
            class="absolute inset-0 z-10 flex flex-col items-center justify-center space-y-1 rounded-b-xl backdrop-blur-[1.5px] bg-background/50"
          >
            <Badge variant="outline" class="h-5 px-2 text-[10.5px] font-mono font-medium border-rose-500/30 bg-background/95 text-rose-600 dark:text-rose-400 shadow-xs">
              {{ appStore.lang === 'zh-CN' ? '离线' : 'Offline' }}
            </Badge>
            <div class="font-mono tabular-nums text-[10px] text-muted-foreground/80">
              {{ offlineTime }}
            </div>
          </div>
          <div class="flex flex-col gap-y-1.5" :class="[!props.node.online && 'blur-xs opacity-60 pointer-events-none']">
            <div class="flex items-center justify-between min-h-[18px]">
              <span class="truncate tracking-wide text-muted-foreground font-medium text-[11px]">
                {{ appStore.lang === 'zh-CN' ? '速率' : 'Speed' }}
              </span>
              <div class="truncate flex flex-row items-center gap-2 font-mono tabular-nums text-[11px]">
                <div class="text-emerald-600 dark:text-emerald-400 font-medium flex flex-row items-center gap-0.5">
                  <Icon icon="tabler:arrow-narrow-up" width="12" height="12" class="text-emerald-500 shrink-0" />
                  {{ formatBytesPerSecond(props.node.net_out ?? 0) }}
                </div>
                <div class="text-sky-600 dark:text-sky-400 font-medium flex flex-row items-center gap-0.5">
                  <Icon icon="tabler:arrow-narrow-down" width="12" height="12" class="text-sky-500 shrink-0" />
                  {{ formatBytesPerSecond(props.node.net_in ?? 0) }}
                </div>
              </div>
            </div>
            <div class="flex items-center justify-between min-h-[18px]">
              <span class="truncate tracking-wide text-muted-foreground font-medium text-[11px]">
                {{ appStore.lang === 'zh-CN' ? '在线' : 'Uptime' }}
              </span>
              <span class="truncate font-mono tabular-nums text-[11px] text-foreground/85 font-medium">
                {{ props.node.uptime > 0 ? formatUptime(props.node.uptime) : '-' }}
              </span>
            </div>
            <div class="flex items-center justify-between min-h-[18px]">
              <span class="truncate tracking-wide text-muted-foreground font-medium text-[11px]">
                {{ appStore.lang === 'zh-CN' ? '费用' : 'Billing' }}
              </span>
              <DataTooltip placement="left" :content="billingTooltip" content-class="whitespace-pre-wrap right-0 mr-0 text-left max-w-xs text-[10.5px]">
                <div class="truncate flex flex-row items-center gap-1 font-mono tabular-nums text-[11px] cursor-pointer">
                  <!-- 每月多少钱 -->
                  <span class="text-foreground/85 font-medium">
                    {{ monthlyCostInfo.text }}
                  </span>
                  <span class="text-muted-foreground/40">·</span>
                  <!-- 剩余时间 -->
                  <span :class="remainingTimeInfo.colorClass" class="font-medium">
                    {{ remainingTimeInfo.text }}
                  </span>
                  <!-- 溢价 (有溢价时展示) -->
                  <template v-if="premiumInfo.hasPremium">
                    <span class="text-muted-foreground/40">·</span>
                    <span class="text-amber-600 dark:text-amber-400 font-medium">
                      {{ appStore.lang === 'zh-CN' ? '溢价' : 'Prem' }} {{ premiumInfo.text }}
                    </span>
                  </template>
                </div>
              </DataTooltip>
            </div>
            <NodePingSummary
              :uuid="props.node.uuid"
              interactive
              @click="openPingDialog"
            />
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
