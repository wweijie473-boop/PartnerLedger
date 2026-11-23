export const dealerSections = [
  { id: 'overview', label: '数据运营', icon: 'overview', views: ['overview'] },
  { id: 'opportunities', label: '获取商机', icon: 'workflow', views: ['opportunities', 'promotion'] },
  { id: 'clients', label: '管理客户', icon: 'customer', views: ['projects', 'ledger', 'visits'] },
  { id: 'orders', label: '下单订购', icon: 'order', views: ['orders', 'assisted', 'retail-orders', 'delivery', 'catalog'] },
  { id: 'finance', label: '资金往来', icon: 'quote', views: ['finance', 'commission'] },
  { id: 'staff', label: '管理员工', icon: 'permission', views: ['account', 'staff'] },
  { id: 'support', label: '服务支持', icon: 'delivery', views: ['support'] },
  { id: 'academy', label: '提能学院', icon: 'catalog', views: ['academy'] },
  { id: 'platform', label: '开放平台', icon: 'workflow', views: ['platform'] },
  { id: 'tools', label: '最新工具', icon: 'product', views: ['tools', 'requirement'] },
  { id: 'tickets', label: '智能工单', icon: 'request', views: ['tickets'] },
]

const pageLabels = {
  orders: '订购项目', finance: '订单金额核对', staff: '当前经办身份', support: '实施与验收',
  academy: '渠道业务指南', platform: '接口目录', tools: '报价试算', tickets: '项目待办',
}

export const dealerPages = dealerSections.filter(s => pageLabels[s.id]).map(s => ({
  id: s.id, label: pageLabels[s.id], icon: s.icon,
}))
