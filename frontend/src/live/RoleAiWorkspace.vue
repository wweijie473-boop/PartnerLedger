<script setup>
import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue'
import { ElDialog } from 'element-plus'
import { UiBadge, UiButton, UiField, UiIcon } from '../ui/index.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { formatCents } from '../channel/quoteMoney.js'
import { createAiSessionStore } from './aiChatSessions.js'
import { formatBubbleHtml, useAiTheme } from './aiMarkdown.js'
import { streamWorkspaceChat } from './aiStreamClient.js'
import { retailApi, readRetailCatalog } from './retailRequest.js'
import { useVoiceInput } from './useVoiceInput.js'
import './ai-workspace.css'

const props = defineProps({
  role: { type: String, required: true }, // 'vendor' | 'customer'
  request: { type: Function, required: true },
  organizationId: { type: String, required: true },
  userId: { type: String, required: true },
  organizationName: { type: String, default: '' },
})
const emit = defineEmits(['navigate', 'open-project', 'badges-changed'])

const isVendor = computed(() => props.role === 'vendor')
const repo = createHttpProjectRepository(props.request)
const controller = new AbortController()
const options = { signal: controller.signal }

const log = ref(null)
const composerInput = ref(null)
const skillsExpanded = ref(false)
const sidebarCollapsed = ref(false)
const mobileSidebarOpen = ref(false)
const searchOpen = ref(false)
const sessionQuery = ref('')

const { theme, toggleTheme } = useAiTheme()

const sessionStore = createAiSessionStore(`${props.role}:${props.organizationId}:${props.userId}`)
const initialStore = sessionStore.loadInitial('')
const sessions = ref(initialStore.sessions)
const activeSessionId = ref(initialStore.activeId)
const currentSessionObj = () => sessions.value.find(item => item.id === activeSessionId.value) ?? sessions.value[0]
const activeSession = computed(() => currentSessionObj())

const input = ref('')
const turns = ref((currentSessionObj()?.turns ?? []).map(item => reactive({ ...item })))
const working = ref('')
const approval = ref(null)
const approvalOpen = ref(false)
const approvalError = ref('')
const disposed = ref(false)
let turnId = turns.value.reduce((max, item) => Math.max(max, Number(item.id) || 0), 0)

const busy = computed(() => Boolean(working.value || approval.value))
const filteredSessions = computed(() => {
  const kw = sessionQuery.value.trim().toLowerCase()
  if (!kw) return sessions.value
  return sessions.value.filter(sess => sess.title.toLowerCase().includes(kw))
})

watch(input, () => {
  nextTick(() => {
    const el = composerInput.value
    if (!el) return
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 168) + 'px'
  })
})

function toggleSidebar() {
  if (window.matchMedia('(max-width: 860px)').matches) {
    mobileSidebarOpen.value = !mobileSidebarOpen.value
  } else {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }
}

function toggleSearch() {
  searchOpen.value = !searchOpen.value
  if (!searchOpen.value) sessionQuery.value = ''
}

const {
  recording, transcribing, autoSending, waveformBars, voiceError,
  startVoice, finishVoice, sendVoice, cancelVoice,
} = useVoiceInput({
  demo: false,
  request: props.request,
  input,
  onReady: () => nextTick(() => composerInput.value?.focus()),
  onSend: () => dispatch(),
})

const vendorShortcuts = [
  { id: 'v_knowledge', icon: 'catalog', label: '提取录入产品知识库', description: '从产品介绍自动提取模块、席位与部署方式并保存', prompt: '帮我把云序 MES 制造执行的知识库更新一下：支持生产排程、工序报工、质检追溯模块，支持云部署和私有化部署，10到800席' },
  { id: 'v_pricing', icon: 'quote', label: '配置直购定价与分佣', description: '一键设定已发布软件的直购单价、代理分佣与年度属性', prompt: '帮我配置云序 MES 制造执行的直购规则：单价 3980 元，代理商分佣 45%，设为年度订阅' },
  { id: 'v_quote', icon: 'request', label: '发送渠道底价单', description: '定位待报价的渠道询价项目并打包生成渠道底价', prompt: '帮我给当前待报价的渠道询价项目发送渠道底价' },
  { id: 'v_receipt', icon: 'order', label: '登记直购订单收款', description: '核对待收款直购订单并录入收款凭证号', prompt: '帮我登记当前待收款直购订单的厂商收款凭证' },
  { id: 'v_settle', icon: 'check', label: '登记代理佣金结算', description: '筛选待结佣订单并录入转账流水完成分账', prompt: '帮我为待结算的直购订单办理代理商佣金结算登记' },
  { id: 'v_ticket', icon: 'delivery', label: '回复渠道售后工单', description: '自动草拟处理方案并弹窗回复待办渠道工单', prompt: '帮我查看待处理的渠道售后工单并准备回复方案' },
]

const customerShortcuts = [
  { id: 'c_create', icon: 'add', label: '痛点梳理一键立项', description: '用大白话描述管理痛点，自动生成规范采购需求并立项', prompt: '我们是 90 人的精密电子厂，想上云端采购、库存批次和车间扫码报工系统，计划下月底上线，帮我整理需求并发起采购立项' },
  { id: 'c_quote', icon: 'quote', label: '审阅确认对客报价', description: '核对代理商发来的对客销售报价明细并一键确认', prompt: '帮我核对当前待确认的对客报价单并准备确认审批' },
  { id: 'c_accept', icon: 'delivery', label: '确认项目实施验收', description: '核查代理商提交的实施交付记录并完成项目验收', prompt: '帮我查看待验收的采购项目并提交确认验收' },
  { id: 'c_order', icon: 'order', label: '确认代客直购订单', description: '核对代理商代下的标准化软件订单并确认生效', prompt: '帮我核对并确认当前待我确认的厂商软件直购订单' },
]

const shortcuts = computed(() => isVendor.value ? vendorShortcuts : customerShortcuts)

const linkLabels = {
  projects: isVendor.value ? '打开渠道询价' : '打开我的采购项目',
  catalog: '管理产品目录',
  pricing: '打开订购价格与佣金',
  'manufacturer-receipts': '打开直收订单与收款',
  'manufacturer-settlements': '打开代理商佣金结算',
  'vendor-finance': '打开财务办理',
  'vendor-tickets': '打开渠道工单',
  'retail-orders': '打开厂商订购订单',
  account: '打开账号与组织',
}

