import { requestApi } from '../api.js'

export const retailDemoApi = '/api/retail-demo'
export const demoWorkspace = '/experience/retail'
export const demoStore = '/experience/retail-store'

export async function requestRetailDemo(path, options) {
  try {
    return await requestApi(retailDemoApi + path, options)
  } catch (error) {
    if (error.code) error.message = error.code
    else if (error.status === 0) error.message = '无法连接演示服务，请检查连接后重试。'
    throw error
  }
}
