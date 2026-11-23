import { randomUUID } from 'node:crypto'

const serviceCreditCents = 120000
const ticketCategories = ['产品使用', '授权与交付', '订单与结算']
const dayMs = 86400000
const now = () => new Date().toISOString()
const sum = (rows, amount) => rows.reduce((total, row) => total + amount(row), 0)

export function createDealerOperations() {
  return {
    customers: [{ id: '1', name: '海河商贸（演示客户）', contact: '王经理（演示）', note: '' }],
    renewals: [{ id: 'sample-renewal', customer: '海河商贸（演示客户）', product_name: '云财务',
      due_at: new Date(Date.now() + 9 * dayMs).toISOString(), note: '' }],
    topups: [], invoices: [], claims: [], tickets: [],
  }
}

export function trackAnnualOrder(operations, order) {
  const due = new Date(order.paid_at)
  due.setUTCFullYear(due.getUTCFullYear() + 1)
  operations.renewals.push({ id: 'order-' + order.id, order_id: order.id, customer: order.customer,
    product_name: order.product_name, due_at: due.toISOString(), note: '' })
}

function fail(status, message) {
  throw Object.assign(new Error(message), { status })
}

function text(value, label, limit, required = true) {
  const result = typeof value === 'string' ? value.trim() : ''
  if ((required && !result) || Array.from(result).length > limit) fail(400, `请核对${label}。`)
  return result
}

function cents(value) {
  if (!Number.isSafeInteger(value) || value < 1 || value > 100000000) fail(400, '请输入 0.01 至 100 万元之间的金额。')
  return value
}

function getRecord(records, id) {
  const record = records.find(item => item.id === id)
  if (!record) fail(404, '该记录不存在。')
  return record
}

function addRecord(records, record) {
  if (records.length >= 200) fail(409, '该类演示记录已达到上限。')
  const row = { id: randomUUID(), created_at: now(), ...record }
  records.unshift(row)
  return row
}

function addUniqueRecord(records, record, uniqueField, comparableFields) {
  const prior = records.find(item => item[uniqueField] === record[uniqueField])
  if (!prior) return addRecord(records, record)
  if (comparableFields.some(field => prior[field] !== record[field])) fail(409, '这个凭证或订单已经用于另一笔申请。')
  return prior
}

function finishFinancialRecord(record, expected, status, body) {
  const reference = text(body.result_reference, '办理凭证编号', 80)
  if (record.status === status && record.result_reference === reference) return record
  if (record.status !== expected) fail(409, '该记录已经办理，请刷新核对。')
  Object.assign(record, { status, result_reference: reference, handled_at: now() })
  return record
}

function serviceBalance(claims) {
  return serviceCreditCents - sum(claims, row => row.amount_cents)
}

function financeData(session, role) {
  const { customers, renewals, topups, invoices, claims, tickets } = session.operations
  const orders = session.orders.map(({ id, source, product_name, customer, total_cents, paid_at, status }) =>
    ({ id, source, product_name, customer, total_cents, paid_at, status }))
  const payouts = session.orders.filter(row => row.payout_ref).map(row => ({
    order_id: row.id, amount_cents: row.commission.cents, reference: row.payout_ref, handled_at: row.commission.settled_at,
  }))
  const paidClaims = claims.filter(row => row.status === 'paid')
  return {
    ...(role === 'dealer' ? { customers, renewals } : {}), orders, topups, invoices, claims, tickets, payouts,
    totals: {
      customers: customers.length, orders: orders.length, promotion_orders: orders.filter(row => row.source === 'promotion').length,
      open_tickets: tickets.filter(row => row.status !== 'closed').length,
      renewals_30_days: renewals.filter(row => {
        const remaining = Date.parse(row.due_at) - Date.now()
        return remaining >= 0 && remaining <= 30 * dayMs
      }).length,
      topup_balance_cents: sum(topups.filter(row => row.status === 'credited'), row => row.amount_cents),
      pending_commission_cents: sum(session.orders.filter(row => row.status === 'settlement'), row => row.commission.cents),
      settled_commission_cents: sum(payouts, row => row.amount_cents),
      service_credit_cents: serviceCreditCents, available_service_cents: serviceBalance(claims),
      claimed_service_cents: sum(claims.filter(row => row.status === 'pending'), row => row.amount_cents),
      paid_service_cents: sum(paidClaims, row => row.amount_cents),
    },
  }
}

