import { demoCompanies } from '../demoCompanies.js'

export const roles = {
  reseller: { label: '代理商', company: demoCompanies.dealer, orgId: 'rs-beichen' },
  manufacturer: { label: '软件厂商', company: demoCompanies.vendor, orgId: 'mf-yunxu' },
  customer: { label: '企业客户', company: '海川精工', orgId: 'cu-haichuan' },
}

export const products = [
  { id: 'ERP-PRO', name: '云序 ERP 专业版', modules: '采购、库存、财务', deployment: '私有部署 / 云部署', license: '按用户授权' },
  { id: 'MES-STD', name: '云序 MES 制造执行', modules: '生产排程、工序、质检', deployment: '私有部署', license: '按产线授权' },
  { id: 'OA-TEAM', name: '云序 OA 协作套件', modules: '审批、合同、文档', deployment: '云部署', license: '按用户订阅' },
].map(product => ({ ...product, modules: product.modules.split('、'), published: true, manufacturerId: roles.manufacturer.orgId }))

export const stageLabels = {
  requested: '待整理需求', customer_info: '待客户补资料', inquired: '待厂商报价', vendor_info: '待补询价资料',
  channel_quoted: '渠道已报价', channel_revision: '渠道报价待改版', sales_quoted: '待客户审阅',
  sales_revision: '对客报价待改版', confirmed: '已确认待下单', ordered: '待厂商供货',
  supplied: '待代理商实施', acceptance: '待客户验收', rectification: '待整改', completed: '已验收',
  processing: '代理商跟进中', channel_wait: '渠道报价已发送', shipped: '已供货',
}
export const projectScopeLabels = { all: '全部项目', mine: '待我处理', partners: '协作方处理中' }
export const actionLabels = {
  requestInfo: '要求补资料', replyInfo: '回复资料', submitInquiry: '提交厂商询价', withdrawInquiry: '撤回询价',
  channelQuote: '发送渠道报价', salesQuote: '发送对客报价', reviseChannel: '请求厂商改价', reviseSales: '请求修改方案',
  confirmSales: '确认此版报价', placeOrder: '确认下单', supply: '登记供货与授权', implement: '提交实施验收',
  rectify: '退回整改', accept: '确认验收', flagGap: '标记不支持的附加需求',
}
const { reseller, customer } = roles
const commercial = ['channel_quoted', 'channel_revision', 'sales_quoted', 'sales_revision', 'confirmed']
const allRoles = ['customer', 'reseller', 'manufacturer']
const downstream = ['customer', 'reseller']
const upstream = ['reseller', 'manufacturer']
const fail = (message, code = 'INVALID') => { throw Object.assign(new Error(message), { code }) }
const copy = value => structuredClone(value)
const last = list => list[list.length - 1]
function field(value, label, max = 3000) {
  if (typeof value !== 'string' || !value.trim() || value.trim().length > max) fail('请填写有效的' + label)
  return value.trim()
}
function dateString(date) {
  return [date.getFullYear(), String(date.getMonth() + 1).padStart(2, '0'), String(date.getDate()).padStart(2, '0')].join('-')
}
function offsetDay(now, days) {
  const result = new Date(now)
  result.setDate(result.getDate() + days)
  return dateString(result)
}
function goodDate(value) {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(value + 'T12:00:00')
  return !Number.isNaN(parsed.valueOf()) && dateString(parsed) === value
}
export function demoActor(role) {
  if (!Object.hasOwn(roles, role)) fail('未知身份')
  return { role, orgId: roles[role].orgId }
}
function visible(project, actor) {
  return !!actor.orgId && project[actor.role + 'Id'] === actor.orgId &&
    (actor.role !== 'manufacturer' || !!project.inquiry)
}
function record(project, now, audience, label, detail = '') {
  project.timeline.push({ at: now.toISOString(), label, detail, audience })
}
function statusOf(quote, now) {
  if (!quote) return ''
  if (quote.status === 'superseded' || quote.status === 'ordered') return quote.status
  return quote.validUntil < dateString(now) ? 'expired' : quote.status
}
function valid(quote, now) { return ['active', 'confirmed'].includes(statusOf(quote, now)) }
function actionsFor(project, actor, now) {
  const { stage } = project
  const channel = last(project.channelQuotes), sales = last(project.salesQuotes)
  if (actor.role === 'customer') {
    if (stage === 'customer_info') return ['replyInfo']
    if (stage === 'sales_quoted') return valid(sales, now) ? ['confirmSales', 'reviseSales'] : ['reviseSales']
    if (stage === 'acceptance') return ['accept', 'rectify']
  }
  if (actor.role === 'manufacturer') {
    if (stage === 'inquired') return ['channelQuote', 'requestInfo', ...(project.inquiry.gapNote ? [] : ['flagGap'])]
    if (['channel_quoted', 'channel_revision'].includes(stage) || (commercial.includes(stage) && !valid(channel, now))) return ['channelQuote']
    if (stage === 'ordered') return ['supply']
  }
  if (actor.role === 'reseller') {
    if (stage === 'requested') return ['submitInquiry', 'requestInfo']
    if (stage === 'inquired' && !project.inquiry.processed) return ['withdrawInquiry']
    if (stage === 'vendor_info') return ['replyInfo', 'requestInfo']
    if (['channel_quoted', 'sales_revision', 'sales_quoted'].includes(stage)) {
      return valid(channel, now) ? ['salesQuote', 'reviseChannel'] : ['reviseChannel']
    }
    if (stage === 'confirmed') {
      if (!valid(channel, now)) return ['reviseChannel']
      return valid(sales, now) ? ['placeOrder'] : ['salesQuote']
    }
    if (['supplied', 'rectification'].includes(stage)) return ['implement']
  }
  return []
}
const owners = {
  requested: 'reseller', customer_info: 'customer', inquired: 'manufacturer', vendor_info: 'reseller',
  channel_quoted: 'reseller', channel_revision: 'manufacturer', sales_quoted: 'customer', sales_revision: 'reseller',
  confirmed: 'reseller', ordered: 'manufacturer', supplied: 'reseller', acceptance: 'customer', rectification: 'reseller',
}
export function projectHandlerLabel(project, role) {
  if (project.stage === 'completed') return '协作已完成'
  const actor = { dealer: 'reseller', vendor: 'manufacturer', customer: 'customer' }[role]
  if (project.isTodo) return (roles[actor]?.label || '当前组织') + '（本方）'
  const owner = ['processing', 'channel_wait'].includes(project.stage) ? 'reseller' : owners[project.stage]
  const label = owner === actor ? '' : roles[owner]?.label
  return label || '协作方'
}
function displayStage(project, role) {
  if (role === 'customer' && ['inquired','vendor_info','channel_quoted','channel_revision'].includes(project.stage)) return 'processing'
  if (role === 'manufacturer') {
    if (project.stage === 'customer_info') return 'vendor_info'
    if (['sales_quoted','sales_revision','confirmed','channel_quoted'].includes(project.stage)) return 'channel_wait'
    if (['supplied','rectification','acceptance'].includes(project.stage)) return 'shipped'
  }
  return project.stage
}
export function viewProject(project, actor, now = new Date()) {
  if (!project || !visible(project, actor)) fail('记录不存在或无权访问', 'NOT_FOUND')
  const { role } = actor, stage = displayStage(project, role)
  const actions = actionsFor(project, actor, now)
  const isTodo = owners[project.stage] === role ||
    (role === 'manufacturer' && commercial.includes(project.stage) && !valid(last(project.channelQuotes), now))
  const result = {
    id: project.id, title: role === 'manufacturer' ? project.product + ' · 渠道采购项目' : project.title,
    stage, stageLabel: stageLabels[stage], revision: project.revision, dueDate: project.dueDate,
    actions, isTodo, nextStep: isTodo && actions.length ? actionLabels[actions[0]] : stageLabels[stage],
    timeline: project.timeline.filter(event => event.audience.includes(role)).map(({ audience, ...event }) => ({...event})),
  }
  const quotes = list => list.map(quote => ({ ...copy(quote), status: statusOf(quote, now) }))
  if (role !== 'manufacturer') Object.assign(result, { title: project.title, requirement: project.requirement,
    customerName: project.customerName, salesQuotes: quotes(project.salesQuotes) })
  if (role !== 'customer') Object.assign(result, { channelQuotes: quotes(project.channelQuotes), inquiry: copy(project.inquiry) })
  if (role !== 'customer' || project.salesQuotes.length) result.product = project.product
  if (project.order) {
    result.order = { id: project.order.id }
    if (role !== 'manufacturer') result.order.salesQuoteId = project.order.salesQuoteId
    if (role !== 'customer') result.order.channelQuoteId = project.order.channelQuoteId
  }
  if (project.supplyNote) result.supplyNote = project.supplyNote
  if (project.implementationNote && role !== 'manufacturer') result.implementationNote = project.implementationNote
  return result
}
export function projectList(state, actor, now = new Date()) {
  return state.projects.filter(project => visible(project, actor)).map(project => viewProject(project, actor, now))
}
export function createProject(state, actor, payload, now = new Date()) {
  if (!['customer','reseller'].includes(actor.role)) fail('该身份不可创建项目', 'FORBIDDEN')
  const title = field(payload.title, '项目名称', 120), requirement = field(payload.requirement, '业务需求')
  if (payload.dueDate && !goodDate(payload.dueDate)) fail('期望日期格式无效')
  if (actor.role === 'reseller' && payload.customerName && payload.customerName.trim() !== customer.company) {
    fail('当前演示只支持已绑定的客户组织「' + customer.company + '」，以便客户能够确认报价与验收。')
  }
  const customerName = customer.company, customerId = actor.role === 'customer' ? actor.orgId : customer.orgId
  const project = {
    id: 'PRJ-' + String(++state.sequence).padStart(4, '0'), revision: 1, title, requirement, customerName, customerId,
    resellerId: actor.role === 'reseller' ? actor.orgId : reseller.orgId, manufacturerId: null, product: null, inquiry: null,
    dueDate: payload.dueDate || '', stage: 'requested', channelQuotes: [], salesQuotes: [], timeline: [], order: null,
  }
  record(project, now, downstream, actor.role === 'customer' ? '客户提交需求' : '代理商登记需求', requirement)
  state.projects.unshift(project)
  return viewProject(project, actor, now)
}
function makeQuote(project, list, prefix, payload, now) {
  if (!goodDate(payload.validUntil) || payload.validUntil < dateString(now)) fail('请填写今天或之后的有效期')
  const deliveryDays = Number(payload.deliveryDays)
  if (!Number.isInteger(deliveryDays) || deliveryDays < 1 || deliveryDays > 3650) fail('交付周期应为 1–3650 天')
  if (!Array.isArray(payload.lines) || !payload.lines.length || payload.lines.length > 100) fail('请填写报价明细')
  let cents = 0
  const lines = payload.lines.map(line => {
    const label = field(line.label, '明细名称', 120), quantity = Number(line.quantity)
    const rawPrice = String(line.unitPrice ?? '')
    if (!Number.isSafeInteger(quantity) || quantity < 1 || quantity > 1000000 || !/^\d{1,8}(\.\d{1,2})?$/.test(rawPrice)) fail('请检查数量与单价（最多两位小数）')
    const unitPrice = Number(rawPrice)
    cents += quantity * Math.round(unitPrice * 100)
    if (!Number.isSafeInteger(cents)) fail('总金额超出范围')
    return { label, quantity, unitPrice }
  })
  if (cents <= 0) fail('报价合计必须大于零')
  const version = list.length + 1
  return { id: prefix + '-' + project.id.slice(4) + '-V' + version, version, lines, total: cents / 100,
    validUntil: payload.validUntil, deliveryDays, note: payload.note ? field(payload.note, '报价备注') : '', status: 'active' }
}
function supersede(list) {
  if (list.length) last(list).status = 'superseded'
}
export function actOnProject(state, actor, id, action, payload = {}, revision, now = new Date()) {
  const project = state.projects.find(item => item.id === id)
  if (!project || !visible(project, actor)) fail('记录不存在或无权访问', 'NOT_FOUND')
  if (action === 'placeOrder' && actor.role === 'reseller' && project.order?.salesQuoteId === payload.quoteId && !!payload.quoteId)
    return viewProject(project, actor, now)
  if (revision !== project.revision) fail('记录已更新，请刷新后重试', 'CONFLICT')
  if (!actionsFor(project, actor, now).includes(action)) fail('当前身份或业务状态不允许此操作，请检查报价有效期', 'CONFLICT')
  const note = () => field(payload.note, '说明')
  const event = (audience, label, detail) => record(project, now, audience, label, detail)
  if (action === 'flagGap') {
    const detail = note()
    project.inquiry.gapNote = detail
    project.inquiry.processed = true
    event(upstream, '厂商标记暂不支持的附加需求', detail)
  } else if (action === 'submitInquiry') {
    const product = state.products.find(item => item.id === payload.product && item.published)
    if (!product) fail('请选择已发布的产品')
    const summary = field(payload.summary, '询价摘要')
    Object.assign(project, { manufacturerId: product.manufacturerId, product: product.id, stage: 'inquired',
      inquiry: { id: 'RFQ-' + project.id.slice(4), summary, processed: false } })
    event(upstream, '代理商提交询价', summary)
  } else if (action === 'withdrawInquiry') {
    project.timeline = project.timeline.map(item => item.audience.includes('manufacturer') ?
      { ...item, audience: item.audience.filter(role => role !== 'manufacturer') } : item)
    Object.assign(project, { manufacturerId: null, product: null, inquiry: null, stage: 'requested' })
    event(['reseller'], '已撤回未处理询价', '')
  } else if (action === 'requestInfo' || action === 'replyInfo') {
    const detail = note()
    const toCustomer = action === 'requestInfo' ? actor.role === 'reseller' : actor.role === 'customer'
    event(toCustomer ? downstream : upstream, action === 'requestInfo' ? '请求补充资料' : '已回复补充资料', detail)
    if (project.inquiry && !toCustomer) project.inquiry.processed = true
    project.stage = action === 'requestInfo' ? (toCustomer ? 'customer_info' : 'vendor_info') :
      (toCustomer ? (project.inquiry ? 'vendor_info' : 'requested') : 'inquired')
  } else if (action === 'channelQuote' || action === 'salesQuote') {
    const channel = action === 'channelQuote', list = channel ? project.channelQuotes : project.salesQuotes
    const quote = makeQuote(project, list, channel ? 'CQ' : 'SQ', payload, now)
    supersede(list)
    if (channel) { supersede(project.salesQuotes); project.inquiry.processed = true }
    list.push(quote)
    project.stage = channel ? 'channel_quoted' : 'sales_quoted'
    event(channel ? upstream : downstream, channel ? '厂商发送渠道报价' : '代理商发送对客报价', quote.id)
  } else if (action === 'reviseChannel' || action === 'reviseSales') {
    const detail = note(), channel = action === 'reviseChannel'
    if (channel) supersede(project.channelQuotes)
    supersede(project.salesQuotes)
    project.stage = channel ? 'channel_revision' : 'sales_revision'
    event(channel ? upstream : downstream, actionLabels[action], detail)
  } else if (action === 'confirmSales' || action === 'placeOrder') {
    const quote = last(project.salesQuotes)
    if (!quote || payload.quoteId !== quote.id || !valid(quote, now)) fail('报价版本无效或已过期', 'CONFLICT')
    if (action === 'confirmSales') {
      quote.status = 'confirmed'; project.stage = 'confirmed'
      event(downstream, '客户确认报价', quote.id)
    } else {
      const channel = last(project.channelQuotes)
      if (!valid(channel, now) || quote.status !== 'confirmed') fail('请更新有效报价并重新确认', 'CONFLICT')
      project.order = { id: 'SO-' + project.id.slice(4), salesQuoteId: quote.id, channelQuoteId: channel.id }
      quote.status = 'ordered'; channel.status = 'ordered'; project.stage = 'ordered'
      event(allRoles, '代理商确认下单', project.order.id)
    }
  } else if (action === 'supply') {
    project.supplyNote = note(); project.stage = 'supplied'
    event(allRoles, '厂商已完成供货或授权', project.supplyNote)
  } else if (action === 'implement') {
    project.implementationNote = note(); project.stage = 'acceptance'
    event(downstream, '代理商已提交实施验收', project.implementationNote)
  } else if (action === 'rectify') {
    const detail = note(); project.stage = 'rectification'
    event(downstream, '客户退回整改', detail)
  } else if (action === 'accept') {
    const detail = payload.note ? note() : ''
    project.stage = 'completed'
    event(downstream, '客户完成验收', detail)
    event(['manufacturer'], '项目已验收', '')
  }
  project.revision += 1
  return viewProject(project, actor, now)
}
export function listProducts(state, actor) {
  return copy(state.products.filter(item => actor.role === 'manufacturer' ? item.manufacturerId === actor.orgId : item.published))
}
export function saveProduct(state, actor, payload) {
  if (actor.role !== 'manufacturer') fail('仅厂商可维护目录', 'FORBIDDEN')
  const existing = payload.id && state.products.find(item => item.id === payload.id)
  if (payload.id && (!existing || existing.manufacturerId !== actor.orgId)) fail('产品不存在或无权访问', 'NOT_FOUND')
  const modules = Array.isArray(payload.modules) ? payload.modules : String(payload.modules || '').split('、')
  if (!modules.length || modules.length > 30) fail('请填写产品功能模块（最多 30 个）')
  const fields = { name: field(payload.name,'产品名称',120), modules: modules.map(name => field(name,'功能模块',100)),
    license: field(payload.license,'授权方式',120), deployment: field(payload.deployment,'部署方式',120),
    published: payload.published === true }
  if (existing) Object.assign(existing, fields)
  else state.products.push({ id: 'SW-' + (++state.sequence), manufacturerId: actor.orgId, ...fields })
  return copy(existing || last(state.products))
}
export function seedState(now = new Date()) {
  const state = { schema: 1, sequence: 1020, projects: [], products: copy(products) }
  const client = demoActor('customer'), dealer = demoActor('reseller'), vendor = demoActor('manufacturer')
  const sample = (title, requirement, steps, daysRemaining) => {
    const eventTime = index => new Date(now.valueOf() - (steps.length - index + 1) * 4 * 3600000)
    let entry = createProject(state, client, { title, requirement, dueDate: offsetDay(now, daysRemaining) }, eventTime(0))
    for (const [index, [actor, action, payload]] of steps.entries()) entry = actOnProject(state, actor, entry.id, action,
      action === 'confirmSales' || action === 'placeOrder' ? { quoteId: last(entry.salesQuotes).id } : payload, entry.revision, eventTime(index + 1))
  }
  const inquiry = [dealer,'submitInquiry',{ product:'ERP-PRO', summary:'采购 25 席 ERP，包含财务、库存、部署与培训。' }]
  const quote = (rate, fee) => ({ lines:[{label:'ERP 用户授权',quantity:25,unitPrice:rate},
    {label:'实施部署与培训',quantity:1,unitPrice:fee}], validUntil:offsetDay(now,14),deliveryDays:10 })
  const quoted = [inquiry, [vendor,'channelQuote',quote(1200,6000)]]
  const sold = [...quoted,[dealer,'salesQuote',quote(1520,9000)]]
  sample('仓储财务一体化 · 上线验收','采购 25 席 ERP，统一仓储与财务操作。', [...sold,[client,'confirmSales'],[dealer,'placeOrder'],
    [vendor,'supply',{note:'25 席授权已交付。'}],[dealer,'implement',{note:'完成上线培训与数据核对，请验收。'}]],2)
  sample('供应链系统 · 年度增购','为新增仓库采购 ERP，要求部署和人员培训。',sold,9)
  sample('财务共享中心 · ERP 升级','整合采购、财务及库存台账，首期 25 人使用。',quoted,15)
  sample('新厂区 · 生产系统对接','希望标准化新厂区的排程、生产工序及质检记录。',[[dealer,'submitInquiry',{product:'MES-STD',summary:'请提供一条产线的 MES 授权、部署及培训报价。'}]],21)
  sample('协同办公 · 审批改造','将线下用印、采购审批迁移到线上，先覆盖总部。',[],30)
  return state
}
