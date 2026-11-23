import { requestApi } from '../api.js'

export const retailDemoApi = '/api/retail-demo'
export const demoWorkspace = '/experience/retail'
export const demoStore = '/experience/retail-store'
export const retailDemoEnabled = import.meta.env.DEV || import.meta.env.MODE === 'retail-demo'

export const retailDemoNavigation = {
  dealer: [
    { id: 'promotion', label: '推广获客二维码', icon: 'workflow' },
    { id: 'assisted', label: '代客下单', icon: 'order' },
    { id: 'retail-orders', label: '厂商直收订单', icon: 'order' },
    { id: 'commission', label: '佣金结算', icon: 'quote' },
  ],
  vendor: [
    { id: 'manufacturer-receipts', label: '直收订单与收款', icon: 'order' },
    { id: 'manufacturer-settlements', label: '代理商佣金结算', icon: 'quote' },
  ],
}

export function retailDemoLocation(role, view) {
  const views = retailDemoNavigation[role]
  const target = views.find(item => item.id === view) ?? views[0]
  return '/experience/' + role + '?' + new URLSearchParams({ view: target.id })
}

export function retailDemoCounterpart(role, view) {
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
