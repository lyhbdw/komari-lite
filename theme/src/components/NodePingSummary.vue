<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { useNodePingDisplay } from '@/composables/useNodePingDisplay'
import { useAppStore } from '@/stores/app'

const props = withDefaults(defineProps<{
  uuid: string
  compact?: boolean
  interactive?: boolean
}>(), {
  compact: false,
  interactive: false,
})

const emit = defineEmits<{
  click: []
}>()

const appStore = useAppStore()
const {
  latencyRenderBars,
  lossRenderBars,
  latencyDisplay,
  lossDisplay,
  latencyPanelTooltip,
  lossPanelTooltip,
  topPingNetworks,
  isAllBlocked,
} = useNodePingDisplay(() => props.uuid)

function emitClick() {
  if (props.interactive)
    emit('click')
}

function emitKeydown(event: KeyboardEvent) {
  if (!props.interactive)
    return
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('click')
  }
}
</script>

<template>
  <div v-if="compact" class="flex flex-col">
    <div v-if="isAllBlocked" class="flex items-center py-0.5">
      <DataTooltip
        placement="top"
        :content="appStore.lang === 'zh-CN' ? '国内探测点全量丢包（IP 可能已被墙或未响应 ICMP）' : '100% loss from mainland probes (IP may be blocked)'"
        content-class="whitespace-pre-wrap w-max px-2 py-1 text-[10.5px]"
      >
        <Badge
          variant="outline"
          class="!h-4 !px-1.5 !py-0 !text-[9.5px] font-mono border-rose-500/30 bg-rose-500/8 text-rose-600 dark:text-rose-400 gap-1 inline-flex items-center cursor-default rounded hover:bg-rose-500/12 transition-colors"
        >
          <span class="size-1 rounded-full bg-rose-500 inline-block animate-pulse" />
          <span>{{ appStore.lang === 'zh-CN' ? '国内阻断' : 'Blocked' }}</span>
        </Badge>
      </DataTooltip>
    </div>
    <div v-else-if="topPingNetworks.length > 0" class="flex flex-row items-center gap-1.5">
      <DataTooltip
        v-for="net in topPingNetworks" :key="net.name" placement="top"
        :content="`${net.name}\n${net.latency}`"
        content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]"
      >
        <div class="truncate text-[10px] font-mono tabular-nums flex items-center gap-0.5">
          <span class="text-[9px] text-muted-foreground/75 font-normal">{{ net.shortName }}</span>
          <span :class="net.toneClass">{{ net.latency }}</span>
        </div>
      </DataTooltip>
    </div>
    <div v-else class="truncate text-[10px] font-mono text-muted-foreground/40 py-0.5">
      -
    </div>

    <div class="flex flex-col gap-[1px] w-full pr-4">
      <div v-for="bars in [latencyRenderBars, lossRenderBars]" :key="bars[0]?.key" class="relative items-center gap-1">
        <div
          class="grid h-1 cursor-auto items-end gap-[1px] transition-all hover:h-2.5"
          :style="{ gridTemplateColumns: `repeat(${bars.length}, minmax(0, 1fr))` }"
        >
          <DataTooltip
            v-for="bar in bars" :key="bar.key" placement="top" :content="bar.tooltip"
            class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]"
          >
            <span class="block h-full w-full rounded-[1px] transition-all hover:scale-y-160" :class="bar.className" />
          </DataTooltip>
        </div>
      </div>
    </div>
  </div>

  <div v-else class="contents">
    <!-- 三网延迟统计行（与速率、在线、费用格式严格对齐） -->
    <div class="flex items-center justify-between min-h-[18px]">
      <span class="truncate tracking-wide text-muted-foreground font-medium text-[11px]">
        {{ appStore.lang === 'zh-CN' ? '三网' : 'Networks' }}
      </span>
      <div v-if="isAllBlocked" class="flex items-center justify-end">
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
      <div v-else-if="topPingNetworks.length > 0" class="flex flex-row items-center justify-end gap-1.5 font-mono tabular-nums text-[11px]">
        <DataTooltip
          v-for="net in topPingNetworks" :key="net.name" placement="top"
          :content="`${net.name}\n${net.latency}`"
          content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[10.5px]"
        >
          <div class="truncate flex items-center gap-0.5 font-mono tabular-nums text-[11px]">
            <span class="text-[10px] text-muted-foreground/75 font-normal">{{ net.shortName }}</span>
            <span :class="net.toneClass" class="font-medium">{{ net.latency }}</span>
          </div>
        </DataTooltip>
      </div>
      <div v-else class="truncate text-right font-mono tabular-nums text-[11px] text-muted-foreground/60">
        -
      </div>
    </div>

    <!-- 底部双栏指标面板（延迟 / 丢包） -->
    <div class="grid grid-cols-6 gap-x-3 pt-0.5">
      <div
        v-for="panel in [
          { label: appStore.lang === 'zh-CN' ? '延迟' : 'Latency', display: latencyDisplay, tooltip: latencyPanelTooltip, bars: latencyRenderBars },
          { label: appStore.lang === 'zh-CN' ? '丢包' : 'Loss', display: lossDisplay, tooltip: lossPanelTooltip, bars: lossRenderBars },
        ]"
        :key="panel.label"
        :role="interactive ? 'button' : undefined"
        :tabindex="interactive ? 0 : undefined"
        :class="[
          'group/panel relative col-span-3 flex h-5 flex-col gap-1 text-left transition-colors',
          interactive && 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        ]"
        :title="panel.tooltip"
        @click.stop="emitClick"
        @keydown.stop="emitKeydown"
      >
        <div class="flex items-center justify-between text-[10px] leading-none relative">
          <span class="text-muted-foreground font-medium">{{ panel.label }}</span>
          <span class="font-medium font-mono tabular-nums text-[10.5px]" :class="isAllBlocked ? 'text-muted-foreground/60' : 'text-foreground/85'">{{ panel.display }}</span>
        </div>
        <div class="grid h-full items-end gap-[1px]" :style="{ gridTemplateColumns: `repeat(${panel.bars.length}, minmax(0, 1fr))` }">
          <DataTooltip
            v-for="bar in panel.bars" :key="bar.key" placement="top" :content="bar.tooltip"
            class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[10.5px]"
          >
            <span class="block h-full w-full rounded-[1px] transition-transform duration-150 group-hover/data-tooltip:scale-y-200" :class="bar.className" />
          </DataTooltip>
        </div>
      </div>
    </div>
  </div>
</template>
