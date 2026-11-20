<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElDialog } from 'element-plus'
import WorkspaceShell from '../WorkspaceShell.vue'
import { UiButton, UiBadge, UiCard, UiIcon, UiMetricCard, UiState } from '../ui/index.js'
import { actionLabels, roles } from './domain.js'
import { createDemoRepository } from './repository.js'
import ChannelAction from './ChannelAction.vue'
import ProjectDetail from './ProjectDetail.vue'
import DemoCatalog from './DemoCatalog.vue'
import OpportunityDesk from './OpportunityDesk.vue'
import VisitDrafts from '../live/VisitDrafts.vue'
import { createVisitDraftStore } from '../live/visitDrafts.js'
import { statusTone } from '../ui/statusTone.js'
import './workspace.css'
import './opportunities.css'

const props = defineProps({ role: { type: String, required: true } })
const roleKeys = { dealer: 'reseller', vendor: 'manufacturer', customer: 'customer' }
const names = { dealer: '代理商', vendor: '软件厂商', customer: '企业客户' }
const copy = {
  dealer: { title:'渠道业务工作台', en:'CHANNEL OPERATIONS', intro:'跟进客户需求、厂商回复与交付节点，优先处理需要你推进的项目。', ledger:'客户项目台账', add:'代录客户需求' },
  vendor: { title:'厂商履约工作台', en:'VENDOR OPERATIONS', intro:'集中处理代理商询价、渠道报价及已成单的授权交付。', ledger:'渠道询价队列', add:'' },
  customer: { title:'企业采购工作台', en:'CUSTOMER OPERATIONS', intro:'提交采购需求、核对方案，在这里跟踪实施与验收。', ledger:'我的采购项目', add:'提交采购需求' },
}[props.role]
const repo = createDemoRepository(roleKeys[props.role])
const org = roles[roleKeys[props.role]]
const nav = [
  { id:'overview', label:'工作总览', icon:'overview' },
  { id:'ledger', label:copy.ledger, icon:'request' },
  ...(props.role === 'dealer' ? [{ id:'opportunities', label:'商机拾遗', icon:'workflow' }] : []),
  ...(props.role === 'dealer' ? [{ id:'visits', label:'记拜访', icon:'request' }] : []),
  { id:'delivery', label:props.role === 'customer' ? '交付与验收' : '订单与交付', icon:'delivery' },
  ...(props.role !== 'customer' ? [{id:'catalog', label:'软件目录', icon:'catalog'}] : []),
]
const projects = ref([])
const products = ref([])
const desk = ref({ items: [], partners: [], gaps: [], reverseMatches: [] })
const opportunity = ref('')
const captureFor = ref('')
const params = () => new URLSearchParams(location.search)
const view = ref('overview')
const selected = ref(params().get('project') || '')
const project = computed(() => projects.value.find(item => item.id === selected.value))
const mode = ref('')
const recordAtOpen = ref(null)
const working = ref(false)
const loading = ref(true)
const loadError = ref('')
const formError = ref('')
const success = ref('')
const search = ref('')
const stage = ref('')
const scope = ref('')
const group = ref('')
const deliveryRows = computed(() => projects.value.filter(item => item.order))
const todos = computed(() => projects.value.filter(item => item.isTodo).sort((left,right) => (left.dueDate || '9999').localeCompare(right.dueDate || '9999')))
const candidateRows = computed(() => view.value === 'overview' ? todos.value : view.value === 'delivery' ? deliveryRows.value : projects.value)
const stateOptions = computed(() => [...new Set(candidateRows.value.map(item => item.stageLabel))])
const metrics = {
  dealer: [
    {id:'pricing', label:'待对客报价', icon:'quote', description:'渠道已报价或客户要求改版', stages:['channel_quoted','sales_revision']},
    {id:'upstream', label:'待厂商回复', icon:'time', description:'待首次报价或渠道报价改版', stages:['inquired','channel_revision']},
    {id:'handover', label:'实施与待验收', icon:'delivery', description:'关注实施、送验和整改事项', stages:['supplied','acceptance','rectification']},
  ],
  vendor: [
    {id:'inquiry', label:'等待报价', icon:'request', description:'代理商新询价与改价请求', stages:['inquired','channel_revision']},
    {id:'await', label:'报价已发出', icon:'quote', description:'已经向代理商提供采购条件', stages:['channel_wait']},
    {id:'fulfill', label:'待授权交付', icon:'delivery', description:'为已下单项目登记交接', stages:['ordered']},
  ],
  customer: [
    {id:'review', label:'待审阅方案', icon:'quote', description:'核对方案、价格与服务范围', stages:['sales_quoted']},
    {id:'progress', label:'履约跟踪', icon:'delivery', description:'留意供货、实施与整改', stages:['ordered','supplied','rectification']},
    {id:'acceptance', label:'待我验收', icon:'order', description:'确认结果或明确整改要求', stages:['acceptance']},
  ],
}[props.role]
const groupMetric = computed(() => metrics.find(item => item.id === group.value))
const rows = computed(() => candidateRows.value.filter(item => (!groupMetric.value || groupMetric.value.stages.includes(item.stage)) && (!stage.value || item.stageLabel === stage.value) && (!scope.value || item.isTodo) && [item.id,item.title,item.customerName,item.order?.id,item.inquiry?.summary].filter(Boolean).join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))
const navTitle = computed(() => nav.find(item => item.id === view.value)?.label)
const dialogTitle = computed(() => mode.value === 'reset' ? '重置三端演示数据' : mode.value === 'roles' ? '切换演示身份' : mode.value === 'create' ? copy.add : actionLabels[mode.value])
let unsub
let noticeTimer
let sequence = 0
function toast(text) {
  clearTimeout(noticeTimer)
  success.value = text
  noticeTimer = setTimeout(() => { success.value = '' }, 3200)
}
async function refresh() {
  const current = ++sequence
  try {
    const [records, catalog, privateDesk] = await Promise.all([repo.list(), props.role === 'customer' ? [] : repo.listProducts(), props.role === 'dealer' ? repo.opportunityDesk() : null])
    if (current !== sequence) return
    projects.value = records; products.value = catalog; loadError.value = ''
    if (privateDesk) desk.value = privateDesk
  } catch (error) {
    if (current === sequence) loadError.value = error.message || '暂时无法读取演示台账。'
  } finally { if (current === sequence) loading.value = false }
}
function restoreUrl() {
  selected.value = params().get('project') || ''
  const target = params().get('view')
  view.value = nav.some(item => item.id === target) ? target : 'overview'
  opportunity.value = view.value === 'opportunities' ? params().get('op') || '' : ''
  captureFor.value = ''
  closeDialog()
  clearFilters()
}
function syncUrl() {
  const url = new URL(location.href)
  if (view.value === 'overview') url.searchParams.delete('view'); else url.searchParams.set('view', view.value)
  if (selected.value) url.searchParams.set('project', selected.value); else url.searchParams.delete('project')
  if (opportunity.value && view.value === 'opportunities') url.searchParams.set('op', opportunity.value); else url.searchParams.delete('op')
  if (url.href !== location.href) history.pushState(null, '', url)
}
function clearFilters() { search.value = ''; stage.value = ''; scope.value = ''; group.value = '' }
function navigate(target) {
  view.value = target; selected.value = ''; opportunity.value = ''; captureFor.value = ''; clearFilters(); syncUrl()
  window.scrollTo(0, 0)
}
function openOpportunity(id = '', captureProjectId = '') {
  if (props.role !== 'dealer') return
  view.value = 'opportunities'; selected.value = ''; opportunity.value = id; captureFor.value = captureProjectId
  clearFilters(); syncUrl(); window.scrollTo(0,0)
}
function openProject(record) {
  selected.value = record.id; syncUrl(); window.scrollTo(0,0)
}
function openDialog(action) {
  formError.value = ''; recordAtOpen.value = project.value; mode.value = action
}
function closeDialog() { if (!working.value) { mode.value = ''; recordAtOpen.value = null; formError.value = '' } }
function roleHref(key) { return '/experience/' + key + (selected.value ? '?project=' + encodeURIComponent(selected.value) : '') }
async function perform(payload) {
  if (working.value) return
  working.value = true; formError.value = ''
  const action = mode.value
  try {
    if (action === 'reset') {
      await repo.reset()
      createVisitDraftStore('demo:' + org.orgId).clear()
      navigate('overview')
    } else {
      const previous = recordAtOpen.value
      const updated = action === 'create' ? await repo.create(payload) : await repo.act(previous.id, action, payload, previous.revision)
      if (action === 'create') { view.value = 'ledger'; openProject(updated) }
    }
    await refresh()
    mode.value = ''; recordAtOpen.value = null
    toast(action === 'reset' ? '已恢复三端示例项目' : '操作已记录')
  } catch (error) {
    formError.value = error.message || '提交失败，请返回详情核对最新状态。'
    await refresh()
  } finally { working.value = false }
}
async function saveProduct(value) { await repo.saveProduct(value); await refresh(); toast('产品已保存') }
async function captureVisit(projectId,note) {
  const current = await repo.opportunityDesk()
  const existing = current.items.find(item => item.projectId === projectId && item.note === note)
  const result = existing || await repo.captureOpportunity(projectId,{note})
  await refresh()
  return result
}
onMounted(() => { restoreUrl(); refresh(); unsub = repo.subscribe(refresh); window.addEventListener('popstate', restoreUrl) })
onUnmounted(() => { unsub?.(); clearTimeout(noticeTimer); window.removeEventListener('popstate', restoreUrl) })
</script>

