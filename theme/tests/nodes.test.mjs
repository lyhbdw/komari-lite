import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { loadSource, memoryStorage } from './helpers.mjs'

const storage = memoryStorage({
  komari_nodes_cache_v1: JSON.stringify([{ uuid: 'hidden-admin', ipv4: 'secret-ip' }]),
  'komari-theme-lite:node-ping-stats:hidden-admin:1': '{"stats":"private"}',
  themeMode: 'dark',
  nodeViewMode: 'list',
})
globalThis.localStorage = storage
globalThis.window = { localStorage: storage }
const { useNodesStore } = await loadSource('src/stores/nodes.ts')

test('never hydrates privileged telemetry; migrates old data keys without deleting UI preferences', () => {
  setActivePinia(createPinia())
  const store = useNodesStore()
  assert.deepEqual(store.nodes, [])
  assert.equal(storage.getItem('komari_nodes_cache_v1'), null)
  assert.equal(storage.getItem('komari-theme-lite:node-ping-stats:hidden-admin:1'), null)
  assert.equal(storage.getItem('themeMode'), 'dark')
  assert.equal(storage.getItem('nodeViewMode'), 'list')
})
