<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElDialog } from 'element-plus'
import { UiButton, UiField, UiState, UiBadge, UiMetricCard } from '../ui/index.js'
import ProjectDetail from '../channel/ProjectDetail.vue'
import ChannelAction from '../channel/ChannelAction.vue'
import { actionLabels } from '../channel/domain.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import '../channel/workspace.css'
import { statusTone } from '../ui/statusTone.js'

const props = defineProps({ request: Function, role: String, organizationId: String, overview: Boolean })
defineEmits(['capture-opportunity', 'open-project'])
const repo = createHttpProjectRepository(props.request)
const rows = ref([])
const links = ref([])
const products = ref([])
const record = ref(null)
const selected = ref(new URLSearchParams(location.search).get('project') || '')
const loading = ref(true)
const loadError = ref('')
const detailError = ref(null)
const detailLoading = ref(false)
const dialog = ref('')
const busy = ref(false)
const formError = ref('')
const notice = ref('')
const keyword = ref('')
const stage = ref('')
const todo = ref(false)
const contextLoading = ref(false)
const contextError = ref('')
const pending = ref(null)
const form = reactive({ title: '', requirement: '', dueDate: '', organization: '' })
const storageKey = 'partnerledger-project-request-' + props.role + '-' + props.organizationId
const results = computed(() => rows.value.filter(item => (!todo.value || item.isTodo) &&
  (!stage.value || item.stage === stage.value) &&
  [item.id,item.title,item.customerName].filter(Boolean).join(' ').toLowerCase().includes(keyword.value.toLowerCase())))
const tasks = computed(() => rows.value.filter(item => item.isTodo))
const stages = computed(() => [...new Map(rows.value.map(item => [item.stage,item.stageLabel])).entries()])
const allowed = new Set(['submitInquiry','withdrawInquiry','requestInfo','replyInfo','flagGap'])
const detail = computed(() => record.value ? { ...record.value, actions:record.value.actions.filter(action => allowed.has(action)) } : null)
const title = computed(() => pending.value ? '核对尚未确认的提交' : dialog.value === 'create' ? '建立采购项目' : actionLabels[dialog.value])
let timer
let unmounted = false
let refreshing = false
let detailSequence = 0

function storePending(value) {
  pending.value = value
  try {
    if (value) sessionStorage.setItem(storageKey,JSON.stringify(value))
    else sessionStorage.removeItem(storageKey)
  } catch { notice.value = '浏览器无法保留待核对提交，关闭页面前请先确认办理结果。' }
}
function setSelected(id) {
  selected.value = id
  const url = new URL(location.href)
  if (id) url.searchParams.set('project',id)
  else url.searchParams.delete('project')
  history.replaceState(null,'',url)
}
async function loadDetail() {
  if (!selected.value) return
  const sequence = ++detailSequence
  detailLoading.value = true
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
async function refresh() {
  if (refreshing || unmounted) return
  refreshing = true
  try {
    const items = await repo.list()
    if (!unmounted) { rows.value = items; loadError.value = '' }
  } catch (error) {
    if (!unmounted && error.name !== 'AbortError') loadError.value = error.message
  } finally {
    refreshing = false
    if (!unmounted) loading.value = false
  }
}
async function openProject(id) {
  setSelected(id)
  record.value = null
  await loadDetail()
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
    }
  } catch (error) { contextError.value = error.message }
  finally { contextLoading.value = false }
}
function closeDialog() {
  if (busy.value || pending.value) return
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
  try {
    const updated = job.action === 'create' ? await repo.create(job.payload,job.requestId) :
      await repo.act(job.id,job.action,job.payload,job.revision,job.requestId)
    storePending(null)
    dialog.value = ''
    record.value = updated
    setSelected(updated.id)
    notice.value = '办理结果已保存，协作方可在关联项目中查看。'
    await refresh()
    await loadDetail()
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
      if (job.id) { setSelected(job.id); await loadDetail() }
    }
  } finally { busy.value = false }
}
onMounted(async () => {
  try {
    const job = JSON.parse(sessionStorage.getItem(storageKey))
    if (job?.requestId && job.payload && job.action) {
      pending.value = job
      dialog.value = job.action
    }
  } catch { notice.value = '上次请求记录无法读取，请先检查项目台账再办理。' }
  await refresh()
  await loadDetail()
  timer = setInterval(backgroundRefresh,20000)
  window.addEventListener('focus',backgroundRefresh)
  document.addEventListener('visibilitychange',backgroundRefresh)
})
onUnmounted(() => {
  unmounted = true
  clearInterval(timer)
  window.removeEventListener('focus',backgroundRefresh)
  document.removeEventListener('visibilitychange',backgroundRefresh)
})
</script>

