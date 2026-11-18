<script setup>
import { onUnmounted, ref, watch } from 'vue'
import { UiBadge, UiButton, UiCard, UiField, UiState } from '../ui/index.js'

const props = defineProps({ request: { type: Function, required: true } })
const text = ref('')
const result = ref(null)
const parsing = ref(false)
const confirmed = ref(false)
const error = ref('')
const resultError = ref('')
const controller = new AbortController()
const labels = {
  enterprise_type: '企业类型', employee_count: '员工人数', required_modules: '所需模块',
  deployment_mode: '部署方式', license_count: '授权用户数',
}

onUnmounted(() => controller.abort())

watch(text, () => {
  result.value = null
  confirmed.value = false
  error.value = ''
  resultError.value = ''
})

async function parse() {
  if (parsing.value) return
  const input = text.value.trim()
  if (!input || [...input].length > 2000) {
    error.value = '请填写 1–2000 字的需求。'
    return
  }
  parsing.value = true
  confirmed.value = false
  result.value = null
  error.value = ''
  resultError.value = ''
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
      missing: data.missing_fields ?? [],
      questions: data.follow_up_questions ?? [],
    }
  } catch (cause) {
    if (cause.name !== 'AbortError') error.value = cause.message
  } finally {
    parsing.value = false
  }
}

function confirmResult() {
  const validCount = (count) => count === '' || (Number.isSafeInteger(count) && count > 0)
  if (!validCount(result.value.employeeCount) || !validCount(result.value.licenseCount)) {
    resultError.value = '人数须为正整数，未知时可以留空。'
    return
  }
  resultError.value = ''
  confirmed.value = true
}
</script>

<template>
  <section class="page-heading"><div><h1>AI 需求解析</h1><p class="muted">把自然语言描述整理为可核对的 ERP 需求字段。</p></div></section>

  <div class="requirement-layout">
    <UiCard title="原始需求">
      <form class="form-grid" @submit.prevent="parse">
        <UiField label="需求描述" hint="使用脱敏示例，最多 2000 字。" required v-slot="{ control }">
          <textarea v-bind="control" v-model="text" :disabled="parsing" rows="7" maxlength="2000" required
            placeholder="例如：一家 120 人的制造企业需要财务和库存模块，希望私有化部署。" />
        </UiField>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
        <UiButton type="submit" variant="primary" icon="request" :loading="parsing">解析需求</UiButton>
      </form>
    </UiCard>

    <UiCard title="解析结果">
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
        <p v-if="confirmed" class="muted requirement-note" role="status">已确认本次结果。结果仅保留在当前页面。</p>
        <UiButton v-if="confirmed" @click="confirmed = false">重新编辑</UiButton>
        <UiButton v-else type="submit" variant="primary" icon="check">确认解析结果</UiButton>
      </form>
    </UiCard>
  </div>
</template>

<style scoped>
.requirement-layout { display: grid; grid-template-columns: minmax(0, .9fr) minmax(0, 1.1fr); gap: var(--ui-space-4); align-items: start; }
.requirement-follow-up { padding: var(--ui-space-3); border-radius: var(--ui-radius-sm); background: var(--ui-surface-muted); font-size: var(--ui-text-sm); }
.requirement-follow-up ul { margin: var(--ui-space-2) 0 0; padding-inline-start: 20px; }
.requirement-note { font-size: var(--ui-text-sm); }
@media (max-width: 850px) { .requirement-layout { grid-template-columns: minmax(0, 1fr); } }
</style>
