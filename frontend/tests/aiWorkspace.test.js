import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope } from 'vue'
import { createDemoRepository } from '../src/channel/repository.js'
import { demoActor } from '../src/channel/domain.js'
import { createWorkflowJournal } from '../src/live/aiWorkflowJournal.js'
import { routeLocalWorkspace } from '../src/live/aiSkillCatalog.js'
import { useAiConversation } from '../src/live/useAiConversation.js'
import { appendTranscript, encodeMonoWavBase64 } from '../src/live/useVoiceInput.js'

function memoryStorage() {
  const map = new Map()
  return {
    getItem: key => (map.has(key) ? map.get(key) : null),
    setItem: (key, value) => { map.set(key, String(value)) },
    removeItem: key => { map.delete(key) },
  }
}

test('多步工作流日志传递跨步上下文，网络中断后从断点以原 requestId 幂等恢复', async () => {
  const storage = memoryStorage()
  const calls = []
  let failSaveDraftOnce = true

  const handlers = {
    async createProject({ input, requestId }) {
      calls.push(['createProject', requestId, input.title])
      return { id: '901', revision: 1 }
    },
    async saveInquiryDraft({ context, requestId }) {
      calls.push(['saveInquiryDraft', requestId, context.create_project.id])
      if (failSaveDraftOnce) {
        failSaveDraftOnce = false
        throw Object.assign(new Error('network timeout'), { status: 0 })
      }
      return { projectId: context.create_project.id, projectRevision: 1, summary: '员工人数：120；所需模块：财务' }
    },
    async submitInquiry({ context, requestId }) {
      calls.push(['submitInquiry', requestId, context.save_draft.projectId])
      return { id: context.save_draft.projectId, stage: 'inquired', stageLabel: '待厂商处理' }
    },
  }

  const journal = createWorkflowJournal('wf-test', handlers, storage)
  await assert.rejects(() => journal.run('submit_inquiry', [
    { id: 'create_project', label: '新建项目', kind: 'createProject', input: { title: '测试项目' } },
    { id: 'save_draft', label: '保存草稿', kind: 'saveInquiryDraft', input: {} },
    { id: 'submit_inquiry', label: '发送询价', kind: 'submitInquiry', input: {} },
  ]))

  assert.equal(journal.state.job.currentIndex, 1)
  const draftReqId = journal.state.job.steps[1].requestId

  // 模拟页面刷新重新挂载恢复
  const reloaded = createWorkflowJournal('wf-test', handlers, storage)
  assert.equal(reloaded.state.job.currentIndex, 1)
  const summary = await reloaded.retry()

  assert.equal(reloaded.state.job, null)
  assert.equal(summary.completed.length, 3)
  assert.equal(calls.filter(([name]) => name === 'createProject').length, 1, '已完成的第一步绝不重复执行')
  assert.equal(calls.filter(([name]) => name === 'saveInquiryDraft')[0][1], draftReqId)
  assert.equal(calls.filter(([name]) => name === 'saveInquiryDraft')[1][1], draftReqId, '断点重试沿用原 requestId')
})

test('本地意图与槽位路由覆盖全部已注册业务工作流技能', () => {
  const quoteRoute = routeLocalWorkspace('给财务共享中心生成对客报价，加价 30%，12 天内交付')
  assert.equal(quoteRoute.intent, 'sales_quote')
  assert.equal(quoteRoute.slots.markup_percent, 30)
  assert.equal(quoteRoute.slots.delivery_days, 12)

  assert.equal(routeLocalWorkspace('帮我直接发起厂商询价：120人制造企业，要财务和库存，云部署').intent, 'submit_inquiry')
  assert.equal(routeLocalWorkspace('先帮我准备询价草稿，暂时不要发送给厂商').intent, 'prepare_inquiry')
  assert.equal(routeLocalWorkspace('请帮我匹配支持财务和库存的软件产品').intent, 'match_products')
  assert.equal(routeLocalWorkspace('客户已确认报价，帮我确认正式下单').intent, 'place_order')
  assert.equal(routeLocalWorkspace('提交实施验收：已完成系统培训与数据迁移').intent, 'submit_acceptance')
  assert.equal(routeLocalWorkspace('记一条拜访：今天去核对了主项目，另外客户还需要 ERP 对接 MES 接口服务').intent, 'capture_visit_opportunity')

  const retailRoute = routeLocalWorkspace('帮我代客直购下单 3 套云财务')
  assert.equal(retailRoute.intent, 'assisted_order')
  assert.equal(retailRoute.slots.quantity, 3)

  assert.equal(routeLocalWorkspace('帮我催办待结算佣金').intent, 'remind_commission')
  const invoiceRoute = routeLocalWorkspace('给已付款订单申请开票，抬头是海川精工制造有限公司')
  assert.equal(invoiceRoute.intent, 'apply_invoice')
  assert.equal(invoiceRoute.slots.invoice_title, '海川精工制造有限公司')
  assert.equal(routeLocalWorkspace('提交一张授权激活相关的服务工单：付款后未收到授权码').intent, 'create_ticket')
})