<template>
  <section>
    <header class="page-heading">
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
        <section class="metric-grid" aria-label="协作数据">
          <UiMetricCard label="可见项目" :value="rows.length" description="当前组织参与的项目" icon="request" />
          <UiMetricCard label="待我处理" :value="tasks.length" description="下一步由本方办理" icon="activity" tone="brand" />
          <UiMetricCard label="协作方处理中" :value="rows.filter(item => !item.isTodo && item.stage !== 'completed').length" description="可打开项目查看最新进度" icon="time" />
        </section>
        <section class="workspace-section" aria-labelledby="project-tasks">
          <header class="workspace-section-heading"><h2 id="project-tasks">今天的项目待办</h2></header>
          <div v-if="tasks.length" class="live-task-list">
            <button v-for="item in tasks" :key="item.id" type="button" @click="$emit('open-project', item.id)">
              <span><strong>{{ item.title }}</strong><small>{{ item.nextStep }}</small></span>
              <UiBadge :tone="statusTone(item.stage)" dot>{{ item.stageLabel }}</UiBadge><span class="live-task-action">去处理 →</span>
            </button>
          </div>
          <UiState v-else title="当前没有待办理项目" description="新的需求或协作方回复到达后，会出现在这里。可从项目台账查看全部进度。" />
        </section>
      </template>
    </template>
    <template v-else-if="selected">
      <UiState v-if="detailLoading" state="loading" title="正在读取项目详情" />
      <UiState v-else-if="detailError" :state="detailError.status === 404 ? 'denied' : 'error'" title="项目暂不可访问" :description="detailError.message">
        <UiButton @click="loadDetail">重试</UiButton><UiButton @click="setSelected('')">返回项目列表</UiButton>
      </UiState>
      <ProjectDetail v-else-if="detail" :project="detail" :role="role" :products="products" live @back="setSelected('')" @action="openAction" />
      <div v-if="detail && !detailError && role === 'dealer'" class="modal-actions"><UiButton @click="$emit('capture-opportunity',detail.id)">记录附加商机</UiButton></div>
    </template>
    <template v-else>
      <UiState v-if="loadError" state="error" title="项目列表暂不可用" :description="loadError"><UiButton @click="refresh">重新载入</UiButton></UiState>
      <section v-else class="workspace-section" aria-labelledby="project-list">
        <header class="workspace-section-heading">
          <h2 id="project-list">协作项目</h2>
          <div class="live-project-filters">
            <input v-model="keyword" type="search" placeholder="搜索项目或编号" aria-label="搜索项目">
            <select v-model="stage" aria-label="项目状态">
              <option value="">全部状态</option>
              <option v-for="[key, label] in stages" :key="key" :value="key">{{ label }}</option>
            </select>
            <label><input v-model="todo" type="checkbox">待我处理</label>
          </div>
        </header>
        <div v-if="results.length" class="table-scroll"><table><thead><tr><th>项目</th><th>进度</th><th>下一步</th><th>操作</th></tr></thead><tbody>
          <tr v-for="item in results" :key="item.id"><td>{{ item.title }}<br><small>{{ item.id }}</small></td><td><UiBadge :tone="statusTone(item.stage)" dot>{{ item.stageLabel }}</UiBadge></td><td>{{ item.nextStep }}</td><td><UiButton compact @click="openProject(item.id)">查看</UiButton></td></tr>
        </tbody></table></div>
        <UiState v-else title="暂无符合条件的项目" :description="role === 'vendor' ? '更换筛选条件，或等待代理商向本组织提交询价。' : '更换筛选条件，或建立一笔新的采购需求。'" />
      </section>
    </template>
    <ElDialog :model-value="!!dialog" :title="title" width="min(94vw, 610px)" :close-on-click-modal="false" :close-on-press-escape="!busy && !pending" :show-close="!busy && !pending" @update:model-value="value => { if (!value) closeDialog() }">
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
      <ChannelAction v-else-if="dialog && record" :key="dialog + record.revision" :action="dialog" :project="record" :role="role" :products="products" :busy="busy" :error="formError" @submit="submit" @cancel="closeDialog" />
    </ElDialog>
  </section>
</template>

<style scoped>
.live-project-actions { margin-left: auto; }
.live-feedback { margin-block: 8px; color: var(--ui-ink-secondary); }
.live-project-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  font-size: 13px;
}
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
.live-project-filters label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: var(--ui-control-height);
  cursor: pointer;
  white-space: nowrap;
}
.live-project-filters input[type='checkbox'] { margin: 0; }
.workspace-section > .ui-state { border-top: 1px solid var(--ui-border); }
.live-task-list button {
  display: flex;
  align-items: center;
  gap: 8px 12px;
  width: 100%;
  padding: 8px 6px;
  border: 0;
  border-top: 1px solid var(--ui-border);
  background: transparent;
  color: var(--ui-ink);
  text-align: left;
}
.live-task-list button:hover, .live-task-list button:focus-visible { background: var(--ui-accent-bg); }
.live-task-list button > span:first-child { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.live-task-list strong, .live-task-list small { display: block; }
.live-task-list strong { font-weight: 500; }
.live-task-list small { color: var(--ui-ink-muted); font-size: 12px; }
.live-task-action { color: var(--ui-link); font-size: 13px; white-space: nowrap; }
@media (max-width: 540px) {
  .live-project-actions { margin-left: 0; }
  .live-task-list button { flex-wrap: wrap; }
  .live-task-list button > span:first-child { flex-basis: 100%; }
}
</style>
