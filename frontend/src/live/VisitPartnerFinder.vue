<script setup>
import { reactive, ref, watch } from 'vue'
import { UiButton, UiCard } from '../ui/index.js'
import OpportunityFields from '../channel/OpportunityFields.vue'
import PartnerRecommendation from '../channel/PartnerRecommendation.vue'
import { suggestOpportunity } from '../channel/opportunities.js'

const props = defineProps({
  note: String,
  opportunityId: String,
  previewPartners: { type: Function, required: true },
  loadRecommendation: { type: Function, required: true },
})
const emit = defineEmits(['criteria'])
const criteria = reactive({category:'', source:'', target:''})
const result = ref(null)
const loading = ref(false)
const failed = ref(false)
const retry = ref(0)
const criteriaOpen = ref(false)

watch(() => props.note, note => {
  const {category, source, target} = suggestOpportunity(note)
  Object.assign(criteria,{category, source, target})
}, {immediate:true})
watch(criteria, () => emit('criteria',{...criteria}), {immediate:true})

watch([() => props.opportunityId, () => criteria.category, () => criteria.source, () => criteria.target, retry],
  ([id, category, source, target], _previous, onCleanup) => {
    const controller = new AbortController()
    result.value = null
    failed.value = false
    loading.value = true
    async function load() {
      try {
        const recommendation = id
          ? await props.loadRecommendation(id, controller.signal)
          : await props.previewPartners({ category, source, target }, controller.signal)
        if (!controller.signal.aborted) {
          result.value = recommendation
          if (!id && recommendation.questions.length) criteriaOpen.value = true
        }
      } catch {
        if (!controller.signal.aborted) failed.value = true
      } finally {
        if (!controller.signal.aborted) loading.value = false
      }
    }
    const timer = setTimeout(load, id ? 0 : 150)
    onCleanup(() => {
      clearTimeout(timer)
      controller.abort()
    })
  }, { immediate: true })
</script>

<template>
  <UiCard class="visit-partner-finder" title="推荐团队伙伴" heading="h3">
    <p v-if="loading" role="status">正在查找团队伙伴…</p>
    <template v-else-if="failed">
      <p role="status">暂时无法读取团队伙伴。</p>
      <UiButton compact @click="retry++">重试</UiButton>
    </template>
    <p v-else-if="!result">暂时无法查看关联商机的匹配结果。</p>
    <template v-else>
      <template v-if="result.questions.length">
        <p>{{ result.questions[0] }}</p>
        <p v-if="opportunityId" class="muted">可以打开商机详情补全信息。</p>
      </template>
      <template v-else>
        <p v-if="!result.matches.length" role="status">团队暂未登记匹配的伙伴。</p>
        <PartnerRecommendation v-for="candidate in result.matches" :key="candidate.partnerId" :candidate="candidate" :brief="result.brief" />
      </template>
    </template>
    <details v-if="!opportunityId" :open="criteriaOpen" @toggle="criteriaOpen = $event.currentTarget.open">
      <summary>调整匹配条件</summary>
      <OpportunityFields :form="criteria" />
    </details>
  </UiCard>
</template>

<style scoped>
.visit-partner-finder :deep(.op-fields) { margin-block-start:10px; }
.visit-partner-finder :deep(.op-fields label) { display:grid; gap:6px; }
.visit-partner-finder :deep(.op-fields select) {
  width:100%;
  min-height:var(--ui-control-height);
  border:1px solid var(--ui-border-strong);
  border-radius:var(--ui-radius-sm);
  background:var(--ui-surface);
  color:var(--ui-ink);
  padding-inline:8px;
}
</style>