test('一步到位询价、对客报价、下单、实施验收与拜访转商机均通过单次弹窗审批自动执行到底', async () => {
  const storage = memoryStorage()
  const dealerRepo = createDemoRepository('reseller', { storage, events: new EventTarget(), locks: null })
  const vendorRepo = createDemoRepository('manufacturer', { storage, events: new EventTarget(), locks: null })
  const customerRepo = createDemoRepository('customer', { storage, events: new EventTarget(), locks: null })

  const scope = effectScope()
  const conv = scope.run(() => useAiConversation({ demoRepository: dealerRepo, storage }, { value: null }))
  await conv.loadProjects()

  // 1. 一键建项 + 保存草稿 + 发送厂商询价（一次弹窗审批连续跑完 3 步）
  await conv.dispatch('我们是一家 120 人的制造企业，计划采购 ERP，需要财务和库存模块，希望云部署，授权席位 20 人。请直接发起厂商询价。')
  assert.equal(conv.approvalOpen.value, true)
  assert.equal(conv.approval.value.kind, 'inquiry_workflow')
  assert.equal(conv.approval.value.projectId, '__create__')
  assert.equal(conv.approval.value.autoSend, true)
  assert.equal(conv.approval.value.selectedProductId, 'ERP-PRO')

  await conv.approve()
  assert.equal(conv.approvalOpen.value, false)
  const lastInquiryTurn = conv.turns.value.at(-1)
  assert.equal(lastInquiryTurn.stepsLog.length, 3)
  assert.equal(lastInquiryTurn.project.stage, 'inquired')
  const createdProjectId = lastInquiryTurn.project.id

  // 厂商发出渠道报价后，代理商通过 AI 一步编制并发送对客报价
  const vendorView = await vendorRepo.get(createdProjectId)
  await vendorRepo.act(createdProjectId, 'channelQuote', {
    lines: [{ label: 'ERP 用户授权', quantity: 20, unitPrice: 1000 }],
    validUntil: '2099-12-31',
    deliveryDays: 10,
  }, vendorView.revision)
  await conv.loadProjects()
  conv.selectedId.value = createdProjectId

  await conv.dispatch('给当前项目按加价 25% 生成并发送对客报价')
  assert.equal(conv.approval.value.kind, 'sales_quote')
  assert.equal(conv.approval.value.lines[0].unitPrice, '1250.00')
  await conv.approve()
  assert.equal(conv.turns.value.at(-1).project.stage, 'sales_quoted')

  // 客户确认对客报价后，代理商通过 AI 一步确认正式下单
  const customerView = await customerRepo.get(createdProjectId)
  await customerRepo.act(createdProjectId, 'confirmSales', {
    quoteId: customerView.salesQuotes.at(-1).id,
  }, customerView.revision)
  await conv.loadProjects()

  await conv.dispatch('客户已确认报价，帮我正式下单')
  assert.equal(conv.approval.value.kind, 'place_order')
  await conv.approve()
  assert.equal(conv.turns.value.at(-1).project.stage, 'ordered')
  assert.ok(conv.turns.value.at(-1).project.order?.id)

  // 厂商供货后，代理商通过 AI 一步提交实施验收
  const afterOrderVendor = await vendorRepo.get(createdProjectId)
  await vendorRepo.act(createdProjectId, 'supply', { note: '20 席授权已发放' }, afterOrderVendor.revision)
  await conv.loadProjects()

  await conv.dispatch('提交实施验收：已完成部署与财务初始化培训')
  assert.equal(conv.approval.value.kind, 'submit_acceptance')
  await conv.approve()
  assert.equal(conv.turns.value.at(-1).project.stage, 'acceptance')

  // 记拜访并转团队商机（一次审批自动完成拜访保存 + 商机创建，不污染主项目）
  await conv.dispatch('记一条拜访：刚去客户现场复盘，另外客户还需要把 ERP 对接 MES 排程接口')
  assert.equal(conv.approval.value.kind, 'visit_opportunity')
  assert.equal(conv.approval.value.criteria.category, 'integration')
  await conv.approve()
  assert.ok(conv.turns.value.at(-1).opportunityLink?.id)
  const desk = await dealerRepo.opportunityDesk()
  assert.ok(desk.items.some(item => item.projectId === createdProjectId && item.note.includes('MES')))

  scope.stop()
  void demoActor
})

