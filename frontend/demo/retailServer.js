import { randomBytes } from 'node:crypto'
import { demoCompanies } from '../src/demoCompanies.js'
import { createDealerOperations, routeDealerOperations, trackAnnualOrder } from './dealerOperations.js'
import { openRetailDatabase } from './retailDatabase.js'

const prefix = '/api/retail-demo/'
const { vendor: maker, dealer } = demoCompanies
const offers = [
  { product_id: '1', name: '云财务', version: '标准版 · 年度', unit_cents: 199800, category: '财务管理', summary: '发票、凭证和月度财务报表。' },
  { product_id: '2', name: '进销存', version: '专业版 · 年度', unit_cents: 399800, category: '经营管理', summary: '采购、销售、库存和往来账款。' },
  { product_id: '3', name: '云 ERP', version: '企业版 · 年度', unit_cents: 699800, category: '业财一体', summary: '连接销售、库存与财务，适合多个部门协同。' },
].map(offer => ({ ...offer, manufacturer_id: '1', manufacturer: maker }))

const token = () => randomBytes(16).toString('hex')
const lifetime = 24 * 60 * 60 * 1000
const page = items => ({ items, next_cursor: null })
const secretFormat = /^[a-f\d]{32}$/
const requestFormat = /^[a-f\d]{8}(?:-[a-f\d]{4}){3}-[a-f\d]{12}$/
const customerPath = secret => '/experience/retail-store?order=' + secret

function fail(status, message) {
  throw Object.assign(new Error(message), { status })
}

async function readBody(req) {
  if (req.headers['content-type']?.split(';')[0] !== 'application/json') fail(415, '请求必须使用 JSON。')
  const chunks = []
  let size = 0
  for await (const chunk of req) {
    size += chunk.length
    if (size > 4096) fail(413, '演示请求过长。')
    chunks.push(chunk)
  }
  let body
  try { body = JSON.parse(Buffer.concat(chunks).toString()) } catch { fail(400, '演示请求格式有误。') }
  if (!body || typeof body !== 'object' || Array.isArray(body)) fail(400, '演示请求格式有误。')
  return body
}

function publicOrder(row) {
  return {
    id: row.id, manufacturer: row.manufacturer, product_name: row.product_name, product_version: row.product_version,
    quantity: row.quantity, total_cents: row.total_cents, customer: row.customer, paid_at: row.paid_at,
  }
}

function orderRow(id, offer, quantity, customer, source) {
  const total = offer.unit_cents * quantity
  return {
    id, source, manufacturer: maker, reseller: dealer, customer: customer.name, customer_id: customer.id,
    product_name: offer.name, product_version: offer.version, quantity, total_cents: total,
    status: 'confirmation', commission: { rate_bps: 5000, cents: total / 2, reminder_count: 0 },
  }
}

function seedOrders(session) {
  const samples = [
    ['海河商贸', 0, 4, 'promotion'], ['津湾精密', 2, 18, 'assisted'], ['北辰仓配', 1, 28, 'promotion'],
    ['远川设备', 2, 46, 'assisted'], ['澄海办公', 0, 78, 'promotion'], ['启航零售', 1, -8, 'assisted'],
  ]
  for (const [index, [name, product, days, source]] of samples.entries()) {
    const customer = { id: String(index + 1), name: name + '（演示客户）', contact: '', note: '' }
    if (index === 0) Object.assign(session.operations.customers[0], customer, { contact: '王经理（演示）' })
    else session.operations.customers.push(customer)
    const id = 'S' + String(index + 1).padStart(3, '0')
    const order = orderRow(id, offers[product], 1, customer, source)
    const paid = new Date(Date.now() + days * lifetime)
    paid.setUTCFullYear(paid.getUTCFullYear() - 1)
    markPaid(session, order, paid)
    order.status = 'settled'
    order.payout_ref = 'DEMO-OUT-' + id
    order.commission.settled_at = paid.toISOString()
    session.orders.push(order)
  }
}

