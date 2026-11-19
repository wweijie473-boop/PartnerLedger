import test from 'node:test'
import assert from 'node:assert/strict'
import { searchPortal } from '../src/portal-navigation.js'

test('search matches feature names, aliases and combined terms', () => {
  assert.equal(searchPortal('表格')[0].href, '/tables')
  assert.equal(searchPortal('Ｅｘｃｅｌ 查找')[0].href, '/tables')
  assert.equal(searchPortal('拜访 草稿')[0].href, '/experience/dealer?view=visits')
  assert.equal(searchPortal('ERP 询价')[0].href, '#journey')
  assert.deepEqual(searchPortal('a nonexistent feature'), [])
})

test('search points to help for login guidance', () => {
  assert.equal(searchPortal('登录')[0].href, '#help')
  assert.ok(searchPortal('').length > 0)
})