test('直购下单、催办佣金、申请开票与提交工单技能均打包审批并自动提交', async () => {
  const storage = memoryStorage()
  const dealerRepo = createDemoRepository('reseller', { storage, events: new EventTarget(), locks: null })
  const submitted = []

  async function mockRetailRequest(path, options = {}) {
    if (options.method === 'POST') {
      submitted.push([path, options.body])
      if (path.endsWith('/orders')) return { id: '501' }
      return {}
    }
    if (path.endsWith('/offers')) {
      return { items: [{ product_id: '11', manufacturer: '畅捷通', name: '云财务', unit_cents: 199800, annual: true }], next_cursor: null }
    }
    if (path === '/api/channel/v1/organization-links') {
      return { items: [{ id: '31', name: '海川精工' }] }
    }
    if (path.includes('view=commission')) {
      return { items: [{ id: '401', status: 'settlement', product_name: '云财务', customer: '海川精工', manufacturer: '畅捷通', commission: { cents: 99900 } }] }
    }
    if (path.endsWith('/operations')) {
      return {
        orders: [{ id: '401', product_name: '云财务', customer: '海川精工', total_cents: 199800, paid_at: '2026-11-20T08:00:00Z' }],
        invoices: [],
        partners: [{ id: '21', name: '畅捷通' }],
      }
    }
    return { items: [], next_cursor: null }
  }

  const scope = effectScope()
  const conv = scope.run(() => useAiConversation({ demoRepository: dealerRepo, retailRequest: mockRetailRequest, storage }, { value: null }))
  await conv.loadProjects()

  await conv.dispatch('帮我代客直购下单 2 套云财务')
  assert.equal(conv.approval.value.kind, 'assisted_order')
  assert.equal(conv.approval.value.quantity, 2)
  await conv.approve()
  assert.equal(submitted.at(-1)[0], '/api/retail/v1/orders')
  assert.equal(submitted.at(-1)[1].quantity, 2)

  await conv.dispatch('帮我催办待结算佣金')
  assert.equal(conv.approval.value.kind, 'remind_commission')
  await conv.approve()
  assert.equal(submitted.at(-1)[0], '/api/retail/v1/orders/401/remind')

  await conv.dispatch('帮我为已收款的直购订单申请开票，抬头海川精工股份有限公司')
  assert.equal(conv.approval.value.kind, 'apply_invoice')
  assert.equal(conv.approval.value.invoiceTitle, '海川精工股份有限公司')
  await conv.approve()
  assert.equal(submitted.at(-1)[0], '/api/retail/v1/operations/invoices')

  await conv.dispatch('帮我提交一张授权激活服务工单：付款后尚未开通云财务账号')
  assert.equal(conv.approval.value.kind, 'create_ticket')
  await conv.approve()
  assert.equal(submitted.at(-1)[0], '/api/retail/v1/operations/tickets')

  scope.stop()
})

