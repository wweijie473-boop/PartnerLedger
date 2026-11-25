<script setup>
import { computed, defineAsyncComponent, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { ElDialog } from 'element-plus'
import WorkspaceShell from '../WorkspaceShell.vue'
import WorkspaceShortcuts from '../WorkspaceShortcuts.vue'
import DealerPage from '../DealerPage.vue'
import { isRetailDemoPage, workspacePages, workspaceShortcuts } from '../workspaceNavigation.js'
import { UiButton, UiCard, UiIcon, UiMetricCard, UiState } from '../ui/index.js'
import { actionLabels, projectScopeLabels, roles } from './domain.js'
import { createDemoRepository } from './repository.js'
import ChannelAction from './ChannelAction.vue'
import ProjectDetail from './ProjectDetail.vue'
import ProjectFilters from './ProjectFilters.vue'
import ProjectList from './ProjectList.vue'
import ProjectPriority from './ProjectPriority.vue'
import DemoCatalog from './DemoCatalog.vue'
import OpportunityDesk from './OpportunityDesk.vue'
import VisitDrafts from '../live/VisitDrafts.vue'
import AiWorkspace from '../live/AiWorkspace.vue'
import { createVisitDraftStore } from '../live/visitDrafts.js'
import { retailDemoCounterpart, retailDemoEnabled, retailDemoLocation } from '../live/retailDemo.js'
import './workspace.css'
import './opportunities.css'

const props = defineProps({ role: { type: String, required: true } })
const roleKeys = { dealer: 'reseller', vendor: 'manufacturer', customer: 'customer' }
const names = { dealer: '代理商', vendor: '软件厂商', customer: '企业客户' }
const copy = {
  dealer: { title:'代理商工作台', intro:'跟进客户需求、厂商回复与交付节点。', add:'代录客户需求' },
  vendor: { title:'软件厂商工作台', intro:'处理渠道询价、报价与授权交付。', add:'' },
  customer: { title:'企业客户工作台', intro:'提交采购需求、核对方案并跟踪实施与验收。', add:'提交采购需求' },
}[props.role]
const repo = createDemoRepository(roleKeys[props.role])
const org = roles[roleKeys[props.role]]
const RetailDemoPanel = defineAsyncComponent(() => import('../live/RetailDemoPanel.vue'))
const nav = workspacePages(props.role, { demo: true, retailDemo: retailDemoEnabled })
const shortcuts = workspaceShortcuts(nav, props.role)
const projects = ref([])
const products = ref([])
const desk = ref({ items: [], partners: [], gaps: [], reverseMatches: [] })
const opportunity = ref('')
const captureFor = ref('')
const params = () => new URLSearchParams(location.search)
const view = ref('overview')
const currentPage = computed(() => nav.find(item => item.id === view.value))
const retailView = computed(() => currentPage.value && isRetailDemoPage(currentPage.value))
const selected = ref(params().get('project') || '')
const aiProject = ref('')
const project = computed(() => projects.value.find(item => item.id === selected.value))
const mode = ref('')
const recordAtOpen = ref(null)
const inquiryDraft = ref(null)
const working = ref(false)
const loading = ref(true)
const loadError = ref('')
const formError = ref('')
const success = ref('')
const search = ref('')
const stage = ref('')
const scope = ref('all')
const group = ref('')
const deliveryRows = computed(() => projects.value.filter(item => item.order))
const todos = computed(() => projects.value.filter(item => item.isTodo).sort((left,right) => (left.dueDate || '9999').localeCompare(right.dueDate || '9999')))
const availableRows = computed(() => view.value === 'delivery' ? deliveryRows.value : projects.value)
const scopes = computed(() => ({
  all: availableRows.value,
  mine: view.value === 'delivery' ? deliveryRows.value.filter(item => item.isTodo) : todos.value,
  partners: availableRows.value.filter(item => !item.isTodo && item.stage !== 'completed'),
}))
const counts = computed(() => ({ all: scopes.value.all.length, mine: scopes.value.mine.length, partners: scopes.value.partners.length }))
const stateOptions = computed(() => [...new Set([...availableRows.value.map(item => item.stageLabel), stage.value].filter(Boolean))]
  .map(label => ({ id: label, label })))
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
const rows = computed(() => scopes.value[scope.value].filter(item => (!groupMetric.value || groupMetric.value.stages.includes(item.stage)) && (!stage.value || item.stageLabel === stage.value) && [item.id,item.title,item.customerName,item.order?.id,item.inquiry?.summary].filter(Boolean).join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))
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
  view.value = nav.some(item => item.id === target) ? target : selected.value ? 'ledger' : 'overview'
  aiProject.value = view.value === 'requirement' ? selected.value : ''
  if (view.value === 'requirement') selected.value = ''
  opportunity.value = view.value === 'opportunities' ? params().get('op') || '' : ''
  captureFor.value = ''
  closeDialog()
  clearFilters()
  const requestedScope = params().get('scope')
  if (['ledger', 'delivery'].includes(view.value) && Object.hasOwn(projectScopeLabels, requestedScope)) scope.value = requestedScope
  group.value = view.value === 'ledger' && metrics.some(metric => metric.id === params().get('metric')) ? params().get('metric') : ''
  const listState = history.state?.demoProjectList
  if (listState) {
    search.value = listState.search
    stage.value = listState.stage
    scope.value = listState.scope || 'all'
    group.value = listState.group
    if (!selected.value) {
      nextTick(() => {
        const row = document.querySelector(`[data-project-id="${CSS.escape(listState.id)}"]`)
        row?.focus({ preventScroll: true })
        window.scrollTo(0, listState.top)
      })
    }
  }
}
function syncUrl(state = null, replace = false) {
  const url = new URL(location.href)
  if (view.value === 'overview') url.searchParams.delete('view'); else url.searchParams.set('view', view.value)
  const projectId = view.value === 'requirement' ? aiProject.value : selected.value
  if (projectId) url.searchParams.set('project', projectId); else url.searchParams.delete('project')
  if (opportunity.value && view.value === 'opportunities') url.searchParams.set('op', opportunity.value); else url.searchParams.delete('op')
  if (['ledger', 'delivery'].includes(view.value) && scope.value !== 'all') url.searchParams.set('scope', scope.value); else url.searchParams.delete('scope')
  if (view.value === 'ledger' && group.value) url.searchParams.set('metric', group.value); else url.searchParams.delete('metric')
  if (url.href !== location.href) {
    if (replace) history.replaceState(state, '', url)
    else history.pushState(state, '', url)
  }
}
function clearFilters() { search.value = ''; stage.value = ''; scope.value = 'all'; group.value = '' }
function clearListFilters() {
  clearFilters()
  syncUrl(null, true)
}
function navigate(target, filters = {}) {
  view.value = target; selected.value = ''; opportunity.value = ''; captureFor.value = ''; clearFilters()
  aiProject.value = target === 'requirement' ? filters.project || '' : ''
  scope.value = filters.scope || 'all'
  group.value = filters.metric || ''
  syncUrl()
  window.scrollTo(0, 0)
}
function chooseScope(value) {
  scope.value = value
  group.value = ''
  syncUrl(null, true)
}
function clearMetric() {
  group.value = ''
  syncUrl(null, true)
}
function openOpportunity(id = '', captureProjectId = '') {
  if (props.role !== 'dealer') return
  view.value = 'opportunities'; selected.value = ''; opportunity.value = id; captureFor.value = captureProjectId
  clearFilters(); syncUrl(); window.scrollTo(0,0)
}
function openProject(record) {
  selected.value = record.id; syncUrl(); window.scrollTo(0,0)
}
async function openAiProject(id) {
  await refresh()
  navigate('ledger')
  openProject({ id })
}
function openProjectFromList(record) {
  const demoProjectList = { search: search.value, stage: stage.value, scope: scope.value, group: group.value, top: window.scrollY, id: record.id }
  history.replaceState({ demoProjectList }, '', location.href)
  selected.value = record.id
  syncUrl({ demoProjectList, fromDemoList: true })
  window.scrollTo(0, 0)
}
function returnToProjects() {
  if (history.state?.fromDemoList) history.back()
  else navigate(view.value)
}
function openDialog(action) {
  formError.value = ''; recordAtOpen.value = project.value; mode.value = action
  inquiryDraft.value = null
  if (action === 'submitInquiry' && recordAtOpen.value) {
    try {
      const draft = repo.getInquiryDraft(recordAtOpen.value.id)
      if (draft?.projectRevision === recordAtOpen.value.revision) inquiryDraft.value = draft
    } catch {
      formError.value = '暂时无法带入询价草稿，可以手工填写。'
    }
  }
}
function closeDialog() { if (!working.value) { mode.value = ''; recordAtOpen.value = null; formError.value = '' } }
function roleHref(key) {
  if (retailView.value && key !== 'customer') {
    return key === props.role ? retailDemoLocation(key, view.value) : retailDemoCounterpart(props.role, view.value)
  }
  return '/experience/' + key + (selected.value ? '?project=' + encodeURIComponent(selected.value) : '')
}
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
async function captureVisit(projectId,note,criteria) {
  const current = await repo.opportunityDesk()
  const existing = current.items.find(item => item.projectId === projectId && item.note === note)
  const result = existing || await repo.captureOpportunity(projectId,{note,...criteria})
  await refresh()
  return result
}
onMounted(() => { restoreUrl(); refresh(); unsub = repo.subscribe(refresh); window.addEventListener('popstate', restoreUrl) })
onUnmounted(() => { unsub?.(); clearTimeout(noticeTimer); window.removeEventListener('popstate', restoreUrl) })
</script>