export async function routeDealerOperations(req, path, actor, readBody) {
  const { role, session } = actor
  if (req.method === 'GET' && path === 'operations') return financeData(session, role)
  if (req.method !== 'POST') fail(404, '没有这个办理入口。')
  const [, kind, id, action, ...extra] = path.split('/')
  if (extra.length) fail(404, '没有这个办理入口。')
  const body = await readBody(req)
  const { customers, renewals, topups, invoices, claims, tickets } = session.operations

  if (role === 'dealer' && kind === 'renewals' && id && !action) {
    const renewal = getRecord(renewals, id)
    const note = text(body.note, '续费跟进记录', 300)
    if (renewal.note === note) return renewal
    return Object.assign(renewal, { note, contacted_at: now() })
  }

  if (role === 'dealer' && kind === 'customers' && !action) {
    const record = { name: text(body.name, '企业名称', 80), contact: text(body.contact, '联系人', 40, false), note: text(body.note, '备注', 200, false) }
    if (id) {
      const row = getRecord(customers, id)
      if (customers.some(item => item.id !== id && item.name.toLowerCase() === record.name.toLowerCase())) fail(409, '已有同名客户。')
      return Object.assign(row, record)
    }
    const duplicate = customers.find(row => row.name.toLowerCase() === record.name.toLowerCase())
    if (duplicate?.name === record.name && duplicate.contact === record.contact && duplicate.note === record.note) return duplicate
    if (duplicate) fail(409, '已有同名客户。')
    return addRecord(customers, record)
  }
  if (role === 'dealer' && !id) {
    if (kind === 'topups') return addUniqueRecord(topups, {
      reference: text(body.reference, '模拟汇款凭证', 80), amount_cents: cents(body.amount_cents), status: 'pending',
    }, 'reference', ['amount_cents'])
    if (kind === 'invoices') {
      const order = getRecord(session.orders, body.order_id)
      if (!order.paid_at) fail(409, '请在订单完成收款后申请开票。')
      return addUniqueRecord(invoices, { order_id: order.id, title: text(body.title, '开票抬头', 80), status: 'requested' }, 'order_id', ['title'])
    }
    if (kind === 'claims') {
      const reference = text(body.reference, '服务费发票编号', 80)
      const amount = cents(body.amount_cents)
      const prior = claims.find(row => row.reference === reference)
      if (!prior && amount > serviceBalance(claims)) fail(409, '申请金额超过可报销服务费。')
      return addUniqueRecord(claims, { reference, amount_cents: amount, status: 'pending' }, 'reference', ['amount_cents'])
    }
    if (kind === 'tickets') {
      if (!ticketCategories.includes(body.category)) fail(400, '请选择工单类型。')
      const requestID = text(body.request_id, '提交编号', 36)
      const orderID = body.order_id ? getRecord(session.orders, body.order_id).id : ''
      return addUniqueRecord(tickets, { request_id: requestID, category: body.category, order_id: orderID,
        subject: text(body.subject, '工单标题', 80), detail: text(body.detail, '问题描述', 500), status: 'open', replies: [] },
      'request_id', ['category', 'order_id', 'subject', 'detail'])
    }
  }
  if (role === 'vendor' && id && action === 'complete') {
    if (kind === 'topups') return finishFinancialRecord(getRecord(topups, id), 'pending', 'credited', body)
    if (kind === 'invoices') return finishFinancialRecord(getRecord(invoices, id), 'requested', 'issued', body)
    if (kind === 'claims') return finishFinancialRecord(getRecord(claims, id), 'pending', 'paid', body)
  }
  if (kind === 'tickets' && id) {
    const ticket = getRecord(tickets, id)
    if (role === 'vendor' && action === 'resolve') {
      const reply = text(body.reply, '处理结果', 500)
      const last = ticket.replies.at(-1)
      if (ticket.status === 'resolved' && last?.role === role && last.text === reply) return ticket
      if (ticket.status !== 'open') fail(409, '工单状态已变化，请刷新核对。')
      ticket.replies.push({ role, text: reply, time: now() })
      ticket.status = 'resolved'
      return ticket
    }
    if (role === 'dealer' && action === 'close') {
      if (ticket.status === 'closed') return ticket
      if (ticket.status !== 'resolved') fail(409, '厂商回复后才可确认关闭。')
      ticket.status = 'closed'
      return ticket
    }
    if (role === 'dealer' && action === 'reopen') {
      const reply = text(body.reply, '补充说明', 500)
      const last = ticket.replies.at(-1)
      if (ticket.status === 'open' && last?.role === role && last.text === reply) return ticket
      if (ticket.status !== 'resolved' || ticket.replies.length >= 20) fail(409, '当前工单无法继续补充，请刷新或另建工单。')
      ticket.replies.push({ role, text: reply, time: now() })
      ticket.status = 'open'
      return ticket
    }
  }
  fail(403, '当前演示身份无法办理此操作。')
}
