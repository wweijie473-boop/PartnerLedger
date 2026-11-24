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

test('search includes live workspace pages and prioritizes the signed-in role', () => {
  assert.equal(searchPortal('资金总览')[0].href, '/login/dealer?view=funds-overview')
  assert.ok(searchPortal('工单').some(result => result.href === '/login/dealer?view=service-tickets'))
  assert.equal(searchPortal('工单', 'vendor')[0].href, '/login/vendor?view=vendor-tickets')
  assert.ok(!searchPortal('工单', 'customer').some(result => result.href.startsWith('/login/vendor')))
  assert.equal(searchPortal('拜访', 'dealer')[0].href, '/login/dealer?view=visits')
})
