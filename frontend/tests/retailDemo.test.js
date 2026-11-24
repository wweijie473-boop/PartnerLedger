import { once } from 'node:events'
import { createServer } from 'node:http'
import { randomUUID } from 'node:crypto'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import assert from 'node:assert/strict'
import test from 'node:test'
import { retailDemo } from '../demo/retailServer.js'
import { pendingRetailOrder, readRetailCheckout } from '../src/live/retailRequest.js'

const checkoutOrder = link => new URL(link, 'http://localhost').searchParams.get('order')

test('订单重试沿用升级前的请求体和请求号', () => {
  const savedStorage = globalThis.sessionStorage
  const storage = new Map()
  globalThis.sessionStorage = {
    getItem: key => storage.get(key),
    setItem: (key, value) => storage.set(key, value),
  }
  try {
    const draft = { product_id: '9', quantity: 1, expected_unit_cents: 101, customer_id: '7' }
    const id = randomUUID()
    storage.set('legacy', JSON.stringify({ id, payload: JSON.stringify(draft) }))
    assert.deepEqual(readRetailCheckout('legacy'), { ...draft, request_id: id })
    assert.deepEqual(pendingRetailOrder('legacy', draft, { annual: true }), { ...draft, request_id: id })
    const order = pendingRetailOrder('new', draft, { annual: true })
    assert.equal(order.expected_annual, true)
    assert.deepEqual(pendingRetailOrder('new', draft, { annual: true }), order)
  } finally {
    if (savedStorage) globalThis.sessionStorage = savedStorage
    else delete globalThis.sessionStorage
  }
})

async function demoAPI(t, options = { samples: false }) {
  let handle
  const server = createServer((req, res) => handle(req, res, () => { res.writeHead(404); res.end() }))
  retailDemo(options).configureServer({ httpServer: server, middlewares: { use: middleware => { handle = middleware } } })
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  const close = () => new Promise(resolve => server.close(resolve))
  t.after(close)
  const origin = `http://127.0.0.1:${server.address().port}/api/retail-demo`
  async function api(path, { token, body, expect = 200 } = {}) {
    const headers = token ? { Authorization: `Bearer ${token}` } : {}
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    const response = await fetch(origin + path, { method: body === undefined ? 'GET' : 'POST', headers, body: JSON.stringify(body) })
    const result = await response.json()
    assert.equal(response.status, expect, result.message)
    return result.data
  }
  return Object.assign(api, { close })
}

