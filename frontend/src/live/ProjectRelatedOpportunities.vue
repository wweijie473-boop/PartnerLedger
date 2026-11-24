<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { opportunityStages } from '../channel/opportunities.js'
import { UiButton, UiCard, UiState } from '../ui/index.js'

const props = defineProps({
  projectId: { type: String, required: true },
  repository: { type: Object, required: true },
})
const emit = defineEmits(['open'])
const rows = ref([])
const next = ref('')
const loading = ref(false)
const error = ref('')
const controller = new AbortController()

async function load() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const page = await props.repository.opportunities(props.projectId, { before: next.value, signal: controller.signal })
    if (controller.signal.aborted) return
    rows.value.push(...page.items)
    next.value = page.next_cursor
  } catch (cause) {
    if (!controller.signal.aborted) error.value = cause.message
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}

onMounted(load)
onUnmounted(() => controller.abort())
</script>

<template>
  <UiCard title="项目附加商机" :aria-busy="loading">
    <UiState v-if="loading && !rows.length" state="loading" title="正在读取附加商机" />
    <UiState v-else-if="!error && !rows.length" title="暂无附加商机" />
    <ul v-if="rows.length" class="related-opportunities">
      <li v-for="item in rows" :key="item.id">
        <button type="button" class="ch-record" @click="emit('open', item.id)">
          <strong>{{ item.note }}</strong>
          <small>{{ opportunityStages[item.stage] || item.stage }}</small>
        </button>
      </li>
    </ul>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <UiButton v-if="next !== null && !loading" compact variant="text" @click="load">{{ error ? '重试' : '加载更多' }}</UiButton>
  </UiCard>
</template>

<style scoped>
.related-opportunities { list-style: none; margin: 0; padding: 0; min-width: 0; overflow-wrap: anywhere; }
.related-opportunities .ch-record { text-align: left; }
</style>
