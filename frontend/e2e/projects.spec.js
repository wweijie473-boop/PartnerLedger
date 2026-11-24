import { expect, test } from '@playwright/test'

const organizations = {
  dealer: { id: '10', kind: 'RESELLER', name: '示例代理商' },
  customer: { id: '20', kind: 'CUSTOMER', name: '示例客户' },
  vendor: { id: '30', kind: 'MANUFACTURER', name: '示例厂商' },
}
const projectsPath = '/api/channel/v1/projects'
const labels = { requested: '待整理需求', inquired: '待厂商处理', processing: '代理商跟进中', vendor_info: '待补询价资料' }

function projectView(project, role) {
  const { id, title, requirement, summary, revision, stage } = project
  const publicStage = role === 'customer' && stage !== 'requested' ? 'processing' : stage
  const action = role === 'dealer' && stage === 'requested' ? 'submitInquiry'
    : role === 'vendor' && stage === 'inquired' ? 'requestInfo' : ''
  return {
    id, title: role === 'vendor' ? '示例 ERP · 渠道采购项目' : title, revision,
    stage: publicStage, stageLabel: labels[publicStage], actions: action ? [action] : [],
    isTodo: Boolean(action), nextStep: labels[publicStage], timeline: [],
    ...(role === 'vendor' ? {} : { requirement, customerName: organizations.customer.name }),
    ...(role === 'customer' || !summary ? {} : { inquiry: { summary } }),
    ...(role === 'customer' || !summary ? {} : { product: '51', productName: '示例 ERP' }),
  }
}

