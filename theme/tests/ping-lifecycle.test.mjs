import assert from 'node:assert/strict'
// eslint-disable-next-line test/no-import-node-test -- Tests use the built-in Node runner.
import test from 'node:test'
import { createRenderer, defineComponent, h, KeepAlive, nextTick, ref } from 'vue'
import { loadSource, memoryStorage } from './helpers.mjs'

const intervals = new Map()
let nextInterval = 0
const savedSetInterval = globalThis.setInterval
const savedClearInterval = globalThis.clearInterval
let calls = 0
let now = 1_000_000
const savedNow = Date.now

globalThis.window = { localStorage: memoryStorage() }
globalThis.document = { hidden: false }
globalThis.setInterval = (callback) => {
  const id = ++nextInterval
  intervals.set(id, callback)
  return id
}
globalThis.clearInterval = id => intervals.delete(id)
Date.now = () => now

globalThis.__pingLifecycleCall = async (method) => {
  calls++
  return method === 'common:getRecords' ? { records: [], tasks: [] } : []
}
const { useNodePingStats } = await loadSource('src/composables/useNodePingStats.ts', {
  stubs: {
    '@/utils/rpc': 'export const getSharedRpc = () => ({ getClient: () => ({ call: globalThis.__pingLifecycleCall }) })',
  },
})

const renderer = createRenderer({
  createElement: type => ({ type, children: [] }),
  insert(child, parent, anchor = null) {
    if (child.parent)
      child.parent.children.splice(child.parent.children.indexOf(child), 1)
    child.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    parent.children.splice(index < 0 ? parent.children.length : index, 0, child)
  },
  remove(child) {
    const parent = child.parent
    if (parent)
      parent.children.splice(parent.children.indexOf(child), 1)
    child.parent = null
  },
  createText: text => ({ text }),
  createComment: text => ({ text }),
  setText: (node, text) => { node.text = text },
  setElementText: (node, text) => { node.text = text },
  parentNode: node => node.parent,
  nextSibling: node => node.parent?.children[node.parent.children.indexOf(node) + 1] ?? null,
  patchProp() {},
})

async function settle() {
  for (let i = 0; i < 6; i++) {
    await Promise.resolve()
    await nextTick()
  }
}

test('cached ping consumers release refresh timers and resume stale data only once', async () => {
  const visible = ref(true)
  const enabled = ref(true)
  const hours = ref(1)
  const Home = defineComponent({
    name: 'HomeView',
    setup() {
      useNodePingStats('node-a', { enabled, hours })
      useNodePingStats('node-b', { enabled, hours })
      return () => h('main')
    },
  })
  const Other = defineComponent({ setup: () => () => h('aside') })
  const app = renderer.createApp({
    setup: () => () => h(KeepAlive, null, { default: () => h(visible.value ? Home : Other) }),
  })
  try {
    app.mount({ children: [] })
    await settle()
    assert.equal(intervals.size, 1, 'same-window consumers share a timer')
    assert.equal(calls, 2, 'initial records and task requests are deduplicated')

    visible.value = false
    await settle()
    assert.equal(intervals.size, 0, 'deactivation stops the last subscriber timer')

    enabled.value = false
    hours.value = 2
    enabled.value = true
    await settle()
    assert.equal(intervals.size, 0, 'inactive reactive changes do not resubscribe')
    assert.equal(calls, 2, 'inactive changes do not request history')

    now += 61_000
    visible.value = true
    await settle()
    assert.equal(intervals.size, 1, 'activation restores one shared timer')
    assert.equal(calls, 4, 'activation fetches the new window once')

    visible.value = false
    await settle()
    visible.value = true
    await settle()
    assert.equal(calls, 4, 'fresh data is reused on immediate return')

    enabled.value = false
    await settle()
    assert.equal(intervals.size, 0, 'disabled consumers release the timer')
    enabled.value = true
    await settle()
    assert.equal(intervals.size, 1)
  }
  finally {
    app.unmount()
    await settle()
    assert.equal(intervals.size, 0, 'unmount releases all subscriptions')
  }
})