test('扫码购买和代客下单共用厂商收款、代理商归因和结算记录', async t => {
  const api = await demoAPI(t)
  const first = await api('/sessions', { body: {} })
  const second = await api('/sessions', { body: {} })
  await api('/orders', { expect: 401 })
  await api('/promotion', { token: first.vendor, body: { manufacturer_id: '1' }, expect: 404 })
  const { code } = await api('/promotion', { token: first.dealer, body: { manufacturer_id: '1' } })
  const { items: [offer] } = await api('/store/' + code)
  assert.equal(offer.manufacturer, '畅捷通')
  const purchase = { product_id: offer.product_id, quantity: 1, expected_unit_cents: offer.unit_cents, request_id: randomUUID() }
  const path = '/store/' + code + '/orders'
  const buyer = { ...purchase, customer_name: '扫码测试企业' }
  const { checkout_path: receiptLink } = await api(path, { body: buyer })
  assert.deepEqual(await api(path, { body: buyer }), { checkout_path: receiptLink })
  await api(path, { body: { ...buyer, quantity: 2 }, expect: 409 })

  const { items: [row] } = await api('/orders?view=commission', { token: first.dealer })
  assert.equal(row.reseller, '天津助飞企业咨询管理有限公司')
  assert.equal(row.manufacturer, '畅捷通')
  assert.equal(row.source, 'promotion')
  assert.equal(row.status, 'settlement')
  assert.equal(row.commission.cents, offer.unit_cents / 2)
  assert.equal(row.commission.rate_bps, 5000)
  assert.deepEqual((await api('/orders', { token: second.vendor })).items, [])
  const receipt = await api('/customer-orders/' + checkoutOrder(receiptLink))
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
  const customerToken = checkoutOrder(unconfirmed.checkout_path)
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

test('客户、资金和工单按场次共享，财务办理与佣金分账', async t => {
  const api = await demoAPI(t)
  const context = await api('/sessions', { body: {} })
  const dealer = { token: context.dealer }
  const vendor = { token: context.vendor }
  const data = options => api('/operations', options)
  const action = (path, options, body, expect) => api('/operations/' + path, { ...options, body, expect })

  const customer = await action('customers', dealer, { name: '测试公司', contact: '测试联系人', note: '演示备注' })
  assert.ok((await api('/organization-links', dealer)).items.some(row => row.id === customer.id && row.name === customer.name))
  assert.equal((await data(vendor)).customers, undefined)
  assert.equal((await data({ token: (await api('/sessions', { body: {} })).dealer })).customers.length, 1)

  const cash = { reference: 'DEMO-BANK-1', amount_cents: 12500 }
  const topup = await action('topups', dealer, cash)
  assert.equal((await action('topups', dealer, cash)).id, topup.id)
  assert.equal((await data(dealer)).totals.topup_balance_cents, 0)
  await action(`topups/${topup.id}/complete`, dealer, { result_reference: 'DEMO-CREDIT-1' }, 403)
  const credited = await action(`topups/${topup.id}/complete`, vendor, { result_reference: 'DEMO-CREDIT-1' })
  assert.equal(credited.status, 'credited')
  assert.equal((await data(dealer)).totals.topup_balance_cents, 12500)

  const { items: [offer] } = await api('/offers', dealer)
  const order = await api('/orders', { ...dealer, body: { customer_id: customer.id, product_id: offer.product_id,
    expected_unit_cents: offer.unit_cents, quantity: 1, request_id: randomUUID() } })
  const invoice = { order_id: order.id, title: customer.name }
  await action('invoices', dealer, invoice, 409)
  const assisted = (await api('/orders', dealer)).items[0]
  const pay = '/customer-orders/' + checkoutOrder(assisted.checkout_path) + '/pay'
  await api(pay, { body: {} })
  await api(pay, { body: {} })
  const renewals = (await data(dealer)).renewals
  assert.equal(renewals.filter(row => row.order_id === order.id).length, 1)
  assert.equal((await data(dealer)).totals.renewals_30_days, 0)
  const followUp = await action('renewals/' + renewals[0].id, dealer, { note: '客户下周确认续费计划。' })
  assert.ok(followUp.contacted_at)
  assert.equal((await data(vendor)).renewals, undefined)
  const requested = await action('invoices', dealer, invoice)
  const issued = await action(`invoices/${requested.id}/complete`, vendor, { result_reference: 'DEMO-INVOICE-1' })
  assert.equal(issued.status, 'issued')

  const claim = await action('claims', dealer, { reference: 'DEMO-SERVICE-1', amount_cents: 30000 })
  await action('claims', dealer, { reference: 'DEMO-SERVICE-2', amount_cents: 100000 }, 409)
  assert.equal((await data(dealer)).totals.claimed_service_cents, 30000)
  await action(`claims/${claim.id}/complete`, vendor, { result_reference: 'DEMO-REIMBURSE-1' })
  const totals = (await data(dealer)).totals
  assert.equal(totals.paid_service_cents, 30000)
  assert.equal(totals.available_service_cents, 90000)
  assert.equal(totals.topup_balance_cents, 12500)
  assert.equal(totals.pending_commission_cents, offer.unit_cents / 2)

  const issue = await action('tickets', dealer, { request_id: randomUUID(), category: '授权与交付', order_id: order.id, subject: '核对授权', detail: '演示授权尚未到账' })
  await action(`tickets/${issue.id}/close`, dealer, {}, 409)
  await action(`tickets/${issue.id}/resolve`, vendor, { reply: '授权已经核对。' })
  await action(`tickets/${issue.id}/reopen`, dealer, { reply: '请再核对授权期限。' })
  await action(`tickets/${issue.id}/resolve`, vendor, { reply: '授权期限确认完成。' })
  await action(`tickets/${issue.id}/close`, dealer, {})
  const result = (await data(dealer)).tickets[0]
  assert.equal(result.status, 'closed')
  assert.equal(result.replies.length, 3)
})

test('重开演示数据库后保留样例、扫码和代客订单、购买链接及续费记录', async t => {
  const dir = mkdtempSync(join(tmpdir(), 'partnerledger-retail-'))
  const database = join(dir, 'retail.sqlite')
  const first = await demoAPI(t, { database })
  const session = await first('/sessions', { body: {} })
  const dealer = { token: session.dealer }
  const initial = await first('/operations', dealer)
  assert.equal(initial.customers.length, 6)
  assert.equal(initial.orders.length, 6)
  assert.equal(initial.totals.renewals_30_days, 3)
  const { code } = await first('/promotion', { ...dealer, body: { manufacturer_id: '1' } })
  const path = '/store/' + code + '/orders'
  const promoBody = { product_id: '1', quantity: 2, expected_unit_cents: 199800, request_id: randomUUID(), customer_name: '扫码验收企业' }
  const promo = await first(path, { body: promoBody })
  const promoReceipt = await first('/customer-orders/' + checkoutOrder(promo.checkout_path))
  const assisted = await first('/orders', { ...dealer, body: { product_id: '2', quantity: 1, expected_unit_cents: 399800,
    customer_id: '1', request_id: randomUUID() } })
  const pending = (await first('/orders?status=confirmation', dealer)).items[0]
  assert.equal(pending.id, assisted.id)
  await first('/operations/renewals/order-' + promoReceipt.id, { ...dealer, body: { note: '下月安排回访。' } })
  await first.close()

  const restarted = await demoAPI(t, { database })
  t.after(() => rmSync(dir, { recursive: true, force: true }))
  assert.equal((await restarted('/session', dealer)).role, 'dealer')
  assert.deepEqual(await restarted(path, { body: promoBody }), promo)
  const paid = await restarted('/customer-orders/' + checkoutOrder(pending.checkout_path) + '/pay', { body: {} })
  assert.ok(paid.paid_at)
  const restored = await restarted('/operations', dealer)
  assert.equal(restored.orders.length, 8)
  assert.equal(restored.customers.length, 7)
  assert.equal(restored.renewals.length, 8)
  const scanned = restored.renewals.find(row => row.order_id === promoReceipt.id)
  assert.equal(scanned.source, 'promotion')
  assert.equal(scanned.customer, promoBody.customer_name)
  assert.equal(scanned.quantity, 2)
  assert.equal(scanned.note, '下月安排回访。')
  assert.equal(restored.renewals.find(row => row.order_id === assisted.id).source, 'assisted')
})
