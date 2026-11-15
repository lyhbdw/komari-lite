import assert from 'node:assert/strict'
import test from 'node:test'
import { readFile } from 'node:fs/promises'

const locale = JSON.parse(await readFile(new URL('../src/i18n/locales/zh_CN.json', import.meta.url), 'utf8'))

test('upgrade confirmation includes exactly one v before the version', () => {
  const template = locale.admin.nodeTable.upgradeSuccess
  const version = 'v1.1.10'
  const message = template.replace('{{version}}', version).replace('{{dispatched}}', '23')
  assert.equal(message, '升级事件已下发（v1.1.10，23 个节点）')
  assert.doesNotMatch(message, /vv1\.1\.10/)
})
