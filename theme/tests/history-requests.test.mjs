import assert from 'node:assert/strict'
// eslint-disable-next-line test/no-import-node-test -- Built-in Node test runner.
import test from 'node:test'
import { loadSource } from './helpers.mjs'

const { scheduleHistory } = await loadSource('src/utils/historyRequests.ts')
const tick = () => new Promise(resolve => setImmediate(resolve))
test('history callers share one slot, cancelled queued work never starts and queue is bounded', async () => {
  let release
  let active = 0
  let peak = 0
  const first = scheduleHistory(async () => {
    active++
    peak = Math.max(peak, active)
    await new Promise((r) => {
      release = r
    })
    active--
    return 1
  })
  await tick()
  const controller = new AbortController()
  let cancelledRan = false
  const cancelled = scheduleHistory(async () => {
    cancelledRan = true
  }, controller.signal).catch(e => e.name)
  controller.abort()
  assert.equal(await cancelled, 'AbortError')
  const pending = []
  for (let i = 0; i < 8; i++) {
    pending.push(scheduleHistory(async () => {
      active++
      peak = Math.max(peak, active)
      await tick()
      active--
      return 2
    }))
  }
  await assert.rejects(scheduleHistory(async () => 3), /queue is full/)
  release()
  assert.equal(await first, 1)
  assert.deepEqual(await Promise.all(pending), Array.from({ length: 8 }).fill(2))
  assert.equal(peak, 1)
  assert.equal(cancelledRan, false)
})
test('only server history busy is retried, at most twice', async () => {
  let attempts = 0
  const busy = { code: -32051, message: 'too many concurrent history queries' }
  assert.equal(await scheduleHistory(async () => {
    if (++attempts < 3)
      throw busy
    return 7
  }), 7)
  assert.equal(attempts, 3)
  attempts = 0
  await assert.rejects(scheduleHistory(async () => {
    attempts++
    throw new Error('permission denied')
  }), /permission denied/)
  assert.equal(attempts, 1)
  attempts = 0
  await assert.rejects(scheduleHistory(async () => {
    attempts++
    throw busy
  }), e => e === busy)
  assert.equal(attempts, 3)
  attempts = 0
  await assert.rejects(scheduleHistory(async () => {
    attempts++
    throw Object.assign(new Error('metric store unavailable'), { code: -32051 })
  }), e => e.message === 'metric store unavailable')
  assert.equal(attempts, 1)
})
test('abort during backoff prevents further attempts', async () => {
  const controller = new AbortController()
  let attempts = 0
  await assert.rejects(scheduleHistory(async () => {
    attempts++
    setTimeout(() => controller.abort(), 5)
    throw Object.assign(new Error('too many concurrent history queries'), { code: -32051 })
  }, controller.signal), e => e.name === 'AbortError')
  assert.equal(attempts, 1)
})