function recordOrder(session, body, source) {
  const { product_id, quantity, expected_unit_cents, request_id } = body
  const offer = offers.find(item => item.product_id === product_id)
  let customer = session.operations.customers.find(item => item.id === body.customer_id)
  const customerName = typeof body.customer_name === 'string' ? body.customer_name.trim() : ''
  if (!offer || !Number.isInteger(quantity) || quantity < 1 || quantity > 10000 ||
      typeof request_id !== 'string' || !requestFormat.test(request_id) ||
      (source === 'assisted' ? !customer : body.customer_id != null || !customerName || Array.from(customerName).length > 80)) {
    fail(400, '请核对订购产品、数量和客户。')
  }
  if (offer.unit_cents !== expected_unit_cents) fail(409, '产品价格已变化，请刷新后重试。')
  const key = source + ':' + request_id
  const fingerprint = JSON.stringify([product_id, quantity, expected_unit_cents, source === 'assisted' ? customer.id : customerName])
  const prior = session.requests[key]
  if (prior) {
    if (prior.fingerprint !== fingerprint) fail(409, '请求已用于另一笔订单。')
    return session.orders.find(row => row.id === prior.orderID)
  }
  if (session.orders.length >= 200) fail(409, '该演示已达到订单上限，请开启新一轮演示。')
  if (source === 'promotion') {
    const customers = session.operations.customers
    customer = customers.find(item => item.name.toLowerCase() === customerName.toLowerCase())
    if (!customer) {
      if (customers.length >= 200) fail(409, '演示客户已达到上限。')
      customer = { id: token(), name: customerName, contact: '', note: '' }
      customers.unshift(customer)
    }
  }
  const row = orderRow(String(++session.orderSequence).padStart(4, '0'), offer, quantity, customer, source)
  if (source === 'promotion') markPaid(session, row)
  session.orders.unshift(row)
  session.requests[key] = { fingerprint, orderID: row.id }
  return row
}

function markPaid(session, row, paid = new Date()) {
  row.status = 'settlement'
  row.paid_at = paid.toISOString()
  row.receipt_ref = 'DEMO-IN-' + row.id
  trackAnnualOrder(session.operations, row)
}

