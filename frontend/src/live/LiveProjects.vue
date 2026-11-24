<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElDialog } from 'element-plus'
import { UiButton, UiField, UiState, UiMetricCard } from '../ui/index.js'
import ProjectDetail from '../channel/ProjectDetail.vue'
import ProjectFilters from '../channel/ProjectFilters.vue'
import ProjectList from '../channel/ProjectList.vue'
import ProjectPriority from '../channel/ProjectPriority.vue'
import ChannelAction from '../channel/ChannelAction.vue'
import { actionLabels, projectScopeLabels } from '../channel/domain.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { createProjectWriteJournal } from './projectWrites.js'
import { useProjectWorkspace } from './useProjectWorkspace.js'
import { vPageFade } from '../ui/pageFade.js'
import '../channel/workspace.css'

const props = defineProps({ request: Function, role: String, organizationId: String, userId: String, overview: Boolean })
const emit = defineEmits(['capture-opportunity', 'open-project', 'open-list'])
const repo = createHttpProjectRepository(props.request)
const journal = createProjectWriteJournal(repo, props.organizationId, props.userId)
const pending = computed(() => journal.state.job)
const busy = computed(() => journal.state.busy)
const links = ref([])
const products = ref([])
const inquiryDraft = ref(null)
const record = ref(null)
const projectParams = () => new URLSearchParams(location.search)
const selected = ref(props.overview ? '' : projectParams().get('project') || '')
const detailError = ref(null)
const detailLoading = ref(false)
const dialog = ref('')
const formError = ref('')
const notice = ref(journal.state.error)
const overviewMetrics = {
  customer: ['采购项目', '待我处理', '协作方处理中'],
  dealer: ['客户项目', '待我跟进', '协作方处理中'],
  vendor: ['渠道询价', '待厂商处理', '协作方处理中'],
}
const listScope = () => {
  const value = projectParams().get('scope')
  return Object.hasOwn(projectScopeLabels, value) ? value : 'all'
}
const scope = ref(listScope())
const savedList = () => {
  const state = history.state?.projectList
  return state?.organizationId === props.organizationId && state.userId === props.userId ? state : null
}
const {
  page, rows, counts, loading, loadError, keyword, stage, stageLabel, cursors, stages,
  loadWorkspace, loadPage, restoreWorkspaceFilters, changeStage, changeKeyword, cancelWorkspaceLoad,
} = useProjectWorkspace(repo, { overview: props.overview, scope, selected, saved: savedList() })
const list = ref(null)
const detailTop = ref(null)
const contextLoading = ref(false)
const contextError = ref('')
const pendingProject = ref(null)
const pendingStatus = computed(() => pending.value?.input.id === record.value?.id ? record.value : pendingProject.value)
const form = reactive({ title: '', requirement: '', dueDate: '', organization: '' })
const detail = computed(() => record.value ? {
  ...record.value, actions: record.value.actions.filter(action => Object.hasOwn(actionLabels, action)),
} : null)
const title = computed(() => pending.value ? '核对尚未确认的提交' : dialog.value === 'create' ? '建立采购项目' : actionLabels[dialog.value])
let timer
let unmounted = false
let detailRequest
let actionRequest
let actionProject

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
  if (scope.value === value) return
  scope.value = value
  const url = new URL(location.href)
  if (value === 'all') url.searchParams.delete('scope')
  else url.searchParams.set('scope', value)
  history.replaceState(null, '', url)
  loadPage([''])
}
function clearFilters() {
  restoreWorkspaceFilters(null)
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
  const scopeChanged = scope.value !== listScope()
  scope.value = listScope()
  if (id === selected.value && !scopeChanged) return
  if (!pending.value) {
    actionRequest?.abort()
    dialog.value = ''
  }
  selected.value = id
  record.value = null
  if (id) {
    cancelWorkspaceLoad()
    loadDetail()
  } else {
    detailRequest?.abort()
    const state = savedList()
    restoreWorkspaceFilters(state).then(loaded => { if (loaded) restoreListPosition(state) })
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
    organizationId: props.organizationId, userId: props.userId, keyword: keyword.value, stage: stage.value,
    stageLabel: stageLabel.value, cursors: [...cursors.value],
    top: window.scrollY, id, index: rows.value.findIndex(item => item.id === id),
  }
}
async function loadDetail() {
  const id = selected.value
  if (!id || unmounted) return
  detailRequest?.abort()
  const controller = new AbortController()
  detailRequest = controller
  detailLoading.value = record.value?.id !== id
  detailError.value = null
  try {
    const item = await repo.get(id, { signal: controller.signal })
    if (!controller.signal.aborted) record.value = item
  } catch (error) {
    if (!controller.signal.aborted && error.name !== 'AbortError') {
      detailError.value = error
      record.value = null
    }
  } finally {
    if (detailRequest === controller) {
      detailRequest = null
      detailLoading.value = false
    }
  }
}
async function openProject(item) {
  setSelected(item.id, projectListState(item.id))
  cancelWorkspaceLoad()
  record.value = null
  window.scrollTo(0, 0)
  loadDetail()
  await nextTick()
  detailTop.value?.focus({ preventScroll: true })
}
async function backgroundRefresh() {
  if (document.hidden || dialog.value || pending.value || detailRequest || loading.value) return
  if (selected.value) await loadDetail()
  else await loadWorkspace()
}
function refreshProgress() {
  if (selected.value) return loadDetail()
  return loadWorkspace({ foreground: true })
}
async function openAction(action) {
  detailRequest?.abort()
  actionRequest?.abort()
  formError.value = ''; contextError.value = ''; products.value = []; links.value = []
  inquiryDraft.value = null
  actionProject = record.value
  dialog.value = action
  if (!['create','submitInquiry'].includes(action)) return
  const controller = new AbortController()
  const options = { signal: controller.signal }
  actionRequest = controller
  contextLoading.value = true
  try {
    if (action === 'create') {
      const page = await props.request('/api/channel/v1/organization-links', options)
      if (controller.signal.aborted) return
      links.value = page.items
      form.organization = links.value.length === 1 ? links.value[0].id : ''
    } else {
      let after = null
      const visited = new Set()
      do {
        const params = new URLSearchParams({limit:'100'})
        if (after) params.set('after',after)
        const page = await props.request('/api/products?' + params, options)
        if (controller.signal.aborted) return
        products.value.push(...page.items.filter(item => item.published))
        after = page.next_cursor
        if (after && visited.has(after)) throw new Error('产品分页游标重复，请重新载入。')
        if (after) visited.add(after)
      } while (after)
      try {
        const draft = await props.request('/api/channel/v1/projects/' + encodeURIComponent(actionProject.id) + '/inquiry-draft', options)
        if (!controller.signal.aborted && draft.projectRevision === actionProject.revision && products.value.some(item => item.id === draft.productId)) {
          inquiryDraft.value = draft
        }
      } catch (error) {
        if (!controller.signal.aborted && error.status !== 404) notice.value = '暂时无法带入询价草稿，可以手工填写。'
      }
    }
  } catch (error) { if (!controller.signal.aborted) contextError.value = error.message }
  finally { if (actionRequest === controller) { actionRequest = null; contextLoading.value = false } }
}
function closeDialog() {
  if (busy.value || pending.value || contextLoading.value) return
  dialog.value = ''; formError.value = ''
}
async function submit(payload) {
  if (busy.value || pending.value) return
  await save(() => journal.run(dialog.value, { id: actionProject?.id, revision: actionProject?.revision, payload }))
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
async function checkPending() {
  const job = pending.value
  if (!job) return
  const id = job.input.id
  if (!id) { await refreshProgress(); return }
  if (id === selected.value) { await loadDetail(); return }
  try {
    const item = await repo.get(id)
    if (pending.value === job && !unmounted) pendingProject.value = item
  } catch (error) {
    if (pending.value === job && !unmounted) {
      pendingProject.value = null
      formError.value = error.message
    }
  }
}
function retryPending() {
  if (busy.value || !pending.value) return
  return save(journal.retry)
}
async function save(execute) {
  formError.value = ''
  const projectAtSubmit = selected.value
  try {
    const updated = await execute()
    if (unmounted || !updated) return
    dialog.value = ''
    pendingProject.value = null
    if (!props.overview && selected.value === projectAtSubmit) {
      cancelWorkspaceLoad()
      detailRequest?.abort()
      record.value = updated
      setSelected(updated.id, projectAtSubmit ? undefined : projectListState(updated.id))
      if (updated.id !== projectAtSubmit) window.scrollTo(0, 0)
    }
    notice.value = '办理结果已保存，协作方可在关联项目中查看。'
    if (selected.value === updated.id) await loadDetail()
    else if (!selected.value) await loadWorkspace({ foreground: true })
  } catch (error) {
    if (unmounted) return
    if (pending.value) {
      formError.value = journal.state.error || error.message
    } else if (error.status >= 400 && error.status < 500) {
      dialog.value = ''
      notice.value = error.message
    await refreshProgress()
    } else {
      formError.value = '浏览器无法保留待核对提交，本次未发出。请检查浏览器存储后重试。'
    }
  }
}
onMounted(async () => {
  if (!props.overview) window.addEventListener('popstate', syncProjectFromHistory)
  await refreshProgress()
  if (unmounted) return
  if (!props.overview && !selected.value && projectParams().get('create') === '1') {
    const url = new URL(location.href)
    url.searchParams.delete('create')
    history.replaceState(history.state, '', url)
    if (!pending.value && props.role !== 'vendor') openAction('create')
  }
  if (!props.overview && !selected.value && savedList()) restoreListPosition(savedList())
  timer = setInterval(backgroundRefresh,20000)
  window.addEventListener('focus',backgroundRefresh)
  document.addEventListener('visibilitychange',backgroundRefresh)
})
onUnmounted(() => {
  unmounted = true
  clearInterval(timer)
  cancelWorkspaceLoad()
  detailRequest?.abort()
  actionRequest?.abort()
  window.removeEventListener('popstate', syncProjectFromHistory)
  window.removeEventListener('focus',backgroundRefresh)
  document.removeEventListener('visibilitychange',backgroundRefresh)
})
</script>

<template>
  <section v-page-fade.changes="selected">
    <header v-if="overview || !selected" class="page-heading">
      <slot v-if="overview" name="heading" />
      <div v-else-if="!selected">
        <h1>{{ role === 'vendor' ? '渠道询价' : role === 'customer' ? '我的采购项目' : '客户项目' }}</h1>
        <p class="muted">需求、询价和补充资料按同一项目流转。</p>
      </div>
      <div class="workspace-actions live-project-actions">
        <UiButton variant="text" :disabled="busy" @click="refreshProgress">刷新协作进度</UiButton>
        <UiButton v-if="!selected && !loading && role !== 'vendor'" icon="add" variant="primary"
          @click="overview ? emit('open-list', 'all', true) : openAction('create')">建立项目</UiButton>
      </div>
    </header>
    <p v-if="notice" class="live-feedback" role="status">{{ notice }}</p>
    <template v-if="overview">
      <UiState v-if="loading" state="loading" title="正在读取项目待办" />
      <UiState v-else-if="loadError" state="error" title="项目待办暂不可用" :description="loadError"><UiButton @click="refreshProgress">重新载入</UiButton></UiState>
      <template v-else>
        <section class="metric-grid workspace-overview-metrics" aria-label="协作数据">
          <UiMetricCard :label="overviewMetrics[role][0]" :value="counts.all" icon="request" interactive @click="$emit('open-list', 'all')" />
          <UiMetricCard :label="overviewMetrics[role][1]" :value="counts.mine" icon="activity" tone="brand" interactive @click="$emit('open-list', 'mine')" />
          <UiMetricCard :label="overviewMetrics[role][2]" :value="counts.partners" icon="time" interactive @click="$emit('open-list', 'partners')" />
        </section>
        <ProjectPriority :projects="rows" :show-customer="role === 'dealer'"
          @open="item => $emit('open-project', item.id, 'mine')" @show-all="$emit('open-list', 'mine')" />
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
      <section ref="list" class="workspace-section" aria-labelledby="project-list" :aria-busy="loading">
        <h2 id="project-list" class="ch-sr">协作项目</h2>
        <ProjectFilters :scope="scope" :counts="counts" :keyword="keyword" :stage="stage" :stages="stages"
          @scope="chooseScope" @keyword="changeKeyword" @stage="changeStage" />
        <UiState v-if="loading" state="loading" title="正在读取项目" />
        <UiState v-else-if="loadError" state="error" title="项目列表暂不可用" :description="loadError"><UiButton @click="refreshProgress">重新载入</UiButton></UiState>
        <ProjectList v-else-if="rows.length" :projects="rows" :show-customer="role === 'dealer'" @open="openProject" />
        <UiState v-else :title="cursors.length > 1 ? '本页暂无项目' : keyword || stage ? '暂无符合条件的项目' : scope === 'mine' ? '暂无待你处理的项目' : scope === 'partners' ? '暂无等待协作方处理的项目' : '暂无协作项目'">
          <UiButton v-if="keyword || stage" variant="text" @click="clearFilters">清除筛选</UiButton>
          <UiButton v-else-if="scope !== 'all'" variant="text" @click="chooseScope('all')">查看全部项目</UiButton>
        </UiState>
        <div v-if="cursors.length > 1 || page?.next_cursor" class="modal-actions live-project-pagination" role="group" aria-label="项目分页">
          <UiButton :disabled="loading || cursors.length < 2" @click="loadPage(cursors.slice(0, -1))">上一页</UiButton>
          <span role="status">第 {{ cursors.length }} 页</span>
          <UiButton :disabled="loading || !!loadError || !page?.next_cursor" @click="loadPage([...cursors, page.next_cursor])">下一页</UiButton>
        </div>
      </section>
    </template>
    <ElDialog :model-value="!!(dialog || pending)" :title="title" width="min(94vw, 610px)" :close-on-click-modal="false" :close-on-press-escape="!busy && !pending && !contextLoading" :show-close="!busy && !pending && !contextLoading" @update:model-value="value => { if (!value) closeDialog() }">
      <div v-if="pending"><p role="status">{{ formError || '已保留本次办理内容，请先核对项目最新状态。' }}</p>
        <p v-if="pendingStatus">最新进度 {{ pendingStatus.stageLabel }} · 版本 {{ pendingStatus.revision }}</p>
        <p v-if="pending.kind === 'create'">用原请求可以取得已保存的结果。</p>
        <div class="modal-actions"><UiButton :disabled="busy" @click="checkPending">检查最新记录</UiButton><UiButton :loading="busy" @click="retryPending">用原请求确认结果</UiButton></div>
      </div>
      <UiState v-else-if="contextLoading" state="loading" title="正在读取可选信息" />
      <UiState v-else-if="contextError" state="error" :description="contextError" title="暂时无法办理"><UiButton @click="openAction(dialog)">重试</UiButton></UiState>
      <template v-else-if="dialog === 'create'">
        <UiState v-if="!links.length" title="尚未关联合作组织" description="请联系维护者配置客户与代理商的代理关系，再建立项目。" />
        <form v-else class="form-grid" @submit.prevent="create">
          <UiField :label="role === 'dealer' ? '客户组织' : '经办代理商'" required v-slot="{control}"><select v-bind="control" v-model="form.organization" required><option disabled value="">选择已关联组织</option><option v-for="link in links" :key="link.id" :value="link.id">{{ link.name }}</option></select></UiField>
          <UiField label="项目名称" required v-slot="{control}"><input v-bind="control" v-model="form.title" required maxlength="120"></UiField>
          <UiField label="采购需求" required v-slot="{control}"><textarea v-autogrow v-bind="control" v-model="form.requirement" required maxlength="3000" rows="4"></textarea></UiField>
          <UiField label="目标日期" v-slot="{control}"><input v-bind="control" v-model="form.dueDate" type="date"></UiField>
          <p v-if="formError" role="alert">{{ formError }}</p><div class="modal-actions"><UiButton @click="closeDialog">取消</UiButton><UiButton type="submit" variant="primary">提交需求</UiButton></div>
        </form>
      </template>
      <UiState v-else-if="dialog === 'submitInquiry' && !products.length" title="暂无可询价的已发布产品" description="请联系软件厂商发布产品后再发起询价。" />
      <ChannelAction v-else-if="dialog && actionProject" :key="dialog + actionProject.revision" :action="dialog" :project="actionProject" :role="role" :products="products" :inquiry-draft="inquiryDraft" :busy="busy" :error="formError" @submit="submit" @cancel="closeDialog" />
    </ElDialog>
  </section>
</template>

<style scoped>
.live-project-actions { margin-left: auto; }
.live-feedback { margin-block: 8px; color: var(--ui-ink-secondary); }
.workspace-section > .ui-state { border-top: 1px solid var(--ui-border); }
.live-project-pagination { padding: 12px 0; align-items: center; }
@media (max-width: 540px) {
  .live-project-actions { margin-left: 0; }
}
</style>
