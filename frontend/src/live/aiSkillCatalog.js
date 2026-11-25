import { parseDemoRequirement } from '../channel/aiInquiryDemo.js'

export const workspaceSkills = Object.freeze({
  match_products: '产品匹配选型',
  prepare_inquiry: '准备采购询价',
  submit_inquiry: '一键建项与厂商询价',
  sales_quote: '编制并发送对客报价',
  place_order: '代客确认正式下单',
  submit_acceptance: '提交项目实施验收',
  capture_visit_opportunity: '记拜访并转团队商机',
  assisted_order: '代关联客户直购下单',
  remind_commission: '催办待结算佣金',
  apply_invoice: '申请已收款订单开票',
  create_ticket: '提交厂商服务工单',
  project_status: '查询项目进展与待办',
})

export const skillShortcuts = Object.freeze([
  {
    id: 'submit_inquiry',
    label: '一键建项并询价',
    icon: 'request',
    description: '提取采购条件、匹配产品、建项并向厂商发询价',
    demoPrompt: '我们是一家 120 人的制造企业，计划采购 ERP，需要财务和库存模块，希望云部署，授权席位 25 人。请直接发起厂商询价。',
    livePrompt: '根据当前项目的采购需求匹配产品并发起厂商询价',
  },
  {
    id: 'sales_quote',
    label: '发送对客报价',
    icon: 'quote',
    description: '基于渠道底价按加价比例生成对客报价并发送',
    demoPrompt: '给财务共享中心项目编制对客报价，按渠道价加价 25%，10 天交付并发给客户',
    livePrompt: '帮我给当前已拿到渠道报价的项目加价 25% 生成并发送对客报价',
  },
  {
    id: 'place_order',
    label: '确认正式下单',
    icon: 'order',
    description: '核对客户已确认报价与上游有效期后一键下单',
    demoPrompt: '帮我给已确认对客报价的项目确认下单',
    livePrompt: '帮我给客户已确认报价的项目正式下单',
  },
  {
    id: 'submit_acceptance',
    label: '提交实施验收',
    icon: 'delivery',
    description: '登记部署、培训与迁移说明并发起客户验收',
    demoPrompt: '帮我提交实施验收：已完成系统部署、关键用户培训与期初库存核对，请客户验收',
    livePrompt: '帮我给已供货项目提交实施验收：已完成部署与关键用户培训，请验收',
  },
  {
    id: 'capture_visit_opportunity',
    label: '记拜访转商机',
    icon: 'workflow',
    description: '拆分拜访中的附加需求、预匹配伙伴并沉淀商机',
    demoPrompt: '记一条拜访：今天回访协同办公项目确认了审批流，另外客户还需要把 ERP 与 MES 生产排程接口打通',
    livePrompt: '记拜访：刚和客户核对了主项目进度，另外客户还需要 ERP 对接 MES 接口开发服务',
  },
  {
    id: 'assisted_order',
    label: '代客直购下单',
    icon: 'catalog',
    description: '选择上架直购软件与份数，一键生成客户确认单',
    demoPrompt: '帮我代客直购下单 2 套云财务',
    livePrompt: '帮我给关联客户代客下单 1 套直购产品',
  },
  {
    id: 'remind_commission',
    label: '催办待结佣金',
    icon: 'time',
    description: '筛选可催办的直购订单并向厂商登记结算催办',
    demoPrompt: '帮我催办待结算的直购订单佣金',
    livePrompt: '帮我催办当前待结算的直购佣金',
  },
  {
    id: 'apply_invoice',
    label: '申请订单开票',
    icon: 'quote',
    description: '定位已收款未开票的直购订单并提交开票抬头',
    demoPrompt: '帮我为已付款的直购订单申请开票',
    livePrompt: '帮我给已收款的直购订单申请开票',
  },
  {
    id: 'create_ticket',
    label: '提交服务工单',
    icon: 'delivery',
    description: '关联直购订单或合作厂商一键发起售后工单',
    demoPrompt: '帮我提交一张授权与交付服务工单：客户付款后尚未收到云财务激活通知，请协助核查',
    livePrompt: '帮我向合作厂商提交服务工单：授权激活与初始化配置需要技术支持',
  },
])

function parseNumber(match, min, max) {
  if (!match) return undefined
  const value = Number(match)
  return Number.isFinite(value) && value >= min && value <= max ? value : undefined
}

