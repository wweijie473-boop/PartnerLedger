<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { createHttpProjectRepository } from '../channel/httpProjects.js'
import { aiDemoProject, matchDemoProducts, parseDemoRequirement } from '../channel/aiInquiryDemo.js'
import { UiBadge, UiButton, UiCard, UiField, UiIcon, UiState } from '../ui/index.js'
import { inquirySummary } from './aiInquiry.js'

const props = defineProps({
  request: Function,
  demoRepository: Object,
  initialProject: { type: String, default: '' },
})
const emit = defineEmits(['open-project', 'open-projects'])
const demo = Boolean(props.demoRepository)
const repo = demo ? props.demoRepository : createHttpProjectRepository(props.request)
const controller = new AbortController()
const text = ref(demo ? aiDemoProject.requirement : '')
const result = ref(null)
const manual = ref(false)
const step = ref(1)
const steps = ['核对需求', '选择产品', '预览草稿']
const stepBar = ref(null)
const parsing = ref(false)
const matching = ref(false)
const saving = ref(false)
const projects = ref([])
const loadingProjects = ref(false)
const selectedId = ref('')
const projectError = ref('')
const error = ref('')
const matches = ref(null)
const candidate = ref(null)
const draft = ref(null)
const selectedProject = computed(() => projects.value.find(item => item.id === selectedId.value))
const busy = computed(() => parsing.value || matching.value || saving.value || loadingProjects.value)
const eligible = computed(() => matches.value?.filter(item => item.status !== 'mismatch') ?? [])
const otherMatches = computed(() => matches.value?.filter(item => item.status === 'mismatch') ?? [])
const previewProduct = computed(() => candidate.value?.product)
const fieldSpecs = [
  { key: 'enterprise_type', label: '企业类型', max: 120 },
  { key: 'employee_count', label: '员工人数', numeric: true },
  { key: 'required_modules', label: '所需模块', hint: '多个模块用顿号或逗号分隔。' },
  { key: 'deployment_mode', label: '部署方式', max: 80 },
  { key: 'license_count', label: '授权用户数', numeric: true },
  { key: 'version', label: '指定版本', max: 100 },
]
const fit = {
  matched: ['符合已知条件', 'success'],
  needs_confirmation: ['待核实', 'warning'],
  mismatch: ['有条件不符', 'danger'],
}
const checks = { met: '符合', unknown: '待核实', missing: '不符' }
let draftAttempt

onUnmounted(() => controller.abort())

function toStep(number) {
  if (step.value === number) return
  error.value = ''
  step.value = number
  nextTick(() => stepBar.value?.querySelector('[aria-current="step"]')?.focus())
}

function resetResult() {
  error.value = ''
  result.value = null
  manual.value = false
  matches.value = null
  candidate.value = null
  draft.value = null
  toStep(1)
}

function showFields(data = {}) {
  result.value = {
    enterprise_type: data.enterprise_type ?? '',
    employee_count: data.employee_count ?? '',
    required_modules: (data.required_modules ?? []).join('、'),
    deployment_mode: data.deployment_mode ?? '',
    license_count: data.license_count ?? '',
    version: data.version ?? '',
    questions: data.follow_up_questions ?? [],
  }
}

function loadSample() {
  selectedId.value = ''
  text.value = aiDemoProject.requirement
  resetResult()
}

function fillManually() {
  manual.value = true
  error.value = ''
  showFields()
}

function draftPath(id) {
  return '/api/channel/v1/projects/' + encodeURIComponent(id) + '/inquiry-draft'
}

async function useProject(id) {
  const attachPreparedWork = !demo && !selectedId.value && id && step.value > 1 && !draft.value
  selectedId.value = id
  if (attachPreparedWork) return
  const project = selectedProject.value
  text.value = project?.requirement ?? (demo ? aiDemoProject.requirement : '')
  resetResult()
  if (!project) return
  try {
    const saved = demo
      ? repo.getInquiryDraft(id)
      : await props.request(draftPath(id), { signal: controller.signal })
    if (!saved || saved.projectRevision !== project.revision || selectedId.value !== id || result.value) return
    const product = demo
      ? (await repo.listProducts()).find(item => item.id === saved.productId)
      : await props.request('/api/products/' + encodeURIComponent(saved.productId), { signal: controller.signal })
    if (selectedId.value !== id || result.value || parsing.value || text.value !== (project.requirement ?? '') || !product) return
    showFields(saved.requirement)
    candidate.value = { product }
    draft.value = saved
    toStep(3)
  } catch (cause) {
    if (selectedId.value === id && cause.name !== 'AbortError' && cause.status !== 404) {
      projectError.value = '暂时无法读取已保存的草稿，可以继续整理当前需求。'
    }
  }
}

