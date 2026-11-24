<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { UiButton, UiCard, UiField, UiState } from '../ui/index.js'
import VisitPartnerFinder from './VisitPartnerFinder.vue'
import { createHttpVisitRepository } from './httpVisits.js'
import { createHttpOpportunityRepository } from '../channel/httpOpportunities.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { createVisitDraftStore, splitVisit } from './visitDrafts.js'
import { createWriteJournal } from './writeJournal.js'
import './visits.css'

const props = defineProps({ request: Function, scope: String })
const emit = defineEmits(['project','opportunity'])
const remote = createHttpVisitRepository(props.request)
const { opportunityDesk } = createHttpOpportunityRepository(props.request)
const projectRepo = createHttpProjectRepository(props.request)
const local = createVisitDraftStore('live:' + props.scope)
const rows = ref([])
const localRows = ref([])
const projects = ref([])
const loading = ref(true)
const loadError = ref('')
const error = ref('')
const message = ref('')
const confirmed = ref(false)
const reviewed = ref(false)
const criteria = ref(null)
const localSource = ref('')
const form = reactive({ id:'', revision:0, projectId:'', text:'', main:'', additional:'', opportunityId:'', confirmed:false })
const partnerNote = computed(() => (reviewed.value ? form.additional : splitVisit(form.text).additional).trim())
const journal = createWriteJournal('partnerledger-visit-write:' + props.scope, job => {
  const { input, requestId } = job
  if (job.kind === 'save') return remote.save(input.id,input.payload,input.revision,requestId)
  if (job.kind === 'confirm') return remote.confirm(input.id,input.revision,input.createOpportunity,requestId,input.criteria)
  return remote.remove(input.id,input.revision,requestId)
})
const state = journal.state
const locked = computed(() => state.busy || !!state.job)
const readonly = computed(() => locked.value || !!form.opportunityId)
const canConfirm = computed(() => reviewed.value && confirmed.value && !!form.projectId && (form.main.trim() || form.additional.trim()))
function open(record) {
  if (locked.value) return
  Object.assign(form,{ id:'', revision:0, projectId:'', text:'', main:'', additional:'', opportunityId:'', confirmed:false },record || {})
  reviewed.value = !!record?.reviewed
  confirmed.value = false
  localSource.value = ''
  error.value = ''; message.value = ''
}
async function refresh() {
  try {
    const [visits, records] = await Promise.all([remote.list(),projectRepo.list()])
    rows.value = visits; projects.value = records; loadError.value = ''
  } catch (cause) { loadError.value = cause.message }
  finally { loading.value = false }
}
function readLocal() {
  try { localRows.value = local.list() }
  catch (cause) { error.value = cause.message }
}
async function openLocal(record) {
  if (locked.value) return
  if (record.serverVisitId) {
    try { open(await remote.get(record.serverVisitId)) }
    catch (cause) { error.value = cause.message }
    return
  }
  open({ projectId:record.projectId || '', text:record.text, main:record.main || '', additional:record.additional || '', reviewed:false })
  localSource.value = record.id
  message.value = '本机原记录已保留。点击保存后将同步到当前账号，核对后再确认生成商机。'
}
function organize() {
  const result = splitVisit(form.text)
  form.main = result.main; form.additional = result.additional
  reviewed.value = true; confirmed.value = false
}
async function afterSuccess(job, record) {
  if (job.kind === 'remove') {
    open()
    message.value = '服务端草稿已删除，已确认的商机可继续办理。'
  } else {
    open(record)
    if (job.input.localSource) {
      try {
        const original = local.list().find(item => item.id === job.input.localSource)
        if (original) local.save({ ...original, serverVisitId:record.id },original.revision)
        readLocal()
      } catch (cause) { message.value = '服务端已保存，本机同步标记更新失败，请按服务端草稿继续办理。' }
    }
    if (job.kind === 'save' && job.input.confirmAfter) {
      await perform('confirm',{ id:record.id, revision:record.revision, createOpportunity:job.input.createOpportunity, criteria:job.input.criteria })
      return
    }
    message.value = record.opportunityId ? '已确认并关联一条正式商机。' : '已保存到当前账号。'
  }
  await refresh()
}
async function failed(job,cause) {
  error.value = cause.message
  await refresh()
  if (cause.status === 409 && job.input.id) {
    try {
      const latest = await remote.get(job.input.id)
      open(latest)
      error.value = '草稿已更新，已重载服务端版本。请重新核对后再提交。'
    } catch (failure) { error.value = failure.message }
  }
}
async function perform(kind,input) {
  if (locked.value) return
  error.value = ''
  const job = {kind,input}
  try {
    const result = await journal.run(kind,input)
    await afterSuccess(job,result)
  } catch (cause) { await failed(job,cause) }
}
async function retry() {
  const job = state.job
  if (!job || state.busy) return
  try { await afterSuccess(job,await journal.retry()) }
  catch (cause) { await failed(job,cause) }
}
async function save(confirmAfter = false) {
  if (!form.text.trim()) { error.value = '请先填写拜访记录。'; return }
  if (confirmAfter && !canConfirm.value) return
  await perform('save',{
    id:form.id, revision:form.revision, localSource:localSource.value,
    confirmAfter, createOpportunity:!!form.additional.trim(), criteria:confirmAfter && form.additional.trim() ? criteria.value : undefined,
    payload:{ projectId:form.projectId, text:form.text.trim(), main:form.main.trim(), additional:form.additional.trim(), reviewed:reviewed.value },
  })
}
onMounted(async () => { readLocal(); await refresh() })
</script>

