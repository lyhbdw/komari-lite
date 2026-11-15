import assert from 'node:assert/strict'
import test from 'node:test'
import { createPinia, setActivePinia } from 'pinia'
import { loadSource, memoryStorage } from './helpers.mjs'

const storage = memoryStorage({
  'komari_nodes_cache_v1': JSON.stringify([{ uuid: 'hidden-admin', ipv4: 'secret-ip' }]),
  'komari-theme-lite:node-ping-stats:hidden-admin:1': '{"stats":"private"}',
  'themeMode': 'dark',
  'nodeViewMode': 'list',
})
globalThis.localStorage = storage
globalThis.window = { localStorage: storage }
const { useNodesStore } = await loadSource('src/stores/nodes.ts')

test('family verdicts hydrate and refresh independently without replacing the node', () => {
  setActivePinia(createPinia())
  const store = useNodesStore()
  const client = { uuid: 'dual-stack', name: 'Dual stack', region: 'US', weight: 0, group: '', possible_china: true, possible_china_ipv4: false, possible_china_ipv6: true }
  store.initNodes({ [client.uuid]: client }, {})
  const original = store.nodes[0]
  assert.equal(original.possible_china_ipv4, false)
  assert.equal(original.possible_china_ipv6, true)
  store.updateNodeClients({ [client.uuid]: { ...client, possible_china_ipv4: null, possible_china_ipv6: false } })
  assert.equal(store.nodes[0], original)
  assert.equal(original.possible_china_ipv4, null)
  assert.equal(original.possible_china_ipv6, false)
  store.updateNodeClients({ [client.uuid]: { ...client, possible_china_ipv4: undefined, possible_china_ipv6: undefined } })
  assert.equal(original.possible_china_ipv4, null)
  assert.equal(original.possible_china_ipv6, null)
})

test('never hydrates privileged telemetry; migrates old data keys without deleting UI preferences', () => {
  setActivePinia(createPinia())
  const store = useNodesStore()
  assert.deepEqual(store.nodes, [])
  assert.equal(storage.getItem('komari_nodes_cache_v1'), null)
  assert.equal(storage.getItem('komari-theme-lite:node-ping-stats:hidden-admin:1'), null)
  assert.equal(storage.getItem('themeMode'), 'dark')
  assert.equal(storage.getItem('nodeViewMode'), 'list')
})
