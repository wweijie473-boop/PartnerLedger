<script setup>
import { onUnmounted, ref, watch } from 'vue'
import { UiBadge, UiButton, UiCard, UiField, UiState } from '../ui/index.js'
import { createHttpProjectRepository } from '../channel/httpProjects.js'

const props = defineProps({ request: { type: Function, required: true } })
const emit = defineEmits(['open-inquiry', 'open-projects'])
const text = ref('')
const result = ref(null)
const parsing = ref(false)
const confirmed = ref(false)
const manual = ref(false)
const error = ref('')
const resultError = ref('')
const matches = ref(null)
const matching = ref(false)
const matchError = ref('')
const candidate = ref(null)
const projects = ref([])
const projectId = ref('')
const loadingProjects = ref(false)
const savingDraft = ref(false)
const draftError = ref('')
const repo = createHttpProjectRepository(props.request)
let draftAttempt
const controller = new AbortController()
const labels = {
  enterprise_type: '企业类型', employee_count: '员工人数', required_modules: '所需模块',
  deployment_mode: '部署方式', license_count: '授权用户数',
}

onUnmounted(() => controller.abort())

function resetMatches() {
  matches.value = null
  matchError.value = ''
  candidate.value = null
  projects.value = []
  projectId.value = ''
  draftError.value = ''
}

watch(text, () => {
  if (manual.value) return
  result.value = null
  confirmed.value = false
  error.value = ''
  resultError.value = ''
  resetMatches()
})

function enterManually() {
  if (parsing.value) return
  manual.value = true
  error.value = ''
  resultError.value = ''
  confirmed.value = false
  result.value = { enterpriseType: '', employeeCount: '', modules: '', deployment: '', licenseCount: '', version: '', missing: [], questions: [] }
  resetMatches()
}

async function parse() {
  if (parsing.value) return
  const input = text.value.trim()
  if (!input || [...input].length > 2000) {
    error.value = '请填写 1–2000 字的需求。'
    return
  }
  parsing.value = true
  manual.value = false
  confirmed.value = false
  result.value = null
  error.value = ''
  resultError.value = ''
  resetMatches()
  try {
    const data = await props.request('/api/ai/parse-requirement', {
      method: 'POST', body: { text: input }, signal: controller.signal, timeout: 32000,
    })
    result.value = {
      enterpriseType: data.enterprise_type ?? '',
      employeeCount: data.employee_count ?? '',
      modules: (data.required_modules ?? []).join('、'),
      deployment: data.deployment_mode ?? '',
      licenseCount: data.license_count ?? '',
      version: '',
      missing: data.missing_fields ?? [],
      questions: data.follow_up_questions ?? [],
    }
  } catch (cause) {
    if (cause.name !== 'AbortError') error.value = cause.message
  } finally {
    parsing.value = false
  }
}

function fields() {
  const value = result.value
  return {
    enterprise_type: value.enterpriseType.trim(), employee_count: value.employeeCount || null,
    required_modules: value.modules.split(/[、，,；;\n]/).map(item => item.trim()).filter(Boolean),
    deployment_mode: value.deployment.trim(), license_count: value.licenseCount || null, version: value.version.trim(),
  }
}

function confirmResult() {
  if (!Object.values(result.value).some(value => typeof value === 'number' || typeof value === 'string' && value.trim())) {
    resultError.value = '请至少填写一个已知需求字段。'
    return
  }
  const validCount = (count) => count === '' || (Number.isSafeInteger(count) && count > 0 && count <= 1000000)
  if (!validCount(result.value.employeeCount) || !validCount(result.value.licenseCount)) {
    resultError.value = '人数须为 1–1000000 之间的整数，未知时可以留空。'
    return
  }
  resultError.value = ''
  confirmed.value = true
}

function editResult() {
  confirmed.value = false
  resetMatches()
}

async function findMatches() {
  if (matching.value || !confirmed.value) return
  resetMatches()
  const current = result.value
  const body = fields()
  const currentRequest = () => confirmed.value && result.value === current && JSON.stringify(fields()) === JSON.stringify(body)
  matching.value = true
  try {
    const res = await props.request('/api/ai/match-products', { method: 'POST', body, signal: controller.signal })
    if (currentRequest()) matches.value = res.items
  } catch (cause) {
    if (cause.name !== 'AbortError' && currentRequest()) matchError.value = cause.message
  } finally { matching.value = false }
}

