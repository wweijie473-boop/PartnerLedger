import { requestApi } from '../api.js'
import { isRetailDemoPage, workspacePages } from '../workspaceNavigation.js'

export const retailDemoApi = '/api/retail-demo'
export const demoWorkspace = '/experience/retail'
export const demoStore = '/experience/retail-store'
export const retailDemoEnabled = import.meta.env.MODE === 'retail-demo'

export function retailDemoLocation(role, view) {
  const views = workspacePages(role, { demo: true, retailDemo: true }).filter(isRetailDemoPage)
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