<template>
  <div class="ch-space">
    <WorkspaceShell demo :nav="nav" :view="view" :organization="org.company" :account-label="names[role] + ' · 演示身份'" environment="本地演示空间" @navigate="navigate" @account="openDialog('roles')">
      <UiState v-if="loading" state="loading" title="正在载入工作台" />
      <UiState v-else-if="loadError" state="error" title="工作台暂时不可用" :description="loadError"><UiButton @click="refresh">重新载入</UiButton><UiButton @click="openDialog('reset')">恢复演示数据</UiButton></UiState>
      <template v-else-if="selected">
        <ProjectDetail v-if="project" :project="project" :role="role" :products="products" :opportunities="role === 'dealer' ? desk.items.filter(item => item.projectId === project.id) : []" @action="openDialog" @back="navigate(view)" @capture="openOpportunity('', project.id)" @opportunity="openOpportunity" />
        <UiState v-else state="denied" title="当前身份无法查看此项目" description="项目不存在、尚未提交给当前组织，或已撤回。"><UiButton @click="navigate('ledger')">查看我的工作列表</UiButton></UiState>
      </template>
      <DemoCatalog v-else-if="view === 'catalog'" :items="products" :role="role" :save="saveProduct" />
      <VisitDrafts v-else-if="role === 'dealer' && view === 'visits'" :scope="'demo:' + org.orgId" :load-projects="repo.list" :capture="captureVisit" @project="id => { navigate('ledger'); openProject({id}) }" />
      <OpportunityDesk v-else-if="role === 'dealer' && view === 'opportunities'" :desk="desk" :repo="repo" :projects="projects" :selected="opportunity" :capture-for="captureFor" :refresh="refresh" @select="openOpportunity" @capture-opened="captureFor = ''" @project="record => { navigate('ledger'); openProject(record) }" @notice="toast" />
      <template v-else>
        <section class="page-heading ch-head"><div><p class="ch-kicker">{{ copy.en }} &nbsp; / &nbsp; {{ org.company }}</p><h1>{{ view === 'overview' ? copy.title : navTitle }}</h1><p class="muted">{{ view === 'overview' ? copy.intro : view === 'delivery' ? '从订单追溯报价，跟进供货、实施及客户验收。' : '按编号、项目名称和状态查找协作记录。' }}</p></div>
          <UiButton v-if="copy.add && view !== 'delivery'" icon="add" variant="primary" @click="openDialog('create')">{{ copy.add }}</UiButton>
        </section>
        <div v-if="view === 'overview'" class="metric-grid">
          <UiMetricCard v-for="metric in metrics" :key="metric.id" :icon="metric.icon" :label="metric.label" :value="projects.filter(item => metric.stages.includes(item.stage)).length" :description="metric.description" interactive @click="navigate('ledger'); group = metric.id" />
        </div>
        <section v-if="role === 'dealer' && view === 'overview'" class="op-banner">
          <div><p class="ch-kicker">渠道经营 · 团队私有</p><h2>商机拾遗</h2><p>主单之外的需求，团队里谁做过？<span v-if="desk.items.length">当前 {{ desk.items.filter(item => item.stage === 'open').length }} 条待承接。</span></p></div>
          <UiButton icon="workflow" @click="openOpportunity()">查看商机台账</UiButton>
          <UiButton @click="navigate('visits')">记拜访</UiButton>
        </section>
        <UiCard v-if="role === 'dealer' && view === 'overview' && desk.tasks?.length" title="今天接着跟进">
          <div class="op-records"><button v-for="task in desk.tasks" :key="task.opportunityId" @click="openOpportunity(task.opportunityId)"><span><b>{{ task.reason }}</b><small>{{ task.projectTitle }}</small></span><span>{{ task.action }}</span></button></div>
        </UiCard>
        <UiCard class="ch-ledger" :title="view === 'overview' ? '优先待办' : '业务记录'" :description="view === 'overview' ? '打开项目，即可处理当前环节。' : undefined" flush>
          <template #actions><span class="ch-count">{{ rows.length }} 笔</span></template>
          <div class="ch-tools ch-table-tools"><label class="ch-search"><UiIcon name="search" :size="15" /><input v-model="search" type="search" aria-label="搜索项目" placeholder="搜索项目名称或编号" /></label><select v-model="stage" aria-label="项目状态"><option value="">全部状态</option><option v-for="label in stateOptions" :key="label">{{ label }}</option></select><select v-if="view !== 'overview'" v-model="scope" aria-label="责任筛选"><option value="">所有经办方</option><option value="mine">待我处理</option></select><UiButton v-if="groupMetric" compact :aria-label="'移除筛选：' + groupMetric.label" @click="group = ''">{{ groupMetric.label }} ×</UiButton></div>
          <div v-if="rows.length" class="table-scroll"><table><thead><tr><th>项目 / 编号</th><th>状态</th><th>下一步</th><th>目标日期</th><th><span class="ch-sr">操作</span></th></tr></thead><tbody>
            <tr v-for="item in rows" :key="item.id"><td><button class="ch-record" @click="openProject(item)"><strong>{{ item.title }}</strong><small>{{ item.id }}<template v-if="item.customerName"> · {{ item.customerName }}</template></small></button></td><td><UiBadge :tone="statusTone(item.stage)" dot>{{ item.stageLabel }}</UiBadge></td><td>{{ item.nextStep }}</td><td class="date">{{ item.dueDate || '未设定' }}</td><td><UiButton compact end-icon="chevron-right" :aria-label="'查看' + item.title" @click="openProject(item)">查看</UiButton></td></tr>
          </tbody></table></div>
          <UiState v-else :title="search || stage || scope || group ? '没有匹配的记录' : view === 'overview' ? '当前没有待你处理的项目' : '暂无业务记录'" :description="search || stage || scope || group ? '可清除筛选或更换关键词。' : '进入项目列表查看进度，或切换演示身份体验协作方的操作。'"><UiButton v-if="search || stage || scope || group" compact @click="clearFilters">清除筛选</UiButton><UiButton v-else-if="view !== 'ledger'" compact @click="navigate('ledger')">查看项目列表</UiButton></UiState>
        </UiCard>
      </template>
      <footer class="demo-footer ch-footer"><span>本地演示 · 三个身份共享此浏览器的示例记录</span><div class="footer-actions"><button @click="openDialog('roles')">切换身份</button><button @click="openDialog('reset')">重置演示</button></div></footer>
    </WorkspaceShell>
    <ElDialog :model-value="!!mode" :title="dialogTitle" width="min(94vw, 610px)" :close-on-click-modal="false" :show-close="!working" @update:model-value="value => { if (!value) closeDialog() }">
      <div v-if="mode === 'roles'" class="role-options"><p class="ch-callout">切换演示身份以处理下一步。正式账号和演示记录相互独立。</p><a v-for="(label,key) in names" :key="key" :href="roleHref(key)" class="role-option ch-role-option"><UiIcon :name="key === 'customer' ? 'request' : key" /><span><strong>{{ label }}工作台</strong><small>{{ roles[roleKeys[key]].company }}</small></span><UiIcon :name="role === key ? 'check' : 'forward'" :size="16" /></a></div>
      <div v-else-if="mode === 'reset'"><p>恢复初始项目、报价和产品目录。三个演示身份新增的记录将从此浏览器移除。</p><p v-if="formError" class="form-error" role="alert">{{ formError }}</p><div class="modal-actions"><UiButton :disabled="working" @click="closeDialog">取消</UiButton><UiButton variant="primary" :loading="working" @click="perform({})">确认重置</UiButton></div></div>
      <ChannelAction v-else-if="mode" :key="mode" :action="mode" :role="role" :project="recordAtOpen" :products="products" :error="formError" :busy="working" @submit="perform" @cancel="closeDialog" />
    </ElDialog>
    <div v-if="success" class="notice" role="status"><UiIcon name="check" :size="16" />{{ success }}</div>
  </div>
</template>
