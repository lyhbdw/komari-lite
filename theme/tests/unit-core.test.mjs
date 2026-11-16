import assert from 'node:assert/strict'
import test from 'node:test'
import { loadSource } from './helpers.mjs'

const { formatBytes, formatBytesSplit } = await loadSource('src/utils/unitCore.ts')

test('byte units sit on the correct side of 1024 power boundaries', () => {
  // 回归：1 GiB 曾被 Math.log 浮点误差降级显示为 1024.0 MB
  assert.equal(formatBytes(1024 ** 3), '1.0 GB')
  assert.equal(formatBytes(1024 ** 4), '1.00 TB') // TB 默认 2 位小数
  assert.equal(formatBytes(1024 ** 2), '1.0 MB')
  assert.deepEqual(formatBytesSplit(1024 ** 3), { value: '1.0', unit: 'GB' })
  assert.deepEqual(formatBytesSplit(1024 ** 4), { value: '1.00', unit: 'TB' })
})

test('values that round up to the next unit are promoted instead of showing 1024.0', () => {
  // 线上真实案例：某节点 swap_total = 1073737728 B（比 1 GiB 差 4096 B），
  // 旧行为显示 1024.0 MB，与同页其他 GB 项不统一；现在升档为 1.0 GB
  assert.equal(formatBytes(1073737728), '1.0 GB')
  assert.equal(formatBytes(1024 ** 3 - 1), '1.0 GB')
  assert.equal(formatBytes(1024 ** 3 - 1, 0), '1 GB')
  assert.deepEqual(formatBytesSplit(1024 ** 3 - 1), { value: '1.0', unit: 'GB' })
  // 未达到升档条件的值保持原单位
  assert.equal(formatBytes(1023.5 * 1024 * 1024), '1023.5 MB')
})

test('byte formatting keeps normal values, zero and negatives stable', () => {
  assert.equal(formatBytes(1536), '2 KB') // KB 默认 0 位小数
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(-5), '0 B')
  assert.equal(formatBytes(1024 ** 3, 2), '1.00 GB')
  assert.equal(formatBytes(2.5 * 1024 ** 3), '2.5 GB')
})
