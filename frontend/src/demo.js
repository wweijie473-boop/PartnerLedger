import { demoCompanies } from './demoCompanies.js'

export const roles = {
  dealer: { label: '经销商', company: demoCompanies.dealer },
  vendor: { label: '厂家', company: demoCompanies.vendor },
}

export const products = [
  { id: 'ERP-PRO', name: '渠智 ERP · 专业版', modules: '财务 · 采购 · 库存 · 销售', deployment: '私有部署 / 云部署', license: '按授权用户计费' },
  { id: 'ERP-STD', name: '渠智 ERP · 标准版', modules: '财务 · 采购 · 销售', deployment: '云部署', license: '按授权用户计费' },
  { id: 'ERP-MFG', name: '渠智 ERP · 制造模块', modules: '生产计划 · 物料 · 工序', deployment: '随专业版部署', license: '模块加购' },
]

export const statuses = {
  draft: ['未提交', 'neutral'],
  submitted: ['待厂家报价', 'warning'],
  supplement: ['待补充资料', 'warning'],
  quoted: ['已收到报价', 'info'],
  revision: ['待修改报价', 'warning'],
  accepted: ['报价已确认', 'success'],
  ordered: ['交付中', 'info'],
  delivered: ['交付完成', 'success'],
  withdrawn: ['已撤回', 'neutral'],
}

export const deliverySteps = ['授权开通', '环境部署', '数据迁移', '上线交接']

const now = new Date()
const expiry = new Date(now)
expiry.setDate(expiry.getDate() + 30)
export const defaultExpiry = expiry.toLocaleDateString('sv-SE')

export function isValidQuote(quote) {
  return quote && quote.expiry >= new Date().toLocaleDateString('sv-SE')
}

export function seedCases() {
  return [
    { id: 'RFQ-1003', title: '华东精工 ERP 升级', product: 'ERP-PRO', seats: 25, description: '需要财务、采购和库存一体化，优先考虑私有部署。', state: 'quoted', date: '11-18', quotes: [{ amount: 48600, expiry: defaultExpiry, notes: '含 25 席授权、上线部署及培训。' }], progress: 0 },
    { id: 'RFQ-1002', title: '恒源商贸财务系统', product: 'ERP-STD', seats: 12, description: '财务与销售协同，使用云部署。', state: 'submitted', date: '11-17', quotes: [], progress: 0 },
    { id: 'RFQ-1001', title: '远川机械协同平台', product: 'ERP-PRO', seats: 30, description: '统一采购和仓库台账。', state: 'ordered', date: '11-16', quotes: [{ amount: 65000, expiry: defaultExpiry, notes: '含私有部署和数据迁移。' }], progress: 2 },
  ]
}

export const money = new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' })
export const orderId = (record) => record.id.replace('RFQ', 'SO')
export const productName = (id) => products.find((product) => product.id === id)?.name ?? '待确认产品'