async function chooseCandidate(item) {
  if (loadingProjects.value || savingDraft.value) return
  candidate.value = item
  projectId.value = ''
  draftError.value = ''
  loadingProjects.value = true
  try {
    const rows = await repo.list({ signal: controller.signal })
    if (candidate.value !== item) return
    projects.value = rows.filter(project => project.actions.includes('submitInquiry'))
    if (projects.value.length === 1) projectId.value = projects.value[0].id
  } catch (cause) {
    if (candidate.value === item) draftError.value = cause.message
  } finally { loadingProjects.value = false }
}

async function prepareDraft() {
  const project = projects.value.find(item => item.id === projectId.value)
  if (!project || !confirmed.value || !candidate.value || savingDraft.value) return
  const body = { revision: project.revision, confirmed: true, productId: candidate.value.product.id, requirement: fields() }
  const signature = JSON.stringify({ project: project.id, body })
  if (draftAttempt?.signature !== signature) draftAttempt = { signature, requestId: crypto.randomUUID() }
  savingDraft.value = true
  draftError.value = ''
  try {
    const draft = await props.request('/api/channel/v1/projects/' + encodeURIComponent(project.id) + '/inquiry-draft', {
      method: 'POST', body: { ...body, requestId: draftAttempt.requestId }, signal: controller.signal,
    })
    emit('open-inquiry', draft.projectId)
  } catch (cause) {
    if (cause.name !== 'AbortError') draftError.value = cause.message
    if ([404, 409].includes(cause.status)) {
      projectId.value = ''
      projects.value = []
    }
  } finally { savingDraft.value = false }
}

const matchLabels = { matched: '符合已知条件', needs_confirmation: '有待核实', mismatch: '有条件不符' }
const checkLabels = { met: '符合', unknown: '待核实', missing: '不符' }
</script>

