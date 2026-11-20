<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElDialog } from 'element-plus'
import { UiButton, UiField, UiState, UiMetricCard } from '../ui/index.js'
import ProjectDetail from '../channel/ProjectDetail.vue'
import ProjectList from '../channel/ProjectList.vue'
import ChannelAction from '../channel/ChannelAction.vue'
import { actionLabels } from '../channel/domain.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import '../channel/workspace.css'

const props = defineProps({ request: Function, role: String, organizationId: String, overview: Boolean })
defineEmits(['capture-opportunity', 'open-project', 'open-list'])
const repo = createHttpProjectRepository(props.request)
const rows = ref([])
const links = ref([])
const products = ref([])
const inquiryDraft = ref(null)
const record = ref(null)
const projectParams = () => new URLSearchParams(location.search)
const selected = ref(props.overview ? '' : projectParams().get('project') || '')
const loading = ref(true)
const loadError = ref('')
const detailError = ref(null)
const detailLoading = ref(false)
const dialog = ref('')
const busy = ref(false)
const formError = ref('')
const notice = ref('')
const scopeLabels = { all: '全部项目', mine: '待我处理', partners: '协作方处理中' }
const listScope = () => {
  const value = projectParams().get('scope')
  return Object.hasOwn(scopeLabels, value) ? value : 'all'
}
const scope = ref(listScope())
const savedList = () => {
  const state = history.state?.projectList
  return state?.organizationId === props.organizationId ? state : null
}
const keyword = ref(savedList()?.keyword || '')
const stage = ref(savedList()?.stage || '')
const stageLabel = ref(savedList()?.stageLabel || '')
const list = ref(null)
const detailTop = ref(null)
const contextLoading = ref(false)
const contextError = ref('')
const pending = ref(null)
const form = reactive({ title: '', requirement: '', dueDate: '', organization: '' })
const storageKey = 'partnerledger-project-request-' + props.role + '-' + props.organizationId
const tasks = computed(() => rows.value.filter(item => item.isTodo)
  .sort((a, b) => (a.dueDate || '9999').localeCompare(b.dueDate || '9999')))
const partnerTasks = computed(() => rows.value.filter(item => !item.isTodo && item.stage !== 'completed'))
const scopedRows = computed(() => ({ all: rows.value, mine: tasks.value, partners: partnerTasks.value }))
const results = computed(() => scopedRows.value[scope.value].filter(item => (!stage.value || item.stage === stage.value) &&
  [item.id, item.title, item.customerName].filter(Boolean).join(' ').toLowerCase().includes(keyword.value.trim().toLowerCase())))
const stages = computed(() => {
  const labels = new Map(rows.value.map(item => [item.stage, item.stageLabel]))
  if (stage.value && stageLabel.value && !labels.has(stage.value)) labels.set(stage.value, stageLabel.value)
  return [...labels]
})
const detail = computed(() => record.value ? {
  ...record.value, actions: record.value.actions.filter(action => Object.hasOwn(actionLabels, action)),
} : null)
const title = computed(() => pending.value ? '核对尚未确认的提交' : dialog.value === 'create' ? '建立采购项目' : actionLabels[dialog.value])
let timer
let unmounted = false
let refreshing = false
let listSequence = 0
let detailSequence = 0

