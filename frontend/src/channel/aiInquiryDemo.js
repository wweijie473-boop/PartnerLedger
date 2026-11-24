import { roles } from './domain.js'

export const aiDemoProject = {
  title: '海川精工 · ERP 采购',
  customerName: roles.customer.company,
  requirement: '我们是一家 120 人的制造企业，计划采购 ERP，需要财务和库存模块，希望云部署。ERP 授权人数还没定，后续可能要接 MES。',
}

export function parseDemoRequirement(text) {
  if (text !== aiDemoProject.requirement) {
    throw new Error('本地演示可解析内置样例。其他需求可以手工填写字段后继续匹配。')
  }
  return {
    enterprise_type: '制造业',
    employee_count: 120,
    required_modules: ['财务', '库存'],
    deployment_mode: '云部署',
    license_count: null,
    follow_up_questions: ['需要为多少位用户开通 ERP？', 'MES 接口是否纳入本次询价？'],
  }
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
