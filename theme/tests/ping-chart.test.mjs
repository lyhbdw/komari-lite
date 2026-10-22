import assert from 'node:assert/strict'
// eslint-disable-next-line test/no-import-node-test -- Built-in Node test runner.
import test from 'node:test'
import { loadSource, renderer, settle } from './helpers.mjs'

let calls = []
const completions = []
let mode = 'immediate'
globalThis.__chartCall = async (method, params) => {
  calls.push([method, params])
  if (method === 'public:getPublicPingTasks')
    return [{ id: 1, name: 'Task' }]
  if (method === 'public:getPingMetricStats')
    return { stats: [{ task_id: '1', loss: 25, total: 4 }] }
  if (mode === 'deferred')
    await new Promise((resolve) => { completions.push(resolve) })
  return { series: [{ metric_key: 'ping.latency_ms', tags: { task_id: '1' }, points: [{ time: '2026-01-01', value: 20 }] }, { metric_key: 'ping.loss', tags: { task_id: '1' }, points: [{ time: '2026-01-01', value: 0.25 }] }] }
}
const ui = 'export const Button={}; export const DataTooltip={}; export const Empty={}; export const Spinner={}; export const Tabs={};export const TabsList={};export const TabsTrigger={};'
const { default: PingChart } = await loadSource('src/components/PingChart.vue', { stubs: {
  '@/utils/rpc': 'export class RpcError extends Error {}; export const getSharedRpc=()=>({getClient:()=>({call:globalThis.__chartCall})})',
  '@/stores/app': 'export const useAppStore=()=>({isDark:false,publicSettings:{ping_record_preserve_time:2160}})',
  '@/composables/useBackgroundSurface': 'export const useBackgroundSurface=()=>({pickSurfaceClass:()=>""})',
  '@/utils/echarts': 'export const getChartThemeColors=()=>({});export const formatChartTimeForTooltip=()=>""',
  'vue-echarts': 'export default {}',
  ...Object.fromEntries(['button', 'data-tooltip', 'empty', 'spinner', 'tabs'].map(name => [`@/components/ui/${name}`, ui])),
} })
test('chart combines latency and loss, retains both series and stats', async () => {
  calls = []
  mode = 'immediate'
  const app = renderer.createApp(PingChart, { uuid: 'one' })
  app.mount({ children: [] })
  await settle()
  try {
    const queries = calls.filter(([method]) => method === 'public:queryMetrics')
    assert.equal(queries.length, 1, 'latency and loss must share one history query')
    assert.deepEqual(queries[0][1].metric_keys, ['ping.latency_ms', 'ping.loss'])
    const state = app._instance.setupState
    assert.equal(state.remoteData.length, 1)
    assert.equal(state.lossRecordsData[0].loss, 0.25)
    assert.equal(state.tasks[0].loss, 25)
  }
  finally { app.unmount() }
})
test('unmounted chart discards late response and starts no follow-up queries', async () => {
  calls = []
  mode = 'deferred'
  const app = renderer.createApp(PingChart, { uuid: 'one' })
  app.mount({ children: [] })
  await settle()
  const state = app._instance.setupState
  app.unmount()
  for (const complete of completions.splice(0))
    complete()
  await settle()
  assert.equal(state.remoteData.length, 0)
  assert.equal(state.tasks.length, 0)
})
