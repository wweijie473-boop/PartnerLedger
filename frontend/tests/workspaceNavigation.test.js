import assert from 'node:assert/strict'
import test from 'node:test'
import { groupWorkspacePages, workspacePages, workspaceShortcuts } from '../src/workspaceNavigation.js'

test('each visible workspace page belongs to one group and shortcuts are available to the same role', () => {
  for (const role of ['dealer', 'vendor', 'customer']) {
    for (const options of [{}, { demo: true }, { demo: true, retailDemo: true }]) {
      const pages = workspacePages(role, options)
      const grouped = groupWorkspacePages(pages, role).flatMap(group => group.pages)
      assert.deepEqual(grouped.map(page => page.id).sort(), pages.map(page => page.id).sort())
      assert.equal(new Set(pages.map(page => page.id)).size, pages.length)
      assert.ok(pages.every(page => page.label))
      assert.ok(workspaceShortcuts(pages, role).every(page => grouped.includes(page)))
    }
  }
})

test('direct checkout and demo operations keep their original role and environment constraints', () => {
  const customerCheckout = workspacePages('customer', { directView: 'checkout' })
  assert.ok(customerCheckout.some(page => page.id === 'checkout'))
  assert.ok(groupWorkspacePages(customerCheckout, 'customer').every(group => group.pages.every(page => page.id !== 'checkout')))
  for (const role of ['dealer', 'vendor']) {
    assert.ok(workspacePages(role, { directView: 'checkout' }).every(page => page.id !== 'checkout'))
    assert.ok(workspacePages(role, { retailDemo: true }).every(page => page.source !== 'operations'))
    assert.ok(workspacePages(role, { demo: true }).every(page => !['retail', 'operations'].includes(page.source)))
  }
  assert.ok(workspacePages('dealer', { demo: true, retailDemo: true }).some(page => page.id === 'renewals'))
  assert.ok(workspacePages('vendor', { demo: true, retailDemo: true }).some(page => page.id === 'vendor-tickets'))
  assert.deepEqual(workspacePages('dealer').filter(page => page.source === 'dealer-operations').map(page => page.id),
    ['business-overview', 'customers', 'renewals'])
  assert.ok(workspacePages('vendor').every(page => page.source !== 'dealer-operations'))
})
