import { aiDemoProject, matchDemoProducts, parseDemoRequirement } from '../channel/aiInquiryDemo.js'
import { roles } from '../channel/domain.js'
import { categoryLabels, suggestOpportunity, systemLabels } from '../channel/opportunities.js'
import { inquirySummary } from './aiInquiry.js'
import { splitVisit } from './visitDrafts.js'

export const canInquire = project => Boolean(project?.actions?.includes('submitInquiry'))

export function editableRequirement(requirement = {}) {
  return {
    enterprise_type: requirement.enterprise_type ?? '',
    employee_count: requirement.employee_count ?? '',
    required_modules: (requirement.required_modules ?? []).join('、'),
    deployment_mode: requirement.deployment_mode ?? '',
    license_count: requirement.license_count ?? '',
    version: requirement.version ?? '',
  }
}

export function normalizeRequirement(fields) {
  const positiveInt = value => {
    if (value === '' || value === null || value === undefined) return null
    const num = Number(value)
    return Number.isInteger(num) && num >= 1 && num <= 1000000 ? num : NaN
  }
  return {
    enterprise_type: String(fields.enterprise_type ?? '').trim(),
    employee_count: positiveInt(fields.employee_count),
    required_modules: [...new Set(String(fields.required_modules ?? '').split(/[、，,；;\n]/).map(item => item.trim()).filter(Boolean))],
    deployment_mode: String(fields.deployment_mode ?? '').trim(),
    license_count: positiveInt(fields.license_count),
    version: String(fields.version ?? '').trim(),
  }
}

export function hasAnyRequirement(requirement) {
  return Boolean(
    requirement.enterprise_type ||
    (Number.isInteger(requirement.employee_count) && requirement.employee_count > 0) ||
    requirement.required_modules.length ||
    requirement.deployment_mode ||
    (Number.isInteger(requirement.license_count) && requirement.license_count > 0) ||
    requirement.version,
  )
}

