<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import { Icon } from '@iconify/vue'
import { computed, ref } from 'vue'
import NodePingListCell from '@/components/NodePingListCell.vue'
import TrafficProgress from '@/components/TrafficProgress.vue'
import { Badge } from '@/components/ui/badge'
import { DataTooltip } from '@/components/ui/data-tooltip'
import { ProgressThin } from '@/components/ui/progress-thin'
import { useBackgroundSurface } from '@/composables/useBackgroundSurface'
import { useNodeFormatters } from '@/composables/useNodeFormatters'
import { useAppStore } from '@/stores/app'
import { formatDateTime, getStatus, getStatusTextClass } from '@/utils/helper'
import { formatOfflineTime, getCustomTags, getPriceTags, getRemainingTimeTagClass, getTrafficUsed, getTrafficUsedPercentage, hasRegion, showTrafficProgress } from '@/utils/nodeHelpers'
import { getOSImage, getOSName } from '@/utils/osImageHelper'
import { getFlagSrc, getRegionDisplayName } from '@/utils/regionHelper'

interface ColumnConfig {
  key: string
  label: string
  width: string | number
  sortable: boolean
}

const props = defineProps<{
  nodes: NodeData[]
  transitionKey?: string
}>()

const emit = defineEmits<{
  click: [node: NodeData]
  pingClick: [node: NodeData]
}>()

const appStore = useAppStore()
const { pickSurfaceClass } = useBackgroundSurface()
const { formatBytes, formatBytesPerSecond, formatUptime } = useNodeFormatters()

const columns: ColumnConfig[] = [
  { key: 'status', label: '状态', width: '40px', sortable: true },
  { key: 'os', label: '系统', width: '40px', sortable: true },
  { key: 'name', label: '节点', width: 'minmax(220px, 1fr)', sortable: true },
  { key: 'cpu', label: 'CPU', width: '100px', sortable: true },
  { key: 'mem', label: '内存', width: '100px', sortable: true },
  { key: 'disk', label: '硬盘', width: '100px', sortable: true },
  { key: 'traffic', label: '流量', width: '100px', sortable: true },
  { key: 'rate', label: '速率', width: '90px', sortable: true },
  { key: 'networks', label: '三网', width: '136px', sortable: false },
]

const sortKey = ref<string>('')
const sortDir = ref<1 | -1>(1)

function handleSort(col: ColumnConfig) {
  if (!col.sortable)
    return
  if (sortKey.value === col.key) {
    sortDir.value = sortDir.value === 1 ? -1 : 1
  }
  else {
    sortKey.value = col.key
    sortDir.value = 1
  }
}

const sortedNodes = computed(() => {
  const nodes = [...props.nodes]
  const key = sortKey.value
  const dir = sortDir.value
  if (!key)
    return nodes
  return nodes.sort((a, b) => {
    switch (key) {
      case 'status': return dir * ((a.online ? 1 : 0) - (b.online ? 1 : 0))
      case 'region': {
        const va = (a.region || '').toLowerCase()
        const vb = (b.region || '').toLowerCase()
        return dir * (va < vb ? -1 : va > vb ? 1 : 0)
      }
      case 'name': {
        const va = (a.name || '').toLowerCase()
        const vb = (b.name || '').toLowerCase()
        return dir * (va < vb ? -1 : va > vb ? 1 : 0)
      }
      case 'os': {
        const va = (a.os || '').toLowerCase()
        const vb = (b.os || '').toLowerCase()
        return dir * (va < vb ? -1 : va > vb ? 1 : 0)
      }
      case 'cpu': return dir * ((a.cpu ?? 0) - (b.cpu ?? 0))
      case 'mem': return dir * ((a.ram ?? 0) / (a.mem_total || 1) - (b.ram ?? 0) / (b.mem_total || 1))
      case 'disk': return dir * ((a.disk ?? 0) / (a.disk_total || 1) - (b.disk ?? 0) / (b.disk_total || 1))
      case 'traffic':
        return dir * (getTrafficUsedPercentage(a) - getTrafficUsedPercentage(b))
      case 'rate':
        return dir * (((a.net_out ?? 0) + (a.net_in ?? 0)) - ((b.net_out ?? 0) + (b.net_in ?? 0)))
      default: return 0
    }
  })
})

const columnKeys = computed(() => columns.map(c => c.key))

