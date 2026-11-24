export const workspaceGroups = [
  { id: 'overview', label: '工作总览', icon: 'overview', pinned: ['customer', 'dealer', 'vendor'] },
  { id: 'clients', label: '客户经营', icon: 'customer', pinned: ['dealer'] },
  { id: 'projects', label: { customer: '采购项目', dealer: '项目协作', vendor: '渠道询价' }, icon: 'request', pinned: ['customer', 'dealer', 'vendor'] },
  { id: 'products', label: '产品管理', icon: 'catalog', pinned: ['vendor'] },
  { id: 'orders', label: { customer: '订购与交付', dealer: '订购结算', vendor: '订单与结算' }, icon: 'order', pinned: ['customer', 'dealer', 'vendor'] },
  { id: 'support', label: '服务支持', icon: 'delivery', pinned: ['dealer'] },
  { id: 'tools', label: '工具与资料', icon: 'product' },
  { id: 'account', label: '账号与组织', icon: 'permission' },
]

const pages = [
  { id: 'overview', label: '工作总览', icon: 'overview', group: 'overview' },
  { id: 'business-overview', label: '经营看板', icon: 'overview', group: 'clients', roles: ['dealer'], source: 'operations' },
  { id: 'customers', label: '客户档案', icon: 'customer', group: 'clients', roles: ['dealer'], source: 'operations' },
  { id: 'visits', label: '记拜访', icon: 'request', group: 'clients', roles: ['dealer'], shortcut: ['dealer'] },
  { id: 'opportunities', label: '商机拾遗', icon: 'workflow', group: 'clients', roles: ['dealer'], shortcut: ['dealer'] },
  { id: 'renewals', label: '续费跟进', icon: 'time', group: 'clients', roles: ['dealer'], source: 'operations', shortcut: ['dealer'] },
  { id: 'promotion', label: '推广获客二维码', icon: 'workflow', group: 'clients', roles: ['dealer'], source: 'retail' },
  { id: 'projects', label: { customer: '我的采购项目', dealer: '客户项目', vendor: '渠道询价' }, icon: 'request', group: 'projects', mode: 'live', shortcut: ['customer', 'vendor'] },
  { id: 'ledger', label: { customer: '我的采购项目', dealer: '客户项目台账', vendor: '渠道询价队列' }, icon: 'request', group: 'projects', mode: 'demo', shortcut: ['customer', 'vendor'] },
  { id: 'requirement', label: 'AI 需求解析', icon: 'request', group: 'projects', roles: ['dealer'], mode: 'live', shortcut: ['dealer'] },
  { id: 'tickets', label: '项目待办', icon: 'request', group: 'projects', roles: ['dealer'], source: 'utility' },
  { id: 'catalog', label: '产品目录', demoLabel: '软件目录', icon: 'catalog', group: 'products', roles: ['dealer', 'vendor'], shortcut: ['vendor'] },
  { id: 'pricing', label: '订购价格与佣金', icon: 'catalog', group: 'products', roles: ['vendor'], source: 'retail', mode: 'live' },
  { id: 'retail-orders', label: { customer: '厂商订购订单', dealer: '厂商直收订单' }, icon: 'order', group: 'orders', roles: ['customer', 'dealer'], source: 'retail', shortcut: ['customer'] },
  { id: 'manufacturer-receipts', label: '直收订单与收款', icon: 'order', group: 'orders', roles: ['vendor'], source: 'retail', shortcut: ['vendor'] },
  { id: 'assisted', label: '代客下单', icon: 'order', group: 'orders', roles: ['dealer'], source: 'retail', shortcut: ['dealer'] },
  { id: 'orders', label: '订购项目', icon: 'order', group: 'orders', roles: ['dealer'], source: 'utility' },
  { id: 'delivery', label: { customer: '交付与验收', dealer: '订单与交付', vendor: '订单与交付' }, icon: 'delivery', group: 'orders', mode: 'demo', shortcut: ['customer'] },
  { id: 'commission', label: '佣金结算', icon: 'quote', group: 'orders', roles: ['dealer'], source: 'retail' },
  { id: 'manufacturer-settlements', label: '代理商佣金结算', icon: 'quote', group: 'orders', roles: ['vendor'], source: 'retail' },
  { id: 'finance', label: '订单金额核对', icon: 'quote', group: 'orders', roles: ['dealer'], source: 'utility' },
  { id: 'funds-overview', label: '资金总览', icon: 'quote', group: 'orders', roles: ['dealer'], source: 'operations' },
  { id: 'funds-transactions', label: '交易明细', icon: 'order', group: 'orders', roles: ['dealer'], source: 'operations' },
  { id: 'funds-invoices', label: '订单开票', icon: 'request', group: 'orders', roles: ['dealer'], source: 'operations' },
  { id: 'funds-claims', label: '服务费报销', icon: 'quote', group: 'orders', roles: ['dealer'], source: 'operations' },
  { id: 'application-queue', label: '申请台账', icon: 'request', group: 'orders', roles: ['dealer'], source: 'operations' },
  { id: 'vendor-finance', label: '财务办理', icon: 'quote', group: 'orders', roles: ['vendor'], source: 'operations' },
  { id: 'checkout', label: '确认厂商订单', icon: 'order', group: 'orders', roles: ['customer'], source: 'retail', mode: 'live', direct: true },
  { id: 'support', label: '实施与验收', icon: 'delivery', group: 'support', roles: ['dealer'], source: 'utility' },
  { id: 'service-tickets', label: '服务工单', icon: 'request', group: 'support', roles: ['dealer'], source: 'operations' },
  { id: 'vendor-tickets', label: '渠道工单', icon: 'request', group: 'support', roles: ['vendor'], source: 'operations' },
  { id: 'academy', label: '渠道业务指南', icon: 'catalog', group: 'tools', roles: ['dealer'], source: 'utility' },
  { id: 'platform', label: '接口目录', icon: 'workflow', group: 'tools', roles: ['dealer'], source: 'utility' },
  { id: 'tools', label: '报价试算', icon: 'product', group: 'tools', roles: ['dealer'], source: 'utility' },
  { id: 'account', label: '账号与组织', icon: 'permission', group: 'account', mode: 'live' },
  { id: 'staff', label: '当前经办身份', icon: 'permission', group: 'account', roles: ['dealer'], source: 'utility', mode: 'demo' },
]

