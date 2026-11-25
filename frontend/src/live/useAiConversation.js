import { computed, getCurrentInstance, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { aiDemoProject, routeDemoWorkspace } from '../channel/aiInquiryDemo.js'
import { createHttpVisitRepository } from './httpVisits.js'
import { createVisitDraftStore } from './visitDrafts.js'
import { createWorkflowJournal } from './aiWorkflowJournal.js'
import { inquirySummary } from './aiInquiry.js'
import { routeLocalWorkspace, skillShortcuts, workspaceSkills } from './aiSkillCatalog.js'
import {
  applyMarkupToApproval, canInquire, hasAnyRequirement, inquiryStepsPreview, normalizeRequirement,
  prepareAcceptanceSkill, prepareInquirySkill, preparePlaceOrderSkill, prepareSalesQuoteSkill,
  prepareSendSavedDraft, prepareVisitOpportunitySkill, queryProductMatches, queryVisitPartners,
} from './aiProjectSkills.js'
import {
  buildCustomerConfirmUrl, createRetailSkillBridge, formatMoneyCents,
  prepareApplyInvoiceSkill, prepareAssistedOrderSkill, prepareCreateTicketSkill, prepareRemindCommissionSkill,
  readCustomersSummary, readOperationsSummary,
} from './aiRetailSkills.js'
import { retailApi } from './retailRequest.js'

export { workspaceSkills, skillShortcuts }

const draftPath = id => '/api/channel/v1/projects/' + encodeURIComponent(id) + '/inquiry-draft'

export function useAiConversation(props, log) {
  const demo = Boolean(props.demoRepository)
  const repo = demo ? props.demoRepository : createHttpProjectRepository(props.request)
  const visitsRepo = demo ? null : createHttpVisitRepository(props.request)
  const demoVisits = demo ? createVisitDraftStore('demo:rs-beichen', props.storage) : null
  const controller = new AbortController()
  const options = { signal: controller.signal }
  const get = path => props.request(path, options)
  const post = (path, body, timeout) => props.request(path, { ...options, method: 'POST', body, timeout })

  const input = ref('')
  const turns = ref([])
  const projects = ref([])
  const selectedId = ref(props.initialProject || '')
  const selectedProject = computed(() => projects.value.find(item => item.id === selectedId.value))
  const inquiryProjects = computed(() => projects.value.filter(canInquire))
  const loadingProjects = ref(false)
  const projectError = ref('')
  const working = ref('')
  const approval = ref(null)
  const approvalOpen = ref(false)
  const approvalError = ref('')
  const disposed = ref(false)
  let turnId = 0

  const stepHandlers = {
    async createProject({ input: payload, requestId }) {
      return demo ? repo.create(payload) : repo.create(payload, requestId)
    },
    async saveInquiryDraft({ input: payload, context, requestId }) {
      const created = context.create_project
      const projectId = payload.projectId || created?.id || ''
      const revision = payload.revision ?? created?.revision
      const body = { revision, confirmed: true, productId: payload.productId, requirement: payload.requirement }
      return demo
        ? repo.saveInquiryDraft(projectId, body, { ...aiDemoProject, title: payload.newProjectTitle || aiDemoProject.title, requirement: payload.source })
        : props.request(draftPath(projectId), { method: 'POST', body: { ...body, requestId } })
    },
    async submitInquiry({ input: payload, context, requestId }) {
      const created = context.create_project
      const draft = context.save_draft
      const projectId = payload.projectId || created?.id || draft?.projectId
      const revision = payload.revision ?? created?.revision ?? draft?.projectRevision
      const body = { product: payload.productId, summary: draft?.summary || payload.summary }
      return demo ? repo.act(projectId, 'submitInquiry', body, revision) : repo.act(projectId, 'submitInquiry', body, revision, requestId)
    },
    async projectAction({ input: payload, requestId }) {
      return demo
        ? repo.act(payload.projectId, payload.action, payload.body, payload.revision)
        : repo.act(payload.projectId, payload.action, payload.body, payload.revision, requestId)
    },
    async saveVisit({ input: payload, requestId }) {
      const visitPayload = {
        projectId: payload.projectId,
        text: payload.rawText,
        main: payload.mainNote,
        additional: payload.additionalNote,
        reviewed: true,
      }
      if (demo) {
        return demoVisits.save({ ...visitPayload, ...payload.criteria, confirmed: true }, 0)
      }
      return visitsRepo.save('', visitPayload, 0, requestId)
    },
    async confirmVisitOpportunity({ input: payload, context, requestId }) {
      if (demo) {
        const current = await repo.opportunityDesk()
        const existing = current.items.find(item => item.projectId === payload.projectId && item.note === payload.additionalNote)
        return existing || repo.captureOpportunity(payload.projectId, { note: payload.additionalNote, ...payload.criteria })
      }
      const saved = context.save_visit
      return visitsRepo.confirm(saved.id, saved.revision, true, requestId, payload.criteria)
    },
    async createAssistedOrder({ input: payload, requestId }) {
      const bridge = await createRetailSkillBridge(ctx)
      const body = {
        product_id: String(payload.productId),
        customer_id: String(payload.customerId),
        quantity: Number(payload.quantity),
        expected_unit_cents: Number(payload.expectedUnitCents),
        ...(payload.expectedAnnual !== undefined ? { expected_annual: payload.expectedAnnual } : {}),
        request_id: requestId,
      }
      return bridge.request(retailApi + '/orders', { method: 'POST', body })
    },
    async remindCommissionOrder({ input: payload }) {
      const bridge = await createRetailSkillBridge(ctx)
      await bridge.request(`${retailApi}/orders/${encodeURIComponent(payload.orderId)}/remind`, { method: 'POST', body: {} })
      return { orderId: payload.orderId }
    },
    async submitOrderInvoice({ input: payload }) {
      const bridge = await createRetailSkillBridge(ctx)
      const path = bridge.demo ? '/operations/invoices' : '/operations/finance/invoices'
      await bridge.request(retailApi + path, {
        method: 'POST',
        body: { order_id: payload.orderId, title: payload.invoiceTitle },
      })
      return { orderId: payload.orderId, title: payload.invoiceTitle }
    },
    async createServiceTicket({ input: payload, requestId }) {
      const bridge = await createRetailSkillBridge(ctx)
      const body = bridge.demo
        ? {
            subject: payload.subject,
            category: payload.category,
            order_id: payload.orderId || '',
            detail: payload.detail,
            request_id: requestId,
          }
        : {
            subject: payload.subject,
            category: payload.category,
            detail: payload.detail,
            ...(payload.orderId ? { order_id: payload.orderId } : { manufacturer_id: payload.manufacturerId }),
            request_id: requestId,
          }
      await bridge.request(retailApi + '/operations/tickets', { method: 'POST', body })
      return { subject: payload.subject, category: payload.category, orderId: payload.orderId }
    },
  }

  const journalKey = demo ? '' : `partnerledger-ai-workflow:${props.organizationId}:${props.userId}`
  const journal = createWorkflowJournal(journalKey, stepHandlers, props.storage)
  const pendingWrite = computed(() => journal.state.job)
  const recoveryError = computed(() => journal.state.error)

  const ctx = {
    demo,
    repo,
    options,
    request: props.request,
    retailRequest: props.retailRequest,
    get,
    post,
    projects,
    inquiryProjects,
    getProduct(id) {
      return demo
        ? repo.listProducts().then(items => items.find(item => String(item.id) === String(id)))
        : props.request('/api/products/' + encodeURIComponent(id), options)
    },
  }

  if (getCurrentInstance()) {
    onUnmounted(() => {
      disposed.value = true
      controller.abort()
    })
  }

  function say(text, extra = {}) {
    if (disposed.value) return null
    const entry = reactive({ id: ++turnId, role: 'assistant', text, ...extra })
    turns.value.push(entry)
    nextTick(() => {
      if (log?.value) log.value.scrollTop = log.value.scrollHeight
    })
    return entry
  }

  async function loadProjects() {
    loadingProjects.value = true
    projectError.value = ''
    try {
      projects.value = await repo.list(options)
      if (selectedId.value && !selectedProject.value) {
        projectError.value = '当前项目暂不可访问，可以选择其他项目。'
        selectedId.value = ''
      }
    } catch (cause) {
      if (cause.name !== 'AbortError') projectError.value = '暂时无法读取项目：' + cause.message
    } finally {
      loadingProjects.value = false
    }
  }

  async function readDraft(id) {
    try {
      return demo ? repo.getInquiryDraft(id) : await props.request(draftPath(id), options)
    } catch (cause) {
      if (cause.status === 404) return null
      throw cause
    }
  }

  async function start() {
    await loadProjects()
    if (disposed.value || !selectedProject.value || !canInquire(selectedProject.value) || pendingWrite.value) return
    const id = selectedId.value
    try {
      const draft = await readDraft(id)
      if (!draft || draft.projectRevision !== selectedProject.value?.revision || selectedId.value !== id || turns.value.length) return
      const product = await ctx.getProduct(draft.productId)
      if (selectedId.value === id && !turns.value.length) {
        say('这个项目已有询价草稿。你可以查看内容，也可以让我直接发起发送审批。', { draft, product })
      }
    } catch (cause) {
      if (cause.name !== 'AbortError' && !turns.value.length) {
        say('暂时无法读取该项目的已保存草稿。你可以继续对话或稍后重试。')
      }
    }
  }

  if (getCurrentInstance()) {
    onMounted(start)
  }

  function projectFromText(text) {
    const matching = projects.value.filter(project => text.includes(project.title) ||
      (demo ? text.includes(project.id) : new RegExp('(?:^|[^0-9])(?:#|项目\\s*)' + project.id + '(?![0-9])').test(text)))
    if (matching.length > 1) throw new Error('这条消息提到了多个项目，请在上方选择目标项目后再发送。')
    const project = matching[0]
    if (project) selectedId.value = project.id
    if (!project && /(?:项目\s*#?\s*\d+|#\d+|PRJ-\d+)/i.test(text)) {
      throw new Error('找不到这条消息所指的项目。请从上方选择有权访问的项目。')
    }
    return project ?? selectedProject.value
  }

  function requestApproval(spec, message) {
    const state = reactive(spec)
    approval.value = state
    approvalError.value = ''
    approvalOpen.value = true
    say(message, { approval: state })
  }

  function closeApproval() {
    approvalOpen.value = false
  }

  function reopenApproval() {
    if (approval.value) approvalOpen.value = true
  }

  function declineApproval() {
    approvalOpen.value = false
    approval.value = null
    approvalError.value = ''
    say('已取消本次审批，未执行任何业务写入。')
  }

  async function refreshApprovalMatches() {
    const current = approval.value
    if (!current || current.kind !== 'inquiry_workflow' || working.value) return
    const normalized = normalizeRequirement(current.fields)
    if (Number.isNaN(normalized.employee_count) || Number.isNaN(normalized.license_count) || !hasAnyRequirement(normalized)) {
      approvalError.value = '请先填写至少一项有效需求字段（人数须为正整数）。'
      return
    }
    approvalError.value = ''
    working.value = '正在重新匹配候选产品'
    try {
      const matches = await queryProductMatches(ctx, normalized)
      current.matches = matches
      current.fieldsDirty = false
      const eligible = matches.filter(item => item.status !== 'mismatch')
      if (!eligible.some(item => String(item.product.id) === String(current.selectedProductId))) {
        const exact = eligible.find(item => item.status === 'matched')
        current.selectedProductId = (exact ?? eligible[0])?.product?.id ? String((exact ?? eligible[0]).product.id) : ''
      }
    } catch (cause) {
      approvalError.value = cause.message || '产品匹配失败，请稍后重试。'
    } finally {
      working.value = ''
    }
  }

  async function refreshVisitPreview() {
    const current = approval.value
    if (!current || current.kind !== 'visit_opportunity') return
    current.partners = await queryVisitPartners(ctx, current.criteria)
  }

  function demoChatReply(text) {
    if (/^(?:你好|您好|嗨|在吗|hello|hi)\b/i.test(text)) {
      return `你好！我是渠智链智能业务助手。当前可见 ${projects.value.length} 个协作项目，你可以直接让我查客户、看项目进度、撰写跟进话术，或下达询价、对客报价、下单、记拜访等业务指令。`
    }
    if (/区别|什么是\s*(?:ERP|MES|WMS)/i.test(text)) {
      return '简单来说：ERP 侧重企业财务、采购、库存与订单经营主干；MES 侧重生产车间排程、工序报工与质检追溯；WMS 侧重库位、批次与扫码出入库。制造客户通常先落 ERP 业财一体，再将车间工序需求作为附加商机对接 MES。'
    }
    if (/话术|微信|跟进|怎么谈/.test(text)) {
      return '可以参考这段跟进话术：\n「王总您好，上次沟通的云端业财一体化方案我们已结合贵司规模梳理好模块范围与实施周期。为确保报价精准不浪费席位，想和您确认一下首期需要开通账号的具体岗位人数，确认后我今天把正式对客方案发您审阅。」'
    }
    return `收到你的问题：「${text}」。在本地离线演示空间中我已为你梳理当前台账（共 ${projects.value.length} 个项目）；登录正式工作台后，我将结合贵组织的实时数据库与火山方舟大模型为你深度解答任意业务或日常话题。`
  }

  async function replyWithChat(text) {
    if (demo) {
      say(demoChatReply(text))
      return
    }
    working.value = '正在结合业务上下文思考回复'
    const history = turns.value
      .filter(item => (item.role === 'user' || item.role === 'assistant') && item.text?.trim())
      .slice(-10)
      .map(item => ({ role: item.role, content: item.text.slice(0, 2000) }))
    const res = await post('/api/ai/workspace/chat', { messages: history }, 38000)
    if (!disposed.value) {
      say(res?.reply || '我已收到你的消息，请告诉我接下来想了解或办理的内容。')
    }
  }

  async function dispatch(value = input.value) {
    const text = value.trim()
    if (!text || working.value || loadingProjects.value || pendingWrite.value) return
    if (approval.value) { reopenApproval(); return }
    if ([...text].length > 2000) { say('单条消息限 2000 字，请缩短后发送。'); return }
    input.value = ''
    say(text, { role: 'user' })
    working.value = '正在理解你的问题与所需技能'
    try {
      const route = demo ? routeDemoWorkspace(text) : await post('/api/ai/workspace/route', { text }, 32000)
      if (disposed.value) return
      if (route.intent === 'chat' || route.intent === 'help' || !Object.hasOwn(workspaceSkills, route.intent)) {
        await replyWithChat(text)
        return
      }
      await openSkillByRoute(route, text)
    } catch (cause) {
      if (cause.name !== 'AbortError') {
        const modelUnavailable = [502, 503, 504].includes(cause.status)
        const fallbackRoute = modelUnavailable ? routeLocalWorkspace(text) : null
        const canManualApprove = fallbackRoute && !['chat', 'help', 'query_customers', 'query_operations', 'project_status'].includes(fallbackRoute.intent)
        say(
          cause.message || '助手暂时不可用，请稍后重试。',
          canManualApprove ? { manualSource: text, fallbackIntent: fallbackRoute.intent } : {},
        )
      }
    } finally {
      working.value = ''
    }
  }

  async function openSkillByRoute(route, text, { manual = false } = {}) {
    working.value = '正在读取业务数据'
    if (route.intent === 'query_customers') {
      const summary = await readCustomersSummary(ctx)
      if (!summary.items.length) {
        say('当前组织暂未登记任何客户档案。你可以前往「客户档案」新增或关联企业客户。', {
          navAction: { label: '打开客户档案', view: 'customers' },
        })
        return
      }
      const head = summary.demo
        ? `当前共有 ${summary.items.length} 家演示客户档案：`
        : `当前你的组织共有 ${summary.items.length} 家客户档案（其中已关联签约客户 ${summary.linkedCount} 家、潜在跟进客户 ${summary.prospectCount} 家）：`
      say(head, { customersCard: summary })
      return
    }

    if (route.intent === 'query_operations') {
      const overview = await readOperationsSummary(ctx)
      say(
        `当前经营数据汇总：客户档案 ${overview.customers} 家，直购订单 ${overview.orders} 笔，待结算佣金 ${overview.pendingCommissionText}，30 天内到期续费 ${overview.renewals30Days} 笔，未关闭服务工单 ${overview.openTickets} 单。`,
        { operationsCard: overview },
      )
      return
    }

    const needsProjectLookup = ['match_products', 'prepare_inquiry', 'submit_inquiry', 'sales_quote', 'place_order', 'submit_acceptance', 'capture_visit_opportunity', 'project_status'].includes(route.intent)
    const contextProject = needsProjectLookup ? projectFromText(text) : null
    let project = contextProject ? await repo.get(contextProject.id, options) : null
    if (project) {
      projects.value = projects.value.map(item => item.id === project.id ? project : item)
    }

    if (route.intent === 'project_status') {
      if (project) {
        say(`这是「${project.title}」的当前进展。`, { project })
      } else if (projects.value.length) {
        const todoCount = projects.value.filter(item => item.isTodo).length
        say(`当前你共有 ${projects.value.length} 个可见协作项目（其中 ${todoCount} 个待你处理）：`, {
          projectsList: projects.value.slice(0, 12),
        })
      } else {
        say('当前暂无可见的协作项目。')
      }
      return
    }

    if (!project && ['prepare_inquiry', 'submit_inquiry'].includes(route.intent) && route.use_project_requirement && !demo && inquiryProjects.value.length) {
      project = await repo.get(inquiryProjects.value[0].id, options)
      if (project) {
        selectedId.value = project.id
        projects.value = projects.value.map(item => item.id === project.id ? project : item)
      }
    }

    if (route.intent === 'submit_inquiry' && route.use_project_requirement && project && canInquire(project)) {
      const existingDraft = await readDraft(project.id)
      if (existingDraft && existingDraft.projectRevision === project.revision) {
        const prepared = await prepareSendSavedDraft(ctx, project, existingDraft)
        if (prepared.error) say(prepared.error, { project: prepared.project })
        else requestApproval(prepared.approval, '检测到该项目已有就绪的询价草稿。请在弹窗确认，批准后我将立即提交给厂商。')
        return
      }
    }

    if (['match_products', 'prepare_inquiry', 'submit_inquiry'].includes(route.intent)) {
      if (project && route.intent !== 'match_products' && !canInquire(project)) {
        say('当前所选项目已进入后续阶段，不能重复发起询价。请选择另一个待询价项目或取消顶部项目选择以自动新建。', { project })
        return
      }
      const source = route.use_project_requirement
        ? project?.requirement || (demo ? aiDemoProject.requirement : '')
        : text
      if (!source && !manual) {
        say('请先选择含采购需求的项目，或直接描述所需模块、人数与部署方式。')
        return
      }
      working.value = '正在提取需求与预匹配产品'
      const spec = await prepareInquirySkill(ctx, { intent: route.intent, source: source || text, project, manual })
      const label = workspaceSkills[route.intent]
      requestApproval(spec, `已为你打包「${label}」方案，请在弹窗核对信息。点击同意后我将自动完成后续全部步骤。`)
      return
    }

    const planners = {
      sales_quote: () => prepareSalesQuoteSkill(ctx, { route, project }),
      place_order: () => preparePlaceOrderSkill(ctx, { project }),
      submit_acceptance: () => prepareAcceptanceSkill(ctx, { route, project }),
      capture_visit_opportunity: () => prepareVisitOpportunitySkill(ctx, { text, route, project }),
      assisted_order: () => prepareAssistedOrderSkill(ctx, { text, route }),
      remind_commission: () => prepareRemindCommissionSkill(ctx),
      apply_invoice: () => prepareApplyInvoiceSkill(ctx, { text, route }),
      create_ticket: () => prepareCreateTicketSkill(ctx, { text, route }),
    }
    const result = await planners[route.intent]()
    if (result.approval) {
      requestApproval(result.approval, `已为你准备好「${workspaceSkills[route.intent]}」审批单，确认同意后我将自动执行到底。`)
    } else {
      say(result.message || '暂未找到符合办理条件的业务记录。', result.project ? { project: result.project } : {})
    }
  }

  async function fillManually(source, fallbackIntent = 'prepare_inquiry') {
    if (working.value || approval.value || pendingWrite.value) return
    try {
      const route = routeLocalWorkspace(source)
      const intent = Object.hasOwn(workspaceSkills, fallbackIntent) && fallbackIntent !== 'help'
        ? fallbackIntent
        : (Object.hasOwn(workspaceSkills, route.intent) ? route.intent : 'prepare_inquiry')
      await openSkillByRoute({ ...route, intent }, source, { manual: true })
    } catch (cause) {
      say(cause.message || '暂时无法打开手工审批表单，请稍后重试。')
    } finally {
      working.value = ''
    }
  }

  async function finishSummary(summary) {
    const { skillId, meta, context, completed } = summary
    await loadProjects()
    const stepsLog = completed.map(step => step.label)

    if (skillId === 'prepare_inquiry' || skillId === 'submit_inquiry') {
      const draft = context.save_draft
      const sentProject = context.submit_inquiry
      const projectId = sentProject?.id || draft?.projectId || context.create_project?.id || meta.projectId
      if (projectId) selectedId.value = projectId
      const latestProject = sentProject || (projectId ? await repo.get(projectId, options).catch(() => null) : null)
      if (sentProject) {
        say('审批已执行完成：询价已提交给软件厂商。', {
          stepsLog,
          matches: meta.chosenMatch ? [meta.chosenMatch] : undefined,
          project: latestProject,
        })
      } else {
        say('审批已执行完成：已自动保存询价草稿，项目仍待询价。', {
          stepsLog,
          matches: meta.chosenMatch ? [meta.chosenMatch] : undefined,
          draft,
          product: meta.product,
        })
      }
      return
    }

    if (['sales_quote', 'place_order', 'submit_acceptance'].includes(skillId)) {
      const updated = context.project_action
      if (updated?.id) selectedId.value = updated.id
      const messages = {
        sales_quote: `对客报价已自动生成并发送给客户（当前阶段：${updated?.stageLabel || '待客户审阅'}）。`,
        place_order: `正式采购订单（${updated?.order?.id || '已生成'}）已确认下达，等待厂商供货。`,
        submit_acceptance: '实施完成说明已登记并提交企业客户验收。',
      }
      say(messages[skillId], { stepsLog, project: updated })
      return
    }

    if (skillId === 'capture_visit_opportunity') {
      const confirmed = context.confirm_opportunity
      const opportunityId = confirmed?.opportunityId || confirmed?.id || ''
      say('拜访记录与团队附加商机已自动沉淀完成，主项目订单不受影响。', {
        stepsLog,
        opportunityLink: {
          id: opportunityId,
          projectId: meta.projectId,
          note: meta.additionalNote,
          partnerCount: meta.partnerCount || 0,
        },
      })
      return
    }

    if (skillId === 'assisted_order') {
      const created = context.assisted_order
      const orderId = created?.id || ''
      say(`代客直购订单 #${orderId} 已创建完成（总价 ${meta.totalText}），可将确认链接发送给客户付款。`, {
        stepsLog,
        navAction: {
          label: '打开厂商直收订单',
          view: 'retail-orders',
          confirmUrl: orderId ? buildCustomerConfirmUrl(orderId) : '',
        },
      })
      return
    }

    if (skillId === 'remind_commission') {
      say(`已完成 ${meta.count} 笔待结算直购订单的佣金催办登记。`, {
        stepsLog,
        navAction: { label: '查看佣金结算台账', view: 'commission' },
      })
      return
    }

    if (skillId === 'apply_invoice') {
      say(`订单 #${meta.orderId} 的开票申请（抬头：${meta.invoiceTitle}）已提交厂商办理。`, {
        stepsLog,
        navAction: { label: '查看订单开票进度', view: 'funds-invoices' },
      })
      return
    }

    if (skillId === 'create_ticket') {
      say(`服务工单「${meta.subject}」已提交合作厂商处理。`, {
        stepsLog,
        navAction: { label: '打开服务工单列表', view: 'service-tickets' },
      })
    }
  }

  async function approve() {
    const item = approval.value
    if (!item || working.value || pendingWrite.value) return
    approvalError.value = ''

    let steps = []
    let meta = {}

    if (item.kind === 'inquiry_workflow') {
      const requirement = normalizeRequirement(item.fields)
      if (Number.isNaN(requirement.employee_count) || Number.isNaN(requirement.license_count)) {
        approvalError.value = '员工人数和授权用户数须为 1–1,000,000 的正整数，未知可留空。'
        return
      }
      if (!hasAnyRequirement(requirement)) {
        approvalError.value = '请至少填写一个已知需求字段。'
        return
      }
      if (requirement.required_modules.length > 30 || requirement.required_modules.some(m => [...m].length > 70)) {
        approvalError.value = '模块最多 30 项，每项不超过 70 字。'
        return
      }

      if (item.intent === 'match_products') {
        approval.value = null
        approvalOpen.value = false
        working.value = '审批已通过，正在检索匹配产品'
        try {
          const matches = item.fieldsDirty || !item.matches.length
            ? await queryProductMatches(ctx, requirement)
            : item.matches
          const eligible = matches.filter(entry => entry.status !== 'mismatch')
          if (!eligible.length) say('没有符合已知条件的候选产品。可以补充或调整需求后再匹配。', { matches })
          else say(`匹配完成，找到 ${eligible.length} 款可进一步核对的产品。`, { matches })
        } catch (cause) {
          reportExecutionError(cause)
        } finally {
          working.value = ''
        }
        return
      }

      if (!item.projectId) {
        approvalError.value = '请选择待询价项目，或选择自动新建采购项目。'
        return
      }
      if (item.projectId === '__create__') {
        if (!item.newProjectTitle?.trim()) {
          approvalError.value = '请填写新采购项目名称。'
          return
        }
        if (!demo && !item.customerId) {
          approvalError.value = '请选择已建立代理关系的企业客户。'
          return
        }
      }

      if (item.fieldsDirty || !item.matches.length) {
        await refreshApprovalMatches()
        if (approvalError.value) return
      }
      const eligible = item.matches.filter(entry => entry.status !== 'mismatch')
      if (!eligible.length) {
        approvalError.value = '当前需求条件没有匹配到可用产品，请调整模块或部署方式后重试。'
        return
      }
      const chosenMatch = eligible.find(entry => String(entry.product.id) === String(item.selectedProductId)) ?? eligible[0]
      const product = chosenMatch.product
      const isCreate = item.projectId === '__create__'
      const targetProject = !isCreate ? inquiryProjects.value.find(p => p.id === item.projectId) : null
      if (!isCreate && !targetProject) {
        approvalError.value = '所选项目已不可询价，请重新选择。'
        return
      }

      if (isCreate) {
        steps.push({
          id: 'create_project',
          label: '自动新建采购项目',
          kind: 'createProject',
          input: demo
            ? { title: item.newProjectTitle.trim(), requirement: item.source }
            : { title: item.newProjectTitle.trim(), requirement: item.source, customerId: item.customerId },
        })
      }
      steps.push({
        id: 'save_draft',
        label: `锁定「${product.name}」并保存询价草稿`,
        kind: 'saveInquiryDraft',
        input: {
          projectId: isCreate ? '' : targetProject.id,
          revision: targetProject?.revision,
          productId: product.id,
          requirement,
          source: item.source,
          newProjectTitle: item.newProjectTitle?.trim(),
        },
      })
      if (item.autoSend) {
        steps.push({
          id: 'submit_inquiry',
          label: '向所属软件厂商发送正式询价',
          kind: 'submitInquiry',
          input: {
            projectId: isCreate ? '' : targetProject.id,
            revision: targetProject?.revision,
            productId: product.id,
            summary: inquirySummary(requirement),
          },
        })
      }
      meta = {
        projectId: isCreate ? '' : targetProject.id,
        product,
        chosenMatch,
      }
    } else if (item.kind === 'send_draft') {
      steps.push({
        id: 'submit_inquiry',
        label: '向所属软件厂商发送正式询价',
        kind: 'submitInquiry',
        input: {
          projectId: item.project.id,
          revision: item.project.revision,
          productId: item.product.id,
          summary: item.summary,
        },
      })
      meta = { projectId: item.project.id, product: item.product }
    } else if (item.kind === 'sales_quote') {
      const deliveryDays = Number(item.deliveryDays)
      if (!item.validUntil || !/^\d{4}-\d{2}-\d{2}$/.test(item.validUntil)) {
        approvalError.value = '请填写有效的报价截止日期。'
        return
      }
      if (!Number.isInteger(deliveryDays) || deliveryDays < 1 || deliveryDays > 3650) {
        approvalError.value = '交付周期须为 1–3650 天的整数。'
        return
      }
      const lines = item.lines.map(line => ({
        label: String(line.label ?? '').trim(),
        quantity: Number(line.quantity),
        unitPrice: String(line.unitPrice ?? '').trim(),
      }))
      if (!lines.length || lines.some(l => !l.label || !Number.isInteger(l.quantity) || l.quantity < 1 || !/^\d{1,8}(\.\d{1,2})?$/.test(l.unitPrice) || Number(l.unitPrice) <= 0)) {
        approvalError.value = '请核对每行报价明细的名称、数量及大于零的人民币单价（最多两位小数）。'
        return
      }
      steps.push({
        id: 'project_action',
        label: '生成并向企业客户发送对客报价',
        kind: 'projectAction',
        input: {
          projectId: item.project.id,
          revision: item.project.revision,
          action: 'salesQuote',
          body: {
            lines,
            validUntil: item.validUntil,
            deliveryDays,
            ...(item.note?.trim() ? { note: item.note.trim() } : {}),
          },
        },
      })
    } else if (item.kind === 'place_order') {
      steps.push({
        id: 'project_action',
        label: '锁定已确认报价并下达正式采购订单',
        kind: 'projectAction',
        input: {
          projectId: item.project.id,
          revision: item.project.revision,
          action: 'placeOrder',
          body: { quoteId: item.salesQuote.id },
        },
      })
    } else if (item.kind === 'submit_acceptance') {
      if (!item.note?.trim()) {
        approvalError.value = '请填写实施交付完成说明。'
        return
      }
      steps.push({
        id: 'project_action',
        label: '登记实施交付说明并提交客户验收',
        kind: 'projectAction',
        input: {
          projectId: item.project.id,
          revision: item.project.revision,
          action: 'implement',
          body: { note: item.note.trim() },
        },
      })
    } else if (item.kind === 'visit_opportunity') {
      if (!item.projectId || !item.additionalNote?.trim() || !item.criteria.category) {
        approvalError.value = '请选择关联项目、填写附加需求描述并指定能力类型。'
        return
      }
      steps.push(
        {
          id: 'save_visit',
          label: '保存个人客户拜访记录',
          kind: 'saveVisit',
          input: {
            projectId: item.projectId,
            rawText: item.rawText,
            mainNote: item.mainNote,
            additionalNote: item.additionalNote.trim(),
            criteria: { ...item.criteria },
          },
        },
        {
          id: 'confirm_opportunity',
          label: '生成团队共享附加商机',
          kind: 'confirmVisitOpportunity',
          input: {
            projectId: item.projectId,
            additionalNote: item.additionalNote.trim(),
            criteria: { ...item.criteria },
          },
        },
      )
      meta = {
        projectId: item.projectId,
        additionalNote: item.additionalNote.trim(),
        partnerCount: item.partners?.length || 0,
      }
    } else if (item.kind === 'assisted_order') {
      const offer = item.offers.find(o => String(o.product_id) === String(item.productId))
      const qty = Number(item.quantity)
      if (!offer || !item.customerId || !Number.isInteger(qty) || qty < 1 || qty > 10000) {
        approvalError.value = '请选择客户、直购产品并填写 1–10000 的整数数量。'
        return
      }
      steps.push({
        id: 'assisted_order',
        label: `创建「${offer.name}」× ${qty} 代客直购单`,
        kind: 'createAssistedOrder',
        input: {
          productId: offer.product_id,
          customerId: item.customerId,
          quantity: qty,
          expectedUnitCents: offer.unit_cents,
          expectedAnnual: offer.annual,
        },
      })
      meta = { totalText: formatMoneyCents(BigInt(offer.unit_cents) * BigInt(qty)) }
    } else if (item.kind === 'remind_commission') {
      if (!item.selectedOrderIds?.length) {
        approvalError.value = '请至少勾选一笔要催办的待结算订单。'
        return
      }
      steps = item.selectedOrderIds.map((orderId, idx) => ({
        id: `remind_${idx}`,
        label: `催办直购订单 #${orderId} 待结佣金`,
        kind: 'remindCommissionOrder',
        input: { orderId },
      }))
      meta = { count: item.selectedOrderIds.length }
    } else if (item.kind === 'apply_invoice') {
      if (!item.orderId || !item.invoiceTitle?.trim()) {
        approvalError.value = '请选择已付款订单并填写有效的开票抬头。'
        return
      }
      steps.push({
        id: 'apply_invoice',
        label: `提交订单 #${item.orderId} 开票申请`,
        kind: 'submitOrderInvoice',
        input: { orderId: item.orderId, invoiceTitle: item.invoiceTitle.trim() },
      })
      meta = { orderId: item.orderId, invoiceTitle: item.invoiceTitle.trim() }
    } else if (item.kind === 'create_ticket') {
      if (!item.subject?.trim() || !item.detail?.trim() || (!item.demo && !item.orderId && !item.manufacturerId)) {
        approvalError.value = '请填写工单标题、问题描述，并关联订单或合作厂商。'
        return
      }
      steps.push({
        id: 'create_ticket',
        label: `提交服务工单「${item.subject.trim()}」`,
        kind: 'createServiceTicket',
        input: {
          subject: item.subject.trim(),
          category: item.category,
          orderId: item.orderId,
          manufacturerId: item.manufacturerId,
          detail: item.detail.trim(),
        },
      })
      meta = { subject: item.subject.trim() }
    }

    const skillId = item.autoSend && item.skillId === 'prepare_inquiry' ? 'submit_inquiry' : item.skillId
    approval.value = null
    approvalOpen.value = false

    try {
      const summary = await journal.run(skillId, steps, meta, progress => {
        working.value = `正在自动执行 (${progress.index + 1}/${progress.total})：${progress.label}`
      })
      if (summary) await finishSummary(summary)
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  function reportExecutionError(cause) {
    if (cause.name === 'AbortError') return
    const donePrefix = cause.completedSteps?.length
      ? `已完成步骤：${cause.completedSteps.map(s => s.label).join(' → ')}。\n`
      : ''
    if (pendingWrite.value) {
      say(donePrefix + '当前步骤提交结果待确认。请点击下方「用原请求确认结果」继续完成剩余流程，避免重复提交。')
    } else if (cause.status === 409 || cause.code === 'CONFLICT') {
      say(donePrefix + '业务记录已被协作方更新或状态冲突，请刷新核对最新状态后重新发起。')
    } else {
      say(donePrefix + (cause.message || '自动执行未完成，请核对业务记录后重试。'))
    }
  }

  async function reviewSavedDraft(turn) {
    if (working.value || approval.value || pendingWrite.value) return
    working.value = '正在核对项目和草稿'
    try {
      const project = await repo.get(turn.draft.projectId, options)
      const draft = await readDraft(project.id)
      if (!draft || draft.productId !== turn.draft.productId || draft.summary !== turn.draft.summary ||
          draft.projectRevision !== turn.draft.projectRevision) {
        say('这张卡片的草稿已更新或项目不再待询价，请重新查询项目。', { project })
        return
      }
      const prepared = await prepareSendSavedDraft(ctx, project, draft, turn.product)
      if (prepared.error) say(prepared.error, { project: prepared.project })
      else requestApproval(prepared.approval, '我已准备好发给厂商的内容。请在弹窗核对产品和摘要，批准后我会提交询价。')
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  async function recoverWrite() {
    if (!pendingWrite.value || working.value) return
    try {
      const summary = await journal.retry(progress => {
        working.value = `正在用原请求恢复 (${progress.index + 1}/${progress.total})：${progress.label}`
      })
      if (summary) await finishSummary(summary)
    } catch (cause) {
      reportExecutionError(cause)
    } finally {
      working.value = ''
    }
  }

  return {
    demo, input, turns, projects, selectedId, inquiryProjects, loadingProjects, projectError, working,
    approval, approvalOpen, approvalError, pendingWrite, recoveryError,
    inquiryStepsPreview, applyMarkupToApproval, refreshApprovalMatches, refreshVisitPreview, formatMoneyCents,
    loadProjects, dispatch, fillManually, closeApproval, reopenApproval, declineApproval, approve, reviewSavedDraft, recoverWrite,
  }
}
