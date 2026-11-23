import { randomBytes } from 'node:crypto'

const prefix = '/api/retail-demo/'
const maker = '畅捷通（演示厂商）'
const dealer = '天津主飞（演示代理商）'
const customers = [{ id: '1', name: '海河商贸（演示客户）' }]
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

function recordOrder(session, body, source) {
  const { product_id, quantity, expected_unit_cents, request_id } = body
  const offer = offers.find(item => item.product_id === product_id)
  const customer = customers.find(item => item.id === body.customer_id)
  if (!offer || !Number.isInteger(quantity) || quantity < 1 || quantity > 10000 ||
      typeof request_id !== 'string' || !requestFormat.test(request_id) || (source === 'assisted' && !customer)) {
    fail(400, '请核对订购产品、数量和客户。')
  }
  if (offer.unit_cents !== expected_unit_cents) fail(409, '产品价格已变化，请刷新后重试。')
  const key = source + ':' + request_id
  const fingerprint = JSON.stringify([product_id, quantity, expected_unit_cents, customer?.id])
  const prior = session.requests.get(key)
  if (prior) {
    if (prior.fingerprint !== fingerprint) fail(409, '请求已用于另一笔订单。')
    return prior.row
  }
  if (session.orders.length >= 200) fail(409, '该演示已达到订单上限，请开启新一轮演示。')
  const id = String(session.orders.length + 1).padStart(4, '0')
  const total = offer.unit_cents * quantity
  const row = {
    id, source, manufacturer: maker, reseller: dealer, customer: customer?.name ?? '扫码客户（演示）',
    product_name: offer.name, product_version: offer.version, quantity, total_cents: total,
    status: source === 'promotion' ? 'settlement' : 'confirmation',
    commission: { rate_bps: 5000, cents: total / 2, reminder_count: 0 },
  }
  if (source === 'promotion') markPaid(row)
  session.orders.unshift(row)
  session.requests.set(key, { fingerprint, row })
  return row
}

function markPaid(row) {
  row.status = 'settlement'
  row.paid_at = new Date().toISOString()
  row.receipt_ref = 'DEMO-IN-' + row.id
}

export function retailDemo() {
  const sessions = new Set()
  const actors = new Map()
  const promotions = new Map()
  const customerOrders = new Map()

  function issueCustomerLink(session, row) {
    if (row.checkout_path) return row.checkout_path
    const secret = token()
    session.customerKeys.push(secret)
    customerOrders.set(secret, { session, row })
    row.checkout_path = customerPath(secret)
    return row.checkout_path
  }

  function start() {
    for (const session of sessions) {
      if (session.expires > Date.now()) continue
      sessions.delete(session)
      promotions.delete(session.code)
      actors.delete(session.dealerKey)
      actors.delete(session.vendorKey)
      for (const secret of session.customerKeys) customerOrders.delete(secret)
    }
    if (sessions.size >= 100) fail(503, '演示场次已满，请稍后重试。')
    const session = { code: token(), dealerKey: token(), vendorKey: token(), expires: Date.now() + lifetime,
      orders: [], requests: new Map(), customerKeys: [] }
    sessions.add(session)
    actors.set(session.dealerKey, { role: 'dealer', session })
    actors.set(session.vendorKey, { role: 'vendor', session })
    return { dealer: session.dealerKey, vendor: session.vendorKey, expires: session.expires }
  }

  async function route(req, url) {
    const path = url.pathname.slice(prefix.length)
    const get = req.method === 'GET'
    const post = req.method === 'POST'
    if (post && path === 'sessions') {
      await readBody(req)
      return start()
    }

    const [resource, id, action, ...extra] = path.split('/')
    if (!extra.length && id && secretFormat.test(id)) {
      if (resource === 'store') {
        const session = promotions.get(id)
        if (!session || session.expires <= Date.now()) fail(404, '推广链接无效或已过期。')
        if (get && !action) return { ...page(offers), manufacturer: maker }
        if (post && action === 'orders') {
          const row = recordOrder(session, await readBody(req), 'promotion')
          return { checkout_path: issueCustomerLink(session, row) }
        }
      }
      if (resource === 'customer-orders') {
        const item = customerOrders.get(id)
        if (!item || item.session.expires <= Date.now()) fail(404, '客户订单链接无效或已过期。')
        if (get && !action) return publicOrder(item.row)
        if (post && action === 'pay') {
          await readBody(req)
          if (item.row.status === 'confirmation') markPaid(item.row)
          return publicOrder(item.row)
        }
      }
    }

    const actor = actors.get(req.headers.authorization?.replace(/^Bearer /, ''))
    if (!actor || actor.session.expires <= Date.now()) fail(401, '演示场次已结束，请开启新一轮演示。')
    const { session, role } = actor
    if (get && path === 'session') return { role }
    if (get && path === 'offers' && role === 'dealer') return page(offers)
    if (get && path === 'organization-links' && role === 'dealer') return page(customers)
    if (post && path === 'promotion' && role === 'dealer') {
      const body = await readBody(req)
      if (body.manufacturer_id !== '1') fail(400, '请选择演示厂商。')
      promotions.set(session.code, session)
      return { code: session.code }
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
      const row = recordOrder(session, await readBody(req), 'assisted')
      issueCustomerLink(session, row)
      return { id: row.id }
    }
    if (post && resource === 'orders' && !extra.length) {
      const row = session.orders.find(item => item.id === id)
      if (!row) fail(404, '该订单不存在。')
      const body = await readBody(req)
      if (role === 'dealer' && action === 'remind' && row.status === 'settlement') {
        const last = row.commission.reminded_at
        if (!last || Date.now() - Date.parse(last) >= lifetime) {
          row.commission.reminded_at = new Date().toISOString()
          row.commission.reminder_count++
        }
        return {}
      }
      if (role === 'vendor' && action === 'settle') {
        const reference = typeof body.reference === 'string' ? body.reference.trim() : ''
        if (!reference || Array.from(reference).length > 100) fail(400, '请填写模拟转账凭证编号。')
        if (row.status === 'settled' && reference === row.payout_ref) return {}
        if (row.status !== 'settlement') fail(409, '订单当前无法登记结算。')
        row.status = 'settled'
        row.payout_ref = reference
        row.commission.settled_at = new Date().toISOString()
        return {}
      }
      fail(409, '订单当前无法办理该操作。')
    }
    fail(404, '演示接口不存在。')
  }

  function configureServer(server) {
    server.middlewares.use(async (req, res, next) => {
      const url = new URL(req.url, 'http://localhost')
      if (!url.pathname.startsWith(prefix)) return next()
      res.setHeader('Content-Type', 'application/json; charset=utf-8')
      res.setHeader('Cache-Control', 'no-store')
      try {
        const data = await route(req, url)
        res.end(JSON.stringify({ code: 200, data }))
      } catch (error) {
        res.statusCode = error.status ?? 500
        res.end(JSON.stringify({ message: error.status ? error.message : '演示服务处理失败。' }))
      }
    })
  }
  return { name: 'partnerledger-retail-demo', configureServer, configurePreviewServer: configureServer }
}
