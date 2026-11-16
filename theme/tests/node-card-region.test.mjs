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

const warnings = []
const root = resolve(dirname(new URL(import.meta.url).pathname), '..')
const directory = await mkdtemp(resolve(root, 'tests/.tmp-card-'))
const outfile = resolve(directory, 'card.mjs')
const passthrough = `import { h } from 'vue'; export default { inheritAttrs: false, setup(_, { slots, attrs }) { return () => h('span', attrs, slots.default?.()) } }`
const stubs = {
  '@/components/NodePingSummary.vue': 'export default { render: () => null }',
  '@/components/ui/badge': `${passthrough.replace('export default', 'const Badge =')}; export { Badge }`,
  '@/components/ui/card-x': `import { h } from 'vue'; export const CardX = { setup(_, { slots, attrs }) { return () => h('article', attrs, [h('header', [slots.header?.(), slots['header-extra']?.()]), h('main', slots.default?.())]) } }`,
  '@/components/ui/data-tooltip': `${passthrough.replace('export default', 'const DataTooltip =')}; export { DataTooltip }`,
  '@/components/ui/progress-thin': `import { h } from 'vue'; export const ProgressThin = { props: ['percentage'], render() { return h('div', { 'data-slot': 'progress', 'data-percentage': this.percentage }) } }`,
  '@/stores/app': 'export const useAppStore = () => ({ lang: "zh-CN" })',
  '@/composables/useNodeFormatters': 'export const useNodeFormatters = () => ({ formatBytes: () => "0 B", formatBytesPerSecond: () => "0 B/s", formatUptime: () => "1d" })',
}
let NodeCard
try {
  await build({
    entryPoints: [resolve(root, 'src/components/NodeCard.vue')],
    outfile,
    bundle: true,
    platform: 'node',
    format: 'esm',
    packages: 'external',
    alias: { '@': resolve(root, 'src') },
    plugins: [{ name: 'render-card', setup(builder) {
      builder.onResolve({ filter: /.*/ }, args => args.path in stubs ? { path: args.path, namespace: 'stub' } : undefined)
      builder.onLoad({ filter: /.*/, namespace: 'stub' }, args => ({ contents: stubs[args.path], loader: 'ts', resolveDir: root }))
      builder.onLoad({ filter: /\.vue$/ }, async (args) => {
        const source = await readFile(args.path, 'utf8')
        const { descriptor } = parse(source, { filename: args.path })
        const compiled = compileScript(descriptor, { id: 'card-test', genDefaultAs: '__component', inlineTemplate: true })
        return { contents: `${compiled.content}${String.fromCharCode(10)}export default __component`, loader: 'ts' }
      })
    } }],
  })
  NodeCard = (await import(pathToFileURL(outfile).href)).default
}
finally {
  await rm(directory, { recursive: true, force: true })
}

async function renderCard(overrides = {}, online = true) {
  warnings.length = 0
  const node = {
    uuid: 'node',
    name: 'Dual stack',
    online,
    time: '2025-01-01T00:00:00Z',
    tags: '',
    region: '',
    os: 'Linux',
    price: 0,
    billing_cycle: 30,
    currency: 'CNY',
    traffic_limit: 0,
    traffic_limit_type: 'sum',
    net_total_up: 0,
    net_total_down: 0,
    ...overrides,
  }
  const app = createSSRApp({ render: () => h(NodeCard, { node }) })
  app.config.warnHandler = message => warnings.push(message)
  return { html: await renderToString(app), warnings: [...warnings] }
}

test('card renders independent family verdicts once in the header and never in the lower area', async () => {
  for (const online of [true, false]) {
    const { html, warnings: renderWarnings } = await renderCard({ possible_china: true, possible_china_ipv4: false, possible_china_ipv6: true }, online)
    const header = html.match(/<header>(.*?)<\/header>/s)?.[1] ?? ''
    const lower = html.match(/<main>(.*?)<\/main>/s)?.[1] ?? ''
    assert.match(header, /IPv4：未送中/)
    assert.match(header, /IPv6：送中/)
    assert.equal((header.match(/IPv4：/g) ?? []).length, 1)
    assert.equal((header.match(/IPv6：/g) ?? []).length, 1)
    assert.doesNotMatch(lower, /送中|未检测|China reroute/)
    assert.equal(renderWarnings.some(warning => warning.includes('Failed to resolve component: Badge')), false)
  }
})

test('missing family verdicts stay untested and do not borrow the combined verdict', async () => {
  const { html } = await renderCard({ possible_china: true })
  assert.match(html, /IPv4：未检测/)
  assert.match(html, /IPv6：未检测/)
})

test('unlimited traffic displays usage against infinity without zero percent or a progress meter', async () => {
  const { html } = await renderCard({ traffic_limit: 0, net_total_up: 1024, net_total_down: 2048 })
  const traffic = html.slice(html.indexOf('data-metric="traffic"'), html.indexOf('<!-- 下半区'))
  assert.ok(traffic.includes('∞'))
  assert.ok(traffic.includes('/ <!--[--> ∞'))
  assert.equal(traffic.includes('0.0%'), false)
  assert.equal(traffic.includes('data-slot="progress"'), false)
})

test('limited traffic shows its measured utilization and progress meter', async () => {
  const { html } = await renderCard({ traffic_limit: 100, net_total_up: 25, net_total_down: 0 })
  const traffic = html.slice(html.indexOf('data-metric="traffic"'), html.indexOf('<!-- 下半区'))
  assert.ok(traffic.includes('25.0%'))
  assert.ok(traffic.includes('data-slot="progress" data-percentage="25"'))
})

test('offline card labels cached metrics with the last report time', async () => {
  const { html } = await renderCard({ possible_china_ipv6: true }, false)
  assert.match(html, /离线/)
  assert.match(html, /最后上报/)
  assert.match(html, /2025/)
  assert.match(html, /opacity-65 saturate-50/)
  assert.match(html, /IPv6：送中/)
})
