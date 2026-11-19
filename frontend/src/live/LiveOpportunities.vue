<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { UiButton, UiCard, UiState } from '../ui/index.js'
import OpportunityDesk from '../channel/OpportunityDesk.vue'
import { createHttpOpportunityRepository } from '../channel/httpOpportunities.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { createWriteJournal } from './writeJournal.js'
import '../channel/workspace.css'
import '../channel/opportunities.css'

const props = defineProps({ request: Function, scope: String, projectId: String, capture: Boolean })
const emit = defineEmits(['project'])
const raw = createHttpOpportunityRepository(props.request)
const projectsRepo = createHttpProjectRepository(props.request)
const desk = ref(null)
const projects = ref([])
const selected = ref(new URLSearchParams(location.search).get('op') || '')
const captureFor = ref(props.capture ? props.projectId : '')
const error = ref('')
const notice = ref('')
const loading = ref(true)
let alive = true
let timer
let sequence = 0
const journal = createWriteJournal('partnerledger-opportunity-write:' + props.scope, job => {
  const { input, requestId } = job
  if (job.kind === 'capture') return raw.captureOpportunity(input.projectId, input.payload, requestId)
  if (job.kind === 'partner') return raw.addPartner(input.payload, requestId)
  return raw.actOpportunity(input.id, input.action, input.payload, input.revision, requestId)
})
const state = journal.state
const repo = {
  captureOpportunity: (projectId, payload) => journal.run('capture', { projectId, payload }),
  addPartner: payload => journal.run('partner', { payload }),
  actOpportunity: (id, action, payload, revision) => journal.run('action', { id, action, payload, revision }),
}
async function refresh() {
  const current = ++sequence
  try {
    const [snapshot, records] = await Promise.all([raw.opportunityDesk(), projectsRepo.list()])
    if (alive && current === sequence) { desk.value = snapshot; projects.value = records; error.value = '' }
  } catch (cause) {
    if (alive && current === sequence) error.value = cause.message
  } finally { if (alive && current === sequence) loading.value = false }
}
function select(id) {
  selected.value = id
  const url = new URL(location.href)
  if (id) url.searchParams.set('op', id)
  else url.searchParams.delete('op')
  history.replaceState(null, '', url)
}
function captureOpened() {
  captureFor.value = ''
  const url = new URL(location.href)
  url.searchParams.delete('captureProject')
  history.replaceState(null, '', url)
}
async function retry() {
  const kind = state.job?.kind
  try {
    const result = await journal.retry()
    if (result && kind !== 'partner') select(result.id)
  } catch (cause) { notice.value = cause.message }
  await refresh()
}
function visibilityRefresh() {
  const dialogVisible = [...document.querySelectorAll('.el-dialog')].some(dialog => dialog.getClientRects().length > 0)
  if (!document.hidden && !state.busy && !dialogVisible) refresh()
}
onMounted(() => {
  refresh()
  timer = setInterval(visibilityRefresh, 20000)
  window.addEventListener('focus', visibilityRefresh)
  document.addEventListener('visibilitychange', visibilityRefresh)
})
onUnmounted(() => {
  alive = false
  clearInterval(timer)
  window.removeEventListener('focus', visibilityRefresh)
  document.removeEventListener('visibilitychange', visibilityRefresh)
})
</script>

<template>
  <section>
    <p v-if="notice" role="status">{{ notice }}</p>
    <UiCard v-if="state.job" title="核对尚未确认的提交">
      <p>{{ state.error || '本次提交内容已保留，可刷新服务端记录确认进度。' }}</p>
      <div class="modal-actions"><UiButton :disabled="state.busy" @click="refresh">检查最新记录</UiButton><UiButton :loading="state.busy" @click="retry">用原请求确认结果</UiButton></div>
    </UiCard>
    <UiState v-if="loading" state="loading" title="正在读取商机台账" />
    <UiState v-else-if="error" state="error" title="无法访问商机台账" :description="error"><UiButton @click="refresh">重试</UiButton></UiState>
    <template v-else-if="desk">
      <div class="modal-actions"><UiButton compact @click="refresh">刷新协作进度</UiButton></div>
      <OpportunityDesk :desk="desk" :repo="repo" :projects="projects" :selected="selected" :capture-for="captureFor" :refresh="refresh" live :locked="!!state.job && !state.busy"
        @select="select" @capture-opened="captureOpened" @project="record => $emit('project',record.id)" @notice="notice = $event" />
    </template>
  </section>
</template>