test('语音输入编码标准 16kHz 单声道 RIFF WAVE 并无损拼接到输入框', () => {
  const chunk = new Float32Array(4800)
  for (let i = 0; i < chunk.length; i++) chunk[i] = Math.sin(i / 10) * 0.5
  const base64 = encodeMonoWavBase64([chunk], 48000, 16000)
  const raw = Buffer.from(base64, 'base64')

  assert.equal(raw.subarray(0, 4).toString('ascii'), 'RIFF')
  assert.equal(raw.subarray(8, 12).toString('ascii'), 'WAVE')
  assert.equal(raw.readUInt16LE(22), 1, '单声道')
  assert.equal(raw.readUInt32LE(24), 16000, '降采样至 16kHz')
  assert.equal(raw.length, 44 + 1600 * 2)

  assert.equal(appendTranscript('', ' 帮我发对客报价 '), '帮我发对客报价')
  assert.equal(appendTranscript('给财务共享中心', '加价25%发报价'), '给财务共享中心加价25%发报价')
  assert.equal(appendTranscript('ERP', 'MES对接'), 'ERP MES对接')
})

test('客户信息查询、经营汇总查询与多轮自由对话直接返回真实回答与数据卡片，不答非所问', async () => {
  const storage = memoryStorage()
  let chatHistorySent = null

  async function mockLiveRequest(path, options = {}) {
    if (path.startsWith('/api/channel/v1/projects?')) {
      return {
        items: [{ id: '1', title: '海川 ERP 采购', stage: 'requested', stageLabel: '待整理需求', nextStep: '提交厂商询价', isTodo: true, actions: ['submitInquiry'] }],
        next_cursor: null,
      }
    }
    if (path === '/api/ai/workspace/route') {
      const text = options.body.text
      if (text.includes('所有客户信息')) return { intent: 'query_customers', use_project_requirement: false }
      return { intent: 'chat', use_project_requirement: false }
    }
    if (path.startsWith('/api/retail/v1/operations/customers')) {
      return {
        items: [
          { id: 'o239', name: '海川精密机械制造有限公司', contact: '陈志远 · 生产副总', note: '120人离散制造企业', linked: true, prospect: false },
          { id: 'p32', name: '星河光电仪器有限公司', contact: '郑浩 · 总经理助理', note: '明年一季度升级云ERP', linked: false, prospect: true },
        ],
        next_cursor: null,
      }
    }
    if (path === '/api/ai/workspace/chat') {
      chatHistorySent = options.body.messages
      return {
        reply: '陈志远是海川精密机械制造有限公司的生产副总，建议重点切入云端业财与车间排程协同。',
        links: ['customers', 'renewals'],
      }
    }
    return {}
  }

  const scope = effectScope()
  const conv = scope.run(() => useAiConversation({
    request: mockLiveRequest,
    organizationId: '236',
    userId: '1',
    storage,
  }, { value: null }))
  await conv.loadProjects()

  await conv.dispatch('帮我查看一下我的所有客户信息')
  const custTurn = conv.turns.value.at(-1)
  assert.equal(custTurn.customersCard.items.length, 2)
  assert.equal(custTurn.customersCard.linkedCount, 1)
  assert.equal(custTurn.customersCard.prospectCount, 1)

  await conv.dispatch('陈志远是哪家公司的？给我一点跟进建议')
  const chatTurn = conv.turns.value.at(-1)
  assert.match(chatTurn.text, /陈志远是海川精密机械制造有限公司/)
  assert.equal(chatTurn.chatLinks.length, 2)
  assert.equal(chatHistorySent.length, 3, '携带前序对话上下文发给大模型')

  const firstSessionId = conv.activeSessionId.value
  conv.startNewSession()
  assert.equal(conv.turns.value.length, 0)
  assert.equal(conv.sessions.value.length, 2)

  conv.switchSession(firstSessionId)
  assert.equal(conv.turns.value.length, 4, '切回历史会话后完整恢复记录')
  scope.stop()

  // 模拟切页面或刷新后同账号恢复，不同账号严格隔离
  const scopeReload = effectScope()
  const reloaded = scopeReload.run(() => useAiConversation({
    request: mockLiveRequest,
    organizationId: '236',
    userId: '1',
    storage,
  }, { value: null }))
  assert.equal(reloaded.turns.value.length, 4, '刷新重进后自动恢复上次活跃对话')
  scopeReload.stop()

  const scopeOtherUser = effectScope()
  const otherUser = scopeOtherUser.run(() => useAiConversation({
    request: mockLiveRequest,
    organizationId: '236',
    userId: '99',
    storage,
  }, { value: null }))
  assert.equal(otherUser.turns.value.length, 0, '同组织其他成员账号不会读取他人对话记录')
  scopeOtherUser.stop()
})