<template>
  <section class="page-heading"><div><h1>需求整理</h1><p class="muted">使用 AI 提取关键字段，或手工填写已知的采购需求。</p></div></section>

  <div class="requirement-layout">
    <UiCard title="原始需求">
      <form class="form-grid" @submit.prevent="parse">
        <UiField label="需求描述" hint="使用脱敏示例，最多 2000 字。" required v-slot="{ control }">
          <textarea v-bind="control" v-model="text" :disabled="parsing || savingDraft" rows="7" maxlength="2000" required
            placeholder="例如：一家 120 人的制造企业需要财务和库存模块，希望私有化部署。" />
        </UiField>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <div class="requirement-actions"><UiButton type="submit" variant="primary" icon="request" :loading="parsing" :disabled="savingDraft">解析需求</UiButton>
          <UiButton v-if="!result" :disabled="parsing" @click="enterManually">手工填写</UiButton>
        </div>
      </form>
    </UiCard>

    <UiCard :title="manual ? '手工需求字段' : '解析结果'">
      <template v-if="confirmed" #actions><UiBadge tone="success" dot>已确认</UiBadge></template>
      <UiState v-if="parsing" state="loading" title="正在解析需求" />
      <UiState v-else-if="!result" title="暂无解析结果" description="提交描述后，核对提取的字段和待补充信息。" />
      <form v-else class="form-grid" @submit.prevent="confirmResult">
        <div class="form-row">
          <UiField label="企业类型" v-slot="{ control }"><input v-bind="control" v-model="result.enterpriseType" :readonly="confirmed" placeholder="待确认" /></UiField>
          <UiField label="部署方式" v-slot="{ control }"><input v-bind="control" v-model="result.deployment" :readonly="confirmed" placeholder="待确认" /></UiField>
        </div>
        <div class="form-row">
          <UiField label="员工人数" v-slot="{ control }"><input v-bind="control" v-model="result.employeeCount" type="number" min="1" :readonly="confirmed" placeholder="待确认" /></UiField>
          <UiField label="授权用户数" v-slot="{ control }"><input v-bind="control" v-model="result.licenseCount" type="number" min="1" :readonly="confirmed" placeholder="待确认" /></UiField>
        </div>
        <UiField label="所需模块" v-slot="{ control }"><input v-bind="control" v-model="result.modules" :readonly="confirmed" placeholder="待确认" /></UiField>
        <div v-if="result.missing.length || result.questions.length" class="requirement-follow-up">
          <p v-if="result.missing.length">解析时缺失：{{ result.missing.map((key) => labels[key] ?? key).join('、') }}</p>
          <ul v-if="result.questions.length"><li v-for="(question, index) in result.questions" :key="index">{{ question }}</li></ul>
        </div>
        <p v-if="resultError" class="form-error" role="alert">{{ resultError }}</p>
        <UiField label="指定版本" v-slot="{ control }"><input v-bind="control" v-model="result.version" :readonly="confirmed" maxlength="100" placeholder="可留空" /></UiField>
        <div v-if="confirmed" class="requirement-actions">
          <UiButton :disabled="savingDraft" @click="editResult">重新编辑</UiButton>
          <UiButton variant="primary" :loading="matching" :disabled="savingDraft" @click="findMatches">匹配已发布产品</UiButton>
        </div>
        <UiButton v-else type="submit" variant="primary" icon="check">{{ manual ? '确认需求字段' : '确认解析结果' }}</UiButton>
      </form>
    </UiCard>
  </div>
  <UiCard v-if="matching || matches || matchError" class="requirement-matches" title="产品匹配">
    <UiState v-if="matching" state="loading" title="正在核对已发布目录" />
    <p v-else-if="matchError" class="form-error" role="alert">{{ matchError }}</p>
    <UiState v-else-if="!matches.length" title="暂无带资料的已发布产品" />
    <template v-else>
      <article v-for="item in matches" :key="item.product.id" class="requirement-candidate">
        <h3>{{ item.product.name }} · {{ item.product.version }} <UiBadge>{{ matchLabels[item.status] }}</UiBadge></h3>
        <ul><li v-for="(check, index) in item.checks" :key="index">{{ checkLabels[check.status] }}{{ check.value ? ' · ' + check.value : '' }}，{{ check.detail }}</li></ul>
        <details v-if="item.sources.length">
          <summary>查看厂家资料</summary>
          <ul><li v-for="source in item.sources" :key="source.title + source.reference"><strong>{{ source.title }}</strong> · {{ source.reference }}<p>{{ source.excerpt }}</p></li></ul>
        </details>
        <UiButton v-if="item.status !== 'mismatch'" compact :disabled="loadingProjects || savingDraft" @click="chooseCandidate(item)">使用此产品准备询价</UiButton>
      </article>
      <form v-if="candidate" class="form-grid requirement-draft" @submit.prevent="prepareDraft">
        <p>询价产品：{{ candidate.product.name }} · {{ candidate.product.version }}</p>
        <UiState v-if="loadingProjects" state="loading" title="正在读取可用项目" />
        <template v-else-if="projects.length">
          <UiField label="关联项目" required v-slot="{ control }">
            <select v-bind="control" v-model="projectId" :disabled="savingDraft" required>
              <option value="">选择待整理需求的项目</option>
              <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.title }} · {{ project.customerName }}</option>
            </select>
          </UiField>
          <UiButton type="submit" variant="primary" :loading="savingDraft" :disabled="!projectId">保存草稿并打开项目</UiButton>
        </template>
        <UiButton v-else-if="draftError" @click="chooseCandidate(candidate)">重新读取项目</UiButton>
        <UiButton v-else @click="$emit('open-projects')">前往项目协作建项</UiButton>
        <p v-if="draftError" class="form-error" role="alert">{{ draftError }}</p>
      </form>
    </template>
  </UiCard>
</template>

<style scoped>
.requirement-layout { display: grid; grid-template-columns: minmax(0, .9fr) minmax(0, 1.1fr); gap: var(--ui-space-4); align-items: start; }
.requirement-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.requirement-follow-up { padding: var(--ui-space-3); border-radius: var(--ui-radius-sm); background: var(--ui-surface-muted); font-size: var(--ui-text-sm); }
.requirement-follow-up ul { margin: var(--ui-space-2) 0 0; padding-inline-start: 20px; }
.requirement-matches { margin-top: var(--ui-space-5); }
.requirement-candidate { padding-block: var(--ui-space-4); border-bottom: 1px solid var(--ui-border); }
.requirement-candidate h3 { display: flex; align-items: center; flex-wrap: wrap; gap: var(--ui-space-2); }
.requirement-candidate :is(ul, details) { margin-block: var(--ui-space-2); }
.requirement-draft { padding-top: var(--ui-space-4); }
@media (max-width: 850px) { .requirement-layout { grid-template-columns: minmax(0, 1fr); } }
</style>
