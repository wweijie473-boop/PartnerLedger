<script setup>
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { ElDialog } from 'element-plus'
import WorkspaceShell from './WorkspaceShell.vue'
import { UiBadge, UiButton, UiCard, UiField, UiIcon, UiIconTile, UiMetricCard, UiProgress, UiState } from './ui/index.js'
import { defaultExpiry, deliverySteps, isValidQuote, money, orderId, productName, products, roles, seedCases, statuses } from './demo'

const storageKey = 'partnerledger-prototype-v1'
function readCache() {
  try { return JSON.parse(localStorage.getItem(storageKey)) } catch { return null }
}
const cached = readCache()
const records = ref(Array.isArray(cached?.records) ? cached.records : seedCases())
const role = ref(cached?.role === null ? null : cached?.role === 'vendor' ? 'vendor' : 'dealer')
watch([records, role], () => {
  try { localStorage.setItem(storageKey, JSON.stringify({ records: records.value, role: role.value })) } catch { /* current tab still works */ }
}, { deep: true })

const nav = [
  { id: 'home', label: '协作总览', icon: 'overview' },
  { id: 'catalog', label: '产品目录', icon: 'catalog' },
  { id: 'requests', label: '需求与询价', icon: 'request' },
  { id: 'quotes', label: '报价中心', icon: 'quote' },
  { id: 'orders', label: '订单与交付', icon: 'delivery' },
]
const steps = [
  { label: '提交询价', note: '经销商 · 确认需求', icon: 'request' },
  { label: '正式报价', note: '厂家 · 发送报价', icon: 'quote' },
  { label: '确认下单', note: '经销商 · 确认版本', icon: 'order' },
  { label: '交付跟踪', note: '厂家 · 更新进度', icon: 'delivery' },
]
const view = ref('home')
const query = ref('')
const filter = ref('all')
const demoState = ref('normal')
const modal = ref('')
const selectedId = ref(null)
const message = ref('')
let noticeTimer
const product = ref(null)
const request = reactive({ title: '', product: '', seats: '', description: '' })
const input = reactive({ amount: '', expiry: defaultExpiry, notes: '', reason: '' })
const error = ref('')
const isDealer = computed(() => role.value === 'dealer')
const selected = computed(() => records.value.find((item) => item.id === selectedId.value))
const latestQuote = computed(() => selected.value?.quotes.at(-1))
const currentPage = computed(() => nav.find((item) => item.id === view.value))
const orders = computed(() => records.value.filter((item) => ['ordered', 'delivered'].includes(item.state)))
const submittedRequests = computed(() => records.value.filter((item) => !['draft', 'withdrawn'].includes(item.state)).length)
const activeQuotes = computed(() => records.value.filter((item) => item.state === 'quoted').length)
const activeOrders = computed(() => orders.value.filter((item) => item.state === 'ordered').length)
const progress = computed(() => Math.round(orders.value.reduce((sum, item) => sum + item.progress, 0) / Math.max(1, orders.value.length * deliverySteps.length) * 100))
const rows = computed(() => {
  let list = records.value
  if (view.value === 'quotes') list = list.filter((item) => item.quotes.length || (!isDealer.value && ['submitted', 'revision'].includes(item.state)))
  if (view.value === 'orders') list = orders.value
  if (!isDealer.value) list = list.filter((item) => item.state !== 'draft')
  const search = query.value.trim().toLowerCase()
  if (search) list = list.filter((item) => `${item.id} ${orderId(item)} ${item.title} ${productName(item.product)}`.toLowerCase().includes(search))
  if (filter.value !== 'all') list = list.filter((item) => item.state === filter.value)
  return view.value === 'home' ? list.slice(0, 4) : list
})
const modalOpen = computed({ get: () => !!modal.value, set: (show) => { if (!show) modal.value = '' } })
const modalTitle = computed(() => ({ detail: '协作记录', request: selected.value?.state === 'draft' ? '编辑需求草稿' : '新建需求', quote: '发送报价', reason: '填写说明', product: '产品详情', role: '切换演示身份', reset: '重置演示数据' }[modal.value] ?? ''))
const reasonAction = ref('')
const today = () => new Date().toLocaleDateString('sv-SE').slice(5)

function notify(text) {
  message.value = text
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => { message.value = '' }, 3400)
}
onUnmounted(() => clearTimeout(noticeTimer))

function go(page, preserveQuery = false) {
  view.value = page
  if (!preserveQuery) query.value = ''
  filter.value = 'all'
  demoState.value = 'normal'
}

function selectRole(next) {
  role.value = next
  modal.value = ''
  selectedId.value = null
  go('home')
}

