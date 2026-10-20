import assert from 'node:assert/strict'
import { resolve } from 'node:path'
// eslint-disable-next-line test/no-import-node-test -- Tests use the built-in Node runner.
import test from 'node:test'
import vue from '@vitejs/plugin-vue'
import { build, loadConfigFromFile } from 'vite'

// Use the actual Vite manualChunks policy and chart entry points. No files are
// written and the packaging plugin is omitted from this focused build.
const loaded = await loadConfigFromFile({ command: 'build', mode: 'production' }, resolve('vite.config.ts'))
assert.ok(loaded, 'theme Vite config must load')
const config = loaded.config

test('basic line charts do not statically import map and geo modules', async () => {
  const result = await build({
    configFile: false,
    logLevel: 'error',
    plugins: [vue()],
    resolve: config.resolve,
    define: config.define,
    build: {
      write: false,
      minify: false,
      reportCompressedSize: false,
      rollupOptions: {
        input: {
          line: resolve('src/utils/echarts.ts'),
          map: resolve('src/components/NodeEarthMaps.vue'),
        },
        output: config.build.rollupOptions.output,
        onwarn: config.build.rollupOptions.onwarn,
      },
    },
  })
  const chunks = new Map(result.output.filter(item => item.type === 'chunk').map(chunk => [chunk.fileName, chunk]))
  const line = [...chunks.values()].find(chunk => chunk.isEntry && chunk.name === 'line')
  assert.ok(line, 'line chart entry must exist')
  const visited = new Set()
  const modules = []
  function visit(file) {
    if (visited.has(file))
      return
    visited.add(file)
    const chunk = chunks.get(file)
    if (!chunk)
      return
    modules.push(...Object.keys(chunk.modules))
    chunk.imports.forEach(visit)
  }
  visit(line.fileName)
  const leaked = modules.filter(id => /echarts\/lib\/(?:chart\/(?:map|scatter)\/|component\/geo\/)/.test(id))
  assert.deepEqual(leaked, [], 'optional map/scatter/geo modules must remain outside the line-chart graph')
})