function storePending(value) {
  pending.value = value
  try {
    if (value) sessionStorage.setItem(storageKey,JSON.stringify(value))
    else sessionStorage.removeItem(storageKey)
  } catch { notice.value = '浏览器无法保留待核对提交，关闭页面前请先确认办理结果。' }
}
function setSelected(id, listState) {
  const url = new URL(location.href)
  if (id) url.searchParams.set('project', id)
  else url.searchParams.delete('project')
  if (listState) {
    history.replaceState({ projectList: listState }, '', location.href)
    history.pushState({ projectList: listState, fromProjectList: true }, '', url)
  } else {
    history.replaceState(history.state, '', url)
  }
  selected.value = id
}
function chooseScope(value) {
  scope.value = value
  const url = new URL(location.href)
  if (value === 'all') url.searchParams.delete('scope')
  else url.searchParams.set('scope', value)
  history.replaceState(null, '', url)
}
function clearFilters() {
  keyword.value = ''
  stage.value = ''
  stageLabel.value = ''
}
function rememberStageLabel(event) {
  stageLabel.value = event.target.selectedOptions[0]?.textContent || ''
}
async function restoreListPosition(state) {
  await nextTick()
  if (!list.value || selected.value) return
  const row = state?.id && list.value.querySelector(`[data-project-id="${CSS.escape(state.id)}"]`)
  const remaining = list.value.querySelectorAll('[data-project-id]')
  const nearby = state?.index >= 0 && remaining[Math.min(state.index, remaining.length - 1)]
  const target = row || nearby || list.value.querySelector('[aria-pressed="true"]')
  target?.focus({ preventScroll: true })
  window.scrollTo(0, state?.top || 0)
}
function syncProjectFromHistory() {
  const id = projectParams().get('project') || ''
  scope.value = listScope()
  if (id === selected.value) return
  selected.value = id
  detailSequence += 1
  detailLoading.value = false
  detailError.value = null
  record.value = null
  if (id) {
    loadDetail()
  } else {
    const state = savedList()
    keyword.value = state?.keyword || ''
    stage.value = state?.stage || ''
    stageLabel.value = state?.stageLabel || ''
    restoreListPosition(state)
  }
}
function returnToList() {
  if (history.state?.fromProjectList && savedList()) {
    history.back()
    return
  }
  const url = new URL(location.href)
  url.searchParams.delete('project')
  history.replaceState(history.state, '', url)
  syncProjectFromHistory()
}
function projectListState(id) {
  return {
    organizationId: props.organizationId, keyword: keyword.value, stage: stage.value, stageLabel: stageLabel.value,
    top: window.scrollY, id, index: results.value.findIndex(item => item.id === id),
  }
}
async function loadDetail() {
  if (!selected.value) return
  const sequence = ++detailSequence
  detailLoading.value = record.value?.id !== selected.value
  detailError.value = null
  try {
    const item = await repo.get(selected.value)
    if (!unmounted && sequence === detailSequence) record.value = item
  } catch (error) {
    if (!unmounted && sequence === detailSequence) {
      detailError.value = error
      record.value = null
    }
  } finally { if (!unmounted && sequence === detailSequence) detailLoading.value = false }
}
async function refresh({ force = false } = {}) {
  if (unmounted || (refreshing && !force)) return
  const sequence = ++listSequence
  refreshing = true
  try {
    const items = await repo.list()
    if (!unmounted && sequence === listSequence) { rows.value = items; loadError.value = '' }
  } catch (error) {
    if (!unmounted && sequence === listSequence && error.name !== 'AbortError') loadError.value = error.message
  } finally {
    if (sequence === listSequence) {
      refreshing = false
      if (!unmounted) loading.value = false
    }
  }
}
async function openProject(item) {
  setSelected(item.id, projectListState(item.id))
  record.value = null
  window.scrollTo(0, 0)
  loadDetail()
  await nextTick()
  detailTop.value?.focus({ preventScroll: true })
}
async function backgroundRefresh() {
  if (document.hidden || dialog.value || busy.value) return
  await refreshProgress()
}
async function refreshProgress() {
  await refresh()
  if (selected.value) await loadDetail()
}
async function openAction(action) {
  formError.value = ''; contextError.value = ''; products.value = []; links.value = []
  inquiryDraft.value = null
  dialog.value = action
  if (!['create','submitInquiry'].includes(action)) return
  contextLoading.value = true
  try {
    if (action === 'create') {
      const page = await props.request('/api/channel/v1/organization-links')
      links.value = page.items
      form.organization = links.value.length === 1 ? links.value[0].id : ''
    } else {
      let after = null
      const visited = new Set()
      do {
        const params = new URLSearchParams({limit:'100'})
        if (after) params.set('after',after)
        const page = await props.request('/api/products?' + params)
        products.value.push(...page.items.filter(item => item.published))
        after = page.next_cursor
        if (after && visited.has(after)) throw new Error('产品分页游标重复，请重新载入。')
        if (after) visited.add(after)
      } while (after)
      try {
        const draft = await props.request('/api/channel/v1/projects/' + encodeURIComponent(record.value.id) + '/inquiry-draft')
        if (draft.projectRevision === record.value?.revision && products.value.some(item => item.id === draft.productId)) {
          inquiryDraft.value = draft
        }
      } catch (error) {
        if (error.status !== 404) notice.value = '暂时无法带入询价草稿，可以手工填写。'
      }
    }
  } catch (error) { contextError.value = error.message }
  finally { contextLoading.value = false }
}
function closeDialog() {
  if (busy.value || pending.value || contextLoading.value) return
  dialog.value = ''; formError.value = ''
}
async function submit(payload) {
  if (busy.value) return
  if (!pending.value) {
    storePending({ requestId:crypto.randomUUID(), action:dialog.value, id:record.value?.id || '',
      revision:record.value?.revision, payload:structuredClone(payload) })
  }
  await sendPending()
}
async function create() {
  if (!links.value.some(item => item.id === form.organization)) {
    formError.value = '请选择已关联的组织。'
    return
  }
  await submit({ title:form.title.trim(), requirement:form.requirement.trim(),
    ...(form.dueDate ? {dueDate:form.dueDate} : {}),
    [props.role === 'dealer' ? 'customerId' : 'resellerId']:form.organization })
}
async function sendPending() {
  if (busy.value || !pending.value) return
  busy.value = true; formError.value = ''
  const job = pending.value
  const projectAtSubmit = selected.value
  try {
    const updated = job.action === 'create' ? await repo.create(job.payload,job.requestId) :
      await repo.act(job.id,job.action,job.payload,job.revision,job.requestId)
    storePending(null)
    if (unmounted) return
    dialog.value = ''
    if (selected.value === projectAtSubmit) {
      record.value = updated
      setSelected(updated.id, projectAtSubmit ? undefined : projectListState(updated.id))
      if (!projectAtSubmit) window.scrollTo(0, 0)
    }
    notice.value = '办理结果已保存，协作方可在关联项目中查看。'
    await Promise.all([refresh({ force: true }), loadDetail()])
  } catch (error) {
    if (error.status >= 400 && error.status < 500) {
      storePending(null)
      dialog.value = ''
      notice.value = error.message
      await refresh()
      await loadDetail()
    } else {
      formError.value = '提交结果尚待确认。已保留本次请求，请先检查最新记录。'
      await refresh()
      if (!unmounted && job.id && selected.value === projectAtSubmit) { setSelected(job.id); await loadDetail() }
    }
  } finally { busy.value = false }
}
onMounted(async () => {
  if (!props.overview) window.addEventListener('popstate', syncProjectFromHistory)
  try {
    const job = JSON.parse(sessionStorage.getItem(storageKey))
    if (job?.requestId && job.payload && job.action) {
      pending.value = job
      dialog.value = job.action
    }
  } catch { notice.value = '上次请求记录无法读取，请先检查项目台账再办理。' }
  await Promise.all([refresh(), loadDetail()])
  if (unmounted) return
  if (!props.overview && !selected.value && savedList()) restoreListPosition(savedList())
  timer = setInterval(backgroundRefresh,20000)
  window.addEventListener('focus',backgroundRefresh)
  document.addEventListener('visibilitychange',backgroundRefresh)
})
onUnmounted(() => {
  unmounted = true
  clearInterval(timer)
  window.removeEventListener('popstate', syncProjectFromHistory)
  window.removeEventListener('focus',backgroundRefresh)
  document.removeEventListener('visibilitychange',backgroundRefresh)
})
</script>