async function mockProjectAPI(context, initial = []) {
  const projects = [...initial]
  const creates = []
  const requests = []
  const drafts = new Map()
  const draftAttempts = []
  let dropNextDraft = false
  const expired = new Set()
  let dropNextCreate = false
  await context.route('**/api/**', async route => {
    const request = route.request()
    const { pathname, searchParams } = new URL(request.url())
    const method = request.method()
    const username = request.headers().authorization?.replace('Bearer fixture-', '')
    const role = username?.split('_')[0]
    const org = organizations[role]
    const reply = (data, status = 200, message = 'success') => route.fulfill({ status, json: { code: status, message, data } })
    requests.push({ pathname, searchParams, method })

    if (pathname === '/api/auth/login') return reply({ access_token: 'fixture-' + request.postDataJSON().username, token_type: 'Bearer', expires_in: 3600 })
    if (!org || expired.has(username)) return reply(null, 401, 'authentication_required')
    if (pathname === '/api/user/profile') return reply({ id: username === 'dealer_b' ? '12' : '11', username, organization: org })
    if (pathname === '/api/org/current') return reply(org)
    if (pathname === '/api/auth/logout') return reply(null)
    if (pathname === '/api/channel/v1/organization-links') return reply({ items: [organizations[role === 'dealer' ? 'customer' : 'dealer']] })
    const product = { id: '51', name: '示例 ERP', version: '1.0', published: true }
    if (pathname === '/api/products') return reply({ items: [product], next_cursor: null })
    if (pathname === '/api/products/51') return reply(product)
    if (pathname === '/api/ai/workspace/route' && role === 'dealer') {
      return reply({ intent: 'prepare_inquiry', use_project_requirement: true })
    }
    if (pathname === '/api/ai/parse-requirement' && role === 'dealer') {
      return reply({ employee_count: 120, required_modules: ['财务'], follow_up_questions: ['需要多少授权用户？'] })
    }
    if (pathname === '/api/ai/match-products' && role === 'dealer') {
      return reply({ items: [{ product, status: 'needs_confirmation', checks: [], sources: [] }] })
    }
    if (pathname === '/api/channel/v1/opportunities/desk' && role === 'dealer') {
      const items = projects.flatMap(project => (project.opportunities ?? []).map(item => ({
        ...item, projectId: project.id, projectTitle: project.title, questions: [], matches: [], history: [],
      })))
      return reply({ items, partners: [], gaps: [], reverseMatches: [], tasks: [] })
    }
    if (pathname === projectsPath + '/workspace') {
      const visible = projects.filter(item => role !== 'vendor' || item.stage !== 'requested').map(item => projectView(item, role))
      const counts = { all: visible.length, mine: visible.filter(item => item.isTodo).length, partners: visible.filter(item => !item.isTodo).length }
      const stages = [...new Map(visible.map(item => [item.stage, { id: item.stage, label: item.stageLabel }])).values()]
      const scope = searchParams.get('scope')
      const stage = searchParams.get('stage')
      const keyword = searchParams.get('q') || ''
      const after = Number(searchParams.get('after'))
      const limit = Number(searchParams.get('limit'))
      const matches = visible.filter(item => Number(item.id) > after && (!stage || item.stage === stage) &&
        (scope === 'all' || item.isTodo === (scope === 'mine')) && item.title.includes(keyword))
      const items = matches.slice(0, limit)
      return reply({ items, counts, stages, next_cursor: matches.length > limit ? items.at(-1).id : null })
    }
    if (pathname === projectsPath && method === 'POST') {
      const input = request.postDataJSON()
      creates.push(input)
      let project = projects.find(item => item.requestId === input.requestId)
      if (!project) {
        project = { ...input, id: String(projects.length + 1), revision: 1, stage: 'requested' }
        projects.push(project)
      }
      if (dropNextCreate) { dropNextCreate = false; return route.abort('failed') }
      return reply(projectView(project, role), 201)
    }
    if (pathname === projectsPath && method === 'GET') {
      const items = projects.filter(item => role !== 'vendor' || item.stage !== 'requested').map(item => projectView(item, role))
      return reply({ items, next_cursor: null })
    }
    if (pathname.startsWith(projectsPath + '/')) {
      const [id, section, action] = pathname.slice(projectsPath.length + 1).split('/')
      const project = projects.find(item => item.id === id)
      if (!project || role === 'vendor' && project.stage === 'requested') return reply(null, 404, 'not_found')
      if (section === 'inquiry-draft') {
        if (role !== 'dealer' || project.stage !== 'requested') return reply(null, 404, 'not_found')
        const key = username + ':' + id
        if (method === 'GET') return drafts.has(key) ? reply(drafts.get(key).draft) : reply(null, 404, 'not_found')
        const input = request.postDataJSON()
        draftAttempts.push(input)
        const previous = drafts.get(key)
        if (previous?.requestId === input.requestId) return reply(previous.draft, 201)
        if (input.revision !== project.revision) return reply(null, 409, 'revision_conflict')
        const draft = {
          projectId: id, projectRevision: input.revision, productId: input.productId, requirement: input.requirement,
          summary: '员工人数：120；所需模块：财务；授权席位：待确认',
        }
        drafts.set(key, { requestId: input.requestId, draft })
        if (dropNextDraft) { dropNextDraft = false; return route.abort('failed') }
        return reply(draft, 201)
      }
      if (section === 'opportunities') {
        if (role !== 'dealer') return reply(null, 403, 'forbidden')
        const before = searchParams.get('before')
        const matches = (project.opportunities ?? []).filter(item => !before || Number(item.id) < Number(before))
        return reply({ items: matches.slice(0, 1), next_cursor: matches.length > 1 ? matches[0].id : null })
      }
      if (section === 'actions') {
        const input = request.postDataJSON()
        if (input.revision !== project.revision) return reply(null, 409, 'revision_conflict')
        project.revision += 1
        project.stage = action === 'submitInquiry' ? 'inquired' : 'vendor_info'
        if (action === 'submitInquiry') project.summary = input.payload.summary
      }
      return reply(projectView(project, role))
    }
    return reply(null, 404, 'not_found')
  })
  return {
    projects, creates, requests, draftAttempts,
    expire: username => expired.add(username), restore: username => expired.delete(username),
    dropCreate: () => { dropNextCreate = true }, dropDraft: () => { dropNextDraft = true },
  }
}

async function login(page, username, open = true) {
  if (open) await page.goto('/login/' + username.split('_')[0] + '?view=projects')
  await page.getByLabel('用户名').fill(username)
  await page.getByLabel('密码').fill('playwright-only-password')
  await page.getByRole('button', { name: '登录', exact: true }).click()
}

async function createProject(page, title) {
  await page.getByRole('button', { name: '建立项目', exact: true }).click()
  const form = page.getByRole('dialog')
  await form.getByLabel('项目名称').fill(title)
  await form.getByLabel('采购需求').fill('客户私有需求：库存管理和财务同步')
  await form.getByRole('button', { name: '提交需求' }).click()
}

