<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { UiButton, UiCard, UiField } from '../ui/index.js'
import VisitPartnerFinder from './VisitPartnerFinder.vue'
import { createVisitDraftStore, splitVisit } from './visitDrafts.js'
import './visits.css'

const props = defineProps({ scope:String, loadProjects:Function, loadDesk:Function, capture:Function })
defineEmits(['project','opportunity'])
const store = createVisitDraftStore(props.scope)
const drafts = ref([])
const projects = ref([])
const error = ref('')
const message = ref('')
const loading = ref(true)
const saving = ref(false)
const confirmed = ref(false)
const review = ref(false)
const criteria = ref(null)
const form = reactive({ id:'',revision:0,projectId:'',text:'',main:'',additional:'',opportunityId:'' })
const partnerNote = computed(() => (review.value ? form.additional : splitVisit(form.text).additional).trim())
function open(draft) {
  if (saving.value) return
  Object.assign(form,{id:'',revision:0,projectId:'',text:'',main:'',additional:'',opportunityId:''},draft || {})
  review.value = !!draft?.reviewed
  confirmed.value = false
  error.value = ''; message.value = ''
}
function restore() { drafts.value = store.list() }
function save(reviewed = false) {
  const record = store.save({...form,reviewed},form.revision)
  Object.assign(form,record)
  restore()
  return record
}
function saveDraft() {
  try {
    if (!form.text.trim()) throw new Error('先记下一句话，再保存草稿。')
    save(review.value)
    message.value = '草稿已保存在此设备，稍后可以接着整理。'
  } catch (cause) { error.value = cause.message }
}
function organize() {
  const fields = splitVisit(form.text)
  form.main = fields.main
  form.additional = fields.additional
  review.value = true; confirmed.value = false
}
async function finish() {
  if (saving.value || !confirmed.value) return
  error.value = ''
  saving.value = true
  try {
    if (!projects.value.some(project => project.id === form.projectId)) throw new Error('请选择可见的关联客户项目。')
    save(true)
    if (props.capture && form.additional.trim() && !form.opportunityId) {
      const result = await props.capture(form.projectId,form.additional.trim(),criteria.value)
      form.opportunityId = result.id
      save(true)
    }
    message.value = props.capture && form.additional.trim() ? '已记录为独立附加商机，可在商机拾遗继续跟进。' : '已保存核对结果，可回到关联项目继续办理。'
  } catch (cause) { error.value = cause.message }
  finally { saving.value = false }
}
function remove(id) {
  if (saving.value) return
  try { store.remove(id); restore(); if (form.id === id) open() }
  catch (cause) { error.value = cause.message }
}
onMounted(async () => {
  try { restore(); projects.value = await props.loadProjects() }
  catch (cause) { error.value = cause.message }
  finally { loading.value = false }
})
</script>

<template>
  <div class="visit-page">
    <header class="page-heading"><div><h1>记拜访</h1><p class="muted">记下客户提到的附加需求，当场看看团队里谁能引荐。</p></div></header>
    <p class="visit-hint">草稿保存在此设备。<template v-if="!capture">附加商机同步接口待开放，核对结果可继续留在草稿中。</template></p>
    <div class="visit-layout"><UiCard title="这次客户提了什么">
      <form class="form-grid" @submit.prevent="finish">
        <UiField label="拜访记录" required v-slot="{control}"><textarea v-bind="control" v-model="form.text" rows="4" maxlength="2000" placeholder="例如：ERP 已定，客户还想把 ERP 工单同步到 MES。" :readonly="saving || !!form.opportunityId" @input="review=false; confirmed=false"></textarea></UiField>
        <VisitPartnerFinder v-if="loadDesk && (partnerNote || form.opportunityId)" :key="form.id" :note="partnerNote" :opportunity-id="form.opportunityId" :load-desk="loadDesk" @criteria="criteria = $event" />
        <UiField label="关联客户 / 项目" v-slot="{control}"><select v-bind="control" v-model="form.projectId" :disabled="loading || saving || !!form.opportunityId" @change="confirmed=false"><option value="">稍后选择客户项目</option><option v-for="project in projects" :key="project.id" :value="project.id">{{ project.customerName }} · {{ project.title }}</option></select></UiField>
        <div class="visit-actions"><UiButton :disabled="saving" @click="saveDraft">保存草稿</UiButton><UiButton v-if="!form.opportunityId" :disabled="saving || !form.text.trim()" @click="organize">核对两类需求</UiButton></div>
        <template v-if="review">
          <UiField label="主采购事项" hint="仅整理记录，项目需求可回到原项目继续核对。" v-slot="{control}"><textarea v-bind="control" v-model="form.main" rows="2" :readonly="saving || !!form.opportunityId" @input="confirmed=false"></textarea></UiField>
          <UiField label="附加需求" hint="类型不清楚时保留原说法，后续向客户澄清。" v-slot="{control}"><textarea v-bind="control" v-model="form.additional" rows="2" :readonly="saving || !!form.opportunityId" @input="confirmed=false"></textarea></UiField>
          <label v-if="!form.opportunityId" class="visit-check"><input v-model="confirmed" type="checkbox" required>已核对分类和关联项目</label>
          <UiButton v-if="!form.opportunityId" type="submit" variant="primary" :disabled="!confirmed || saving" :loading="saving">{{ capture && form.additional.trim() ? '保存并记录附加商机' : '保存核对结果' }}</UiButton>
          <UiButton v-else compact variant="text" @click="$emit('opportunity',form.opportunityId)">查看关联商机</UiButton>
        </template>
        <UiButton v-if="form.projectId && projects.some(project => project.id === form.projectId)" @click="$emit('project',form.projectId)">打开关联项目</UiButton>
        <p v-if="error" role="alert">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
      </form>
    </UiCard><UiCard title="接着整理">
      <template #actions><UiButton compact @click="open()">新记一条</UiButton></template>
      <p v-if="!drafts.length" class="muted">保存后，草稿会出现在这里。</p>
      <article v-for="draft in drafts" :key="draft.id" class="visit-draft"><button @click="open(draft)">{{ draft.text }}</button><small>{{ new Date(draft.updatedAt).toLocaleDateString('zh-CN') }} · {{ draft.opportunityId ? '已记录商机' : draft.reviewed ? '已核对' : '待核对' }}</small><UiButton compact variant="text" @click="remove(draft.id)">删除草稿</UiButton></article>
    </UiCard></div>
  </div>
</template>