export function retailDemo({ database = ':memory:', samples = true } = {}) {

  function issueCustomerLink(db, session, row) {
    if (row.checkout_path) return row.checkout_path
    const secret = token()
    db.addCheckout(secret, session, row)
    row.checkout_path = customerPath(secret)
    return row.checkout_path
  }

  function start(db) {
    const session = { code: token(), dealerKey: token(), vendorKey: token(), promotionActive: false,
      orders: [], orderSequence: 0, requests: {}, operations: createDealerOperations() }
    if (samples) seedOrders(session)
    db.save(session)
    return { dealer: session.dealerKey, vendor: session.vendorKey }
  }

  function route(db, req, url, body) {
    const path = url.pathname.slice(prefix.length)
    const get = req.method === 'GET'
    const post = req.method === 'POST'
    if (post && path === 'sessions') return start(db)

    const save = (session, result) => {
      db.save(session)
      return result
    }

    const [resource, id, action, ...extra] = path.split('/')
    if (!extra.length && id && secretFormat.test(id)) {
      if (resource === 'store') {
        const session = db.promotion(id)
        if (!session) fail(404, '推广链接无效，请联系代理商获取新链接。')
        if (get && !action) return { ...page(offers), manufacturer: maker }
        if (post && action === 'orders') {
          const row = recordOrder(session, body, 'promotion')
          return save(session, { checkout_path: issueCustomerLink(db, session, row) })
        }
      }
      if (resource === 'customer-orders') {
        const item = db.checkout(id)
        if (!item?.row) fail(404, '客户订单链接无效，请从原订单重新打开。')
        if (get && !action) return publicOrder(item.row)
        if (post && action === 'pay') {
          if (item.row.status === 'confirmation') {
            markPaid(item.session, item.row)
            db.save(item.session)
          }
          return publicOrder(item.row)
        }
      }
    }

    const actor = db.actor(req.headers.authorization?.replace(/^Bearer /, '') ?? '')
    if (!actor) fail(401, '找不到这场演示，请开启演示。')
    const { session, role } = actor
    if (resource === 'operations') {
      const result = routeDealerOperations(req, path, actor, body)
      return post ? save(session, result) : result
    }
    if (get && path === 'session') return { role }
    if (get && path === 'offers' && role === 'dealer') return page(offers)
    if (get && path === 'organization-links' && role === 'dealer') return page(session.operations.customers.map(({ id, name }) => ({ id, name })))
    if (post && path === 'promotion' && role === 'dealer') {
      if (body.manufacturer_id !== '1') fail(400, '请选择演示厂商。')
      session.promotionActive = true
      return save(session, { code: session.code })
    }
    if (get && path === 'orders') {
      const status = url.searchParams.get('status')
      const commissionOnly = url.searchParams.get('view') === 'commission'
      const rows = commissionOnly ? session.orders.filter(row => row.paid_at) : session.orders
      const result = page(rows.filter(row => !status || status === row.status)
        .map(({ checkout_path, ...row }) => role === 'dealer' && row.source === 'assisted' ? { ...row, checkout_path } : row))
      if (!commissionOnly) return result
      const summary = { settlement_cents: 0, settled_cents: 0, reminded: 0 }
      for (const row of rows) {
        if (row.status === 'settled') summary.settled_cents += row.commission.cents
        else {
          summary.settlement_cents += row.commission.cents
          if (row.commission.reminder_count) summary.reminded++
        }
      }
      return { ...result, summary }
    }
    if (post && path === 'orders' && role === 'dealer') {
      const row = recordOrder(session, body, 'assisted')
      issueCustomerLink(db, session, row)
      return save(session, { id: row.id })
    }
    if (post && resource === 'orders' && !extra.length) {
      const row = session.orders.find(item => item.id === id)
      if (!row) fail(404, '该订单不存在。')
      if (role === 'dealer' && action === 'remind' && row.status === 'settlement') {
        const last = row.commission.reminded_at
        if (!last || Date.now() - Date.parse(last) >= lifetime) {
          row.commission.reminded_at = new Date().toISOString()
          row.commission.reminder_count++
        }
        return save(session, {})
      }
      if (role === 'vendor' && action === 'settle') {
        const reference = typeof body.reference === 'string' ? body.reference.trim() : ''
        if (!reference || Array.from(reference).length > 100) fail(400, '请填写模拟转账凭证编号。')
        if (row.status === 'settled' && reference === row.payout_ref) return {}
        if (row.status !== 'settlement') fail(409, '订单当前无法登记结算。')
        row.status = 'settled'
        row.payout_ref = reference
        row.commission.settled_at = new Date().toISOString()
        return save(session, {})
      }
      fail(409, '订单当前无法办理该操作。')
    }
    fail(404, '演示接口不存在。')
  }

  function configureServer(server) {
    const db = openRetailDatabase(database)
    server.httpServer?.once('close', () => db.close())
    server.middlewares.use(async (req, res, next) => {
      const url = new URL(req.url, 'http://localhost')
      if (!url.pathname.startsWith(prefix)) return next()
      res.setHeader('Content-Type', 'application/json; charset=utf-8')
      res.setHeader('Cache-Control', 'no-store')
      try {
        const post = req.method === 'POST'
        const body = post ? await readBody(req) : undefined
        const data = post ? db.write(() => route(db, req, url, body)) : route(db, req, url, body)
        res.end(JSON.stringify({ code: 200, data }))
      } catch (error) {
        res.statusCode = error.status ?? 500
        res.end(JSON.stringify({ message: error.status ? error.message : '演示服务处理失败。' }))
      }
    })
  }
  return { name: 'partnerledger-retail-demo', configureServer, configurePreviewServer: configureServer }
}
