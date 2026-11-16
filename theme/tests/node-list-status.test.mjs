/* eslint-disable test/no-import-node-test */
import assert from 'node:assert/strict'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import test from 'node:test'
import { pathToFileURL } from 'node:url'
import { compileScript, parse } from '@vue/compiler-sfc'
import { renderToString } from '@vue/server-renderer'
import { build } from 'esbuild'
import { createSSRApp, h } from 'vue'

const root = resolve(dirname(new URL(import.meta.url).pathname), '..')
const directory = await mkdtemp(resolve(root, 'tests/.tmp-list-'))
const outfile = resolve(directory, 'list.mjs')
const passthrough = `import { h } from 'vue'; export default { inheritAttrs: false, setup(_, { slots, attrs }) { return () => h('span', attrs, slots.default?.()) } }`
const stubs = {
  '@iconify/vue': `import { h } from 'vue'; export const Icon = { render: () => h('i') }`,
  '@/components/NodePingListCell.vue': 'export default { render: () => null }',
  '@/components/TrafficProgress.vue': `import { h } from 'vue'; export default { render: () => h('div', { 'data-traffic-progress': 'true' }) }`,
  '@/components/ui/badge': `${passthrough.replace('export default', 'const Badge =')}; export { Badge }`,
  '@/components/ui/data-tooltip': `${passthrough.replace('export default', 'const DataTooltip =')}; export { DataTooltip }`,
  '@/components/ui/progress-thin': 'export const ProgressThin = { render: () => null }',
  '@/composables/useBackgroundSurface': 'export const useBackgroundSurface = () => ({ pickSurfaceClass: () => "" })',
  '@/composables/useNodeFormatters': 'export const useNodeFormatters = () => ({ formatBytes: () => "3 B", formatBytesPerSecond: () => "0 B/s", formatUptime: () => "1d" })',
  '@/stores/app': 'export const useAppStore = () => ({ lang: "zh-CN", disablePageAnimation: true })',
  '@/utils/helper': 'export const formatDateTime = () => "2025-01-01"; export const getStatus = () => "default"; export const getStatusTextClass = () => ""',
  '@/utils/nodeHelpers': 'export const formatOfflineTime = () => "2025-01-01"; export const getCustomTags = () => []; export const getDiskPercentage = () => 0; export const getMemPercentage = () => 0; export const getPriceTags = () => []; export const getRemainingTimeTagClass = () => ""; export const getTrafficUsed = node => (node.net_total_up || 0) + (node.net_total_down || 0); export const getTrafficUsedPercentage = node => node.traffic_limit > 0 ? ((node.net_total_up || 0) + (node.net_total_down || 0)) / node.traffic_limit * 100 : 0; export const hasRegion = () => false; export const showTrafficProgress = node => node.traffic_limit > 0',
  '@/utils/osImageHelper': 'export const getOSImage = () => ""; export const getOSName = () => "Linux"',
  '@/utils/regionHelper': 'export const getFlagSrc = () => ""; export const getRegionDisplayName = () => ""',
}

let NodeList
try {
  await build({
    entryPoints: [resolve(root, 'src/components/NodeList.vue')],
    outfile,
    bundle: true,
    platform: 'node',
    format: 'esm',
    packages: 'external',
    alias: { '@': resolve(root, 'src') },
    plugins: [{ name: 'render-list', setup(builder) {
      builder.onResolve({ filter: /.*/ }, args => args.path in stubs ? { path: args.path, namespace: 'stub' } : undefined)
      builder.onLoad({ filter: /.*/, namespace: 'stub' }, args => ({ contents: stubs[args.path], loader: 'ts', resolveDir: root }))
      builder.onLoad({ filter: /\.vue$/ }, async (args) => {
        const source = await readFile(args.path, 'utf8')
        const { descriptor } = parse(source, { filename: args.path })
        const compiled = compileScript(descriptor, { id: 'list-test', genDefaultAs: '__component', inlineTemplate: true })
        return { contents: `${compiled.content}${String.fromCharCode(10)}export default __component`, loader: 'ts' }
      })
    } }],
  })
  NodeList = (await import(pathToFileURL(outfile).href)).default
}
finally {
  await rm(directory, { recursive: true, force: true })
}

async function renderNode(trafficLimit, possibleChina, online = true) {
  const node = {
    uuid: 'node',
    name: 'Edge node',
    online,
    time: '2025-01-01T00:00:00Z',
    region: '',
    tags: '',
    os: 'Linux',
    price: 0,
    billing_cycle: 30,
    currency: 'CNY',
    traffic_limit: trafficLimit,
    traffic_limit_type: 'sum',
    net_total_up: 1,
    net_total_down: 2,
    net_out: 0,
    net_in: 0,
    uptime: 1,
    cpu: 0,
    load: 0,
    load5: 0,
    load15: 0,
    ram: 0,
    mem_total: 1,
    disk: 0,
    disk_total: 1,
    possible_china: possibleChina,
  }
  return renderToString(createSSRApp({ render: () => h(NodeList, { nodes: [node] }) }))
}

test('list shows one derived verdict, used traffic and no empty meter for unlimited nodes', async () => {
  const html = await renderNode(0, true)
  const traffic = html.slice(html.indexOf('<!-- 流量 -->'), html.indexOf('<!-- 速率 -->'))
  assert.match(html, /送中/)
  assert.doesNotMatch(html, /未送中/)
  assert.match(traffic, /3 B \/ ∞/)
  assert.doesNotMatch(traffic, /data-traffic-progress/)
  assert.doesNotMatch(traffic, /0\.0%/)
})

test('list shows neutral unknown verdict and utilization meter when limited', async () => {
  const html = await renderNode(10, null)
  assert.match(html, /未知/)
  assert.match(html, /30\.0%/)
  assert.match(html, /data-traffic-progress="true"/)
})

test('list keeps the reroute verdict visible in the offline row overlay', async () => {
  const html = await renderNode(0, false, false)
  assert.match(html, /离线/)
  assert.match(html, /未送中/)
})