<template>
  <section class="visit-page">
    <header class="page-heading"><div><h1>记拜访</h1><p class="muted">记下客户提到的附加需求，当场看看团队里谁能引荐。同步草稿仅自己可读。</p></div><UiButton :disabled="locked" @click="refresh">刷新</UiButton></header>
    <UiCard v-if="state.job" title="核对尚未确认的提交">
      <p>{{ state.error || '已保留本次请求，请先检查服务端记录。' }}</p>
      <div class="modal-actions"><UiButton :disabled="state.busy" @click="refresh">检查最新记录</UiButton><UiButton :loading="state.busy" @click="retry">用原请求确认结果</UiButton></div>
    </UiCard>
    <UiState v-if="loading" state="loading" title="正在读取拜访草稿" />
    <UiState v-else-if="loadError" state="error" title="无法访问拜访草稿" :description="loadError"><UiButton @click="refresh">重试</UiButton></UiState>
    <div v-else class="visit-layout">
      <UiCard title="本次拜访">
        <form class="form-grid" @submit.prevent="save(true)">
          <fieldset class="visit-inputs" :disabled="readonly">
            <UiField label="拜访记录" required v-slot="{control}"><textarea v-bind="control" v-model="form.text" rows="4" maxlength="2000" placeholder="例如：ERP 已定，客户还想把 ERP 工单同步到 MES。" required @input="reviewed=false; confirmed=false"></textarea></UiField>
            <UiField label="关联客户项目" v-slot="{control}"><select v-bind="control" v-model="form.projectId" @change="confirmed=false"><option value="">稍后关联项目</option><option v-for="project in projects" :key="project.id" :value="project.id">{{ project.customerName }} · {{ project.title }}</option></select></UiField>
            <div class="visit-actions"><UiButton @click="save(false)">保存草稿</UiButton><UiButton :disabled="!form.text.trim()" @click="organize">核对两类需求</UiButton></div>
            <template v-if="reviewed">
              <UiField label="主采购事项" v-slot="{control}"><textarea v-bind="control" v-model="form.main" rows="2" maxlength="2000" @input="confirmed=false"></textarea></UiField>
              <UiField label="附加需求" v-slot="{control}"><textarea v-bind="control" v-model="form.additional" rows="2" maxlength="2000" @input="confirmed=false"></textarea></UiField>
              <label class="visit-check"><input v-model="confirmed" type="checkbox">已核对记录和关联项目</label>
              <UiButton type="submit" variant="primary" :disabled="!canConfirm">{{ form.additional.trim() ? '保存并确认生成商机' : '保存并确认核对结果' }}</UiButton>
            </template>
          </fieldset>
        </form>
        <VisitPartnerFinder v-if="partnerNote || form.opportunityId" :key="form.id" :note="partnerNote" :opportunity-id="form.opportunityId" :load-desk="opportunityDesk" @criteria="criteria = $event" />
        <p v-if="form.opportunityId">已关联正式商机 {{ form.opportunityId }}。</p>
        <div class="modal-actions"><UiButton v-if="form.projectId" :disabled="locked" @click="$emit('project',form.projectId)">打开关联项目</UiButton><UiButton v-if="form.opportunityId" :disabled="locked" @click="$emit('opportunity',form.opportunityId)">查看正式商机</UiButton></div>
        <p v-if="error" role="alert">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
      </UiCard>
      <div class="visit-lists">
        <UiCard title="我的已同步草稿">
          <template #actions><UiButton compact :disabled="locked" @click="open()">新记一条</UiButton></template>
          <p v-if="!rows.length" class="muted">保存后，可用同一账号在其他浏览器继续整理。</p>
          <article v-for="draft in rows" :key="draft.id" class="visit-draft">
            <button :disabled="locked" @click="open(draft)">{{ draft.text }}</button><small>版本 {{ draft.revision }} · {{ draft.opportunityId ? '已关联商机' : draft.confirmed ? '已确认' : draft.reviewed ? '已核对待确认' : '待核对' }}</small>
            <UiButton compact variant="text" :disabled="locked" @click="perform('remove',{id:draft.id,revision:draft.revision})">删除草稿</UiButton>
          </article>
        </UiCard>
        <UiCard v-if="localRows.length" title="原有本机草稿" description="选择一条记录，再保存到当前账号。原本机内容保留。">
          <article v-for="draft in localRows" :key="draft.id" class="visit-draft"><p>{{ draft.text }}</p><UiButton compact :disabled="locked" @click="openLocal(draft)">{{ draft.serverVisitId ? '打开已同步记录' : '选择同步此草稿' }}</UiButton></article>
        </UiCard>
      </div>
    </div>
  </section>
</template>

<style scoped>
.visit-inputs { display:grid; gap:16px; border:0; padding:0; margin:0; min-width:0; }
.visit-lists { display:grid; gap:16px; }
</style>