const gridStyle = computed(() => ({
  gridTemplateColumns: columns.map(c => c.width).join(' '),
}))

const offlineOverlayContentStyle = computed(() => {
  const keys = columnKeys.value
  const statusIndex = keys.indexOf('status')
  const regionIndex = keys.indexOf('region')
  const nameIndex = keys.indexOf('name')
  const startColumn = nameIndex !== -1
    ? nameIndex + 1
    : regionIndex !== -1
      ? regionIndex + 2
      : statusIndex === -1 ? 1 : statusIndex + 2
  return { gridColumn: `${startColumn} / -1` }
})

function handleClick(node: NodeData) {
  emit('click', node)
}

function openPingDialog(node: NodeData) {
  emit('pingClick', node)
}

function getRowTransitionKey(node: NodeData): string {
  return props.transitionKey ? `${props.transitionKey}-${node.uuid}` : node.uuid
}
</script>

<template>
  <div class="w-full max-w-full overflow-x-auto overflow-y-hidden min-w-0 rounded-md">
    <div class="min-w-fit w-full flex flex-col gap-1">
      <!-- 表头 -->
      <div
        class="grid gap-2 rounded-md px-2 py-2 border border-border/60 select-none"
        :class="pickSurfaceClass('bg-muted/60', 'bg-muted/30')"
        :style="gridStyle"
      >
        <div
          v-for="col in columns" :key="col.key"
          :class="[
            col.sortable ? 'cursor-pointer hover:text-foreground transition-colors' : '',
            ['status', 'os'].includes(col.key) ? 'text-center' : ['cpu', 'mem', 'disk', 'traffic', 'rate'].includes(col.key) ? 'text-right' : 'text-left',
          ]"
          @click="handleSort(col)"
        >
          <span
            class="text-xs font-semibold text-foreground/90 dark:text-foreground/80 flex items-center gap-1 tracking-wide"
            :class="[
              ['status', 'os'].includes(col.key) ? 'justify-center' : ['cpu', 'mem', 'disk', 'traffic', 'rate'].includes(col.key) ? 'justify-end' : '',
              col.sortable && sortKey === col.key ? 'text-foreground font-bold' : '',
            ]"
          >
            {{ col.label }}
            <span v-if="col.sortable && sortKey === col.key" class="text-primary text-[10px] font-bold">
              {{ sortDir === 1 ? '▲' : '▼' }}
            </span>
          </span>
        </div>
      </div>

      <TransitionGroup
        :appear="!appStore.disablePageAnimation"
        :css="!appStore.disablePageAnimation"
        name="node-row-switch"
        tag="div"
        class="flex flex-col gap-1"
      >
        <div
          v-for="(node, index) in sortedNodes"
          :key="getRowTransitionKey(node)"
          class="relative flex min-h-[3.25rem] py-2 cursor-pointer flex-col justify-center rounded-md px-2 border border-border/70 transition-all hover:bg-muted/80 dark:hover:bg-muted/40 hover:border-foreground/35 hover:shadow-xs"
          :class="[
            !node.online && '!border-destructive/30',
            index % 2 === 1 ? 'bg-muted/25 dark:bg-muted/15' : 'bg-card',
          ]"
          @click="handleClick(node)"
        >
          <div class="grid gap-2 items-center" :style="gridStyle">
            <template v-for="col in columns" :key="col.key">
              <!-- 在线状态指示器 -->
              <div v-if="col.key === 'status'" class="flex justify-center">
                <div class="size-2 rounded-full relative" :class="[node.online ? 'bg-emerald-500' : 'bg-rose-500']">
                  <div
                    class="absolute -inset-0.5 rounded-full opacity-25"
                    :class="[node.online ? 'bg-emerald-400' : 'bg-rose-400']"
                  />
                </div>
              </div>

              <!-- 节点名称 -->
              <div v-if="col.key === 'name'" class="flex flex-col gap-0.5 min-w-0 pr-2">
                <div class="flex flex-row items-center gap-1.5 min-w-0">
                  <img
                    v-if="hasRegion(node.region)" :src="getFlagSrc(node.region)"
                    :alt="getRegionDisplayName(node.region)" class="size-4 shrink-0 rounded-xs"
                  >
                  <span class="truncate font-medium text-foreground/90" :title="node.name">{{ node.name }}</span>
                  <template v-if="getCustomTags(node).length > 0">
                    <Badge
                      v-for="(tag, tagIndex) in getCustomTags(node)" :key="tagIndex" variant="outline"
                      class="!text-[10px] rounded font-medium text-foreground/80 dark:text-muted-foreground border-border/80 bg-muted/40 px-1 py-0 shrink-0"
                    >
                      {{ tag }}
                    </Badge>
                  </template>
                </div>
                <div class="flex flex-row text-[11px] text-muted-foreground font-mono">
                  <DataTooltip
                    v-if="node.online" :content="formatUptime(node.uptime ?? 0)" class="shrink-0" placement="right"
                    content-class="whitespace-pre-wrap left-0 ml-0 w-max"
                  >
                    <span>
                      {{ formatUptime(node.uptime ?? 0, 'day') }}
                    </span>
                  </DataTooltip>
                  <DataTooltip
                    v-if="getPriceTags(node, appStore.lang).length > 0" placement="left"
                    :content="formatDateTime(node.expired_at, 'YYYY-MM-DD')"
                    content-class="whitespace-nowrap right-0 mr-0"
                  >
                    <div class="truncate">
                      <template v-for="(tag, tagIndex) in getPriceTags(node, appStore.lang)" :key="tagIndex">
                        <span class="mx-1">·</span>
                        <span :class="tag.highlight ? getRemainingTimeTagClass(node) : ''">
                          {{ tag.text }}
                        </span>
                      </template>
                    </div>
                  </DataTooltip>
                </div>
              </div>

              <!-- 三网 -->
              <div v-else-if="col.key === 'networks'" class="flex flex-col gap-0.5">
                <NodePingListCell
                  :uuid="node.uuid"
                  :online="node.online"
                  role="button"
                  tabindex="0"
                  class="outline-none"
                  :aria-label="`${node.name} 延迟 / 丢包`"
                  @click.stop="openPingDialog(node)"
                  @keydown.enter.stop.prevent="openPingDialog(node)"
                  @keydown.space.stop.prevent="openPingDialog(node)"
                />
              </div>

              <!-- 操作系统 -->
              <div v-else-if="col.key === 'os'" class="flex justify-center">
                <img :src="getOSImage(node.os)" :alt="getOSName(node.os)" class="size-4">
              </div>

              <!-- CPU -->
              <div v-else-if="col.key === 'cpu'" class="group text-right">
                <div class="space-y-1">
                  <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums text-right">
                    <span class="inline group-hover:hidden font-medium" :class="getStatusTextClass(node.cpu ?? 0)">
                      {{ (node.cpu ?? 0).toFixed(1) }}%
                    </span>
                    <span class="hidden group-hover:inline">
                      {{ node.load.toFixed(2) ?? 0 }}, {{ node.load5.toFixed(2) ?? 0 }}, {{ node.load15.toFixed(2) ?? 0
                      }}
                    </span>
                  </div>
                  <ProgressThin :percentage="node.cpu ?? 0" :status="getStatus(node.cpu ?? 0)" :height="4" />
                </div>
              </div>

              <!-- 内存 -->
              <div v-else-if="col.key === 'mem'" class="group text-right">
                <DataTooltip placement="top" class="block" :content-class="[!node.swap && '!hidden']">
                  <div class="space-y-1">
                    <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums text-right">
                      <span class="inline group-hover:hidden font-medium" :class="getStatusTextClass((node.ram ?? 0) / (node.mem_total || 1) * 100)">
                        {{ ((node.ram ?? 0) / (node.mem_total || 1) * 100).toFixed(1) }}%
                      </span>
                      <span class="hidden group-hover:inline">
                        {{ formatBytes(node.ram ?? 0) }} / {{ formatBytes(node.mem_total ?? 0) }}
                      </span>
                    </div>
                    <ProgressThin
                      :percentage="(node.ram ?? 0) / (node.mem_total || 1) * 100"
                      :status="getStatus((node.ram ?? 0) / (node.mem_total || 1) * 100)" :height="4"
                    />
                  </div>
                  <template #content>
                    <div class="flex items-center justify-between gap-3 whitespace-nowrap">
                      <span class="text-background/70">Swap</span>
                      <span>{{ formatBytes(node.swap ?? 0) }}</span>
                    </div>
                  </template>
                </DataTooltip>
              </div>

              <!-- 硬盘 -->
              <div v-else-if="col.key === 'disk'" class="group text-right">
                <div class="space-y-1">
                  <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums text-right">
                    <span class="inline group-hover:hidden font-medium" :class="getStatusTextClass((node.disk ?? 0) / (node.disk_total || 1) * 100)">
                      {{ ((node.disk ?? 0) / (node.disk_total || 1) * 100).toFixed(1) }}%
                    </span>
                    <span class="hidden group-hover:inline">
                      {{ formatBytes(node.disk ?? 0) }} / {{ formatBytes(node.disk_total ?? 0) }}
                    </span>
                  </div>
                  <ProgressThin
                    :percentage="(node.disk ?? 0) / (node.disk_total || 1) * 100"
                    :status="getStatus((node.disk ?? 0) / (node.disk_total || 1) * 100)" :height="4"
                  />
                </div>
              </div>

              <!-- 流量 -->
              <div v-else-if="col.key === 'traffic'" class="group text-right">
                <DataTooltip placement="top" class="flex items-center gap-2" content-class="mb-1.5">
                  <div class="space-y-1 w-full">
                    <div class="text-[10px] text-muted-foreground truncate font-mono tabular-nums text-right">
                      <span class="inline group-hover:hidden">
                        {{ getTrafficUsedPercentage(node).toFixed(1) }}%
                      </span>
                      <span class="hidden group-hover:inline">
                        {{ formatBytes(getTrafficUsed(node)) }} /
                        <template v-if="showTrafficProgress(node)">{{ formatBytes(node.traffic_limit) }}</template>
                        <template v-else>∞</template>
                      </span>
                    </div>
                    <TrafficProgress
                      :upload="node.net_total_up ?? 0" :download="node.net_total_down ?? 0"
                      :traffic-limit="node.traffic_limit" :traffic-limit-type="(node.traffic_limit_type || 'sum')"
                      height="4px"
                    />
                  </div>
                  <template #content>
                    <span class="flex flex-row gap-0.5 items-center whitespace-nowrap">
                      <Icon icon="tabler:chevron-up" width="12" height="12" />
                      {{ formatBytes(node.net_total_up ?? 0) }}
                    </span>
                    <span class="flex flex-row gap-0.5 items-center whitespace-nowrap">
                      <Icon icon="tabler:chevron-down" width="12" height="12" />
                      {{ formatBytes(node.net_total_down ?? 0) }}
                    </span>
                  </template>
                </DataTooltip>
              </div>

              <!-- 速率 -->
              <div v-else-if="col.key === 'rate'" class="text-right">
                <div class="text-[10px] flex flex-col items-end font-mono tabular-nums">
                  <span class="text-emerald-600 dark:text-emerald-400 font-medium flex flex-row gap-0.5 items-center justify-end">
                    <Icon icon="tabler:chevron-up" width="12" height="12" class="text-emerald-500 shrink-0" />
                    {{ formatBytesPerSecond(node.net_out ?? 0) }}
                  </span>
                  <span class="text-sky-600 dark:text-sky-400 font-medium flex flex-row gap-0.5 items-center justify-end">
                    <Icon icon="tabler:chevron-down" width="12" height="12" class="text-sky-500 shrink-0" />
                    {{ formatBytesPerSecond(node.net_in ?? 0) }}
                  </span>
                </div>
              </div>
            </template>
          </div>

          <div
            v-if="!node.online" class="absolute inset-0 z-2 p-2 bg-background/10 rounded-lg flex items-center"
            aria-hidden="true"
          >
            <div class="grid gap-2 items-center justify-center" :style="gridStyle">
              <div class="h-full space-y-1" :style="offlineOverlayContentStyle">
                <div class="text-sm font-semibold truncate">
                  <span class="text-red-500">离线</span> {{ node.name }}
                </div>
                <div class="text-xs text-muted-foreground">
                  {{ formatOfflineTime(node) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </div>
</template>

<style scoped>
.node-row-switch-enter-active,
.node-row-switch-leave-active {
  transition:
    opacity 150ms ease,
    transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
}

.node-row-switch-move {
  transition: transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
}

.node-row-switch-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.node-row-switch-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (prefers-reduced-motion: reduce) {
  .node-row-switch-enter-active,
  .node-row-switch-leave-active,
  .node-row-switch-move {
    transition: none;
  }

  .node-row-switch-enter-from,
  .node-row-switch-leave-to {
    opacity: 1;
    transform: none;
  }
}
</style>
