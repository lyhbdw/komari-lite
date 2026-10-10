<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { useNodePingDisplay } from '@/composables/useNodePingDisplay'
import { useAppStore } from '@/stores/app'

const props = defineProps<{
  uuid: string
  online: boolean
}>()

const appStore = useAppStore()

const {
  latencyRenderBars,
  lossRenderBars,
  topPingNetworks,
  isAllBlocked,
} = useNodePingDisplay(() => props.uuid)
</script>

<template>
  <div class="flex flex-col">
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
      <div class="relative items-center gap-1">
        <div
          class="grid h-1 cursor-auto items-end gap-[1px] transition-all hover:h-2.5"
          :style="{ gridTemplateColumns: `repeat(${latencyRenderBars.length}, minmax(0, 1fr))` }"
        >
          <DataTooltip
            v-for="bar in latencyRenderBars" :key="bar.key" placement="top" :content="bar.tooltip"
            class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]"
          >
            <span
              class="block h-full w-full rounded-[1px] transition-all hover:scale-y-160"
              :class="bar.className"
            />
          </DataTooltip>
        </div>
      </div>
      <div class="relative items-center gap-1">
        <div
          class="grid h-1 cursor-auto items-end gap-[1px] transition-all hover:h-2.5"
          :style="{ gridTemplateColumns: `repeat(${lossRenderBars.length}, minmax(0, 1fr))` }"
        >
          <DataTooltip
            v-for="bar in lossRenderBars" :key="bar.key" placement="top" :content="bar.tooltip"
            class="h-full w-full" content-class="whitespace-pre-wrap w-max px-1.5 !leading-[1.2] text-[11px]"
          >
            <span
              class="block h-full w-full rounded-[1px] transition-all hover:scale-y-160"
              :class="bar.className"
            />
          </DataTooltip>
        </div>
      </div>
    </div>
  </div>
</template>
