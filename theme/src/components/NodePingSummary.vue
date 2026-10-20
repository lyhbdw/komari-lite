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
  <div :class="compact ? 'flex flex-col' : 'contents'">
    <div v-if="isAllBlocked" :class="compact ? 'flex items-center py-0.5' : 'flex items-center justify-end w-full'">
      <DataTooltip
        placement="top"
        :content="appStore.lang === 'zh-CN' ? '国内探测点全量丢包（IP 可能已被墙或未响应 ICMP）' : '100% loss from mainland probes (IP may be blocked)'"
        content-class="whitespace-pre-wrap w-max px-2 py-1 text-[10.5px]"
      >
        <Badge
          variant="outline"
          :class="compact
            ? '!h-4 !px-1.5 !py-0 !text-[9.5px] font-mono border-rose-500/30 bg-rose-500/8 text-rose-600 dark:text-rose-400 gap-1 inline-flex items-center cursor-default rounded hover:bg-rose-500/12 transition-colors'
            : '!h-4.5 !px-1.5 !py-0 !text-[10px] font-mono border-rose-500/30 bg-rose-500/8 text-rose-600 dark:text-rose-400 gap-1 inline-flex items-center cursor-default rounded-md hover:bg-rose-500/12 transition-colors'"
        >
          <span class="size-1 rounded-full bg-rose-500 inline-block animate-pulse" />
          <span>{{ appStore.lang === 'zh-CN' ? '国内阻断' : 'Blocked' }}</span>
        </Badge>
      </DataTooltip>
    </div>
    <div v-else-if="topPingNetworks.length > 0" :class="compact ? 'flex flex-row items-center gap-1.5' : 'flex flex-row items-center justify-end gap-1.5 w-full'">
      <DataTooltip
        v-for="net in topPingNetworks" :key="net.name" placement="top"
        :content="`${net.name}\n${net.latency}`"
        :content-class="compact ? 'whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]' : 'whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[10.5px]'"
      >
        <div :class="compact ? 'truncate text-[10px] font-mono tabular-nums flex items-center gap-0.5' : 'truncate flex items-center gap-0.5 font-mono tabular-nums text-[10.5px]'">
          <span :class="compact ? 'text-[9px] text-muted-foreground/75 font-normal' : 'text-[9.5px] text-muted-foreground/75 font-normal'">{{ net.shortName }}</span>
          <span :class="net.toneClass">{{ net.latency }}</span>
        </div>
      </DataTooltip>
    </div>
    <div v-else :class="compact ? 'truncate text-[10px] font-mono text-muted-foreground/40 py-0.5' : 'truncate text-right text-muted-foreground/60 w-full'">
      -
    </div>

    <div v-if="compact" class="flex flex-col gap-[1px] w-full pr-4">
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
    <template v-else>
      <div class="grid grid-cols-6 gap-x-3 pt-0.5">
        <div
          v-for="panel in [
            { label: '延迟', display: latencyDisplay, tooltip: latencyPanelTooltip, bars: latencyRenderBars },
            { label: '丢包', display: lossDisplay, tooltip: lossPanelTooltip, bars: lossRenderBars },
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
            <span class="text-muted-foreground">{{ panel.label }}</span>
            <span class="font-medium font-mono tabular-nums" :class="isAllBlocked ? 'text-muted-foreground/60' : 'text-foreground/85'">{{ panel.display }}</span>
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
    </template>
  </div>
</template>
