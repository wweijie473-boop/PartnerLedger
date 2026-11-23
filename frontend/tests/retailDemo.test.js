import { once } from 'node:events'
import { createServer } from 'node:http'
import { randomUUID } from 'node:crypto'
import assert from 'node:assert/strict'
import test from 'node:test'
import { retailDemo } from '../demo/retailServer.js'

test('扫码购买和代客下单共用厂商收款、代理商归因和结算记录', async t => {
  let handle
  retailDemo().configureServer({ middlewares: { use: middleware => { handle = middleware } } })
  const server = createServer((req, res) => handle(req, res, () => { res.writeHead(404); res.end() }))
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  t.after(() => new Promise(resolve => server.close(resolve)))
  const origin = `http://127.0.0.1:${server.address().port}/api/retail-demo`
  async function api(path, { token, body, expect = 200 } = {}) {
    const headers = token ? { Authorization: `Bearer ${token}` } : {}
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    const response = await fetch(origin + path, { method: body === undefined ? 'GET' : 'POST', headers, body: JSON.stringify(body) })
    const result = await response.json()
    assert.equal(response.status, expect, result.message)
    return result.data
  }

  const first = await api('/sessions', { body: {} })
  const second = await api('/sessions', { body: {} })
  await api('/orders', { expect: 401 })
  await api('/promotion', { token: first.vendor, body: { manufacturer_id: '1' }, expect: 404 })
  const { code } = await api('/promotion', { token: first.dealer, body: { manufacturer_id: '1' } })
  const { items: [offer] } = await api('/store/' + code)
  const purchase = { product_id: offer.product_id, quantity: 1, expected_unit_cents: offer.unit_cents, request_id: randomUUID() }
  const path = '/store/' + code + '/orders'
  const { checkout_path: receiptLink } = await api(path, { body: purchase })
  assert.deepEqual(await api(path, { body: purchase }), { checkout_path: receiptLink })
  await api(path, { body: { ...purchase, quantity: 2 }, expect: 409 })

  const { items: [row] } = await api('/orders?view=commission', { token: first.dealer })
  assert.equal(row.source, 'promotion')
  assert.equal(row.status, 'settlement')
  assert.equal(row.commission.cents, offer.unit_cents / 2)
  assert.equal(row.commission.rate_bps, 5000)
  assert.deepEqual((await api('/orders', { token: second.vendor })).items, [])
  const receipt = await api('/customer-orders/' + new URL(receiptLink, origin).searchParams.get('order'))
  assert.ok(receipt.paid_at)
  assert.equal(receipt.commission, undefined)
  await api('/orders/' + row.id + '/remind', { token: first.dealer, body: {} })
  await api('/orders/' + row.id + '/remind', { token: first.dealer, body: {} })
  const vendorOrder = (await api('/orders', { token: first.vendor })).items[0]
  assert.equal(vendorOrder.total_cents, offer.unit_cents)
  assert.equal(vendorOrder.commission.reminder_count, 1)
  assert.ok(vendorOrder.receipt_ref)
  const settle = '/orders/' + row.id + '/settle'
  await api(settle, { token: first.dealer, body: { reference: 'MOCK-1' }, expect: 409 })
  await api(settle, { token: first.vendor, body: { reference: 'MOCK-1' } })
  await api(settle, { token: first.vendor, body: { reference: 'MOCK-1' } })
  const settled = (await api('/orders?status=settled', { token: first.dealer })).items[0]
  assert.equal(settled.payout_ref, 'MOCK-1')

  const assisted = { ...purchase, request_id: randomUUID(), customer_id: '1' }
  const { id } = await api('/orders', { token: first.dealer, body: assisted })
  const unconfirmed = (await api('/orders?status=confirmation', { token: first.dealer })).items[0]
  assert.equal(unconfirmed.id, id)
  assert.equal((await api('/orders?view=commission', { token: first.dealer })).items.length, 1)
  const vendorUnconfirmed = (await api('/orders?status=confirmation', { token: first.vendor })).items[0]
  assert.equal(vendorUnconfirmed.checkout_path, undefined)
  const customerToken = new URL(unconfirmed.checkout_path, origin).searchParams.get('order')
  const customerPath = '/customer-orders/' + customerToken
  assert.equal((await api(customerPath)).paid_at, undefined)
  const paid = await api(customerPath + '/pay', { body: {} })
  assert.ok(paid.paid_at)
  assert.deepEqual(await api(customerPath + '/pay', { body: {} }), paid)
  const pending = (await api('/orders?view=commission&status=settlement', { token: first.vendor })).items[0]
  assert.equal(pending.id, id)
  assert.equal(pending.source, 'assisted')
  assert.equal(pending.total_cents, pending.commission.cents * 2)
  const filtered = await api('/orders?view=commission&status=settled', { token: first.vendor })
  assert.equal(filtered.items.length, 1)
  assert.deepEqual(filtered.summary, { settlement_cents: pending.commission.cents, settled_cents: settled.commission.cents, reminded: 0 })
})
