export function inquirySummary(requirement) {
  const fields = [
    ['企业类型', requirement.enterprise_type],
    ['员工人数', requirement.employee_count],
    ['所需模块', requirement.required_modules.join('、')],
    ['部署要求', requirement.deployment_mode],
    ['版本要求', requirement.version],
    ['授权席位', requirement.license_count ?? '待确认'],
  ]
  return fields.filter(([, value]) => value).map(([label, value]) => label + '：' + value).join('；')
}
