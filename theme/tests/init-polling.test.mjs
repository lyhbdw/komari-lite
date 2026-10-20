import assert from 'node:assert/strict'
// eslint-disable-next-line test/no-import-node-test -- Use the project's built-in Node runner.
import test from 'node:test'
import { loadSource, settle } from './helpers.mjs'

async function setup(t) {
  const calls = { ping: 0, clients: 0, status: 0 }
  const intervals = new Map()
  let nextInterval = 0
  let statusError = null
  let statusGate = null
  const clients = { 'node-a': { uuid: 'node-a', name: 'Node A' } }
  const statuses = { 'node-a': { online: true } }
  const app = {
    loading: true,
    connectionError: false,
    rpcTransportMode: 'http',
    updateLoginState() {},
  }
  const nodes = {
    nodes: [],
    wsReconnectAttempts: 0,
    initNodes(value) { this.nodes = Object.values(value) },
    updateWsState() {},
    updateNodeClients(value) { this.clients = value },
    updateNodeStatuses(value) { this.statuses = value },
    clearNodes() { this.nodes = [] },
  }
  const rpc = {
    async ping() {
      calls.ping++
      return 'pong'
    },
    async getNodes() {
      calls.clients++
      return clients
    },
    async getNodesLatestStatus() {
      calls.status++
      if (statusGate)
        await statusGate
      if (statusError)
        throw statusError
      return statuses
    },
    getClient: () => ({ setTransport() {} }),
    close() {},
  }
  const api = {
    getPublicSettings: async () => ({ theme_settings: { dataUpdateInterval: 3 } }),
    getMe: async () => ({ logged_in: false }),
  }
  t.mock.method(globalThis, 'setInterval', (callback, interval) => {
    const id = ++nextInterval
    intervals.set(id, { callback, interval })
    return id
  })
  t.mock.method(globalThis, 'clearInterval', id => intervals.delete(id))
  const savedDocument = globalThis.document
  globalThis.document = { hidden: false, addEventListener() {} }
  globalThis.__initPolling = { app, nodes, rpc, api }
  const manager = await loadSource('src/utils/init.ts', {
    stubs: {
      '@/stores/app': 'export const useAppStore = () => globalThis.__initPolling.app',
      '@/stores/nodes': 'export const useNodesStore = () => globalThis.__initPolling.nodes',
      '@/utils/api': 'export const getSharedApi = () => globalThis.__initPolling.api',
      '@/utils/rpc': `export const getSharedRpc = () => globalThis.__initPolling.rpc;
        export class RpcError extends Error { constructor(code, message) { super(message); this.code = code } }`,
      'vue-sonner': 'export const toast = { warning() {} }',
    },
  })
  t.after(() => {
    manager.destroyInitManager()
    assert.equal(intervals.size, 0)
    globalThis.document = savedDocument
    delete globalThis.__initPolling
  })
  await manager.initApp()
  assert.equal(intervals.size, 1)
  const timer = [...intervals.values()][0]
  assert.equal(timer.interval, 3000)
  return {
    app,
    nodes,
    calls,
    clients,
    statuses,
    tick: async () => {
      timer.callback()
      await settle()
    },
    setStatusError(error) { statusError = error },
    setStatusGate(gate) { statusGate = gate },
  }
}

test('startup pings once; periodic status requests reuse health signals and preserve metadata cadence', async (t) => {
  const f = await setup(t)
  assert.deepEqual(f.calls, { ping: 1, clients: 1, status: 1 })
  await f.tick()
  assert.deepEqual(f.calls, { ping: 1, clients: 2, status: 2 })
  assert.equal(f.nodes.clients, f.clients)
  assert.equal(f.nodes.statuses, f.statuses)
  for (let i = 0; i < 19; i++)
    await f.tick()
  assert.deepEqual(f.calls, { ping: 1, clients: 2, status: 21 })
  await f.tick()
  assert.deepEqual(f.calls, { ping: 1, clients: 3, status: 22 })
})

test('status failure sets connection error and the next successful poll clears it without a health ping', async (t) => {
  const f = await setup(t)
  t.mock.method(console, 'error', () => {})
  f.setStatusError(new Error('status unavailable'))
  await f.tick()
  assert.equal(f.app.connectionError, true)
  f.setStatusError(null)
  await f.tick()
  assert.equal(f.app.connectionError, false)
  assert.equal(f.calls.ping, 1)
  assert.equal(f.calls.status, 3)
})

test('polling does not overlap while a status request is pending', async (t) => {
  const f = await setup(t)
  let release
  f.setStatusGate(new Promise((resolve) => {
    release = resolve
  }))
  await f.tick()
  await f.tick()
  assert.equal(f.calls.status, 2)
  release()
  await settle()
  f.setStatusGate(null)
  await f.tick()
  assert.equal(f.calls.status, 3)
  assert.equal(f.calls.ping, 1)
})