function offsetDateString(days) {
  const date = new Date()
  date.setDate(date.getDate() + days)
  return [
    date.getFullYear(),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-')
}

function pickBestCandidate(matches) {
  const eligible = matches.filter(item => item.status !== 'mismatch')
  const exact = eligible.find(item => item.status === 'matched')
  return (exact ?? eligible[0])?.product?.id ? String((exact ?? eligible[0]).product.id) : ''
}

function defaultProjectTitle(customerName, requirement) {
  const prefix = customerName || '企业客户'
  const moduleHint = requirement.required_modules?.[0] ? `${requirement.required_modules[0]}及配套` : 'ERP 软件'
  return `${prefix} · ${moduleHint}采购`
}

export async function queryProductMatches(ctx, requirement) {
  if (!hasAnyRequirement(requirement)) return []
  return ctx.demo
    ? matchDemoProducts(await ctx.repo.listProducts(), requirement)
    : (await ctx.post('/api/ai/match-products', requirement)).items
}

export async function prepareInquirySkill(ctx, { intent, source, project, manual = false }) {
  const extracted = manual
    ? {}
    : ctx.demo
      ? parseDemoRequirement(source)
      : await ctx.post('/api/ai/parse-requirement', { text: source }, 32000)

  const fields = editableRequirement(extracted)
  const normalized = normalizeRequirement(fields)
  const matches = hasAnyRequirement(normalized) ? await queryProductMatches(ctx, normalized) : []

  let customers = []
  if (!ctx.demo && intent !== 'match_products') {
    try {
      customers = (await ctx.get('/api/channel/v1/organization-links'))?.items ?? []
    } catch {
      customers = []
    }
  }

  const targetProject = canInquire(project) ? project : null

  const canAutoCreate = intent !== 'match_products' && (ctx.demo || customers.length > 0)
  const projectId = targetProject
    ? targetProject.id
    : (!ctx.demo && ctx.inquiryProjects.value.length === 1
        ? ctx.inquiryProjects.value[0].id
        : (canAutoCreate ? '__create__' : ''))
  const customerId = customers[0]?.id ?? ''
  const customerName = ctx.demo ? roles.customer.company : (customers.find(c => c.id === customerId)?.name ?? '')
  const selectedProductId = pickBestCandidate(matches)

  return {
    kind: 'inquiry_workflow',
    skillId: intent,
    title: intent === 'match_products' ? '审批产品匹配条件' : '审批采购询价工作流',
    intent,
    source,
    projectId,
    canAutoCreate,
    customers,
    customerId,
    newProjectTitle: ctx.demo ? aiDemoProject.title : defaultProjectTitle(customerName, normalized),
    fields,
    questions: extracted.follow_up_questions ?? [],
    matches,
    selectedProductId,
    autoSend: intent === 'submit_inquiry',
    fieldsDirty: false,
  }
}

export function inquiryStepsPreview(approval) {
  if (approval.intent === 'match_products') {
    return [
      '按审批确认的结构化条件检索已发布产品知识库',
      '输出逐项符合性结论与厂商原始资料摘录（不建项、不外发）',
    ]
  }
  const steps = []
  if (approval.projectId === '__create__') {
    steps.push('自动关联客户并新建采购协作项目')
  } else {
    steps.push('锁定目标待询价项目与当前版本号')
  }
  steps.push('按所选候选产品生成脱敏摘要并保存询价草稿')
  if (approval.autoSend) {
    steps.push('自动向产品所属软件厂商正式发送询价')
  }
  return steps
}

export async function prepareSendSavedDraft(ctx, project, draft, product) {
  if (!canInquire(project) || draft.projectRevision !== project.revision) {
    return { error: '项目版本或状态已变化，请打开项目核对后重新发起。', project }
  }
  const targetProduct = product ?? await ctx.getProduct(draft.productId)
  if (!targetProduct?.published) {
    return { error: '草稿关联的产品已下架或不可用，请重新准备询价。', project }
  }
  return {
    approval: {
      kind: 'send_draft',
      skillId: 'submit_inquiry',
      title: '审批发送厂商询价',
      project,
      product: targetProduct,
      summary: draft.summary,
      stepsPreview: [
        '校验项目版本与所选软件产品上架状态',
        '向产品所属软件厂商正式提交询价单',
      ],
    },
  }
}

function calcMarkedLines(channelLines, markupPercent) {
  const ratio = 1 + (Number(markupPercent) || 0) / 100
  return channelLines.map(line => {
    const baseCents = Math.round(Number(line.unitPrice) * 100)
    const saleCents = Math.max(1, Math.round(baseCents * ratio))
    return {
      label: line.label,
      quantity: Number(line.quantity),
      channelUnitPrice: Number(line.unitPrice).toFixed(2),
      unitPrice: (saleCents / 100).toFixed(2),
    }
  })
}

export async function prepareSalesQuoteSkill(ctx, { route, project }) {
  const candidate = project?.actions?.includes('salesQuote')
    ? project
    : ctx.projects.value.find(item => item.actions?.includes('salesQuote'))
  if (!candidate) {
    return { message: '当前没有处于「渠道已报价 / 对客报价待改版」阶段的项目。请先等待厂商发送渠道报价。' }
  }
  const detail = await ctx.repo.get(candidate.id, ctx.options)
  if (!detail.actions?.includes('salesQuote')) {
    return { message: `项目「${detail.title}」当前状态（${detail.stageLabel}）暂不可发送对客报价。`, project: detail }
  }
  const channelQuote = detail.channelQuotes?.at(-1)
  if (!channelQuote?.lines?.length) {
    return { message: `项目「${detail.title}」尚未读取到有效渠道报价明细。`, project: detail }
  }
  const markupPercent = route.slots?.markup_percent ?? 25
  const validDays = route.slots?.valid_days ?? 14
  const deliveryDays = route.slots?.delivery_days ?? channelQuote.deliveryDays ?? 10
  return {
    approval: {
      kind: 'sales_quote',
      skillId: 'sales_quote',
      title: '审批编制并发送对客报价',
      project: detail,
      channelQuote,
      markupPercent,
      lines: calcMarkedLines(channelQuote.lines, markupPercent),
      validUntil: offsetDateString(validDays),
      deliveryDays,
      note: route.slots?.note || '包含标准软件授权、实施部署及关键用户培训服务。',
      stepsPreview: [
        `锁定项目「${detail.title}」最新渠道底价（${channelQuote.id}）`,
        '生成独立版本号的对客报价单并隐藏上游渠道进价',
        '正式向企业客户发送对客方案报价',
      ],
    },
  }
}

export function applyMarkupToApproval(approval) {
  if (approval?.kind !== 'sales_quote' || !approval.channelQuote?.lines) return
  approval.lines = calcMarkedLines(approval.channelQuote.lines, approval.markupPercent)
}

export async function preparePlaceOrderSkill(ctx, { project }) {
  const candidate = project?.actions?.includes('placeOrder')
    ? project
    : ctx.projects.value.find(item => item.actions?.includes('placeOrder'))
  if (!candidate) {
    return { message: '当前没有「客户已确认报价、待正式下单」的项目。请先等待客户确认对客报价。' }
  }
  const detail = await ctx.repo.get(candidate.id, ctx.options)
  if (!detail.actions?.includes('placeOrder')) {
    return { message: `项目「${detail.title}」当前不可执行正式下单（可能报价已过期或状态已变更）。`, project: detail }
  }
  const salesQuote = detail.salesQuotes?.at(-1)
  const channelQuote = detail.channelQuotes?.at(-1)
  if (!salesQuote || !channelQuote) {
    return { message: '未能读取到配对的有效对客报价或渠道报价。', project: detail }
  }
  return {
    approval: {
      kind: 'place_order',
      skillId: 'place_order',
      title: '审批确认正式采购下单',
      project: detail,
      salesQuote,
      channelQuote,
      stepsPreview: [
        `核验客户已确认报价（${salesQuote.id}）与渠道报价（${channelQuote.id}）有效性`,
        '锁定双端成交版本并生成正式采购订单，通知厂商供货',
      ],
    },
  }
}

export async function prepareAcceptanceSkill(ctx, { route, project }) {
  const candidate = project?.actions?.includes('implement')
    ? project
    : ctx.projects.value.find(item => item.actions?.includes('implement'))
  if (!candidate) {
    return { message: '当前没有处于「待代理商实施 / 待整改」阶段的项目。' }
  }
  const detail = await ctx.repo.get(candidate.id, ctx.options)
  if (!detail.actions?.includes('implement')) {
    return { message: `项目「${detail.title}」当前状态（${detail.stageLabel}）暂不可提交实施验收。`, project: detail }
  }
  return {
    approval: {
      kind: 'submit_acceptance',
      skillId: 'submit_acceptance',
      title: '审批提交项目实施验收',
      project: detail,
      note: route.slots?.note || '已完成软件环境部署、权限初始化、关键用户培训及期初业务数据核对，请客户验收。',
      stepsPreview: [
        `校验项目「${detail.title}」供货与实施前置状态`,
        '登记实施交付说明并将项目流转至企业客户验收',
      ],
    },
  }
}

function matchDemoPartners(criteria, partners = []) {
  if (!criteria.category || criteria.category === 'other') return []
  return partners.filter(partner => {
    if (partner.category !== criteria.category) return false
    return ['source', 'target'].every(key => !criteria[key] || !partner[key] || criteria[key] === partner[key])
  })
}

export async function queryVisitPartners(ctx, criteria) {
  try {
    if (ctx.demo) {
      const desk = await ctx.repo.opportunityDesk()
      return matchDemoPartners(criteria, desk.partners)
    }
    const res = await ctx.post('/api/channel/v1/opportunities/preview', criteria)
    return res?.matches ?? []
  } catch {
    return []
  }
}

export async function prepareVisitOpportunitySkill(ctx, { text, route, project }) {
  if (!ctx.projects.value.length) {
    return { message: '当前暂无可关联的客户项目，请先建立客户采购项目后再登记附加商机。' }
  }
  const target = project ?? ctx.projects.value[0]
  const parsed = splitVisit(text)
  const additional = parsed.additional || route.slots?.note || text.replace(/^记.{0,4}拜访[：:\s]*/u, '').trim()
  const suggested = suggestOpportunity(additional || text)
  const criteria = {
    category: suggested.category || 'integration',
    source: suggested.source || 'erp',
    target: suggested.target || (suggested.source === 'erp' ? 'mes' : ''),
  }
  const partners = await queryVisitPartners(ctx, criteria)
  return {
    approval: {
      kind: 'visit_opportunity',
      skillId: 'capture_visit_opportunity',
      title: '审批记拜访并沉淀团队商机',
      projectId: target.id,
      rawText: text,
      mainNote: parsed.main || target.title,
      additionalNote: additional,
      criteria,
      partners,
      categoryOptions: categoryLabels,
      systemOptions: systemLabels,
      stepsPreview: [
        '将本次客户拜访原文与主/副需求拆分记录到个人拜访台账',
        '关联主项目自动创建独立团队共享商机（不改动主订单）',
        '绑定能力与系统匹配条件以供团队跟进内部伙伴引荐',
      ],
    },
  }
}