test('the formal AI workspace carries the current project into an approved draft and recovers a lost response', async ({ page, context }) => {
  const requirement = '120 人的制造企业需要财务模块'
  const api = await mockProjectAPI(context, [{ id: '1', title: 'ERP 采购试点', requirement, revision: 3, stage: 'requested' }])
  await page.goto('/login/dealer?project=1')
  await login(page, 'dealer_a', false)
  await page.getByRole('button', { name: '用 AI 整理需求' }).click()
  await expect(page.getByLabel('项目上下文')).toHaveValue('1')
  await page.getByRole('button', { name: '整理当前项目需求' }).click()
  const dialog = page.getByRole('dialog', { name: '审批需求字段' })
  await expect(dialog.getByLabel('授权用户数')).toHaveValue('')
  await expect(dialog.getByLabel('关联项目')).toHaveValue('1')
  await dialog.getByText('查看需求原文').click()
  await expect(dialog.getByText(requirement)).toBeVisible()
  expect(api.requests.some(item => item.method === 'POST' && item.pathname.includes('/inquiry-draft'))).toBe(false)
  api.dropDraft()
  const posted = page.waitForRequest(request => request.method() === 'POST' && request.url().endsWith('/1/inquiry-draft'))
  await dialog.getByRole('button', { name: '同意并继续' }).click()
  expect((await posted).postDataJSON()).toMatchObject({
    revision: 3, productId: '51', confirmed: true, requestId: expect.any(String),
    requirement: { employee_count: 120, license_count: null },
  })
  await expect(page.getByRole('button', { name: '用原请求确认结果' })).toBeVisible()
  expect(api.requests.some(item => item.method === 'POST' && item.pathname.includes('/actions/'))).toBe(false)
  await page.reload()
  await page.getByRole('button', { name: '用原请求确认结果' }).click()
  await expect(page.getByText('已确认草稿保存成功。')).toBeVisible()
  expect(api.draftAttempts).toHaveLength(2)
  expect(api.draftAttempts[1]).toEqual(api.draftAttempts[0])
  await page.reload()
  await expect(page.getByText('询价草稿 · 示例 ERP')).toBeVisible()
})

test('a pending project write survives lost response and expiry for its original account only', async ({ page, context }) => {
  const api = await mockProjectAPI(context)
  api.dropCreate()
  await login(page, 'dealer_a')
  await createProject(page, '恢复测试项目')
  const dialog = page.getByRole('dialog', { name: '核对尚未确认的提交' })
  const retry = dialog.getByRole('button', { name: '用原请求确认结果' })
  await expect(dialog.getByText(/提交结果待确认/)).toBeVisible()
  await page.reload()
  await expect(retry).toBeVisible()
  api.expire('dealer_a')
  await retry.click()
  await expect(page.getByRole('alert')).toHaveText('登录已失效，请重新登录。')

  await login(page, 'dealer_b', false)
  await expect(page.getByRole('heading', { name: '代理商工作台' })).toBeVisible()
  await expect(dialog).toHaveCount(0)
  expect(api.creates).toHaveLength(1)
  await page.goto('/login/dealer?view=account')
  await page.getByRole('button', { name: '退出登录' }).click()

  api.restore('dealer_a')
  await login(page, 'dealer_a', false)
  await retry.click()
  await expect(page.getByText('办理结果已保存，协作方可在关联项目中查看。')).toBeVisible()
  expect(api.creates).toEqual([api.creates[0], api.creates[0]])
  expect(api.projects).toHaveLength(1)
})

