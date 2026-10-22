import assert from 'node:assert/strict'
// eslint-disable-next-line test/no-import-node-test -- Built-in Node test runner.
import test from 'node:test'
import { loadSource } from './helpers.mjs'

globalThis.window = { location: { origin: 'http://localhost', protocol: 'http:', host: 'localhost' } }
const { KomariRpc } = await loadSource('src/utils/rpc.ts')
const tick = () => new Promise(resolve => setImmediate(resolve))
test('real SDK HTTP history methods coordinate across clients, busy retries do not retry business failures', async () => {
  const saved = globalThis.fetch
  let active = 0
  let peak = 0
  const calls = []
  const attempts = new Map()
  globalThis.fetch = async (_url, options) => {
    const request = JSON.parse(options.body)
    calls.push(request)
    if (request.method === 'public:getPublicPingTasks')
      return Response.json({ jsonrpc: '2.0', id: request.id, result: [] })
    active++
    peak = Math.max(peak, active)
    await tick()
    active--
    const count = (attempts.get(request.method) ?? 0) + 1
    attempts.set(request.method, count)
    if (request.method === 'public:getPingMetricStats' && count === 1)
      return Response.json({ jsonrpc: '2.0', id: request.id, error: { code: -32051, message: 'too many concurrent history queries' } })
    return Response.json({ jsonrpc: '2.0', id: request.id, result: { method: request.method } })
  }
  try {
    const first = new KomariRpc().getClient()
    const second = new KomariRpc().getClient()
    const results = await Promise.all([first.call('public:queryMetrics'), first.call('public:getPingMetricStats'), second.call('common:getRecords'), first.call('public:getPublicPingTasks')])
    assert.equal(results.length, 4)
    assert.equal(peak, 1)
    assert.equal(attempts.get('public:getPingMetricStats'), 2)
    assert.equal(calls.filter(r => r.method === 'public:queryMetrics').length, 1)
    let errors = 0
    globalThis.fetch = async () => {
      errors++
      return Response.json({ jsonrpc: '2.0', error: { code: -32602, message: 'bad entity' } })
    }
    await assert.rejects(first.call('public:queryMetrics'), /bad entity/)
    assert.equal(errors, 1)
  }
  finally { globalThis.fetch = saved }
})
test('superseded chart windows cancel queued work before any HTTP send', async () => {
  const saved = globalThis.fetch
  let release
  const requests = []
  globalThis.fetch = async (_url, options) => {
    const request = JSON.parse(options.body)
    requests.push(request)
    if (request.params?.hours === 1)
      await new Promise((r) => { release = r })
    return Response.json({ jsonrpc: '2.0', id: request.id, result: {} })
  }
  try {
    const client = new KomariRpc().getClient()
    const first = client.call('public:queryMetrics', { hours: 1 })
    await tick()
    const cancelled = []
    for (let hours = 2; hours <= 20; hours++) {
      const controller = new AbortController()
      cancelled.push(client.call('public:queryMetrics', { hours }, controller.signal).catch(e => e.name))
      controller.abort()
    }
    const latest = client.call('public:queryMetrics', { hours: 21 })
    release()
    await first
    await latest
    assert.ok((await Promise.all(cancelled)).every(name => name === 'AbortError'))
    assert.deepEqual(requests.map(r => r.params.hours), [1, 21])
  }
  finally { globalThis.fetch = saved }
})