test('hidden HomeView children never restart polling when mounted or replaced', async () => {
  const callBaseline = calls
  const visible = ref(true)
  const nodes = ref(['node-a', 'node-b'])
  const hours = ref(1)
  globalThis.__pingLifecycleMounted = []
  globalThis.__pingLifecycleVue = { defineComponent, h }
  const { default: Home } = await loadSource('src/views/HomeView.vue', {
    stubs: {
      '@/utils/rpc': 'export const getSharedRpc = () => ({ getClient: () => ({ call: globalThis.__pingLifecycleCall }) })',
      'vue-router': 'export const useRouter = () => ({ push() {} })',
      '@iconify/vue': 'export const Icon = {}',
      '@/stores/app': 'export const useAppStore = () => ({ homeScrollPosition: 0, nodeSelectedGroup: "all" })',
      '@/stores/nodes': 'export const useNodesStore = () => ({ groups: [], nodes: [], earthNodes: [] })',
      '@/composables/useBackgroundSurface': 'export const useBackgroundSurface = () => ({ pickSurfaceClass: () => "" })',
      '@/components/ui/alert': 'export const Alert = {}, AlertDescription = {}, AlertTitle = {}',
      '@/components/ui/dialog': 'export const Dialog = {}, DialogContent = {}, DialogHeader = {}, DialogTitle = {}',
      '@/components/ui/empty': 'export const Empty = {}',
      '@/components/ui/input': 'export const Input = {}',
      '@/components/ui/tabs': 'export const Tabs = {}, TabsContent = {}, TabsList = {}, TabsTrigger = {}',
      '@/components/NodeCard.vue': `
        import { useNodePingStats } from '@/composables/useNodePingStats'
        const { defineComponent, h } = globalThis.__pingLifecycleVue
        const Consumer = defineComponent({
          props: ['uuid', 'hours'],
          setup(props) {
            globalThis.__pingLifecycleMounted.push(props.uuid)
            useNodePingStats(() => props.uuid, { hours: () => props.hours })
            return () => h('article')
          },
        })
        globalThis.__pingLifecycleConsumer = Consumer
        export default Consumer
      `,
      '@/components/NodeCardSkeleton.vue': 'export default {}',
      '@/components/NodeGeneralCards.vue': 'export default {}',
      '@/components/NodeGeneralCardsSkeleton.vue': 'export default {}',
      '@/components/NodeList.vue': 'export default {}',
      '@/components/PingChart.vue': 'export default {}',
    },
  })
  // Exercise the production HomeView setup while rendering a controllable,
  // keyed child list through Vue's real renderer and KeepAlive lifecycle.
  Home.render = () => h('main', nodes.value.map(uuid => h(globalThis.__pingLifecycleConsumer, {
    key: uuid,
    uuid,
    hours: hours.value,
  })))
  const Other = defineComponent({ setup: () => () => h('aside') })
  const app = renderer.createApp({
    setup: () => () => h(KeepAlive, null, { default: () => h(visible.value ? Home : Other) }),
  })
  try {
    app.mount({ children: [] })
    await settle()
    assert.equal(intervals.size, 1, 'existing children share one timer')
    assert.equal(calls - callBaseline, 2, 'existing children deduplicate initial RPCs')

    visible.value = false
    await settle()
    assert.equal(intervals.size, 0, 'existing children release the timer on deactivation')
    const timersBeforeHiddenMount = nextInterval
    const assertInactive = () => assert.deepEqual(
      { timers: intervals.size, createdTimers: nextInterval, calls: calls - callBaseline },
      { timers: 0, createdTimers: timersBeforeHiddenMount, calls: 2 },
      '隐藏期间不应创建 timer 或 RPC',
    )

    now += 61_000
    nodes.value.push('node-c')
    await settle()
    assert.deepEqual(globalThis.__pingLifecycleMounted, ['node-a', 'node-b', 'node-c'], 'new child actually mounts in the inactive subtree')
    assertInactive()

    nodes.value = ['node-d', 'node-e']
    hours.value = 3
    await settle()
    assert.deepEqual(globalThis.__pingLifecycleMounted.slice(-2).sort(), ['node-d', 'node-e'], 'replacement children actually mount while hidden')
    assertInactive()
    hours.value = 1
    await settle()
    assertInactive()

    visible.value = true
    await settle()
    assert.equal(intervals.size, 1, 'replacement children restore one shared timer')
    assert.equal(calls - callBaseline, 4, 'stale cache catches up with one deduplicated RPC pair')

    visible.value = false
    await settle()
    const timersBeforeFreshMount = nextInterval
    nodes.value = ['node-f', 'node-g']
    await settle()
    assert.deepEqual(globalThis.__pingLifecycleMounted.slice(-2).sort(), ['node-f', 'node-g'])
    assert.deepEqual(
      { timers: intervals.size, createdTimers: nextInterval, calls: calls - callBaseline },
      { timers: 0, createdTimers: timersBeforeFreshMount, calls: 4 },
      'fresh hidden replacements do not create even a transient timer or RPC',
    )

    visible.value = true
    await settle()
    assert.equal(intervals.size, 1, 'fresh replacement consumers share one timer on return')
    assert.equal(calls - callBaseline, 4, 'fresh cache is reused without RPCs')
  }
  finally {
    app.unmount()
    await settle()
    assert.equal(intervals.size, 0, 'unmount releases all child subscriptions')
    delete globalThis.__pingLifecycleConsumer
    delete globalThis.__pingLifecycleMounted
    delete globalThis.__pingLifecycleVue
  }
})

test.after(() => {
  globalThis.setInterval = savedSetInterval
  globalThis.clearInterval = savedClearInterval
  Date.now = savedNow
})