<template>
  <div class="ch-space">
    <WorkspaceShell demo :role="role" :nav="nav" :view="view" :detail-key="selected" :organization="org.company" :account-label="names[role] + ' · 演示身份'" environment="本地演示空间" @navigate="navigate" @account="openDialog('roles')">
      <RetailDemoPanel v-if="retailView" :role="role" :view="view" :title="currentPage.label" :operations="currentPage.source === 'operations'" @navigate="navigate" />
      <UiState v-else-if="loading" state="loading" title="正在载入工作台" />
      <UiState v-else-if="loadError" state="error" title="工作台暂时不可用" :description="loadError"><UiButton @click="refresh">重新载入</UiButton><UiButton @click="openDialog('reset')">恢复演示数据</UiButton></UiState>
      <template v-else-if="selected">
        <ProjectDetail v-if="project" :project="project" :role="role" :products="products" :opportunities="role === 'dealer' ? desk.items.filter(item => item.projectId === project.id) : []" @action="openDialog" @back="returnToProjects" @capture="openOpportunity('', project.id)" @opportunity="openOpportunity">
          <template v-if="role === 'dealer' && project.actions.includes('submitInquiry')" #toolbar>
            <UiButton compact variant="text" icon="ai" @click="navigate('requirement', { project: project.id })">用 AI 整理需求</UiButton>
          </template>
        </ProjectDetail>
        <UiState v-else state="denied" title="当前身份无法查看此项目" description="项目不存在、尚未提交给当前组织，或已撤回。"><UiButton @click="navigate('ledger')">查看我的工作列表</UiButton></UiState>
      </template>
      <AiWorkspace v-else-if="view === 'requirement' && role === 'dealer'" :key="aiProject" :demo-repository="repo" :initial-project="aiProject"
        @open-project="openAiProject" @open-projects="navigate('ledger')"
        @open-opportunity="openOpportunity" @navigate="navigate" />
      <DealerPage v-else-if="currentPage?.source === 'utility'" :key="view" :view="view" :title="currentPage.label" :projects="projects" :organization="org.company" :account="names[role] + ' · 演示身份'" @project="id => openProjectFromList({ id })" @navigate="navigate" @account="openDialog('roles')" />
      <DemoCatalog v-else-if="view === 'catalog'" :items="products" :role="role" :save="saveProduct" />
      <VisitDrafts v-else-if="role === 'dealer' && view === 'visits'" :scope="'demo:' + org.orgId" :load-projects="repo.list" :load-desk="repo.opportunityDesk" :capture="captureVisit" @project="id => { navigate('ledger'); openProject({id}) }" @opportunity="openOpportunity" />
      <OpportunityDesk v-else-if="role === 'dealer' && view === 'opportunities'" :desk="desk" :repo="repo" :projects="projects" :selected="opportunity" :capture-for="captureFor" :refresh="refresh" @select="openOpportunity" @capture-opened="captureFor = ''" @project="record => { navigate('ledger'); openProject(record) }" @notice="toast" />
      <template v-else>
        <section class="page-heading ch-head"><div :key="view"><p v-if="view === 'overview'" class="workspace-kicker">{{ org.company }}</p><h1>{{ view === 'overview' ? copy.title : currentPage?.label }}</h1><p class="muted">{{ view === 'overview' ? copy.intro : view === 'delivery' ? '从订单追溯报价，跟进供货、实施及客户验收。' : '按编号、项目名称和状态查找协作记录。' }}</p></div>
          <UiButton v-if="copy.add && view !== 'delivery'" icon="add" variant="primary" @click="openDialog('create')">{{ copy.add }}</UiButton>
        </section>
        <template v-if="view === 'overview'">
          <section class="metric-grid workspace-overview-metrics" aria-label="协作数据">
            <UiMetricCard v-for="metric in metrics" :key="metric.id" :icon="metric.icon" :label="metric.label" :value="projects.filter(item => metric.stages.includes(item.stage)).length" :description="metric.description" interactive @click="navigate('ledger', { metric: metric.id })" />
          </section>
          <ProjectPriority :projects="todos" :show-customer="role === 'dealer'" @open="openProjectFromList" @show-all="navigate('ledger', { scope: 'mine' })" />
          <UiCard v-if="role === 'dealer' && desk.tasks?.length" title="商机待跟进">
            <div class="op-records"><button v-for="task in desk.tasks.slice(0, 5)" :key="task.opportunityId" @click="openOpportunity(task.opportunityId)"><span><b>{{ task.reason }}</b><small>{{ task.projectTitle }}</small></span><span>{{ task.action }}</span></button></div>
          </UiCard>
          <WorkspaceShortcuts :pages="shortcuts" @navigate="navigate" />
        </template>
        <section v-else class="workspace-section" aria-labelledby="project-list">
          <h2 id="project-list" class="ch-sr">协作项目</h2>
          <ProjectFilters :scope="scope" :counts="counts" :keyword="search" :stage="stage" :stages="stateOptions"
            @scope="chooseScope" @keyword="event => search = event.target.value" @stage="event => stage = event.target.value">
            <UiButton v-if="groupMetric" compact :aria-label="'移除筛选：' + groupMetric.label" @click="clearMetric">{{ groupMetric.label }} ×</UiButton>
          </ProjectFilters>
          <ProjectList v-if="rows.length" :projects="rows" :show-customer="role === 'dealer'" @open="openProjectFromList" />
          <UiState v-else :title="search || stage || group ? '暂无符合条件的项目' : scope === 'mine' ? '暂无待你处理的项目' : scope === 'partners' ? '暂无等待协作方处理的项目' : '暂无协作项目'">
            <UiButton v-if="search || stage || group" variant="text" @click="clearListFilters">清除筛选</UiButton>
            <UiButton v-else-if="scope !== 'all'" variant="text" @click="chooseScope('all')">查看全部项目</UiButton>
          </UiState>
        </section>
      </template>
      <footer class="demo-footer ch-footer"><span>本地演示 · 三个身份共享此浏览器的示例记录</span><div class="footer-actions"><button @click="openDialog('roles')">切换身份</button><button @click="openDialog('reset')">重置演示</button></div></footer>
    </WorkspaceShell>
    <ElDialog :model-value="!!mode" :title="dialogTitle" width="min(94vw, 610px)" :close-on-click-modal="false" :show-close="!working" @update:model-value="value => { if (!value) closeDialog() }">
      <div v-if="mode === 'roles'" class="role-options"><p class="ch-callout">切换演示身份以处理下一步。正式账号和演示记录相互独立。</p><a v-for="(label,key) in names" :key="key" :href="roleHref(key)" class="role-option ch-role-option"><UiIcon :name="key === 'customer' ? 'request' : key" /><span><strong>{{ label }}工作台</strong><small>{{ roles[roleKeys[key]].company }}</small></span><UiIcon :name="role === key ? 'check' : 'forward'" :size="16" /></a></div>
      <div v-else-if="mode === 'reset'"><p>恢复初始项目、报价和产品目录。三个演示身份新增的记录将从此浏览器移除。</p><p v-if="formError" class="form-error" role="alert">{{ formError }}</p><div class="modal-actions"><UiButton :disabled="working" @click="closeDialog">取消</UiButton><UiButton variant="primary" :loading="working" @click="perform({})">确认重置</UiButton></div></div>
      <ChannelAction v-else-if="mode" :key="mode" :action="mode" :role="role" :project="recordAtOpen" :products="products" :inquiry-draft="inquiryDraft" :error="formError" :busy="working" @submit="perform" @cancel="closeDialog" />
    </ElDialog>
    <div v-if="success" class="notice" role="status"><UiIcon name="check" :size="16" />{{ success }}</div>
  </div>
</template>
