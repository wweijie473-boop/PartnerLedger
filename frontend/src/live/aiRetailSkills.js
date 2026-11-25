import { formatCents } from '../channel/quoteMoney.js'
import { customerOrderLink, readRetailCatalog, retailApi } from './retailRequest.js'

const demoSessionKey = 'partnerledger-retail-demo-session-v1'

export async function createRetailSkillBridge(ctx) {
  if (ctx.retailRequest) {
    return { demo: ctx.demo, request: ctx.retailRequest }
  }
  if (!ctx.demo) {
    return {
      demo: false,
      request: (path, options = {}) => ctx.request(path, { ...ctx.options, ...options }),
    }
  }
  const { requestRetailDemo, retailDemoEnabled } = await import('./retailDemo.js')
  if (!retailDemoEnabled) {
    throw new Error('当前环境未开启本地直购与运营演示服务。')
  }
  let saved = null
  try {
    saved = JSON.parse(globalThis.localStorage?.getItem(demoSessionKey))
    if (saved?.dealer) await requestRetailDemo('/session', { token: saved.dealer })
    else saved = null
  } catch {
    saved = null
  }
  if (!saved?.dealer) {
    saved = await requestRetailDemo('/sessions', { method: 'POST', body: {} })
    globalThis.localStorage?.setItem(demoSessionKey, JSON.stringify(saved))
  }
  return {
    demo: true,
    request(path, options = {}) {
      const endpoint = path === '/api/channel/v1/organization-links'
        ? '/organization-links'
        : path.startsWith(retailApi)
          ? path.slice(retailApi.length)
          : path
      return requestRetailDemo(endpoint, { ...options, token: saved.dealer })
    },
  }
}

export function formatMoneyCents(cents) {
  return formatCents(BigInt(cents ?? 0))
}

export async function prepareAssistedOrderSkill(ctx, { text, route }) {
  const bridge = await createRetailSkillBridge(ctx)
  const [offers, linksPage] = await Promise.all([
    readRetailCatalog(path => bridge.request(path), retailApi + '/offers'),
    bridge.request('/api/channel/v1/organization-links'),
  ])
  const customers = linksPage?.items ?? []
  if (!offers.length) {
    return { message: '当前暂无软件厂商开放直购的产品，请先由厂商配置订购价格与佣金。' }
  }
  if (!customers.length) {
    return { message: '当前组织暂无已建立代理关联的企业客户，无法代客直购下单。' }
  }
  const matchedOffer = offers.find(item => text.includes(item.name)) ?? offers[0]
  const matchedCustomer = customers.find(item => text.includes(item.name)) ?? customers[0]
  const quantity = route.slots?.quantity ?? 1

  return {
    approval: {
      kind: 'assisted_order',
      skillId: 'assisted_order',
      title: '审批代关联客户直购下单',
      demo: bridge.demo,
      offers,
      customers,
      productId: String(matchedOffer.product_id),
      customerId: String(matchedCustomer.id),
      quantity,
      stepsPreview: [
        '核验所选软件产品的实时直购单价、年费周期与分佣规则',
        '为已关联企业客户创建待确认直购订单并锁定本组织佣金归因',
        '生成客户专属订单确认链接供客户核对付款',
      ],
    },
  }
}

const canRemindOrder = row => row.status === 'settlement' && (
  !row.commission?.reminded_at || Date.now() - Date.parse(row.commission.reminded_at) >= 86400000
)

export async function prepareRemindCommissionSkill(ctx) {
  const bridge = await createRetailSkillBridge(ctx)
  const page = await bridge.request(retailApi + '/orders?view=commission')
  const pendingOrders = (page?.items ?? []).filter(row => row.status === 'settlement')
  if (!pendingOrders.length) {
    return { message: '当前没有处于「待厂商结算」状态的直购佣金订单。' }
  }
  const remindable = pendingOrders.filter(canRemindOrder)
  if (!remindable.length) {
    return { message: '所有待结算佣金订单在最近 24 小时内均已催办过，请等待厂商登记转账凭证。' }
  }
  return {
    approval: {
      kind: 'remind_commission',
      skillId: 'remind_commission',
      title: '审批催办待结算直购佣金',
      demo: bridge.demo,
      orders: remindable,
      selectedOrderIds: remindable.map(item => String(item.id)),
      stepsPreview: [
        `核验 ${remindable.length} 笔待结算直购订单的催办冷却时间（24 小时限制）`,
        '自动向对应软件厂商批量提交佣金结算催办记录',
      ],
    },
  }
}