function showDetail(record) {
  selectedId.value = record.id
  error.value = ''
  modal.value = 'detail'
}

function openRequest(productId = '', draft = null) {
  if (!isDealer.value) return
  selectedId.value = draft?.id ?? null
  Object.assign(request, { title: draft?.title ?? '', product: draft?.product ?? productId, seats: draft?.seats ?? '', description: draft?.description ?? '' })
  error.value = ''
  modal.value = 'request'
}

function saveRequest(submit) {
  if (!isDealer.value) return
  const title = request.title.trim()
  const description = request.description.trim()
  const seats = Number(request.seats)
  if (!title || !description || !products.some((item) => item.id === request.product)) {
    error.value = '请填写需求名称、产品和需求描述。'
    return
  }
  if (request.seats !== '' && (!Number.isSafeInteger(seats) || seats <= 0)) {
    error.value = '授权人数需为正整数，暂未确认时可留空。'
    return
  }
  const fields = { title, description, product: request.product, seats: seats || null, state: submit ? 'submitted' : 'draft', date: today() }
  let record = selected.value
  if (record?.state === 'draft') Object.assign(record, fields)
  else {
    const next = Math.max(1000, ...records.value.map((item) => Number(item.id.slice(4)))) + 1
    record = { ...fields, id: `RFQ-${next}`, quotes: [], progress: 0 }
    records.value.unshift(record)
  }
  go('requests')
  showDetail(record)
  notify(submit ? '询价已提交给厂家' : '需求草稿已保存')
}

function openQuote() {
  if (isDealer.value || !['submitted', 'revision'].includes(selected.value?.state)) return
  Object.assign(input, { amount: latestQuote.value?.amount ?? '', expiry: defaultExpiry, notes: latestQuote.value?.notes ?? '' })
  error.value = ''
  modal.value = 'quote'
}

function sendQuote() {
  const item = selected.value
  if (isDealer.value || !item || !['submitted', 'revision'].includes(item.state)) return
  const amount = Number(input.amount)
  if (!/^\d+(\.\d{1,2})?$/.test(String(input.amount)) || amount <= 0 || !Number.isSafeInteger(Math.round(amount * 100))) {
    error.value = '请输入大于 0 的金额，最多两位小数。'
    return
  }
  if (!isValidQuote({ expiry: input.expiry }) || !input.notes.trim()) {
    error.value = '请填写明细，并选择今天或以后的有效期。'
    return
  }
  item.quotes.push({ amount, expiry: input.expiry, notes: input.notes.trim() })
  item.state = 'quoted'
  item.date = today()
  showDetail(item)
  notify('报价已发送，旧版本可继续查阅')
}

function changeState(from, to, actor, feedback) {
  const item = selected.value
  if (!item || role.value !== actor || !from.includes(item.state)) return false
  if (['accepted', 'ordered'].includes(to) && !isValidQuote(latestQuote.value)) {
    notify('当前报价已过期，请请求厂家发送新版报价')
    return false
  }
  item.state = to
  item.date = today()
  notify(feedback)
  return true
}

function openReason(action) {
  reasonAction.value = action
  input.reason = ''
  error.value = ''
  modal.value = 'reason'
}

function submitReason() {
  if (!input.reason.trim()) { error.value = '请填写说明。'; return }
  const item = selected.value
  let changed
  if (reasonAction.value === 'revision') changed = changeState(['quoted', 'accepted'], 'revision', 'dealer', '修改请求已发送给厂家')
  if (reasonAction.value === 'supplement') changed = changeState(['submitted'], 'supplement', 'vendor', '已请求经销商补充资料')
  if (reasonAction.value === 'reply') changed = changeState(['supplement'], 'submitted', 'dealer', '补充资料已提交')
  if (changed) {
    item.followUp = input.reason.trim()
    showDetail(item)
  }
}

function nextDelivery() {
  const item = selected.value
  if (role.value !== 'vendor' || item?.state !== 'ordered' || item.progress >= deliverySteps.length) return
  const step = deliverySteps[item.progress]
  item.progress++
  item.date = today()
  if (item.progress === deliverySteps.length) item.state = 'delivered'
  notify(`${step}已完成`)
}

function reset() {
  records.value = seedCases()
  selectRole('dealer')
  notify('已恢复初始演示数据')
}
</script>

