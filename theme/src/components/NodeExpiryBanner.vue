<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import { Icon } from '@iconify/vue'
import { computed, ref } from 'vue'
import { useBackgroundSurface } from '@/composables/useBackgroundSurface'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/helper'
import { hasRegion } from '@/utils/nodeHelpers'
import { getFlagSrc, getRegionDisplayName } from '@/utils/regionHelper'
import { getDaysUntilExpired, getExpireTextClass } from '@/utils/tagHelper'

const props = defineProps<{
  nodes: NodeData[]
}>()

const emit = defineEmits<{
  click: [node: NodeData]
}>()

const appStore = useAppStore()
const { pickSurfaceClass } = useBackgroundSurface()
const isExpanded = ref(false)

interface ExpiringItem {
  node: NodeData
  days: number
  isExpired: boolean
  formattedDate: string
  colorClass: string
}

const expiringList = computed<ExpiringItem[]>(() => {
  const list: ExpiringItem[] = []

  for (const node of props.nodes) {
    if (!node.expired_at)
      continue
    const days = getDaysUntilExpired(node.expired_at)
    // 过滤即将到期（<= 15天）或刚过期不久（-7 ~ 0天）的节点
    if (days >= -7 && days <= 15) {
      list.push({
        node,
        days,
        isExpired: days < 0,
        formattedDate: formatDateTime(node.expired_at, 'YYYY-MM-DD'),
        colorClass: getExpireTextClass(node.expired_at),
      })
    }
  }

  // 按剩余天数升序排列（最紧急的在前）
  return list.sort((a, b) => a.days - b.days)
})

const nearestExpiring = computed(() => expiringList.value[0] ?? null)
</script>

<template>
  <div
    v-if="nearestExpiring"
    class="w-full rounded-lg border transition-all select-none overflow-hidden"
    :class="[
      nearestExpiring.days <= 3
        ? 'border-rose-500/30 bg-rose-500/5 dark:bg-rose-500/10'
        : 'border-amber-500/30 bg-amber-500/5 dark:bg-amber-500/10',
      pickSurfaceClass('backdrop-blur-xs', 'backdrop-blur-xs'),
    ]"
  >
    <!-- 头部横条 -->
    <div
      class="flex flex-wrap items-center justify-between gap-2.5 px-3.5 py-2 cursor-pointer transition-colors hover:bg-muted/30"
      @click="isExpanded = !isExpanded"
    >
      <div class="flex items-center gap-2 text-xs font-medium">
        <Icon
          :icon="nearestExpiring.days <= 3 ? 'tabler:alert-triangle-filled' : 'tabler:clock-exclamation'"
          class="size-4 shrink-0"
          :class="nearestExpiring.days <= 3 ? 'text-rose-500' : 'text-amber-500'"
        />
        <span class="font-semibold text-foreground">
          {{ appStore.lang === 'zh-CN' ? '续费与到期提醒' : 'Renewal Alert' }}
        </span>
        <span
          class="h-4.5 px-1.5 py-0 inline-flex items-center rounded-md border text-[10px] font-mono font-semibold"
          :class="nearestExpiring.days <= 3 ? 'border-rose-500/40 text-rose-600 dark:text-rose-400 bg-rose-500/10' : 'border-amber-500/40 text-amber-600 dark:text-amber-400 bg-amber-500/10'"
        >
          {{ expiringList.length }} {{ appStore.lang === 'zh-CN' ? '台待处理' : 'nodes' }}
        </span>
        <span class="text-muted-foreground hidden sm:inline text-[11px]">
          {{ appStore.lang === 'zh-CN' ? '最近到期:' : 'Nearest:' }}
          <span class="font-mono font-semibold text-foreground ml-1">{{ nearestExpiring.node.name }}</span>
          <span class="ml-1 font-mono" :class="nearestExpiring.colorClass">
            ({{ nearestExpiring.days < 0 ? '已过期' : `剩 ${nearestExpiring.days} 天` }})
          </span>
        </span>
      </div>

      <div class="flex items-center gap-1.5 text-xs text-muted-foreground font-medium">
        <span>{{ isExpanded ? (appStore.lang === 'zh-CN' ? '收起' : 'Collapse') : (appStore.lang === 'zh-CN' ? '展开明细' : 'Details') }}</span>
        <Icon
          :icon="isExpanded ? 'tabler:chevron-up' : 'tabler:chevron-down'"
          class="size-3.5 transition-transform"
        />
      </div>
    </div>

    <!-- 展开详情列表 -->
    <div
      v-if="isExpanded"
      class="border-t border-border/40 p-2.5 grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2 bg-background/50"
    >
      <div
        v-for="item in expiringList"
        :key="item.node.uuid"
        class="flex items-center justify-between p-2 rounded-md border border-border/50 bg-card/60 hover:bg-card hover:border-border transition-all cursor-pointer"
        @click="emit('click', item.node)"
      >
        <div class="flex items-center gap-2 min-w-0">
          <img
            v-if="hasRegion(item.node.region)"
            :src="getFlagSrc(item.node.region)"
            :alt="getRegionDisplayName(item.node.region)"
            class="size-3.5 rounded-[2px] ring-1 ring-border/40 shrink-0 object-cover"
          >
          <div class="flex flex-col min-w-0">
            <span class="text-xs font-semibold text-foreground truncate max-w-[150px]">
              {{ item.node.name }}
            </span>
            <span class="text-[10px] text-muted-foreground font-mono">
              {{ item.formattedDate }}
            </span>
          </div>
        </div>

        <div class="flex flex-col items-end shrink-0 pl-2">
          <span
            class="h-4.5 px-1.5 py-0 inline-flex items-center rounded-md border text-[10.5px] font-mono font-bold"
            :class="item.days <= 3 ? 'border-rose-500/50 bg-rose-500/10 text-rose-600 dark:text-rose-400' : 'border-amber-500/50 bg-amber-500/10 text-amber-600 dark:text-amber-400'"
          >
            {{ item.days < 0 ? (appStore.lang === 'zh-CN' ? `已过期 ${Math.abs(item.days)}d` : `Expired ${Math.abs(item.days)}d`) : (appStore.lang === 'zh-CN' ? `${item.days} 天后` : `${item.days}d left`) }}
          </span>
          <span v-if="item.node.price" class="text-[10px] text-muted-foreground font-mono mt-0.5">
            {{ item.node.price }} {{ item.node.currency || 'USD' }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
