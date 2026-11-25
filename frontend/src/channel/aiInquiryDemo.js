import { roles } from './domain.js'
import { routeLocalWorkspace } from '../live/aiSkillCatalog.js'

export const aiDemoProject = {
  title: '海川精工 · ERP 采购',
  customerName: roles.customer.company,
  requirement: '我们是一家 120 人的制造企业，计划采购 ERP，需要财务和库存模块，希望云部署。ERP 授权人数还没定，后续可能要接 MES。',
}

export function parseDemoRequirement(text) {
  const count = value => {
    const number = Number(value)
    return number >= 1 && number <= 1000000 ? number : null
  }
  const employeeCount = count(text.match(/(\d+)\s*人(?:的|规模|左右|企业|公司|员工)|(?:员工人数|员工)\s*[:：]?\s*(\d+)\s*人/)?.slice(1).find(Boolean))
  const licenseCount = count(text.match(/(?:授权|账号|席位)\s*[:：]?\s*(\d+)\s*(?:席|个|位|人)?|(\d+)\s*(?:个|位|名|席)?\s*(?:授权|账号|席位)/)?.slice(1).find(Boolean))
  const modules = ['财务', '库存', '生产排程', '工序', '质检', '审批', '合同', '文档'].filter(module => text.includes(module))
  if (/采购(?:管理|模块)|[、和与及]采购|采购[、和与及]/.test(text)) modules.push('采购')
  const deployment = ['私有化部署', '私有部署', '本地部署', '云部署', 'SaaS'].find(mode => text.includes(mode)) ?? ''
  return {
    enterprise_type: text.includes('制造企业') ? '制造业' : '',
    employee_count: employeeCount,
    required_modules: modules,
    deployment_mode: deployment === '私有化部署' ? '私有部署' : deployment,
    license_count: licenseCount,
    follow_up_questions: [
      ...(!licenseCount ? ['需要为多少位用户开通软件授权？'] : []),
      ...(text.includes('MES') && text.includes('可能') ? ['MES 接口是否纳入本次询价？'] : []),
    ],
  }
}

export function routeDemoWorkspace(text) {
  return routeLocalWorkspace(text)
}

export function matchDemoProducts(products, requirement) {
  const matches = products.filter(product => product.published).map(product => {
    const checks = []
    const check = (value, met, detail) => checks.push({ value, status: met ? 'met' : 'missing', detail })
    for (const module of requirement.required_modules) {
      const met = product.modules.includes(module)
      check(module, met, met ? '目录列出此模块' : '目录未列出此模块')
    }
    if (requirement.deployment_mode) {
      const met = product.deployment.split('/').some(mode => mode.trim() === requirement.deployment_mode)
      check(requirement.deployment_mode, met, met ? '目录列出此部署方式' : '目录未列出此部署方式')
    }
    if (requirement.version) checks.push({ value: requirement.version, status: 'unknown', detail: '演示目录未提供版本号，请向厂商核实' })
    checks.push({
      value: '授权席位', status: 'unknown',
      detail: requirement.license_count ? '目录未给出席位范围，请向厂商核实' : '请先确认授权用户数',
    })
    return {
      product: { ...product, manufacturerName: roles.manufacturer.company },
      status: checks.some(item => item.status === 'missing') ? 'mismatch' : 'needs_confirmation',
      checks,
      sources: [{
        title: product.name, reference: '演示软件目录',
        excerpt: product.modules.join('、') + ' · ' + product.deployment + ' · ' + product.license,
      }],
    }
  })
  return matches.sort((left, right) => Number(left.status === 'mismatch') - Number(right.status === 'mismatch'))
}