const forRole = (value, role) => typeof value === 'string' ? value : value[role]

export function workspacePages(role, { demo = false, retailDemo = false, directView = '' } = {}) {
  if (!['customer', 'dealer', 'vendor'].includes(role)) return []
  return pages.filter(page => {
    if (page.roles && !page.roles.includes(role)) return false
    if (page.mode && page.mode !== (demo ? 'demo' : 'live')) return false
    if (page.direct && page.id !== directView) return false
    if (page.source === 'operations') return demo && retailDemo
    if (page.source === 'retail' && demo) return retailDemo && role !== 'customer'
    return true
  }).map(page => ({ ...page, label: forRole(demo && page.demoLabel ? page.demoLabel : page.label, role) }))
}

export function groupWorkspacePages(navigation, role) {
  if (!role) {
    return navigation.map(page => ({ id: page.id, label: page.label, pinned: true, pages: [page] }))
  }
  return workspaceGroups.map(group => ({
    ...group,
    label: forRole(group.label, role),
    pinned: Boolean(group.pinned?.includes(role)),
    pages: navigation.filter(page => page.group === group.id && !page.direct),
  })).filter(group => group.pages.length)
}

export function workspaceShortcuts(navigation, role) {
  return navigation.filter(page => !page.direct && page.shortcut?.includes(role))
}

export function isRetailDemoPage(page) {
  return page.source === 'operations' || page.source === 'retail'
}