function persistCurrentSession() {
  const active = currentSessionObj()
  if (!active) return
  const updated = sessionStore.buildUpdatedSession(active, turns.value, '')
  sessions.value = [updated, ...sessions.value.filter(item => item.id !== updated.id)]
  sessionStore.saveSessions(activeSessionId.value, sessions.value)
}

function startNewSession() {
  if (working.value || approval.value) return
  mobileSidebarOpen.value = false
  const active = currentSessionObj()
  if (active && active.turns.length === 0) {
    input.value = ''
    nextTick(() => composerInput.value?.focus())
    return
  }
  const fresh = sessionStore.makeEmptySession('')
  sessions.value = [fresh, ...sessions.value]
  activeSessionId.value = fresh.id
  turns.value = []
  input.value = ''
  sessionStore.saveSessions(fresh.id, sessions.value)
  nextTick(() => composerInput.value?.focus())
}

function switchSession(id) {
  if (working.value || approval.value) return
  const target = sessions.value.find(item => item.id === id)
  if (!target) return
  activeSessionId.value = target.id
  turns.value = (target.turns ?? []).map(item => reactive({ ...item }))
  turnId = turns.value.reduce((max, item) => Math.max(max, Number(item.id) || 0), 0)
  sessionStore.saveSessions(activeSessionId.value, sessions.value)
  mobileSidebarOpen.value = false
  nextTick(() => { if (log.value) log.value.scrollTop = log.value.scrollHeight })
}

function deleteSession(id) {
  if (working.value || approval.value) return
  const remaining = sessions.value.filter(item => item.id !== id)
  if (!remaining.length) {
    const fresh = sessionStore.makeEmptySession('')
    sessions.value = [fresh]
    activeSessionId.value = fresh.id
    turns.value = []
  } else {
    sessions.value = remaining
    if (activeSessionId.value === id) {
      activeSessionId.value = remaining[0].id
      turns.value = (remaining[0].turns ?? []).map(item => reactive({ ...item }))
      turnId = turns.value.reduce((max, item) => Math.max(max, Number(item.id) || 0), 0)
    }
  }
  sessionStore.saveSessions(activeSessionId.value, sessions.value)
}

function clearCurrentSession() {
  deleteSession(activeSessionId.value)
}

onUnmounted(() => {
  disposed.value = true
  controller.abort()
})

function say(text, extra = {}) {
  if (disposed.value) return null
  const entry = reactive({ id: ++turnId, role: 'assistant', text, ...extra })
  turns.value.push(entry)
  persistCurrentSession()
  nextTick(() => { if (log.value) log.value.scrollTop = log.value.scrollHeight })
  return entry
}

