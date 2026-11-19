const apiMessages = {
  invalid_credentials: '用户名或密码不正确。',
  authentication_required: '登录已失效，请重新登录。',
  forbidden: '当前组织无权执行此操作。',
  not_found: '这条记录不存在或已不可见。',
  invalid_product: '请检查产品名称、版本和说明。',
  invalid_requirement: '请填写 1–2000 字的需求。',
  invalid_request: '请检查输入后再试。',
  revision_conflict: '项目已被协作方更新，请重新核对后办理。',
  idempotency_conflict: '此请求的内容已变更，请重新核对后提交。',
  invalid_transition: '项目状态已变化，当前操作暂不可办理。',
  organization_link_required: '请选择已关联的组织，或联系维护者建立代理关系。',
  opportunity_exists: '此项目已记录相同的附加需求，请刷新商机台账查看。',
  dependencies_unavailable: '服务依赖暂不可用，请稍后重试。',
  model_unavailable: 'AI 服务暂不可用，请稍后重试。',
  model_response_error: 'AI 未返回可用结果，请调整需求或重试。',
  invalid_sheet_request: '表格参数无效，或请求超出本次处理范围。',
  invalid_sheet_cell: '批量编辑包含无效单元格，未保存本次改动。',
  sheet_not_found: '表格不存在或已不可见。',
  sheet_write_forbidden: '当前账号没有表格编辑权限。',
  sheet_revision_conflict: '其他成员已更新表格，请核对后重试。',
  sheet_request_conflict: '同一请求编号已用于不同的表格改动。',
  sheet_request_too_large: '表格请求过大。',
  sheet_storage_unavailable: '表格存储暂不可用，请稍后重试。',
  sheet_search_busy: '表格查找暂时繁忙，请稍后重试。',
}

export class ApiError extends Error {
  constructor(status, code, fallback) {
    super(apiMessages[code] || fallback || `请求失败（${status}），请稍后重试。`)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export async function requestApi(path, { method = 'GET', token, body, signal, timeout = 12000 } = {}) {
  const headers = { Accept: 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const deadline = AbortSignal.timeout(timeout)
  const requestSignal = signal ? AbortSignal.any([signal, deadline]) : deadline

  let response, envelope
  try {
    response = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: requestSignal,
      cache: 'no-store',
      redirect: 'error',
    })
    envelope = await response.json().catch((error) => {
      if (requestSignal.aborted) throw error
      return null
    })
  } catch (error) {
    if (signal?.aborted) throw error
    const message = deadline.aborted ? '请求超时，请确认结果后再重试。' : '无法连接后端，请确认 Go 服务已启动。'
    throw new ApiError(0, '', message)
  }

  if (!response.ok) {
    const fallback = response.status >= 500 && !envelope ? '无法连接后端，请确认 Go 服务已启动。' : undefined
    throw new ApiError(response.status, envelope?.message, fallback)
  }
  if (envelope?.code !== response.status || !Object.hasOwn(envelope, 'data')) {
    throw new ApiError(response.status, '', '服务返回了无法识别的数据，请检查后端配置。')
  }
  return envelope.data
}
