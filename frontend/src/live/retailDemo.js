import { requestApi } from '../api.js'

export const retailDemoApi = '/api/retail-demo'
export const demoWorkspace = '/experience/retail'
export const demoStore = '/experience/retail-store'
export const retailDemoEnabled = import.meta.env.DEV || import.meta.env.MODE === 'retail-demo'

export const retailDemoNavigation = {
  dealer: [
    { id: 'business-overview', label: '经营看板', icon: 'overview' },
    { id: 'customers', label: '客户档案', icon: 'customer' },
    { id: 'renewals', label: '续费跟进', icon: 'time' },
    { id: 'promotion', label: '推广获客二维码', icon: 'workflow' },
    { id: 'assisted', label: '代客下单', icon: 'order' },
    { id: 'retail-orders', label: '厂商直收订单', icon: 'order' },
    { id: 'commission', label: '佣金结算', icon: 'quote' },
    { id: 'funds-overview', label: '资金总览', icon: 'quote' },
    { id: 'funds-transactions', label: '交易明细', icon: 'order' },
    { id: 'funds-invoices', label: '订单开票', icon: 'request' },
    { id: 'funds-claims', label: '服务费报销', icon: 'quote' },
    { id: 'application-queue', label: '申请台账', icon: 'request' },
    { id: 'service-tickets', label: '服务工单', icon: 'request' },
  ],
  vendor: [
    { id: 'manufacturer-receipts', label: '直收订单与收款', icon: 'order' },
    { id: 'manufacturer-settlements', label: '代理商佣金结算', icon: 'quote' },
    { id: 'vendor-finance', label: '财务办理', icon: 'quote' },
    { id: 'vendor-tickets', label: '渠道工单', icon: 'request' },
  ],
}

export function retailDemoLocation(role, view) {
  const views = retailDemoNavigation[role]
  const target = views.find(item => item.id === view)?.id ?? (role === 'dealer' ? 'promotion' : 'manufacturer-receipts')
  return '/experience/' + role + '?' + new URLSearchParams({ view: target })
}

export function retailDemoCounterpart(role, view) {
  if (view === 'vendor-finance') return retailDemoLocation('dealer', 'funds-overview')
  if (view === 'vendor-tickets') return retailDemoLocation('dealer', 'service-tickets')
  if (view.startsWith('funds-') || view === 'application-queue') return retailDemoLocation('vendor', 'vendor-finance')
  if (view === 'service-tickets') return retailDemoLocation('vendor', 'vendor-tickets')
  if (role === 'vendor') return retailDemoLocation('dealer', view === 'manufacturer-settlements' ? 'commission' : 'retail-orders')
  return retailDemoLocation('vendor', view === 'commission' ? 'manufacturer-settlements' : 'manufacturer-receipts')
}

export async function requestRetailDemo(path, options) {
  try {
    return await requestApi(retailDemoApi + path, options)
  } catch (error) {
    if (error.code) error.message = error.code
    else if (error.status === 0) error.message = '无法连接演示服务，请检查连接后重试。'
    throw error
  }
}