function offsetDate(days = 30) {
  const dt = new Date(Date.now() + days * 86400000)
  return dt.toLocaleDateString('sv-SE', { timeZone: 'Asia/Shanghai' })
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

async function openRoleSkill(route) {
  working.value = '正在拉取待办单据组装审批单'
  const intent = route.intent

  if (intent === 'vendor_save_knowledge') {
    const page = await props.request('/api/products?limit=50', options)
    const products = page?.items ?? []
    if (!products.length) {
      say('当前名下暂无软件产品，请先前往「产品目录」创建产品。', { navAction: { label: '前往产品目录', view: 'catalog' } })
      return
    }
    const chosen = products.find(p => String(p.id) === String(route.product_id || '').replace(/^#/, '')) ?? products[0]
    let existing = { category: '', modules: [], deployment_modes: [], license: {}, sources: [] }
    try {
      existing = await props.request(`/api/products/${encodeURIComponent(chosen.id)}/knowledge`, options)
    } catch {}
    requestApproval({
      kind: 'vendor_save_knowledge',
      title: '审批更新产品匹配知识库',
      products,
      productId: String(chosen.id),
      category: route.category || existing.category || '企业管理软件',
      modulesText: (Array.isArray(route.modules) && route.modules.length ? route.modules : existing.modules || []).join('、'),
      modes: Array.isArray(route.deployment_modes) && route.deployment_modes.length ? [...route.deployment_modes] : [...(existing.deployment_modes || ['cloud'])],
      minimum: route.min_seats ?? existing.license?.minimum ?? 5,
      maximum: route.max_seats ?? existing.license?.maximum ?? 500,
      note: route.license_note || existing.license?.note || '支持按席位阶梯扩容。',
      sourceTitle: route.source_title || existing.sources?.[0]?.title || `${chosen.name} 产品白皮书`,
      sourceRef: route.source_reference || existing.sources?.[0]?.reference || `${chosen.version} 官方规格说明`,
      sourceExcerpt: route.source_excerpt || existing.sources?.[0]?.excerpt || '支持标准功能模块灵活选配与多端协同。',
      existingSources: existing.sources || [],
    }, '已从你的描述中提取结构化产品知识库，请在弹窗核对后保存，保存后代理商 AI 选品立即可读。')
    return
  }

  if (intent === 'vendor_set_pricing') {
    const [page, configured] = await Promise.all([
      props.request('/api/products?limit=50', options),
      props.request(retailApi + '/manufacturer-offers', options),
    ])
    const products = (page?.items ?? []).filter(p => p.published)
    if (!products.length) {
      say('当前没有已发布的软件产品可配置直购规则，请先在「产品目录」发布产品。', { navAction: { label: '前往产品目录', view: 'catalog' } })
      return
    }
    const chosen = products.find(p => String(p.id) === String(route.product_id || '').replace(/^#/, '')) ?? products[0]
    const prev = (configured?.items ?? []).find(o => String(o.product_id) === String(chosen.id))
    requestApproval({
      kind: 'vendor_set_pricing',
      title: '审批配置软件直购单价与代理分佣',
      products,
      productId: String(chosen.id),
      unitPrice: route.unit_price != null ? Number(route.unit_price).toFixed(2) : (prev ? (prev.unit_cents / 100).toFixed(2) : '1998.00'),
      commissionPercent: route.commission_percent != null ? String(route.commission_percent) : (prev ? String(prev.commission_bps / 100) : '45'),
      annual: route.annual != null ? Boolean(route.annual) : (prev?.annual ?? true),
      active: true,
    }, '已准备好直购价格与代理商分佣规则审批单，确认后代理商可立即发起推广与代客下单。')
    return
  }

  if (intent === 'vendor_channel_quote') {
    const list = await repo.list(options)
    const eligible = list.filter(p => p.actions?.includes('channelQuote'))
    if (!eligible.length) {
      say('当前没有可发送渠道底价的询价项目。', { navAction: { label: '查看渠道询价队列', view: 'projects' } })
      return
    }
    const cleanId = String(route.project_id || '').replace(/^#/, '')
    const target = eligible.find(p => String(p.id) === cleanId) ?? (eligible.find(p => p.isTodo) ?? eligible[0])
    const detail = await repo.get(target.id, options)
    const last = detail.channelQuotes?.at(-1)
    requestApproval({
      kind: 'vendor_channel_quote',
      title: '审批发布厂商渠道底价',
      project: detail,
      lineLabel: route.line_label || last?.lines?.[0]?.label || `${detail.productName || '软件企业授权'} 标准授权包`,
      quantity: route.quantity || last?.lines?.[0]?.quantity || 1,
      unitPrice: route.unit_price != null ? Number(route.unit_price).toFixed(2) : (last?.lines?.[0]?.unitPrice || '36000.00'),
      deliveryDays: route.delivery_days || last?.deliveryDays || 10,
      validUntil: offsetDate(route.valid_days || 30),
      note: route.reply || '厂商标准渠道结算底价（含原厂二线技术支持与版本升级保障）。',
    }, `已为渠道项目「${detail.title}」打包好渠道底价审批单（该底价仅经办代理商可见，终端客户不可见）。`)
    return
  }

  if (intent === 'vendor_confirm_receipt' || intent === 'vendor_settle_commission') {
    const isReceipt = intent === 'vendor_confirm_receipt'
    const targetStatus = isReceipt ? 'payment' : 'settlement'
    const res = await props.request(`${retailApi}/orders?status=${targetStatus}`, options)
    const items = res?.items ?? []
    if (!items.length) {
      say(isReceipt ? '当前没有处于「待厂商确认收款」状态的直购订单。' : '当前没有处于「待厂商结算佣金」状态的直购订单。', {
        navAction: { label: isReceipt ? '打开直收订单' : '打开佣金结算', view: isReceipt ? 'manufacturer-receipts' : 'manufacturer-settlements' },
      })
      return
    }
    const cleanId = String(route.order_id || '').replace(/^#/, '')
    const chosen = items.find(o => String(o.id) === cleanId) ?? items[0]
    requestApproval({
      kind: intent,
      title: isReceipt ? '审批登记直购订单收款' : '审批登记代理商佣金结算',
      orders: items,
      orderId: String(chosen.id),
      reference: route.reference || ((isReceipt ? 'RCV-' : 'SET-') + new Date().toISOString().slice(0, 10).replace(/-/g, '') + '-' + chosen.id),
    }, isReceipt ? `已定位待收款订单 #${chosen.id}，核对收款凭证号后即可确认入账。` : `已定位待结佣订单 #${chosen.id}，核对转账流水号后即可完成佣金核销。`)
    return
  }

  if (intent === 'vendor_resolve_ticket') {
    const tickets = await readRetailCatalog(path => props.request(path, options), `${retailApi}/operations/tickets`)
    const openItems = tickets.filter(t => t.status === 'open')
    if (!openItems.length) {
      say('当前没有待厂商处理的渠道售后工单。', { navAction: { label: '打开渠道工单', view: 'vendor-tickets' } })
      return
    }
    const cleanId = String(route.ticket_id || '').replace(/^#/, '')
    const chosen = openItems.find(t => String(t.id) === cleanId) ?? openItems[0]
    requestApproval({
      kind: 'vendor_resolve_ticket',
      title: '审批回复渠道售后工单',
      ticket: chosen,
      reply: route.reply || '原厂技术团队已核实该工单问题并提供配置补丁与交付指引，请代理商协助客户验证后确认关闭。',
    }, `已为你准备好渠道工单 #${chosen.id}《${chosen.subject}》的处理回复审批单。`)
    return
  }

  if (intent === 'customer_create_project') {
    const linkPage = await props.request('/api/channel/v1/organization-links', options)
    const dealers = linkPage?.items ?? []
    if (!dealers.length) {
      say('贵司当前尚未绑定合作代理商。请先前往「账号与组织」输入代理商提供的绑定码建立关联，即可发起采购立项。', {
        navAction: { label: '前往账号与组织绑定代理商', view: 'account' },
      })
      return
    }
    requestApproval({
      kind: 'customer_create_project',
      title: '审批发起软件采购项目',
      dealers,
      resellerId: String(dealers[0].id),
      projectTitle: route.title || `${props.organizationName || '企业'} · 数字化软件采购项目`,
      requirement: route.requirement || '',
      dueDate: route.due_date || offsetDate(45),
    }, '已将你的业务痛点梳理为结构化采购立项单，确认后将直接同步至合作代理商跟进选型与报价。')
    return
  }

  if (intent === 'customer_confirm_quote') {
    const list = await repo.list(options)
    const eligible = list.filter(p => p.actions?.includes('confirmSales'))
    if (!eligible.length) {
      say('当前没有处于「待客户确认对客报价」阶段的采购项目。', { navAction: { label: '查看我的采购项目', view: 'projects' } })
      return
    }
    const cleanId = String(route.project_id || '').replace(/^#/, '')
    const target = eligible.find(p => String(p.id) === cleanId) ?? eligible[0]
    const detail = await repo.get(target.id, options)
    const quote = detail.salesQuotes?.at(-1)
    if (!quote) {
      say('暂未读取到该项目的有效对客销售报价明细。')
      return
    }
    requestApproval({
      kind: 'customer_confirm_quote',
      title: '审批确认代理商对客报价方案',
      project: detail,
      quote,
    }, `已调出项目「${detail.title}」的最新对客报价单（${quote.id}），核对金额与交付期无误后即可确认锁价。`)
    return
  }

  if (intent === 'customer_accept_delivery') {
    const list = await repo.list(options)
    const eligible = list.filter(p => p.actions?.includes('accept'))
    if (!eligible.length) {
      say('当前没有处于「待客户确认验收」阶段的采购项目。', { navAction: { label: '查看我的采购项目', view: 'projects' } })
      return
    }
    const cleanId = String(route.project_id || '').replace(/^#/, '')
    const target = eligible.find(p => String(p.id) === cleanId) ?? eligible[0]
    const detail = await repo.get(target.id, options)
    requestApproval({
      kind: 'customer_accept_delivery',
      title: '审批确认项目实施验收',
      project: detail,
      note: route.note || '系统核心模块运行稳定、关键岗位培训已到位，同意正式结项验收。',
    }, `已调出项目「${detail.title}」的实施交付记录，确认无误后将完成正式验收。`)
    return
  }

  if (intent === 'customer_confirm_order') {
    const res = await props.request(`${retailApi}/orders?status=confirmation`, options)
    const items = res?.items ?? []
    if (!items.length) {
      say('当前没有待您确认的代理商代客直购订单。', { navAction: { label: '查看厂商订购订单', view: 'retail-orders' } })
      return
    }
    const cleanId = String(route.order_id || '').replace(/^#/, '')
    const chosen = items.find(o => String(o.id) === cleanId) ?? items[0]
    requestApproval({
      kind: 'customer_confirm_order',
      title: '审批确认厂商软件直购订单',
      order: chosen,
    }, `已调出待确认直购订单 #${chosen.id}（${chosen.product_name} × ${chosen.quantity} 份），确认后进入付款交付流程。`)
  }
}

async function approve() {
  const item = approval.value
  if (!item || working.value) return
  approvalError.value = ''
  working.value = '正在提交审批执行结果'
  try {
    if (item.kind === 'vendor_save_knowledge') {
      const modules = item.modulesText.split(/[\r\n,，、]+/).map(v => v.trim()).filter(Boolean)
      if (!modules.length || !item.modes.length) {
        approvalError.value = '请至少填写一项支持模块并勾选一种部署方式。'
        working.value = ''
        return
      }
      await props.request(`/api/products/${encodeURIComponent(item.productId)}/knowledge`, {
        method: 'PUT',
        body: {
          category: item.category.trim(),
          modules,
          deployment_modes: item.modes,
          license: {
            minimum: item.minimum === '' || item.minimum == null ? null : Number(item.minimum),
            maximum: item.maximum === '' || item.maximum == null ? null : Number(item.maximum),
            note: item.note.trim(),
          },
          sources: [{
            title: item.sourceTitle.trim() || '官方产品说明',
            reference: item.sourceRef.trim() || 'v1.0',
            excerpt: item.sourceExcerpt.trim() || '支持企业级业务协同。',
          }],
        },
      })
      approvalOpen.value = false
      approval.value = null
      say('产品匹配知识库已更新保存，全平台合作代理商 AI 选品工作流已实时同步。', {
        navAction: { label: '查看产品目录', view: 'catalog' },
      })
    } else if (item.kind === 'vendor_set_pricing') {
      const unitCents = Math.round(Number(item.unitPrice) * 100)
      const commBps = Math.round(Number(item.commissionPercent) * 100)
      if (!(unitCents > 0 && unitCents <= 100000000 && commBps >= 0 && commBps <= 10000)) {
        approvalError.value = '请填写有效的单价与 0–100% 之间的代理分佣比例。'
        working.value = ''
        return
      }
      await props.request(`${retailApi}/manufacturer-offers/${encodeURIComponent(item.productId)}`, {
        method: 'PUT',
        body: { unit_cents: unitCents, commission_bps: commBps, active: Boolean(item.active), annual: Boolean(item.annual) },
      })
      approvalOpen.value = false
      approval.value = null
      say(`直购定价与分佣规则（单价 ¥${Number(item.unitPrice).toFixed(2)}，分佣 ${item.commissionPercent}%）已生效。`, {
        navAction: { label: '查看订购价格与佣金', view: 'pricing' },
      })
    } else if (item.kind === 'vendor_channel_quote') {
      const updated = await repo.act(item.project.id, 'channelQuote', {
        lines: [{ label: item.lineLabel.trim(), quantity: Number(item.quantity), unitPrice: Number(item.unitPrice).toFixed(2) }],
        validUntil: item.validUntil,
        deliveryDays: Number(item.deliveryDays),
        note: item.note.trim(),
      }, item.project.revision)
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(`渠道底价已发送至经办代理商（当前阶段：${updated.stageLabel}）。`, {
        navAction: { label: '打开渠道询价队列', view: 'projects' },
      })
    } else if (item.kind === 'vendor_confirm_receipt' || item.kind === 'vendor_settle_commission') {
      if (!item.reference?.trim()) {
        approvalError.value = '请填写有效的凭证流水号。'
        working.value = ''
        return
      }
      const act = item.kind === 'vendor_confirm_receipt' ? 'receipt' : 'settle'
      await props.request(`${retailApi}/orders/${encodeURIComponent(item.orderId)}/${act}`, {
        method: 'POST',
        body: { reference: item.reference.trim() },
      })
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(act === 'receipt'
        ? `直购订单 #${item.orderId} 已确认收款，代理商佣金已自动计入待结算台账。`
        : `直购订单 #${item.orderId} 的代理商佣金已登记结清。`, {
        navAction: {
          label: act === 'receipt' ? '查看直收订单' : '查看佣金结算台账',
          view: act === 'receipt' ? 'manufacturer-receipts' : 'manufacturer-settlements',
        },
      })
    } else if (item.kind === 'vendor_resolve_ticket') {
      if (!item.reply?.trim()) {
        approvalError.value = '请填写工单处理结果。'
        working.value = ''
        return
      }
      await props.request(`${retailApi}/operations/tickets/${encodeURIComponent(item.ticket.id)}/resolve`, {
        method: 'POST',
        body: { reply: item.reply.trim(), revision: item.ticket.revision },
      })
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(`渠道工单 #${item.ticket.id} 处理结果已回复代理商，状态流转为「待代理商确认」。`, {
        navAction: { label: '查看渠道工单', view: 'vendor-tickets' },
      })
    } else if (item.kind === 'customer_create_project') {
      if (!item.projectTitle.trim() || !item.requirement.trim()) {
        approvalError.value = '请填写项目名称与具体业务需求。'
        working.value = ''
        return
      }
      const created = await repo.create({
        title: item.projectTitle.trim(),
        requirement: item.requirement.trim(),
        dueDate: item.dueDate || '',
        resellerId: item.resellerId,
      })
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(`采购项目 #${created.id}「${created.title}」已成功立项并推送给合作代理商跟进。`, {
        project: created,
      })
    } else if (item.kind === 'customer_confirm_quote') {
      const updated = await repo.act(item.project.id, 'confirmSales', { quoteId: item.quote.id }, item.project.revision)
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(`已确认对客报价单 ${item.quote.id}，合作代理商收到通知后将正式锁定采购订单。`, {
        project: updated,
      })
    } else if (item.kind === 'customer_accept_delivery') {
      const updated = await repo.act(item.project.id, 'accept', { note: item.note.trim() }, item.project.revision)
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(`项目「${updated.title}」已确认验收结项！`, {
        project: updated,
      })
    } else if (item.kind === 'customer_confirm_order') {
      await props.request(`${retailApi}/orders/${encodeURIComponent(item.order.id)}/confirm`, {
        method: 'POST',
        body: {},
      })
      approvalOpen.value = false
      approval.value = null
      emit('badges-changed')
      say(`软件直购订单 #${item.order.id} 已确认，请按厂商指引完成付款。`, {
        navAction: { label: '查看厂商订购订单', view: 'retail-orders' },
      })
    }
  } catch (cause) {
    approvalError.value = cause.message || '执行失败，请刷新核对后重试。'
  } finally {
    working.value = ''
  }
}

async function dispatch(value = input.value) {
  const text = value.trim()
  if (!text || working.value) return
  if (approval.value) { reopenApproval(); return }
  if ([...text].length > 2000) { say('单条消息限 2000 字，请缩短后发送。'); return }
  input.value = ''
  say(text, { role: 'user' })
  working.value = '正在理解你的诉求并按需调取数据'
  let streamingTurn = null

  try {
    let routedSkill = null
    const history = turns.value
      .filter(item => (item.role === 'user' || item.role === 'assistant') && item.text?.trim())
      .slice(-14)
      .map(item => ({ role: item.role, content: item.text.slice(0, 3500) }))

    await streamWorkspaceChat({
      messages: history,
      signal: controller.signal,
      onStatus(label) {
        if (!disposed.value && label) working.value = label
      },
      onRoute(route) {
        routedSkill = route
      },
      onDelta(chunk) {
        if (disposed.value) return
        if (!streamingTurn) {
          streamingTurn = reactive({ id: ++turnId, role: 'assistant', text: '', streaming: true })
          turns.value.push(streamingTurn)
          working.value = '正在流式生成回答'
        }
        streamingTurn.text += chunk
        nextTick(() => { if (log.value) log.value.scrollTop = log.value.scrollHeight })
      },
      onDone({ links }) {
        if (disposed.value || !streamingTurn) return
        streamingTurn.streaming = false
        const chatLinks = (links ?? [])
          .filter(v => linkLabels[v])
          .map(v => ({ view: v, label: linkLabels[v] }))
        if (chatLinks.length) streamingTurn.chatLinks = chatLinks
        persistCurrentSession()
      },
    })

    if (disposed.value) return

    if (routedSkill) {
      if (streamingTurn) {
        streamingTurn.streaming = false
        turns.value = turns.value.filter(item => item !== streamingTurn)
        streamingTurn = null
      }
      await openRoleSkill(routedSkill)
      return
    }

    if (streamingTurn) {
      streamingTurn.streaming = false
      persistCurrentSession()
    }
  } catch (cause) {
    if (streamingTurn) {
      streamingTurn.streaming = false
      if (streamingTurn.text.trim()) {
        streamingTurn.text += '\n\n（流式输出中断，以上为已生成内容）'
        persistCurrentSession()
        return
      }
      turns.value = turns.value.filter(item => item !== streamingTurn)
    }
    if (cause.name !== 'AbortError') {
      say(cause.message || 'AI 顾问暂时不可用，请稍后重试。')
    }
  } finally {
    working.value = ''
  }
}

function triggerSkill(skill) {
  skillsExpanded.value = false
  dispatch(skill.prompt)
}

function onEnter(event) {
  if (event.isComposing || event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) return
  event.preventDefault()
  skillsExpanded.value = false
  dispatch()
}

const money = cents => formatCents(BigInt(cents ?? 0))
const deploymentOptions = { cloud: '云部署', saas: 'SaaS', private: '私有化部署', on_premises: '本地部署' }
</script>

<template>
  <section
    class="ai-codex-shell"
    :class="{ 'is-sidebar-collapsed': sidebarCollapsed, 'is-mobile-sidebar-open': mobileSidebarOpen }"
    :data-ai-theme="theme"
    aria-labelledby="role-ai-title"
  >
    <div v-if="mobileSidebarOpen" class="ai-sidebar-backdrop" aria-hidden="true" @click="mobileSidebarOpen = false" />

    <aside class="ai-sidebar" aria-label="AI 会话列表">
      <div class="ai-sidebar-top">
        <div class="ai-brand">
          <span class="ai-brand-dot" aria-hidden="true" />
          <strong>渠智链 AI</strong>
          <small>{{ isVendor ? '软件厂商' : '企业客户' }}</small>
        </div>
        <div class="ai-sidebar-tools">
          <button type="button" class="ai-rail-icon-btn" :aria-pressed="searchOpen" title="搜索历史对话" @click="toggleSearch">
            <UiIcon name="search" :size="15" />
          </button>
          <button type="button" class="ai-rail-icon-btn" title="收起侧栏" @click="toggleSidebar">
            <UiIcon name="sidebar-close" :size="15" />
          </button>
        </div>
      </div>

      <div v-if="searchOpen" class="ai-sidebar-search">
        <input v-model="sessionQuery" type="search" placeholder="筛选历史会话…" aria-label="筛选历史会话" />
      </div>

      <button type="button" class="ai-new-chat-btn" :disabled="busy" @click="startNewSession">
        <UiIcon name="new-chat" :size="15" />
        <span>新对话</span>
      </button>

      <div class="ai-sidebar-scroll" aria-label="历史对话列表">
        <div class="ai-session-section">
          <div class="ai-section-label">最近</div>
          <div
            v-for="sess in filteredSessions"
            :key="sess.id"
            class="ai-session-row"
            :class="{ 'is-active': sess.id === activeSessionId }"
          >
            <button type="button" class="ai-session-btn" :disabled="busy" @click="switchSession(sess.id)">
              <span>{{ sess.title }}</span>
            </button>
            <button
              v-if="sessions.length > 1 || sess.turns.length"
              type="button"
              class="ai-session-del"
              :disabled="busy"
              aria-label="删除此对话"
              title="删除此对话"
              @click.stop="deleteSession(sess.id)"
            >
              <UiIcon name="close" :size="13" />
            </button>
          </div>
        </div>
      </div>

      <div class="ai-sidebar-foot">
        <span class="ai-org-pill">
          <UiIcon :name="isVendor ? 'vendor' : 'customer'" :size="13" />
          <span>{{ organizationName || (isVendor ? '软件厂商组织' : '企业客户组织') }}</span>
        </span>
      </div>
    </aside>

    <div class="ai-stage">
      <header class="ai-topbar">
        <div class="ai-topbar-left">
          <button
            type="button"
            class="ai-topbar-icon-btn ai-open-rail-btn"
            aria-label="展开历史会话侧栏"
            title="展开历史会话侧栏"
            @click="toggleSidebar"
          >
            <UiIcon name="sidebar-open" :size="16" />
          </button>
          <h1 id="role-ai-title" class="ai-topbar-title">
            {{ turns.length ? (activeSession?.title || 'AI 工作台') : (isVendor ? '软件厂商 AI 工作台' : '企业客户 AI 采购工作台') }}
          </h1>
        </div>

        <div class="ai-topbar-right">
          <button type="button" class="ai-topbar-pill" :aria-expanded="skillsExpanded" @click="skillsExpanded = !skillsExpanded">
            <UiIcon name="workflow" :size="14" />
            <span>技能 ({{ shortcuts.length }})</span>
          </button>
          <button
            v-if="turns.length"
            type="button"
            class="ai-topbar-icon-btn"
            :disabled="busy"
            title="清空当前对话"
            aria-label="清空当前对话"
            @click="clearCurrentSession"
          >
            <UiIcon name="trash" :size="15" />
          </button>
          <button
            type="button"
            class="ai-topbar-icon-btn"
            :title="theme === 'dark' ? '切换为极简浅色' : '切换为 Codex 深色'"
            :aria-label="theme === 'dark' ? '切换为极简浅色' : '切换为 Codex 深色'"
            @click="toggleTheme"
          >
            <UiIcon :name="theme === 'dark' ? 'sun' : 'moon'" :size="15" />
          </button>
        </div>
      </header>

      <div ref="log" class="ai-log" :class="{ 'is-empty': !turns.length }" role="log" aria-label="AI 对话记录">
        <div v-if="!turns.length" class="ai-empty-hero">
          <div class="ai-codex-emblem" aria-hidden="true">
            <svg viewBox="0 0 48 48" width="46" height="46" fill="none">
              <path
                d="M24 6c3.2 0 5.9 1.7 7.3 4.3 2.9-.5 5.9.8 7.4 3.4 1.5 2.6 1.1 5.8-.8 8 1.9 2.2 2.3 5.4.8 8-1.5 2.6-4.5 3.9-7.4 3.4-1.4 2.6-4.1 4.3-7.3 4.3s-5.9-1.7-7.3-4.3c-2.9.5-5.9-.8-7.4-3.4-1.5-2.6-1.1-5.8.8-8-1.9-2.2-2.3-5.4-.8-8 1.5-2.6 4.5-3.9 7.4-3.4C18.1 7.7 20.8 6 24 6Z"
                stroke="currentColor"
                stroke-width="2"
                stroke-linejoin="round"
              />
              <path d="M18.5 20.5L22.5 24l-4 3.5M24.5 27.5h5" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </div>
          <h2 class="ai-empty-title">{{ isVendor ? '今天要处理哪项渠道事务？' : '今天想推进哪项采购计划？' }}</h2>
          <p class="ai-empty-sub">
            {{ isVendor
              ? '实时盘点渠道询价、分佣结算与售后工单，一句话提取知识库并弹窗审批执行。'
              : '用自然语言梳理企业数字化痛点并一键立项，实时核对报价方案、订单与验收。' }}
          </p>

          <div class="ai-starter-chips">
            <button
              type="button"
              class="ai-starter-chip is-accent"
              :disabled="!!working"
              @click="dispatch(isVendor ? '帮我全面盘点一下目前有哪些待我处理的渠道询价、收款、结佣、财务申请和售后工单？' : '帮我看看目前名下所有采购项目和软件订单进展到哪一步了，有没有需要我拍板的？')"
            >
              <UiIcon name="ai" :size="14" />
              <span>{{ isVendor ? '盘点全渠道待办事项' : '盘点我的项目与待确认事项' }}</span>
            </button>
            <button
              v-for="skill in shortcuts.slice(0, 3)"
              :key="skill.id"
              type="button"
              class="ai-starter-chip"
              :disabled="!!working"
              @click="triggerSkill(skill)"
            >
              <UiIcon :name="skill.icon" :size="14" />
              <span>{{ skill.label }}</span>
            </button>
          </div>
        </div>

        <div v-else class="ai-stream-column">
          <div v-for="turn in turns" :key="turn.id" class="ai-turn" :class="{ 'is-user': turn.role === 'user' }">
            <template v-if="turn.role === 'user'">
              <div class="ai-user-bubble">{{ turn.text }}</div>
            </template>
            <template v-else>
              <div class="ai-assistant-row">
                <div class="ai-assistant-badge" aria-hidden="true"><UiIcon name="ai" :size="15" /></div>
                <div class="ai-turn-content">
                  <div class="ai-prose" :class="{ 'is-streaming': turn.streaming }" v-html="formatBubbleHtml(turn.text)" />

                  <div v-if="turn.approval" class="ai-result">
                    <div class="ai-result-head">
                      <strong>{{ turn.approval.title }}</strong>
                      <UiBadge :tone="approval === turn.approval ? 'warning' : 'neutral'">
                        {{ approval === turn.approval ? '待人工审批' : '审批已结束' }}
                      </UiBadge>
                    </div>
                    <button v-if="approval === turn.approval" type="button" class="ai-action-chip is-primary" @click="reopenApproval">
                      <UiIcon name="permission" :size="14" />
                      <span>打开审批弹窗</span>
                    </button>
                  </div>

                  <article v-if="turn.project" class="ai-result">
                    <div class="ai-result-head">
                      <strong>{{ turn.project.title }}</strong>
                      <UiBadge tone="info">{{ turn.project.stageLabel }}</UiBadge>
                    </div>
                    <p v-if="turn.project.nextStep">下一步：{{ turn.project.nextStep }}</p>
                    <button type="button" class="ai-action-chip" @click="emit('open-project', turn.project.id)">
                      <span>打开项目详情</span>
                      <UiIcon name="open-detail" :size="13" />
                    </button>
                  </article>

                  <div v-if="turn.chatLinks?.length" class="ai-prompts">
                    <button v-for="link in turn.chatLinks" :key="link.view" type="button" class="ai-action-chip" @click="emit('navigate', link.view)">
                      <span>{{ link.label }}</span>
                      <UiIcon name="open-detail" :size="13" />
                    </button>
                  </div>

                  <article v-if="turn.navAction" class="ai-result">
                    <button type="button" class="ai-action-chip" @click="emit('navigate', turn.navAction.view)">
                      <span>{{ turn.navAction.label }}</span>
                      <UiIcon name="open-detail" :size="13" />
                    </button>
                  </article>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>

      <div class="ai-dock">
        <div class="ai-dock-inner">
          <section v-if="skillsExpanded" class="ai-skill-popover" aria-label="专属工作流技能">
            <div class="ai-skill-popover-head">
              <strong>选择要调用的专属业务技能</strong>
              <button type="button" class="ai-rail-icon-btn" aria-label="关闭技能菜单" @click="skillsExpanded = false">
                <UiIcon name="close" :size="14" />
              </button>
            </div>
            <div class="ai-skill-grid">
              <button v-for="skill in shortcuts" :key="skill.id" type="button" class="ai-skill-card" :disabled="!!working" @click="triggerSkill(skill)">
                <span class="ai-skill-card-head"><UiIcon :name="skill.icon" :size="15" /><strong>{{ skill.label }}</strong></span>
                <small>{{ skill.description }}</small>
              </button>
            </div>
          </section>

          <div class="ai-context-strip">
            <div class="ai-context-items">
              <span class="ai-context-chip">
                <UiIcon :name="isVendor ? 'vendor' : 'customer'" :size="13" />
                <span>{{ organizationName || (isVendor ? '软件厂商组织' : '企业客户组织') }}</span>
              </span>
              <span class="ai-context-chip">
                <UiIcon name="folder" :size="13" />
                <span>{{ isVendor ? '渠道询价与产品台账' : '企业采购与直购台账' }}</span>
              </span>
            </div>
            <div v-if="working" class="ai-working-inline" role="status">
              <UiIcon name="loading" :size="13" class="ai-spin" />
              <span>{{ working }}…</span>
            </div>
          </div>

          <form
            class="ai-composer-pod"
            :class="{ 'is-voice-mode': recording || transcribing }"
            @submit.prevent="dispatch()"
          >
            <template v-if="recording || transcribing">
              <div class="ai-voice-capsule" role="status" :aria-label="transcribing ? '正在转写语音' : '正在录音'">
                <button
                  type="button"
                  class="ai-voice-circle-btn is-cancel"
                  aria-label="取消录音"
                  title="取消录音"
                  @click="cancelVoice"
                >
                  <UiIcon name="close" :size="16" />
                </button>

                <div class="ai-waveform-track" :class="{ 'is-transcribing': transcribing }" aria-hidden="true">
                  <span
                    v-for="(lvl, idx) in waveformBars"
                    :key="idx"
                    class="ai-wave-bar"
                    :class="{ 'is-active': lvl > 0.02 }"
                    :style="{ height: `${lvl > 0.02 ? Math.round(4 + lvl * 22) : 2}px` }"
                  />
                </div>

                <div class="ai-voice-actions">
                  <button
                    type="button"
                    class="ai-voice-circle-btn is-stop"
                    aria-label="完成说词并转写"
                    title="停止录音并转为文字编辑"
                    :disabled="transcribing"
                    @click="finishVoice"
                  >
                    <UiIcon v-if="transcribing && !autoSending" name="loading" :size="15" class="ai-spin" />
                    <span v-else class="ai-stop-square" aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    class="ai-voice-circle-btn is-send"
                    aria-label="完成录音并直接发送"
                    title="停止录音并直接发送"
                    :disabled="transcribing"
                    @click="sendVoice"
                  >
                    <UiIcon v-if="transcribing && autoSending" name="loading" :size="15" class="ai-spin" />
                    <UiIcon v-else name="send-up" :size="16" />
                  </button>
                </div>
              </div>
            </template>

            <template v-else>
              <p v-if="voiceError" class="ai-voice-error" role="alert">{{ voiceError }}</p>

              <textarea
                ref="composerInput"
                v-model="input"
                :aria-label="isVendor ? '给原厂运营助手发消息' : '给企业采购顾问发消息'"
                rows="1"
                maxlength="2000"
                :placeholder="isVendor ? '随心输入渠道运营指令或分析问题…' : '随心输入采购需求、报价核对或验收指令…'"
                @keydown.enter="onEnter"
              />

              <div class="ai-composer-bar">
                <div class="ai-composer-left">
                  <button
                    type="button"
                    class="ai-plus-trigger"
                    :aria-expanded="skillsExpanded"
                    title="打开专属技能面板"
                    @click="skillsExpanded = !skillsExpanded"
                  >
                    <UiIcon name="add" :size="15" />
                  </button>
                  <span class="ai-guard-badge">
                    <UiIcon name="shield-check" :size="13" />
                    <span>弹窗审批执行</span>
                  </span>
                </div>

                <div class="ai-composer-right">
                  <span class="ai-model-label">渠智链多模池 · 自动轮换</span>
                  <button
                    type="button"
                    class="ai-mic-btn"
                    aria-label="语音输入"
                    title="语音输入"
                    :disabled="!!working"
                    @click="startVoice"
                  >
                    <UiIcon name="mic" :size="15" />
                  </button>
                  <button
                    type="submit"
                    class="ai-send-btn"
                    aria-label="发送"
                    title="发送消息"
                    :disabled="!input.trim() || !!working"
                  >
                    <UiIcon name="send-up" :size="16" />
                  </button>
                </div>
              </div>
            </template>
          </form>
        </div>
      </div>
    </div>

    <ElDialog
      :model-value="approvalOpen && !!approval"
      :title="approval?.title || '人工审批确认'"
      width="min(620px, calc(100vw - 24px))"
      align-center
      :close-on-click-modal="false"
      @close="closeApproval"
    >
      <div v-if="approval" class="ai-approval">
        <template v-if="approval.kind === 'vendor_save_knowledge'">
          <UiField label="目标软件产品" v-slot="{ control }">
            <select v-bind="control" v-model="approval.productId">
              <option v-for="p in approval.products" :key="p.id" :value="String(p.id)">{{ p.name }} · {{ p.version }}</option>
            </select>
          </UiField>
          <UiField label="产品分类" v-slot="{ control }"><input v-bind="control" v-model="approval.category" maxlength="80" /></UiField>
          <UiField label="支持功能模块（顿号分隔）" v-slot="{ control }"><textarea v-bind="control" v-model="approval.modulesText" rows="2" /></UiField>
          <div class="ai-checkbox-row">
            <label v-for="(label, code) in deploymentOptions" :key="code" style="margin-right: 12px;">
              <input v-model="approval.modes" type="checkbox" :value="code" /> {{ label }}
            </label>
          </div>
          <div class="ai-fields">
            <UiField label="最低授权席位" v-slot="{ control }"><input v-bind="control" v-model.number="approval.minimum" type="number" min="1" /></UiField>
            <UiField label="最高授权席位" v-slot="{ control }"><input v-bind="control" v-model.number="approval.maximum" type="number" min="1" /></UiField>
          </div>
          <UiField label="资料出处名称" v-slot="{ control }"><input v-bind="control" v-model="approval.sourceTitle" maxlength="120" /></UiField>
          <UiField label="关键规格摘录" v-slot="{ control }"><textarea v-bind="control" v-model="approval.sourceExcerpt" rows="2" maxlength="500" /></UiField>
        </template>

        <template v-else-if="approval.kind === 'vendor_set_pricing'">
          <UiField label="已发布产品" v-slot="{ control }">
            <select v-bind="control" v-model="approval.productId">
              <option v-for="p in approval.products" :key="p.id" :value="String(p.id)">{{ p.name }} · {{ p.version }}</option>
            </select>
          </UiField>
          <div class="ai-fields">
            <UiField label="直购单价（元）" v-slot="{ control }"><input v-bind="control" v-model="approval.unitPrice" inputmode="decimal" /></UiField>
            <UiField label="代理商分佣比例（%）" v-slot="{ control }"><input v-bind="control" v-model="approval.commissionPercent" inputmode="decimal" /></UiField>
          </div>
          <label class="ai-checkbox-row"><input v-model="approval.annual" type="checkbox" /> 设为年度授权（收款后自动进入代理商续费跟进台账）</label>
        </template>

        <template v-else-if="approval.kind === 'vendor_channel_quote'">
          <p class="ai-muted-hint">项目：{{ approval.project.title }}（#{{ approval.project.id }}）· 此渠道底价仅经办代理商可见。</p>
          <UiField label="授权明细项" v-slot="{ control }"><input v-bind="control" v-model="approval.lineLabel" maxlength="100" /></UiField>
          <div class="ai-fields-three">
            <UiField label="数量" v-slot="{ control }"><input v-bind="control" v-model.number="approval.quantity" type="number" min="1" /></UiField>
            <UiField label="渠道底价单价（元）" v-slot="{ control }"><input v-bind="control" v-model="approval.unitPrice" inputmode="decimal" /></UiField>
            <UiField label="交付周期（天）" v-slot="{ control }"><input v-bind="control" v-model.number="approval.deliveryDays" type="number" min="1" /></UiField>
          </div>
          <UiField label="报价有效期至" v-slot="{ control }"><input v-bind="control" v-model="approval.validUntil" type="date" /></UiField>
          <UiField label="渠道说明" v-slot="{ control }"><textarea v-bind="control" v-model="approval.note" rows="2" /></UiField>
        </template>

        <template v-else-if="approval.kind === 'vendor_confirm_receipt' || approval.kind === 'vendor_settle_commission'">
          <UiField label="目标直购订单" v-slot="{ control }">
            <select v-bind="control" v-model="approval.orderId">
              <option v-for="o in approval.orders" :key="o.id" :value="String(o.id)">
                订单 #{{ o.id }} · {{ o.product_name }} · {{ o.customer }}（总额 {{ money(o.total_cents) }}）
              </option>
            </select>
          </UiField>
          <UiField :label="approval.kind === 'vendor_confirm_receipt' ? '收款凭证编号' : '佣金转账凭证编号'" v-slot="{ control }">
            <input v-bind="control" v-model="approval.reference" maxlength="100" />
          </UiField>
        </template>

        <template v-else-if="approval.kind === 'vendor_resolve_ticket'">
          <dl class="ai-send-details">
            <div><dt>工单标题</dt><dd>#{{ approval.ticket.id }} · {{ approval.ticket.subject }}</dd></div>
            <div><dt>问题详情</dt><dd>{{ approval.ticket.detail }}</dd></div>
          </dl>
          <UiField label="原厂处理结果回复" v-slot="{ control }"><textarea v-bind="control" v-model="approval.reply" rows="3" maxlength="500" /></UiField>
        </template>

        <template v-else-if="approval.kind === 'customer_create_project'">
          <UiField label="指派合作代理商" v-slot="{ control }">
            <select v-bind="control" v-model="approval.resellerId">
              <option v-for="d in approval.dealers" :key="d.id" :value="String(d.id)">{{ d.name }}</option>
            </select>
          </UiField>
          <UiField label="采购项目标题" v-slot="{ control }"><input v-bind="control" v-model="approval.projectTitle" maxlength="100" /></UiField>
          <UiField label="结构化业务需求描述" v-slot="{ control }"><textarea v-bind="control" v-model="approval.requirement" rows="4" maxlength="2000" /></UiField>
          <UiField label="期望上线日期" v-slot="{ control }"><input v-bind="control" v-model="approval.dueDate" type="date" /></UiField>
        </template>

        <template v-else-if="approval.kind === 'customer_confirm_quote'">
          <dl class="ai-send-details">
            <div><dt>采购项目</dt><dd>{{ approval.project.title }}（#{{ approval.project.id }}）</dd></div>
            <div><dt>报价单号</dt><dd>{{ approval.quote.id }} · 合计 ¥{{ approval.quote.total }}</dd></div>
            <div><dt>交付与效期</dt><dd>{{ approval.quote.deliveryDays }} 天交付 · 有效期至 {{ approval.quote.validUntil }}</dd></div>
            <div><dt>服务说明</dt><dd>{{ approval.quote.note || '标准授权与实施服务' }}</dd></div>
          </dl>
        </template>

        <template v-else-if="approval.kind === 'customer_accept_delivery'">
          <dl class="ai-send-details">
            <div><dt>采购项目</dt><dd>{{ approval.project.title }}（#{{ approval.project.id }}）</dd></div>
            <div><dt>实施记录</dt><dd>{{ approval.project.implementationNote || '已完成系统部署与培训交接' }}</dd></div>
          </dl>
          <UiField label="企业验收意见" v-slot="{ control }"><textarea v-bind="control" v-model="approval.note" rows="3" maxlength="500" /></UiField>
        </template>

        <template v-else-if="approval.kind === 'customer_confirm_order'">
          <dl class="ai-send-details">
            <div><dt>直购订单</dt><dd>#{{ approval.order.id }} · {{ approval.order.product_name }} {{ approval.order.product_version }}</dd></div>
            <div><dt>订购数量</dt><dd>{{ approval.order.quantity }} 份</dd></div>
            <div><dt>应付总额</dt><dd>{{ money(approval.order.total_cents) }}（收款方：{{ approval.order.manufacturer }}）</dd></div>
          </dl>
        </template>

        <p v-if="approvalError" class="form-error" role="alert">{{ approvalError }}</p>
        <div class="ai-approval-actions">
          <UiButton :disabled="!!working" @click="declineApproval">不同意</UiButton>
          <UiButton variant="primary" :loading="!!working" @click="approve">同意并自动执行</UiButton>
        </div>
      </div>
    </ElDialog>
  </section>
</template>