export function extractLocalSlots(text) {
  const slots = {}
  const markup = text.match(/(?:加价|上浮|毛利|溢价)\s*(\d{1,3}(?:\.\d{1,2})?)\s*%/)
  if (markup) slots.markup_percent = parseNumber(markup[1], 0, 500)

  const delivery = text.match(/(\d{1,4})\s*(?:个工作)?(?:天|日)(?:内)?(?:交付|上线|实施|完成)/)
  if (delivery) slots.delivery_days = parseNumber(delivery[1], 1, 3650)

  const valid = text.match(/有效期?\s*(\d{1,3})\s*天/)
  if (valid) slots.valid_days = parseNumber(valid[1], 1, 365)

  const qty = text.match(/(?:下单|订购|买|购买)\s*(\d{1,4})\s*(?:套|份|个|席|年)|(\d{1,4})\s*(?:套|份)(?:直购|云财务|软件|产品)/)
  if (qty) slots.quantity = parseNumber(qty[1] || qty[2], 1, 10000)

  const colonBody = text.match(/[：:]\s*(.{4,400})$/)?.[1]?.trim()
  if (colonBody) slots.note = colonBody.slice(0, 500)

  const titleMatch = text.match(/抬头(?:为|是|填)?\s*[「“"']?([^，,。；;”"'\n]{2,80})[」”"']?/)
  if (titleMatch) slots.invoice_title = titleMatch[1].trim()

  if (/授权|激活|交付|开通/.test(text)) slots.ticket_category = '授权与交付'
  else if (/结算|佣金|发票|账单|收款/.test(text)) slots.ticket_category = '订单与结算'
  else if (/报错|使用|登录|配置|功能/.test(text)) slots.ticket_category = '产品使用'

  return Object.keys(slots).length ? slots : undefined
}

export function routeLocalWorkspace(text) {
  const clean = text.trim()
  const slots = extractLocalSlots(clean)
  const isQuestionAboutSending = /(?:不|别|无需|先别|不用|暂不).{0,6}(?:发送|提交|发给)|(?:是否|怎么|如何).{0,6}(?:发送|提交)|[？?]$/.test(clean)

  if (/工单|报修|售后支持|技术支持/.test(clean) && !/项目待办/.test(clean)) {
    return { intent: 'create_ticket', use_project_requirement: true, slots }
  }
  if (/开票|发票/.test(clean) && !/报销/.test(clean)) {
    return { intent: 'apply_invoice', use_project_requirement: true, slots }
  }
  if (/催.{0,6}(?:佣金|结算)|(?:佣金|分佣).{0,6}催/.test(clean)) {
    return { intent: 'remind_commission', use_project_requirement: true, slots }
  }
  if (/代客.{0,6}(?:直购|下单)|直购.{0,6}下单|(?:下单|订购|购买).{0,8}云财务/.test(clean)) {
    return { intent: 'assisted_order', use_project_requirement: true, slots }
  }
  if (/记.{0,4}拜访|拜访.{0,6}(?:记录|客户)|回访.{0,8}另外|(?:另外|额外|还需).{0,16}(?:接口|对接|MES|WMS|OA|CRM|插件)/i.test(clean) &&
      !/询价草稿|发送.{0,6}询价/.test(clean)) {
    return { intent: 'capture_visit_opportunity', use_project_requirement: false, slots }
  }
  if (/(?:提交|发起|登记).{0,6}(?:实施)?验收|实施完成.{0,8}验收/.test(clean)) {
    return { intent: 'submit_acceptance', use_project_requirement: true, slots }
  }
  if (/(?:对客|销售|给客户).{0,6}报价|加价\s*\d+\s*%|生成.{0,4}报价/.test(clean) && !/渠道报价/.test(clean)) {
    return { intent: 'sales_quote', use_project_requirement: true, slots }
  }
  if (/(?:确认|正式|生成).{0,4}下单|报价已确认.{0,6}下单/.test(clean) && !/代客|直购/.test(clean)) {
    return { intent: 'place_order', use_project_requirement: true, slots }
  }
  if (/进展|进度|到哪|状态|待办/.test(clean)) {
    return { intent: 'project_status', use_project_requirement: true, slots }
  }

  const extracted = parseDemoRequirement(clean)
  const hasExplicitRequirement = Boolean(
    extracted.employee_count || extracted.license_count || extracted.enterprise_type ||
    extracted.required_modules.length || extracted.deployment_mode,
  )
  const sends = /(?:发送|提交|发给|发起|直接发).{0,12}询价|询价.{0,12}(?:发给|发送|提交)/.test(clean) && !isQuestionAboutSending
  const matchOnly = /匹配|推荐|找.{0,4}产品|选型/.test(clean) && !/草稿|询价/.test(clean)
  const prepare = /询价|草稿|采购|需求|ERP|MES/i.test(clean)

  const intent = sends
    ? 'submit_inquiry'
    : matchOnly
      ? 'match_products'
      : prepare
        ? 'prepare_inquiry'
        : 'help'

  return {
    intent,
    use_project_requirement: !hasExplicitRequirement,
    slots,
  }
}
