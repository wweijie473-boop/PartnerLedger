import { entrances as portalEntrances } from '../entrances.js'

const details = {
  dealer: { english: 'Channel partner', steps: ['整理客户需求', '编制销售报价', '跟进实施验收'] },
  vendor: { english: 'Software vendor', steps: ['发布软件产品', '处理渠道询价', '更新授权进度'] },
  customer: { english: 'Enterprise customer', steps: ['提交业务需求', '确认采购方案', '验收项目成果'] },
}
export const entrances = Object.fromEntries(portalEntrances.map(entry => [entry.role, { ...entry, ...details[entry.role] }]))

export function organizationRole(profile, organization) {
  if (!profile?.username || !organization?.id || !organization.name ||
      profile.organization?.id !== organization.id || profile.organization?.kind !== organization.kind) return null
  if (organization.kind === 'MANUFACTURER') return 'vendor'
  if (organization.kind === 'RESELLER') return 'dealer'
  return null
}