async function loadProjects(initialId = '') {
  loadingProjects.value = true
  projectError.value = ''
  try {
    projects.value = (await repo.list({ signal: controller.signal })).filter(project => project.actions.includes('submitInquiry'))
    if (initialId && projects.value.some(project => project.id === initialId)) await useProject(initialId)
    else if (initialId) projectError.value = '该项目当前不可询价，请选择其他待询价项目。'
  } catch (cause) {
    if (cause.name !== 'AbortError') projectError.value = cause.message
  } finally {
    loadingProjects.value = false
  }
}
onMounted(() => loadProjects(props.initialProject))

async function parse() {
  const input = text.value.trim()
  if (!input || [...input].length > 2000) {
    error.value = '请填写 1–2000 字的需求。'
    return
  }
  resetResult()
  parsing.value = true
  try {
    const data = demo ? parseDemoRequirement(input) : await props.request('/api/ai/parse-requirement', {
      method: 'POST', body: { text: input }, signal: controller.signal, timeout: 32000,
    })
    showFields(data)
  } catch (cause) {
    if (cause.name !== 'AbortError') error.value = cause.message
  } finally {
    parsing.value = false
  }
}

function fields() {
  const value = result.value
  return {
    enterprise_type: value.enterprise_type.trim(),
    employee_count: value.employee_count || null,
    required_modules: [...new Set(value.required_modules.split(/[、，,；;\n]/).map(item => item.trim()).filter(Boolean))],
    deployment_mode: value.deployment_mode.trim(),
    license_count: value.license_count || null,
    version: value.version.trim(),
  }
}
const summary = computed(() => draft.value?.summary ?? (result.value ? inquirySummary(fields()) : ''))

async function match() {
  const body = fields()
  if (!Object.values(body).some(value => Array.isArray(value) ? value.length : value)) {
    error.value = '请至少填写一个已知需求字段。'
    return
  }
  if (body.required_modules.length > 30 || body.required_modules.some(module => [...module].length > 70)) {
    error.value = '模块最多 30 项，每项不超过 70 字。'
    return
  }
  matching.value = true
  matches.value = null
  candidate.value = null
  draft.value = null
  error.value = ''
  try {
    matches.value = demo
      ? matchDemoProducts(await repo.listProducts(), body)
      : (await props.request('/api/ai/match-products', { method: 'POST', body, signal: controller.signal })).items
    toStep(2)
  } catch (cause) {
    if (cause.name !== 'AbortError') error.value = cause.message
  } finally {
    matching.value = false
  }
}

function chooseCandidate(item) {
  candidate.value = item
  draft.value = null
  toStep(3)
}

