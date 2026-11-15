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
const directory = await mkdtemp(resolve(root, 'tests/.tmp-card-'))
const outfile = resolve(directory, 'card.mjs')
const passthrough = `import { h } from 'vue'; export default { inheritAttrs: false, setup(_, { slots, attrs }) { return () => h('span', attrs, slots.default?.()) } }`
const stubs = {
  '@/components/NodePingSummary.vue': 'export default { render: () => null }',
  '@/components/ui/card-x': `import { h } from 'vue'; export const CardX = { setup(_, { slots }) { return () => h('article', [h('header', [slots.header?.(), slots['header-extra']?.()]), h('main', slots.default?.())]) } }`,
  '@/components/ui/data-tooltip': `${passthrough.replace('export default', 'const DataTooltip =')}; export { DataTooltip }`,
  '@/components/ui/progress-thin': 'export const ProgressThin = { render: () => null }',
  '@/components/ui/badge': `${passthrough.replace('export default', 'const Badge =')}; export { Badge }`,
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
      builder.onResolve({ filter: /^@\/components\/ui\/badge$/ }, args => ({ path: args.path, namespace: 'stub' }))
      builder.onResolve({ filter: /.*/ }, args => args.path in stubs ? { path: args.path, namespace: 'stub' } : undefined)
      builder.onLoad({ filter: /.*/, namespace: 'stub' }, args => ({ contents: stubs[args.path], loader: 'ts', resolveDir: root }))
      builder.onLoad({ filter: /\.vue$/ }, async (args) => {
        const source = await readFile(args.path, 'utf8')
        const { descriptor } = parse(source, { filename: args.path })
        const compiled = compileScript(descriptor, { id: 'card-test', genDefaultAs: '__component', inlineTemplate: true })
        return { contents: `${compiled.content}\nexport default __component`, loader: 'ts' }
      })
    } }],
  })
  NodeCard = (await import(pathToFileURL(outfile).href)).default
}
finally {
  await rm(directory, { recursive: true, force: true })
}

async function renderCard(verdicts = {}, online = true) {
  const node = { uuid: 'node', name: 'Dual stack', online, tags: '', region: '', os: 'Linux', price: 0, traffic_limit: 0, ...verdicts }
  const app = createSSRApp({ render: () => h(NodeCard, { node }) })
  app.component('Badge', {
    inheritAttrs: false,
    setup(_, { slots, attrs }) {
      return () => h('span', attrs, slots.default?.())
    },
  })
  return renderToString(app)
}

test('card renders independent family verdicts once in the header and never in the lower area', async () => {
  for (const online of [true, false]) {
    const html = await renderCard({ possible_china: true, possible_china_ipv4: false, possible_china_ipv6: true }, online)
    const header = html.match(/<header>(.*?)<\/header>/s)?.[1] ?? ''
    const lower = html.match(/<main>(.*?)<\/main>/s)?.[1] ?? ''
    assert.match(header, /IPv4：未送中/)
    assert.match(header, /IPv6：送中/)
    assert.equal((header.match(/IPv4：/g) ?? []).length, 1)
    assert.equal((header.match(/IPv6：/g) ?? []).length, 1)
    assert.doesNotMatch(lower, /送中|未检测|China reroute/)
  }
})

test('missing family verdicts stay untested and do not borrow the combined verdict', async () => {
  const html = await renderCard({ possible_china: true })
  assert.match(html, /IPv4：未检测/)
  assert.match(html, /IPv6：未检测/)
})