test('customer, dealer and vendor work on the same project through their own views', async ({ page, context }) => {
  const api = await mockProjectAPI(context)
  await login(page, 'customer_a')
  await createProject(page, '三方协作测试')
  await expect(page.getByRole('heading', { name: '三方协作测试' })).toBeVisible()
  const dealer = await context.newPage()
  await login(dealer, 'dealer_a')
  await dealer.getByRole('button', { name: /三方协作测试.*去处理/ }).click()
  await dealer.getByRole('button', { name: '提交厂商询价', exact: true }).click()
  await dealer.getByLabel('询价产品').selectOption('51')
  await dealer.getByLabel('对厂商的询价摘要').fill('仅给厂商的模块需求')
  await dealer.getByRole('checkbox', { name: /已核对产品与摘要/ }).check()
  await dealer.getByRole('button', { name: '确认并提交询价' }).click()
  await expect(dealer.getByText('办理结果已保存，协作方可在关联项目中查看。')).toBeVisible()
  await dealer.reload()
  await expect(dealer.getByText('示例 ERP', { exact: true })).toBeVisible()

  const vendor = await context.newPage()
  await login(vendor, 'vendor_a')
  await vendor.getByRole('button', { name: /示例 ERP.*去处理/ }).click()
  await expect(vendor.getByText('仅给厂商的模块需求')).toBeVisible()
  await expect(vendor.getByText(/客户私有需求/)).toHaveCount(0)
  await vendor.getByRole('button', { name: '要求补资料', exact: true }).click()
  const question = vendor.getByRole('dialog')
  await question.getByLabel('需要补充哪些信息？').fill('请补充授权数量')
  await question.getByRole('button', { name: '要求补资料', exact: true }).click()
  await expect(vendor.getByText('办理结果已保存，协作方可在关联项目中查看。')).toBeVisible()
  await page.getByRole('button', { name: '刷新协作进度' }).click()
  await expect(page.getByText('代理商跟进中', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('仅给厂商的模块需求')).toHaveCount(0)
  expect(api.projects).toHaveLength(1)
  expect(api.projects[0].revision).toBe(3)
})

test('the reseller can page project opportunities and open an existing record', async ({ page, context }) => {
  await mockProjectAPI(context, [{
    id: '31', title: '现有采购项目', stage: 'requested', revision: 1,
    opportunities: [
      { id: '52', note: '需要库存接口', stage: 'open' },
      { id: '51', note: '历史培训需求', stage: 'settled' },
    ],
  }])
  await login(page, 'dealer_a')
  await page.getByRole('button', { name: /现有采购项目.*去处理/ }).click()
  await expect(page.getByRole('heading', { name: '项目附加商机' })).toBeVisible()
  await expect(page.getByRole('button', { name: '需要库存接口 待跟进' })).toBeVisible()
  await page.getByRole('button', { name: '加载更多' }).click()
  await expect(page.getByRole('button', { name: '历史培训需求 介绍费已结清' })).toBeVisible()
  await page.getByRole('button', { name: '历史培训需求 介绍费已结清' }).click()
  await expect(page).toHaveURL(/view=opportunities&op=51$/)
  await expect(page.getByRole('heading', { name: '商机跟进' })).toBeVisible()
  await expect(page.getByText('历史培训需求', { exact: true })).toBeVisible()
})

test('workspace pages, filters and detail refresh fetch only the requested slice', async ({ page, context }) => {
  const input = Array.from({ length: 22 }, (_, index) => ({ id: String(index + 1), title: `项目样例 ${index + 1}`, stage: 'requested', revision: 1 }))
  const api = await mockProjectAPI(context, input)
  await login(page, 'dealer_a')
  const rows = page.getByRole('list', { name: '项目列表' }).getByRole('button')
  await expect(rows).toHaveCount(20)
  await expect(page.getByRole('button', { name: '全部项目 22' })).toBeVisible()
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(rows).toHaveCount(2)
  const row = page.getByRole('button', { name: /项目样例 22/ })
  await row.click()
  const workspaceCalls = () => api.requests.filter(({ pathname }) => pathname === projectsPath + '/workspace')
  const before = workspaceCalls().length
  await expect(page.getByRole('heading', { name: '项目样例 22' })).toBeVisible()
  const detailRequest = page.waitForResponse(response => new URL(response.url()).pathname === projectsPath + '/22')
  await page.getByRole('button', { name: '刷新协作进度' }).click()
  await detailRequest
  expect(workspaceCalls()).toHaveLength(before)
  await page.getByRole('button', { name: '返回项目列表' }).click()
  await expect(row).toBeFocused()
  await expect(rows).toHaveCount(2)
  await page.getByRole('searchbox', { name: '搜索项目' }).fill('样例 22')
  await expect(rows).toHaveCount(1)
  expect(workspaceCalls().at(-1).searchParams.get('q')).toBe('样例 22')
  expect(workspaceCalls().at(-1).searchParams.get('after')).toBeNull()
  const beforeDirectLink = workspaceCalls().length
  await page.goto('/login/dealer?view=projects&project=22')
  await expect(page.getByRole('heading', { name: '项目样例 22' })).toBeVisible()
  expect(workspaceCalls()).toHaveLength(beforeDirectLink)
  expect(api.requests.some(({ pathname }) => pathname === projectsPath)).toBe(false)
})