async function saveDraft() {
  if (saving.value || draft.value || !candidate.value || (!demo && !selectedProject.value)) return
  const id = selectedId.value
  const body = {
    revision: selectedProject.value?.revision,
    confirmed: true,
    productId: candidate.value.product.id,
    requirement: fields(),
  }
  const signature = JSON.stringify({ id, body })
  saving.value = true
  error.value = ''
  try {
    if (demo) {
      draft.value = await repo.saveInquiryDraft(id, body, { ...aiDemoProject, requirement: text.value.trim() || summary.value })
    } else {
      if (draftAttempt?.signature !== signature) draftAttempt = { signature, requestId: crypto.randomUUID() }
      draft.value = await props.request(draftPath(id), {
        method: 'POST', body: { ...body, requestId: draftAttempt.requestId }, signal: controller.signal,
      })
    }
    if (!id) {
      await loadProjects()
      selectedId.value = draft.value.projectId
    }
  } catch (cause) {
    if (cause.name !== 'AbortError') error.value = cause.message
    if (cause.status === 409) {
      await loadProjects()
      selectedId.value = ''
      toStep(1)
      error.value = '项目已更新。请重新关联项目，核对需求和产品后再保存。'
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="ai-workspace" aria-labelledby="ai-workspace-title">
    <header class="ai-hero">
      <div class="ai-hero-copy">
        <p class="ai-eyebrow"><UiIcon name="ai" :size="16" /> 项目协作</p>
        <h1 id="ai-workspace-title">AI 工作台</h1>
        <p>把客户需求整理成有产品依据的询价草稿。</p>
      </div>
      <div class="ai-hero-mark" aria-hidden="true"><UiIcon name="ai" :size="88" /></div>
      <UiBadge v-if="demo" class="ai-mode" tone="info">本地样例</UiBadge>
    </header>

    <UiCard class="ai-process">
      <div class="ai-process-head">
        <UiField label="关联项目" v-slot="{ control }">
          <select v-bind="control" :value="selectedId" :disabled="busy" @change="useProject($event.target.value)">
            <option value="">{{ demo ? '海川精工 · ERP 演示案例' : '选择待询价项目，或先整理需求' }}</option>
            <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.title }} · {{ project.id }}</option>
          </select>
        </UiField>
        <UiButton v-if="demo" compact variant="text" icon="request" :disabled="busy" @click="loadSample">载入演示案例</UiButton>
        <UiButton v-else-if="selectedProject" compact variant="text" end-icon="open-detail" @click="emit('open-project', selectedId)">打开关联项目</UiButton>
      </div>
      <p v-if="projectError" class="ai-project-error" role="status">{{ projectError }} <button type="button" :disabled="loadingProjects" @click="loadProjects()">刷新项目</button></p>
      <ol ref="stepBar" class="ai-steps" aria-label="询价准备步骤">
        <li v-for="(label, index) in steps" :key="label">
          <button type="button" :aria-current="step === index + 1 ? 'step' : undefined" :disabled="(busy && step !== index + 1) || index + 1 > step || (index === 1 && !matches)"
            @click="toStep(index + 1)">
            <span class="ai-step-number"><UiIcon v-if="step > index + 1" name="check" :size="15" /><template v-else>{{ index + 1 }}</template></span>
            {{ label }}
          </button>
        </li>
      </ol>

      <div v-if="step === 1" class="ai-review">
        <section class="ai-original" aria-labelledby="ai-original-title">
          <h2 id="ai-original-title"><UiIcon name="request" />客户需求原文</h2>
          <form class="ai-stack" @submit.prevent="parse">
            <UiField label="需求描述" :hint="selectedProject ? '已带入项目需求，可删去与询价无关的内容。' : '输入需求时请使用脱敏内容。'" v-slot="{ control }">
              <textarea v-bind="control" v-model="text" :disabled="busy" maxlength="2000" rows="6"
                placeholder="例如：需要财务和库存模块，希望云部署，授权人数待确认。" @input="resetResult" />
            </UiField>
            <div class="ai-actions">
              <UiButton type="submit" variant="primary" icon="ai" :loading="parsing" :disabled="busy || !text.trim()">{{ result ? '重新整理' : '用 AI 整理' }}</UiButton>
              <UiButton v-if="!result" :disabled="busy" @click="fillManually">手工填写</UiButton>
            </div>
          </form>
        </section>
        <section class="ai-fields-section" aria-labelledby="ai-fields-title">
          <div class="ai-section-heading">
            <h2 id="ai-fields-title">需求字段</h2>
            <UiBadge v-if="result" :tone="manual ? 'neutral' : 'info'">{{ manual ? '手工录入' : demo ? '样例提取结果' : 'AI 已提取' }}</UiBadge>
          </div>
          <UiState v-if="parsing" state="loading" title="正在整理需求" />
          <UiState v-else-if="!result" title="等待整理需求" description="核对提取的字段后，再匹配产品。" />
          <form v-else class="ai-stack" @submit.prevent="match">
            <div class="ai-fields">
              <UiField v-for="field in fieldSpecs" :key="field.key" :label="field.label" :hint="field.hint" v-slot="{ control }">
                <input v-if="field.numeric" v-bind="control" v-model="result[field.key]" :disabled="busy" type="number" min="1" max="1000000" placeholder="待确认">
                <input v-else v-bind="control" v-model="result[field.key]" :disabled="busy" :maxlength="field.max" placeholder="待确认">
              </UiField>
            </div>
            <aside v-if="result.questions.length" class="ai-follow-up">
              <strong><UiIcon name="info" :size="16" /> 建议核实</strong>
              <ul><li v-for="question in result.questions" :key="question">{{ question }}</li></ul>
            </aside>
            <UiButton type="submit" variant="primary" end-icon="forward" :disabled="busy" :loading="matching">确认需求并匹配</UiButton>
          </form>
        </section>
      </div>

      <section v-else-if="step === 2" class="ai-stack" aria-labelledby="ai-matches-title">
        <div class="ai-section-heading"><h2 id="ai-matches-title">选择询价产品</h2><UiBadge tone="info">{{ eligible.length }} 款可选</UiBadge></div>
        <p class="ai-summary">{{ summary }}</p>
        <UiState v-if="!eligible.length" title="暂无符合已知条件的产品" description="可返回核对需求，或在项目中手工发起询价。" />
        <div v-if="eligible.length || otherMatches.length" class="ai-products">
          <article v-for="item in [...eligible, ...otherMatches]" :key="item.product.id" class="ai-product">
            <div class="ai-section-heading">
              <h3>{{ item.product.name }} {{ item.product.version }}</h3>
              <UiBadge :tone="fit[item.status][1]">{{ fit[item.status][0] }}</UiBadge>
            </div>
            <ul class="ai-checks">
              <li v-for="(check, index) in item.checks" :key="index">
                <span :data-check="check.status">{{ checks[check.status] }}</span>
                <span>{{ check.value ? check.value + ' · ' : '' }}{{ check.detail }}</span>
              </li>
            </ul>
            <details v-if="item.sources.length">
              <summary>查看厂商资料</summary>
              <div v-for="source in item.sources" :key="source.title + source.reference" class="ai-source">
                <strong>{{ source.title }}</strong> · {{ source.reference }}<p>{{ source.excerpt }}</p>
              </div>
            </details>
            <UiButton v-if="item.status !== 'mismatch'" compact variant="primary" end-icon="forward" @click="chooseCandidate(item)">用此产品预览草稿</UiButton>
          </article>
        </div>
        <UiButton compact icon="back" @click="toStep(1)">返回核对需求</UiButton>
      </section>

      <section v-else class="ai-stack ai-preview" aria-labelledby="ai-preview-title">
        <div class="ai-section-heading">
          <h2 id="ai-preview-title">询价草稿</h2>
          <UiBadge :tone="draft ? 'success' : 'warning'" :role="draft ? 'status' : undefined">{{ draft ? '已保存 · 未提交' : '待保存' }}</UiBadge>
        </div>
        <dl class="ai-draft">
          <div v-if="selectedProject || demo"><dt>关联项目</dt><dd>{{ (selectedProject || aiDemoProject).title }}</dd></div>
          <div v-if="previewProduct"><dt>询价产品</dt><dd>{{ previewProduct.name }} {{ previewProduct.version }}</dd></div>
          <div v-if="previewProduct?.manufacturerName"><dt>厂商</dt><dd>{{ previewProduct.manufacturerName }}</dd></div>
          <div><dt>询价摘要</dt><dd>{{ summary }}</dd></div>
        </dl>
        <p v-if="draft" class="ai-summary">打开项目后，可在「提交厂商询价」中核对草稿并发送。</p>
        <p v-else-if="!demo && !selectedProject" class="ai-summary">请在上方关联待询价项目后保存。</p>
        <p v-else class="ai-summary">{{ demo && !selectedProject ? '保存时会建立本地演示项目。' : '草稿仅当前经办人可见。' }}正式询价由经办人在项目中提交。</p>
        <div class="ai-actions">
          <UiButton v-if="matches && !draft" compact icon="back" :disabled="busy" @click="toStep(2)">重新选产品</UiButton>
          <UiButton v-if="draft" variant="primary" end-icon="open-detail" @click="emit('open-project', draft.projectId)">打开项目</UiButton>
          <UiButton v-else variant="primary" icon="check" :disabled="busy || (!demo && !selectedProject)" :loading="saving" @click="saveDraft">保存询价草稿</UiButton>
          <UiButton v-if="!demo && !draft && !projects.length && !loadingProjects" @click="emit('open-projects')">前往项目协作</UiButton>
        </div>
      </section>
      <p v-if="error" class="form-error ai-error" role="alert">{{ error }}</p>
    </UiCard>
  </section>
</template>

<style scoped src="./ai-workspace.css"></style>
