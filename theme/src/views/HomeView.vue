<script setup lang="ts">
import type { NodeData } from '@/stores/nodes'
import { Icon } from '@iconify/vue'
import { useDebounceFn } from '@vueuse/core'
import { computed, defineAsyncComponent, nextTick, onActivated, onDeactivated, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import NodeCard from '@/components/NodeCard.vue'
import NodeCardSkeleton from '@/components/NodeCardSkeleton.vue'
import NodeGeneralCards from '@/components/NodeGeneralCards.vue'
import NodeGeneralCardsSkeleton from '@/components/NodeGeneralCardsSkeleton.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Empty } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useBackgroundSurface } from '@/composables/useBackgroundSurface'
import { provideViewActivity } from '@/composables/useViewActivity'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import * as financeHelper from '@/utils/financeHelper'
import { sharedExchangeRates } from '@/utils/financeHelper'
import { getTrafficUsedPercentage, isNodeInGroup, parseNodeGroups } from '@/utils/nodeHelpers'
import { isRegionMatch } from '@/utils/regionHelper'
import { getDaysUntilExpired } from '@/utils/tagHelper'

defineOptions({ name: 'HomeView' })

const NodeList = defineAsyncComponent(() => import('@/components/NodeList.vue'))
const NodeExpiryBanner = defineAsyncComponent(() => import('@/components/NodeExpiryBanner.vue'))
const PingChart = defineAsyncComponent(() => import('@/components/PingChart.vue'))

const appStore = useAppStore()
const { pickSurfaceClass } = useBackgroundSurface()
const nodesStore = useNodesStore()
const router = useRouter()

provideViewActivity()

onActivated(() => {
  if (appStore.homeScrollPosition > 0) {
    nextTick(() => {
      window.scrollTo({ top: appStore.homeScrollPosition, behavior: 'instant' })
    })
  }
})

onDeactivated(() => {
  appStore.homeScrollPosition = window.scrollY
})

const searchText = ref('')
const debouncedSearchText = ref('')
const selectedPingNodeUuid = ref<string | null>(null)

type SortMode = 'default' | 'expiry' | 'cost_desc' | 'cost_asc' | 'traffic'
const currentSort = ref<SortMode>('default')

const updateDebouncedSearch = useDebounceFn((value: string) => {
  debouncedSearchText.value = value
}, 300)

watch(searchText, (value) => {
  updateDebouncedSearch(value)
})

const groups = computed(() => [
  { tab: '全部节点', name: 'all' },
  ...nodesStore.groups.map(g => ({ tab: g, name: g })),
])

watch(
  () => nodesStore.groups,
  (gs) => {
    const cur = appStore.nodeSelectedGroup
    if (cur !== 'all' && !gs.includes(cur)) {
      appStore.nodeSelectedGroup = 'all'
    }
  },
  { immediate: true },
)

function isNodeMatchSearch(node: typeof nodesStore.nodes[number], search: string): boolean {
  if (!search.trim())
    return true
  const lowerSearch = search.toLowerCase().trim()
  if (node.name.toLowerCase().includes(lowerSearch))
    return true
  if (node.region && isRegionMatch(node.region, search))
    return true
  if (node.os && node.os.toLowerCase().includes(lowerSearch))
    return true
  if (parseNodeGroups(node.group).some(group => group.toLowerCase().includes(lowerSearch)))
    return true
  if (node.tags && node.tags.toLowerCase().includes(lowerSearch))
    return true
  if (node.public_remark && node.public_remark.toLowerCase().includes(lowerSearch))
    return true
  return false
}

const groupNodeList = computed(() => {
  return nodesStore.nodes.filter(node => isNodeInGroup(node.group, appStore.nodeSelectedGroup))
})

const sampledGroupNodeList = computed(() => {
  return nodesStore.earthNodes.filter(node => isNodeInGroup(node.group, appStore.nodeSelectedGroup))
})

const nodeList = computed(() => {
  let filtered = groupNodeList.value

  // 搜索关键字过滤
  if (debouncedSearchText.value.trim()) {
    filtered = filtered.filter(n => isNodeMatchSearch(n, debouncedSearchText.value))
  }

  // 排序逻辑
  return [...filtered].sort((a, b) => {
    if (currentSort.value === 'expiry') {
      const da = a.expired_at ? getDaysUntilExpired(a.expired_at) : 99999
      const db = b.expired_at ? getDaysUntilExpired(b.expired_at) : 99999
      return da - db
    }
    if (currentSort.value === 'cost_desc') {
      const ca = financeHelper.calculateMonthlyAverageCostCNY(a, sharedExchangeRates.value)
      const cb = financeHelper.calculateMonthlyAverageCostCNY(b, sharedExchangeRates.value)
      return cb - ca
    }
    if (currentSort.value === 'cost_asc') {
      const ca = financeHelper.calculateMonthlyAverageCostCNY(a, sharedExchangeRates.value)
      const cb = financeHelper.calculateMonthlyAverageCostCNY(b, sharedExchangeRates.value)
      return ca - cb
    }
    if (currentSort.value === 'traffic') {
      const ta = getTrafficUsedPercentage(a)
      const tb = getTrafficUsedPercentage(b)
      return tb - ta
    }
    // 默认稳定排序：在线节点在前、离线节点在后，组内保持原有顺序
    return (a.online === b.online ? 0 : a.online ? -1 : 1)
  })
})

const selectedPingNode = computed(() => {
  if (!selectedPingNodeUuid.value)
    return null
  return nodesStore.nodes.find(node => node.uuid === selectedPingNodeUuid.value) ?? null
})

const pingDialogOpen = computed({
  get: () => selectedPingNode.value !== null,
  set: (open: boolean) => {
    if (!open)
      selectedPingNodeUuid.value = null
  },
})

function handleNodeClick(node: typeof nodesStore.nodes[number]) {
  router.push({ name: 'instance-detail', params: { id: node.uuid } })
}

function handlePingClick(node: NodeData) {
  selectedPingNodeUuid.value = node.uuid
}

function getNodeItemTransitionKey(node: typeof nodesStore.nodes[number]): string {
  return `${appStore.nodeSelectedGroup}-${node.uuid}`
}
</script>

<template>
  <div class="home-view pb-20">
    <div v-if="appStore.connectionError" class="alert px-4">
      <Alert
        variant="destructive"
        :class="pickSurfaceClass('border-none bg-red-400/10 rounded-md', 'border-none bg-red-400/10 backdrop-blur-xs rounded-md')"
      >
        <AlertTitle>RPC 服务错误</AlertTitle>
        <AlertDescription>连接服务器失败，请检查网络设置或刷新页面后再试。</AlertDescription>
      </Alert>
    </div>

    <NodeGeneralCardsSkeleton v-if="appStore.loading && nodesStore.nodes.length === 0 && appStore.earthViewMode !== 'hide'" />
    <NodeGeneralCards
      v-else-if="appStore.earthViewMode !== 'hide'"
      :nodes="groupNodeList"
      :globe-nodes="sampledGroupNodeList"
      :transition-key="appStore.nodeSelectedGroup"
    />

    <div class="node-info p-4 pt-0 flex flex-col gap-4 relative z-1" :class="appStore.earthViewMode === 'hide' && 'pt-4'">
      <!-- 续费与到期预警 Banner -->
      <NodeExpiryBanner :nodes="nodesStore.nodes" @click="handleNodeClick" />

      <div class="nodes">
        <Tabs v-model="appStore.nodeSelectedGroup" class="w-full flex-col gap-3">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
            <!-- 分组标签 -->
            <div class="overflow-x-auto no-scrollbar py-0.5">
              <TabsList class="w-max h-9 p-0.5 bg-muted/80 dark:bg-muted/60 border border-border/60 rounded-md">
                <TabsTrigger
                  v-for="g in groups" :key="g.name" :value="g.name"
                  class="h-8 px-3 text-xs rounded-[5px] text-muted-foreground transition-all data-[state=active]:text-foreground data-[state=active]:bg-card data-[state=active]:shadow-xs data-[state=active]:font-semibold cursor-pointer"
                >
                  {{ g.tab }}
                </TabsTrigger>
              </TabsList>
            </div>

            <!-- 右侧工具栏：排序、视图模式切换、搜索框 -->
            <div class="flex flex-wrap items-center gap-2 self-end sm:self-auto shrink-0">
              <!-- 排序下拉选择 -->
              <div class="flex items-center h-9 px-2 rounded-md border border-border/60 bg-muted/80 dark:bg-muted/60 text-xs text-muted-foreground">
                <Icon icon="tabler:arrows-sort" class="size-3.5 mr-1 shrink-0 text-muted-foreground" />
                <select
                  v-model="currentSort"
                  aria-label="节点排序"
                  class="h-full bg-transparent py-0 text-xs text-foreground font-medium focus:outline-none cursor-pointer pr-1"
                >
                  <option value="default" class="bg-card text-foreground">默认排序</option>
                  <option value="expiry" class="bg-card text-foreground">到期日优先</option>
                  <option value="cost_desc" class="bg-card text-foreground">月费最高</option>
                  <option value="cost_asc" class="bg-card text-foreground">月费最低</option>
                  <option value="traffic" class="bg-card text-foreground">流量使用率</option>
                </select>
              </div>

              <!-- 视图模式切换：卡片、列表 -->
              <div class="flex items-center p-0.5 bg-muted/80 dark:bg-muted/60 border border-border/60 rounded-md h-9">
                <button
                  type="button"
                  title="卡片视图"
                  aria-label="卡片视图"
                  class="h-8 w-8 flex items-center justify-center rounded-[5px] transition-all cursor-pointer"
                  :class="appStore.nodeViewMode === 'card' ? 'bg-card text-foreground shadow-xs font-semibold' : 'text-muted-foreground hover:text-foreground'"
                  @click="appStore.nodeViewMode = 'card'"
                >
                  <Icon icon="tabler:layout-grid" :width="14" :height="14" />
                </button>
                <button
                  type="button"
                  title="列表视图"
                  aria-label="列表视图"
                  class="h-8 w-8 flex items-center justify-center rounded-[5px] transition-all cursor-pointer"
                  :class="appStore.nodeViewMode === 'list' ? 'bg-card text-foreground shadow-xs font-semibold' : 'text-muted-foreground hover:text-foreground'"
                  @click="appStore.nodeViewMode = 'list'"
                >
                  <Icon icon="tabler:table" :width="14" :height="14" />
                </button>
              </div>

              <!-- 搜索框 -->
              <div class="relative">
                <Input
                  v-model="searchText" placeholder="搜索节点、地区、系统..."
                  class="h-9 w-40 sm:w-48 pl-7.5 pr-2.5 rounded-md border border-border/60 bg-muted/80 dark:bg-muted/60 shadow-none text-xs transition-all placeholder:text-muted-foreground/70 focus:w-48 sm:focus:w-56 focus:bg-card focus:border-border focus:ring-1 focus:ring-foreground/20"
                />
                <Icon
                  icon="tabler:search" :width="14" :height="14"
                  class="absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none text-muted-foreground"
                />
              </div>
            </div>
          </div>

          <TabsContent :key="appStore.nodeSelectedGroup" :value="appStore.nodeSelectedGroup">
            <!-- 骨架屏加载态 -->
            <div
              v-if="appStore.loading && nodesStore.nodes.length === 0"
              class="gap-3 grid grid-cols-1 sm:grid-cols-[repeat(auto-fill,minmax(300px,1fr))]"
            >
              <NodeCardSkeleton v-for="i in 6" :key="i" />
            </div>

            <!-- 正常卡片视图 -->
            <TransitionGroup
              v-else-if="nodeList.length !== 0 && appStore.nodeViewMode === 'card'"
              :appear="false"
              name="node-card-switch"
              tag="div"
              class="gap-3 grid grid-cols-1 sm:grid-cols-[repeat(auto-fill,minmax(300px,1fr))]"
            >
              <div
                v-for="node in nodeList"
                :key="getNodeItemTransitionKey(node)"
                class="min-w-0"
              >
                <NodeCard
                  :node="node"
                  @click="handleNodeClick(node)"
                  @ping-click="handlePingClick"
                />
              </div>
            </TransitionGroup>

            <!-- 列表视图 -->
            <NodeList
              v-else-if="nodeList.length !== 0 && appStore.nodeViewMode === 'list'"
              :nodes="nodeList"
              :transition-key="appStore.nodeSelectedGroup"
              @click="handleNodeClick"
              @ping-click="handlePingClick"
            />

            <div v-else class="text-muted-foreground text-center py-8">
              <Empty description="暂无匹配节点" />
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </div>

    <!-- Ping 详情弹窗 -->
    <Dialog v-model:open="pingDialogOpen">
      <DialogContent
        v-if="selectedPingNode"
        class="w-[96vw] max-w-5xl max-h-[92vh] gap-0 overflow-hidden border border-border/80 p-0 shadow-2xl transition-all bg-card rounded-xl"
      >
        <DialogHeader class="flex h-12 flex-row items-center px-4 border-b border-border/60 shrink-0">
          <DialogTitle class="truncate text-sm sm:text-base font-semibold">
            {{ selectedPingNode.name }} 延迟 / 丢包
          </DialogTitle>
        </DialogHeader>
        <div class="max-h-[calc(92vh-3rem)] overflow-y-auto p-3 sm:p-4">
          <PingChart :uuid="selectedPingNode.uuid" />
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.node-card-switch-enter-active,
.node-card-switch-leave-active {
  transition:
    opacity 150ms ease,
    transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
}

.node-card-switch-move {
  transition: transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
}

.node-card-switch-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.node-card-switch-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (prefers-reduced-motion: reduce) {
  .node-card-switch-enter-active,
  .node-card-switch-leave-active,
  .node-card-switch-move {
    transition: none;
  }

  .node-card-switch-enter-from,
  .node-card-switch-leave-to {
    opacity: 1;
    transform: none;
  }
}
</style>