<template>
  <div v-if="!role" class="login-screen">
    <img class="brand-mark" src="/brand/partnerledger-logo.png" width="38" height="38" alt="" />
    <p class="eyebrow">PARTNERLEDGER</p>
    <h1>进入渠道协作工作台</h1>
    <p class="muted">选择一个演示身份体验 ERP 分销协作流程。</p>
    <div class="role-options">
      <button v-for="(item, key) in roles" :key="key" class="role-option" @click="selectRole(key)">
        <UiIcon :name="key" :size="22" />
        <span><strong>{{ item.label }}</strong><small>{{ item.company }}</small></span><UiIcon name="forward" />
      </button>
    </div>
    <p class="demo-footer"><a class="live-entry" href="/">使用后端账号登录</a></p>
  </div>

  <WorkspaceShell v-else :nav="nav" :view="view" :organization="roles[role].company"
    :account-label="`${roles[role].label}工作台`" environment="本地演示环境" @navigate="go" @account="modal = 'role'">
    <template #actions>
      <form class="top-search" @submit.prevent="go('requests', true)"><UiIcon name="search" :size="16" /><input v-model="query" placeholder="搜索记录" aria-label="搜索记录" /></form>
    </template>
        <section class="page-heading">
          <div><h1>{{ currentPage.label }}</h1>
            <p class="muted">{{ { home: '跟进从客户需求到订单交付的每一步。', catalog: '查看可供询价的 ERP 产品、模块和授权方式。', requests: '跟进需求、提交询价并补充资料。', quotes: '查看报价明细、有效期和历史版本。', orders: '从订单查看授权、部署和上线交接进度。' }[view] }}</p></div>
          <UiButton v-if="isDealer && ['home', 'requests'].includes(view)" variant="primary" icon="add" @click="openRequest()">新建需求</UiButton>
        </section>

        <template v-if="view === 'home'">
          <section class="metric-grid" aria-label="协作数据">
            <UiMetricCard label="询价记录" :value="submittedRequests" description="已提交的业务需求" icon="request" tone="brand" interactive @click="go('requests')" />
            <UiMetricCard label="待确认报价" :value="activeQuotes" description="等待经销商确认" icon="quote" tone="warning" interactive @click="go('quotes')" />
            <UiMetricCard label="交付中订单" :value="activeOrders" description="已下单并进入交付" icon="order" tone="info" interactive @click="go('orders')" />
          </section>

          <section class="insight-grid">
            <UiCard title="协作流程">
              <div class="journey-steps">
                <div v-for="(step, index) in steps" :key="step.label" class="journey-step">
                  <UiIconTile :name="step.icon" />
                  <UiIcon v-if="index < steps.length - 1" name="chevron-right" class="step-arrow" :size="15" />
                  <strong>{{ step.label }}</strong><small>{{ step.note }}</small>
                </div>
              </div>
            </UiCard>
            <UiCard title="交付进度" description="交付节点完成率" muted>
              <div class="progress-value"><strong>{{ progress }}<span>%</span></strong><small>{{ orders.length }} 笔订单</small></div>
              <UiProgress :value="progress" label="交付节点完成率" />
            </UiCard>
          </section>
        </template>

        <section v-if="view === 'catalog'" class="catalog-grid" aria-label="产品目录">
          <UiCard v-for="item in products" :key="item.id" as="article" :title="item.name" :description="item.modules">
            <template #actions><UiIconTile name="product" /></template>
            <dl class="details"><dt>部署方式</dt><dd>{{ item.deployment }}</dd><dt>授权模式</dt><dd>{{ item.license }}</dd></dl>
            <template #footer><UiButton variant="text" end-icon="forward" @click="product = item; modal = 'product'">查看产品</UiButton></template>
          </UiCard>
        </section>

        <UiCard v-else class="records" :title="view === 'home' ? '最近协作记录' : '记录列表'" :description="view === 'home' ? '' : `${rows.length} 条记录`" flush>
          <template #actions>
            <UiButton v-if="view === 'home'" variant="text" end-icon="forward" @click="go('requests')">查看全部</UiButton>
            <label v-else class="filter"><span>状态</span><select v-model="filter"><option value="all">全部状态</option><option v-for="(data, key) in statuses" :key="key" :value="key">{{ data[0] }}</option></select></label>
          </template>
          <UiState v-if="demoState === 'loading'" state="loading" title="正在加载协作记录" />
          <UiState v-else-if="demoState === 'denied'" state="denied" title="你没有权限查看这些记录" description="请联系当前组织的管理员。" />
          <UiState v-else-if="demoState === 'error'" state="error" title="记录加载失败"><UiButton compact @click="demoState = 'normal'">重试</UiButton></UiState>
          <UiState v-else-if="demoState === 'empty' || !rows.length" title="暂无协作记录" :description="query ? '试试其他关键词。' : '符合当前条件的记录会显示在这里。'" />
          <div v-else class="table-scroll"><table>
            <thead><tr><th>{{ view === 'orders' ? '订单' : '需求 / 询价' }}</th><th>产品</th><th>状态</th><th>最近更新</th><th>操作</th></tr></thead>
            <tbody><tr v-for="item in rows" :key="item.id">
              <td><span class="record-label"><UiIconTile class="record-icon" :name="view === 'orders' ? 'order' : 'request'" :size="16" /><span><strong>{{ item.title }}</strong><small>{{ view === 'orders' ? orderId(item) : item.id }}</small></span></span></td>
              <td>{{ productName(item.product) }}</td><td><UiBadge :tone="statuses[item.state][1]" dot>{{ statuses[item.state][0] }}</UiBadge></td>
              <td class="date">{{ item.date }}</td><td><UiButton variant="text" compact end-icon="chevron-right" class="details-link" :aria-label="`查看 ${item.title}`" @click="showDetail(item)">查看</UiButton></td>
            </tr></tbody>
          </table></div>
        </UiCard>

        <footer class="demo-footer"><span>演示数据 · 保存在当前浏览器</span><div class="footer-actions"><label v-if="view !== 'catalog'">界面状态 <select v-model="demoState"><option value="normal">正常</option><option value="loading">加载中</option><option value="empty">空数据</option><option value="denied">无权限</option><option value="error">加载失败</option></select></label><button @click="modal = 'reset'">重置数据</button><a href="/">正式登录</a><a href="/ui">组件库</a></div></footer>
  </WorkspaceShell>

  <ElDialog v-model="modalOpen" :title="modalTitle" width="min(584px, calc(100vw - 28px))" align-center destroy-on-close>
    <div v-if="modal === 'role'" class="role-options">
      <button v-for="(item, key) in roles" :key="key" class="role-option" @click="selectRole(key)">
        <UiIcon :name="key" :size="22" />
        <span><strong>{{ item.label }}</strong><small>{{ item.company }}</small></span>
        <UiBadge v-if="role === key" tone="brand">当前身份</UiBadge><UiIcon v-else name="forward" :size="17" />
      </button>
      <UiButton variant="text" icon="sign-out" @click="selectRole(null)">退出演示身份</UiButton>
    </div>

    <div v-if="modal === 'reset'">
      <p>重置会清除此浏览器中创建的需求、报价和订单，恢复初始演示数据。</p>
      <div class="modal-actions"><UiButton @click="modal = ''">取消</UiButton><UiButton variant="danger" @click="reset">确认重置</UiButton></div>
    </div>

    <div v-if="modal === 'product' && product">
      <h2>{{ product.name }}</h2>
      <dl class="details"><dt>模块</dt><dd>{{ product.modules }}</dd><dt>部署方式</dt><dd>{{ product.deployment }}</dd><dt>授权模式</dt><dd>{{ product.license }}</dd></dl>
      <div v-if="isDealer" class="modal-actions"><UiButton variant="primary" @click="openRequest(product.id)">为此产品发起询价</UiButton></div>
    </div>

    <form v-if="modal === 'request'" class="form-grid" @submit.prevent="saveRequest(true)">
      <UiField label="需求名称" required v-slot="{ control }"><input v-bind="control" v-model="request.title" maxlength="70" placeholder="例如：生产企业 ERP 升级" /></UiField>
      <div class="form-row">
        <UiField label="ERP 产品" required v-slot="{ control }">
          <select v-bind="control" v-model="request.product"><option value="" disabled>选择产品</option><option v-for="item in products" :key="item.id" :value="item.id">{{ item.name }}</option></select>
        </UiField>
        <UiField label="授权用户数" hint="待确认时可以留空。" v-slot="{ control }"><input v-bind="control" v-model="request.seats" type="number" min="1" /></UiField>
      </div>
      <UiField label="需求描述" required v-slot="{ control }">
        <textarea v-bind="control" v-model="request.description" rows="4" maxlength="800" placeholder="写明所需模块、部署方式和交付要求。"></textarea>
      </UiField>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <div class="modal-actions"><UiButton @click="saveRequest(false)">保存草稿</UiButton><UiButton type="submit" variant="primary">提交询价</UiButton></div>
    </form>

    <form v-if="modal === 'quote'" class="form-grid" @submit.prevent="sendQuote">
      <p class="muted">{{ selected.title }} · 版本 V{{ selected.quotes.length + 1 }}</p>
      <div class="form-row">
        <UiField label="报价总额（元）" required v-slot="{ control }"><input v-bind="control" v-model="input.amount" inputmode="decimal" placeholder="0.00" /></UiField>
        <UiField label="报价有效期" required v-slot="{ control }"><input v-bind="control" v-model="input.expiry" type="date" /></UiField>
      </div>
      <UiField label="报价明细" required v-slot="{ control }">
        <textarea v-bind="control" v-model="input.notes" rows="4" maxlength="600" placeholder="说明授权、实施、培训费用与交付周期。"></textarea>
      </UiField>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <div class="modal-actions"><UiButton icon="back" @click="showDetail(selected)">返回</UiButton><UiButton type="submit" variant="primary">发送报价</UiButton></div>
    </form>

    <form v-if="modal === 'reason'" class="form-grid" @submit.prevent="submitReason">
      <UiField :label="{ revision: '报价需要如何修改？', supplement: '需要经销商补充什么？', reply: '补充资料' }[reasonAction]" required v-slot="{ control }">
        <textarea v-bind="control" v-model="input.reason" rows="4" maxlength="500"></textarea>
      </UiField>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <div class="modal-actions"><UiButton @click="showDetail(selected)">返回</UiButton><UiButton type="submit" variant="primary">提交说明</UiButton></div>
    </form>

    <div v-if="modal === 'detail' && selected" class="record-detail">
      <div class="detail-heading"><div><p class="eyebrow">{{ selected.id }}<template v-if="['ordered', 'delivered'].includes(selected.state)"> / {{ orderId(selected) }}</template></p><h2>{{ selected.title }}</h2></div><UiBadge :tone="statuses[selected.state][1]" dot>{{ statuses[selected.state][0] }}</UiBadge></div>
      <dl class="details"><dt>产品</dt><dd>{{ productName(selected.product) }}</dd><dt>授权用户数</dt><dd>{{ selected.seats ?? '待确认' }}</dd><dt>需求说明</dt><dd>{{ selected.description }}</dd><template v-if="selected.followUp"><dt>协作备注</dt><dd>{{ selected.followUp }}</dd></template></dl>
      <section v-if="selected.quotes.length" class="detail-section"><h3>报价版本</h3><div v-for="(quote, index) in selected.quotes" :key="index" class="quote-line"><div><strong>V{{ index + 1 }} · {{ money.format(quote.amount) }}</strong><p>{{ quote.notes }}</p></div><small>{{ quote.expiry }} 到期<br />{{ index < selected.quotes.length - 1 ? '已替代' : isValidQuote(quote) ? '最新版本' : '已过期' }}</small></div></section>
      <section v-if="['ordered', 'delivered'].includes(selected.state)" class="detail-section"><h3>交付节点</h3><ol class="delivery-list"><li v-for="(step, index) in deliverySteps" :key="step"><span :class="{ done: index < selected.progress }"></span>{{ step }}<small>{{ index < selected.progress ? '已完成' : index === selected.progress ? '进行中' : '待开始' }}</small></li></ol></section>
      <div class="modal-actions">
        <template v-if="isDealer">
          <UiButton v-if="selected.state === 'draft'" variant="primary" @click="openRequest('', selected)">编辑草稿</UiButton>
          <UiButton v-if="selected.state === 'submitted' && !selected.quotes.length" variant="danger" @click="changeState(['submitted'], 'withdrawn', 'dealer', '询价已撤回')">撤回询价</UiButton>
          <UiButton v-if="selected.state === 'supplement'" variant="primary" @click="openReason('reply')">补充资料</UiButton>
          <UiButton v-if="['quoted', 'accepted'].includes(selected.state)" @click="openReason('revision')">请求修改报价</UiButton>
          <UiButton v-if="selected.state === 'quoted' && isValidQuote(latestQuote)" variant="primary" @click="changeState(['quoted'], 'accepted', 'dealer', '已确认当前报价版本')">确认报价</UiButton>
          <UiButton v-if="selected.state === 'accepted' && isValidQuote(latestQuote)" variant="primary" @click="changeState(['accepted'], 'ordered', 'dealer', '订单已生成')">生成订单</UiButton>
        </template>
        <template v-else>
          <UiButton v-if="selected.state === 'submitted'" @click="openReason('supplement')">请求补充资料</UiButton>
          <UiButton v-if="['submitted', 'revision'].includes(selected.state)" variant="primary" @click="openQuote">{{ selected.quotes.length ? '发送新版报价' : '填写正式报价' }}</UiButton>
          <UiButton v-if="selected.state === 'ordered'" variant="primary" @click="nextDelivery">完成{{ deliverySteps[selected.progress] }}</UiButton>
        </template>
      </div>
    </div>
  </ElDialog>
  <Transition><div v-if="message" class="notice" role="status"><UiIcon name="activity" :size="19" />{{ message }}</div></Transition>
</template>
