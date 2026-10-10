import { build } from 'esbuild'
import { compileScript, parse } from '@vue/compiler-sfc'
import { readFile, mkdtemp, rm } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { createRenderer, nextTick } from 'vue'

const root = resolve(dirname(new URL(import.meta.url).pathname), '..')

// Test-only transpilation uses the existing Vite/TypeScript toolchain. Tests stay
// outside src; no test runtime or node-only imports enter the application build.
export async function loadSource(path, { stubs = {} } = {}) {
  const directory = await mkdtemp(resolve(root, 'tests/.tmp-'))
  const outfile = resolve(directory, 'module.mjs')
  await build({
    entryPoints: [resolve(root, path)],
    outfile,
    bundle: true,
    platform: 'node',
    format: 'esm',
    packages: 'external',
    alias: { '@': resolve(root, 'src') },
    define: { 'import.meta.env': JSON.stringify({}) },
    plugins: [{
      name: 'vue-setup-test',
      setup(builder) {
        builder.onResolve({ filter: /.*/ }, (args) => {
          if (args.path in stubs)
            return { path: args.path, namespace: 'test-stub' }
        })
        builder.onLoad({ filter: /.*/, namespace: 'test-stub' }, args => ({ contents: stubs[args.path], loader: 'ts' }))
        builder.onLoad({ filter: /\.vue$/ }, async (args) => {
          const source = await readFile(args.path, 'utf8')
          const { descriptor } = parse(source, { filename: args.path })
          const compiled = compileScript(descriptor, { id: 'test', genDefaultAs: '__component' })
          return { contents: `${compiled.content}\n__component.render = () => null; export default __component;`, loader: 'ts' }
        })
      },
    }],
  })
  const loaded = await import(pathToFileURL(outfile).href)
  await rm(directory, { recursive: true, force: true })
  return loaded
}

export function memoryStorage(entries = {}) {
  const data = new Map(Object.entries(entries))
  const writes = []
  return {
    get length() { return data.size },
    key: index => [...data.keys()][index] ?? null,
    getItem: key => data.get(key) ?? null,
    setItem: (key, value) => { writes.push([key, value]); data.set(key, value) },
    removeItem: key => data.delete(key),
    writes,
  }
}

export const renderer = createRenderer({
  createElement: type => ({ type, children: [] }),
  createText: text => ({ text }),
  createComment: text => ({ text }),
  setText: (node, text) => { node.text = text },
  setElementText: (node, text) => { node.text = text },
  parentNode: node => node.parent ?? null,
  nextSibling: () => null,
  insert: (node, parent) => { node.parent = parent; parent.children.push(node) },
  remove: (node) => { if (node.parent) node.parent.children = node.parent.children.filter(child => child !== node) },
  patchProp: () => {},
})

export function mountSetup(setup, plugins = []) {
  const app = renderer.createApp({ setup, render: () => null })
  plugins.forEach(plugin => app.use(plugin))
  app.mount({ children: [] })
  return app
}

export async function settle() {
  await nextTick()
  await new Promise(resolve => setImmediate(resolve))
  await nextTick()
}
