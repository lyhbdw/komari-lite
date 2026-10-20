import assert from 'node:assert/strict'
// eslint-disable-next-line test/no-import-node-test -- Use the project's built-in Node test runner.
import test from 'node:test'
import { loadSource } from './helpers.mjs'

const { buildRetentionOptions } = await loadSource('src/utils/chartRetention.ts')

const loadPresets = [
  { label: '1 小时', hours: 1 },
  { label: '6 小时', hours: 6 },
  { label: '1 天', hours: 24 },
  { label: '7 天', hours: 168 },
  { label: '30 天', hours: 720 },
  { label: '60 天', hours: 1440 },
  { label: '90 天', hours: 2160 },
]
const pingPresets = [
  ...loadPresets.slice(0, 2),
  { label: '12 小时', hours: 12 },
  ...loadPresets.slice(2),
]

for (const [name, presets] of [['load', loadPresets], ['ping', pingPresets]]) {
  for (const hours of [-Infinity, -1, 0, 0.5, Number.NaN]) {
    test(`${name}: ${hours} hours produces no historical options`, () => {
      assert.deepEqual(buildRetentionOptions(presets, hours), [])
    })
  }

  for (const [index, preset] of presets.entries()) {
    test(`${name}: matching ${preset.hours} hours includes presets without duplication`, () => {
      assert.deepEqual(buildRetentionOptions(presets, preset.hours), presets.slice(0, index + 1))
    })
  }

  for (const [hours, label, count] of [
    [1.5, '1.5 小时', 1],
    [8, '8 小时', 2],
    [18, '18 小时', name === 'ping' ? 3 : 2],
    [48, '2 天', name === 'ping' ? 4 : 3],
    [1441, '1441 小时', presets.length - 1],
    [2161, '2161 小时', presets.length],
    [2400, '100 天', presets.length],
    [Infinity, 'Infinity 小时', presets.length],
  ]) {
    test(`${name}: nonmatching ${hours} hours appends the exact retention label`, () => {
      assert.deepEqual(buildRetentionOptions(presets, hours), [
        ...presets.slice(0, count),
        { label, hours },
      ])
    })
  }
}

test('retains supplied order and labels without mutating presets', () => {
  const presets = Object.freeze([
    Object.freeze({ label: 'six', hours: 6 }),
    Object.freeze({ label: 'one', hours: 1 }),
  ])
  assert.deepEqual(buildRetentionOptions(presets, 6), [
    { label: 'six', hours: 6 },
    { label: 'one', hours: 1 },
    { label: '6 小时', hours: 6 },
  ])
  assert.deepEqual(presets, [{ label: 'six', hours: 6 }, { label: 'one', hours: 1 }])
  assert.notEqual(buildRetentionOptions(presets, 6), buildRetentionOptions(presets, 6))
})

test('below-first retention still appends a custom option only above one hour', () => {
  const presets = [{ label: '6 小时', hours: 6 }]
  assert.deepEqual(buildRetentionOptions(presets, 0.5), [])
  assert.deepEqual(buildRetentionOptions(presets, 1), [])
  assert.deepEqual(buildRetentionOptions(presets, 2), [{ label: '2 小时', hours: 2 }])
})

test('empty presets preserve the custom-option threshold and nonfinite behavior', () => {
  for (const hours of [-Infinity, Number.NaN, 0, 1])
    assert.deepEqual(buildRetentionOptions([], hours), [])
  assert.deepEqual(buildRetentionOptions([], 2), [{ label: '2 小时', hours: 2 }])
  assert.deepEqual(buildRetentionOptions([], 48), [{ label: '2 天', hours: 48 }])
  assert.deepEqual(buildRetentionOptions([], Infinity), [{ label: 'Infinity 小时', hours: Infinity }])
})