export async function prepareApplyInvoiceSkill(ctx, { text, route }) {
  const bridge = await createRetailSkillBridge(ctx)
  let paidOrders = []
  let existingInvoices = []
  if (bridge.demo) {
    const ops = await bridge.request(retailApi + '/operations')
    paidOrders = (ops?.orders ?? []).filter(order => order.paid_at)
    existingInvoices = ops?.invoices ?? []
  } else {
    const [orders, finance] = await Promise.all([
      readRetailCatalog(path => bridge.request(path), retailApi + '/orders', 'before'),
      readRetailCatalog(path => bridge.request(path), retailApi + '/operations/finance'),
    ])
    paidOrders = orders.filter(order => order.paid_at)
    existingInvoices = finance.filter(row => row.kind === 'invoices')
  }
  const invoicedOrderIds = new Set(existingInvoices.map(item => String(item.order_id)))
  const eligibleOrders = paidOrders.filter(order => !invoicedOrderIds.has(String(order.id)))
  if (!eligibleOrders.length) {
    return { message: '当前没有「已收款且尚未申请开票」的直购订单。' }
  }
  const matchedOrder = eligibleOrders.find(order => text.includes(String(order.id)) || text.includes(order.customer) || text.includes(order.product_name)) ?? eligibleOrders[0]
  return {
    approval: {
      kind: 'apply_invoice',
      skillId: 'apply_invoice',
      title: '审批申请直购订单开票',
      demo: bridge.demo,
      orders: eligibleOrders,
      orderId: String(matchedOrder.id),
      invoiceTitle: route.slots?.invoice_title || matchedOrder.customer || '',
      stepsPreview: [
        '核验直购订单已由厂商登记实收且未重复申请发票',
        '向所属软件厂商财务台账提交正式订单开票申请',
      ],
    },
  }
}

export async function prepareCreateTicketSkill(ctx, { text, route }) {
  const bridge = await createRetailSkillBridge(ctx)
  let orders = []
  let partners = []
  if (bridge.demo) {
    const ops = await bridge.request(retailApi + '/operations')
    orders = ops?.orders ?? []
    partners = ops?.partners ?? []
  } else {
    ;[orders, partners] = await Promise.all([
      readRetailCatalog(path => bridge.request(path), retailApi + '/orders', 'before'),
      readRetailCatalog(path => bridge.request(path), retailApi + '/operations/partners'),
    ])
  }
  if (!bridge.demo && !partners.length && !orders.length) {
    return { message: '当前组织暂无直购合作厂商记录，完成首次直购下单后即可提交厂商服务工单。' }
  }
  const matchedOrder = orders.find(order => text.includes(String(order.id)) || text.includes(order.product_name))
  const detail = route.slots?.note || text.replace(/^.*?(?:工单|技术支持|售后支持)[：:\s]*/u, '').trim() || text.trim()
  const subject = route.slots?.ticket_subject || (matchedOrder ? `${matchedOrder.product_name} · 售后服务支持` : detail.slice(0, 28))
  return {
    approval: {
      kind: 'create_ticket',
      skillId: 'create_ticket',
      title: '审批提交厂商服务工单',
      demo: bridge.demo,
      orders,
      partners,
      subject: subject.slice(0, 80),
      category: route.slots?.ticket_category || '产品使用',
      orderId: matchedOrder ? String(matchedOrder.id) : (orders[0] ? String(orders[0].id) : ''),
      manufacturerId: partners[0]?.id ? String(partners[0].id) : '',
      detail: detail.slice(0, 500),
      stepsPreview: [
        '关联目标直购订单或合作软件厂商通道',
        '生成幂等工单请求并提交至厂商渠道工单队列',
      ],
    },
  }
}

export function buildCustomerConfirmUrl(orderId) {
  const path = customerOrderLink(orderId)
  return globalThis.location?.origin ? new URL(path, globalThis.location.origin).href : path
}

export async function readCustomersSummary(ctx) {
  const bridge = await createRetailSkillBridge(ctx)
  const rows = bridge.demo
    ? ((await bridge.request(retailApi + '/operations'))?.customers ?? []).map(item => ({ ...item, linked: true, prospect: false }))
    : await readRetailCatalog(path => bridge.request(path), retailApi + '/operations/customers')
  const linkedCount = rows.filter(row => row.linked).length
  const prospectCount = rows.filter(row => row.prospect).length
  return { demo: bridge.demo, items: rows, linkedCount, prospectCount }
}

export async function readOperationsSummary(ctx) {
  const bridge = await createRetailSkillBridge(ctx)
  const totals = bridge.demo
    ? (await bridge.request(retailApi + '/operations'))?.totals
    : await bridge.request(retailApi + '/operations/overview')
  return {
    customers: Number(totals?.customers ?? 0),
    orders: Number(totals?.orders ?? 0),
    promotionOrders: Number(totals?.promotion_orders ?? 0),
    renewals30Days: Number(totals?.renewals_30_days ?? 0),
    pendingCommissionText: formatMoneyCents(totals?.pending_commission_cents ?? 0),
    openTickets: Number(totals?.open_tickets ?? 0),
  }
}