<template>
  <section>
    <header v-if="overview || !selected" v-enter class="page-heading">
      <slot v-if="overview" name="heading" />
      <div v-else-if="!selected">
        <h1>{{ role === 'vendor' ? '渠道询价' : role === 'customer' ? '我的采购项目' : '客户项目' }}</h1>
        <p class="muted">需求、询价和补充资料按同一项目流转。</p>
      </div>
      <div class="workspace-actions live-project-actions">
        <UiButton variant="text" :disabled="busy" @click="refreshProgress">刷新协作进度</UiButton>
        <UiButton v-if="!overview && !selected && !loading && role !== 'vendor'" icon="add" variant="primary" @click="openAction('create')">建立项目</UiButton>
      </div>
    </header>
    <p v-if="notice" class="live-feedback" role="status">{{ notice }}</p>
    <UiState v-if="loading" state="loading" title="正在读取项目" />
    <template v-else-if="overview">
      <UiState v-if="loadError" state="error" title="项目待办暂不可用" :description="loadError"><UiButton @click="refresh">重新载入</UiButton></UiState>
      <template v-else>
        <section v-enter:children="1" class="metric-grid" aria-label="协作数据">
          <UiMetricCard label="可见项目" :value="rows.length" description="当前组织参与的项目" icon="request" interactive @click="$emit('open-list', 'all')" />
          <UiMetricCard label="待我处理" :value="tasks.length" description="下一步由本方办理" icon="activity" tone="brand" interactive @click="$emit('open-list', 'mine')" />
          <UiMetricCard label="协作方处理中" :value="partnerTasks.length" description="等待协作方推进" icon="time" interactive @click="$emit('open-list', 'partners')" />
        </section>
        <section v-enter="2" class="workspace-section" aria-labelledby="project-tasks">
          <header class="workspace-section-heading">
            <h2 id="project-tasks">项目待办</h2>
            <UiButton v-if="tasks.length" variant="text" end-icon="forward" @click="$emit('open-list', 'mine')">查看全部待办</UiButton>
          </header>
          <ProjectList v-if="tasks.length" :projects="tasks.slice(0, 5)" :show-customer="role === 'dealer'" @open="item => $emit('open-project', item.id, 'mine')" />
          <UiState v-else title="当前没有待办理项目" description="新的需求或协作方回复到达后，会出现在这里。可从项目台账查看全部进度。" />
        </section>
      </template>
    </template>
    <div v-else-if="selected" ref="detailTop" tabindex="-1">
      <UiState v-if="detailLoading" state="loading" title="正在读取项目详情" />
      <UiState v-else-if="detailError" :state="detailError.status === 404 ? 'denied' : 'error'" title="项目暂不可访问" :description="detailError.message">
        <UiButton @click="loadDetail">重试</UiButton><UiButton @click="returnToList">返回项目列表</UiButton>
      </UiState>
      <ProjectDetail v-else-if="detail" :project="detail" :role="role" :products="products" live @back="returnToList" @action="openAction">
        <template #toolbar>
          <UiButton variant="text" :disabled="busy" @click="refreshProgress">刷新协作进度</UiButton>
          <UiButton v-if="role === 'dealer'" variant="text" @click="$emit('capture-opportunity', detail.id)">记录附加商机</UiButton>
        </template>
      </ProjectDetail>
    </div>
    <template v-else>
      <UiState v-if="loadError" state="error" title="项目列表暂不可用" :description="loadError"><UiButton @click="refresh">重新载入</UiButton></UiState>
      <section v-else v-enter="1" ref="list" class="workspace-section" aria-labelledby="project-list">
        <header class="workspace-section-heading">
          <div class="live-project-scopes" role="group" aria-label="项目范围">
            <h2 id="project-list" class="ch-sr">协作项目</h2>
            <button v-for="(label, key) in scopeLabels" :key="key" type="button"
              :aria-pressed="scope === key" @click="chooseScope(key)">{{ label }} <span>{{ scopedRows[key].length }}</span></button>
          </div>
          <div class="live-project-filters">
            <input v-model="keyword" type="search" placeholder="搜索项目或编号" aria-label="搜索项目">
            <select v-model="stage" aria-label="项目状态" @change="rememberStageLabel">
              <option value="">全部状态</option>
              <option v-for="[key, label] in stages" :key="key" :value="key">{{ label }}</option>
            </select>
          </div>
        </header>
        <ProjectList v-if="results.length" :projects="results" :show-customer="role === 'dealer'" @open="openProject" />
        <UiState v-else :title="keyword || stage ? '暂无符合条件的项目' : scope === 'mine' ? '暂无待你处理的项目' : scope === 'partners' ? '暂无等待协作方处理的项目' : '暂无协作项目'">
          <UiButton v-if="keyword || stage" variant="text" @click="clearFilters">清除筛选</UiButton>
          <UiButton v-else-if="scope !== 'all'" variant="text" @click="chooseScope('all')">查看全部项目</UiButton>
        </UiState>
      </section>
    </template>
    <ElDialog :model-value="!!dialog" :title="title" width="min(94vw, 610px)" :close-on-click-modal="false" :close-on-press-escape="!busy && !pending && !contextLoading" :show-close="!busy && !pending && !contextLoading" @update:model-value="value => { if (!value) closeDialog() }">
      <div v-if="pending"><p role="status">{{ formError || '已保留本次办理内容，请先核对项目最新状态。' }}</p>
        <p v-if="record">最新进度 {{ record.stageLabel }} · 版本 {{ record.revision }}</p>
        <p v-if="pending.action === 'create'">项目列表已有 {{ rows.length }} 条记录。重放本次请求会取得已保存的结果。</p>
        <div class="modal-actions"><UiButton :disabled="busy" @click="refresh(); loadDetail()">检查最新记录</UiButton><UiButton :loading="busy" @click="sendPending">用原请求确认结果</UiButton></div>
      </div>
      <UiState v-else-if="contextLoading" state="loading" title="正在读取可选信息" />
      <UiState v-else-if="contextError" state="error" :description="contextError" title="暂时无法办理"><UiButton @click="openAction(dialog)">重试</UiButton></UiState>
      <template v-else-if="dialog === 'create'">
        <UiState v-if="!links.length" title="尚未关联合作组织" description="请联系维护者配置客户与代理商的代理关系，再建立项目。" />
        <form v-else class="form-grid" @submit.prevent="create">
          <UiField :label="role === 'dealer' ? '客户组织' : '经办代理商'" required v-slot="{control}"><select v-bind="control" v-model="form.organization" required><option disabled value="">选择已关联组织</option><option v-for="link in links" :key="link.id" :value="link.id">{{ link.name }}</option></select></UiField>
          <UiField label="项目名称" required v-slot="{control}"><input v-bind="control" v-model="form.title" required maxlength="120"></UiField>
          <UiField label="采购需求" required v-slot="{control}"><textarea v-bind="control" v-model="form.requirement" required maxlength="3000" rows="4"></textarea></UiField>
          <UiField label="目标日期" v-slot="{control}"><input v-bind="control" v-model="form.dueDate" type="date"></UiField>
          <p v-if="formError" role="alert">{{ formError }}</p><div class="modal-actions"><UiButton @click="closeDialog">取消</UiButton><UiButton type="submit" variant="primary">提交需求</UiButton></div>
        </form>
      </template>
      <UiState v-else-if="dialog === 'submitInquiry' && !products.length" title="暂无可询价的已发布产品" description="请联系软件厂商发布产品后再发起询价。" />
      <ChannelAction v-else-if="dialog && record" :key="dialog + record.revision" :action="dialog" :project="record" :role="role" :products="products" :inquiry-draft="inquiryDraft" :busy="busy" :error="formError" @submit="submit" @cancel="closeDialog" />
    </ElDialog>
  </section>
</template>

<style scoped>
.live-project-actions { margin-left: auto; }
.live-feedback { margin-block: 8px; color: var(--ui-ink-secondary); }
.live-project-filters, .live-project-scopes {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  font-size: 13px;
}
.live-project-scopes { gap: 2px 4px; }
.live-project-scopes button {
  min-height: var(--ui-control-height);
  padding: 4px 7px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--ui-ink-secondary);
  white-space: nowrap;
}
.live-project-scopes button:hover { background: var(--ui-accent-bg); }
.live-project-scopes button[aria-pressed="true"] { border-bottom-color: var(--ui-accent); color: var(--ui-link); font-weight: 600; }
.live-project-scopes button span { margin-left: 2px; font-variant-numeric: tabular-nums; }
.live-project-filters :is(input[type='search'], select) {
  min-height: var(--ui-control-height);
  min-width: 0;
  max-width: 100%;
  padding: 4px 8px;
  border: 1px solid var(--ui-border-strong);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-surface);
  color: var(--ui-ink);
  font-size: inherit;
}
.workspace-section > .ui-state { border-top: 1px solid var(--ui-border); }
@media (max-width: 540px) {
  .live-project-actions { margin-left: 0; }
}
</style>
