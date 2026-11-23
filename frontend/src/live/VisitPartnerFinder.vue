<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { UiButton, UiCard } from '../ui/index.js'
import OpportunityFields from '../channel/OpportunityFields.vue'
import PartnerRecommendation from '../channel/PartnerRecommendation.vue'
import { previewPartnerMatches, suggestOpportunity } from '../channel/opportunities.js'

const props = defineProps({ note:String, opportunityId:String, loadDesk: {type:Function, required:true} })
const emit = defineEmits(['criteria'])
const criteria = reactive({category:'', source:'', target:''})
const desk = ref(null)
const loading = ref(false)
const failed = ref(false)
const retry = ref(0)

watch(() => props.note, note => {
  const {category, source, target} = suggestOpportunity(note)
  Object.assign(criteria,{category, source, target})
}, {immediate:true})
watch(criteria, () => emit('criteria',{...criteria}), {immediate:true})

watch([() => props.opportunityId, retry], async (_value, _oldValue, onCleanup) => {
  let current = true
  onCleanup(() => { current = false })
  desk.value = null
  failed.value = false
  loading.value = true
  try {
    const result = await props.loadDesk()
    if (current) desk.value = result
  } catch {
    if (current) failed.value = true
  } finally {
    if (current) loading.value = false
  }
}, {immediate:true})

const result = computed(() => {
  if (!desk.value) return null
  if (props.opportunityId) return desk.value.items.find(item => item.id === props.opportunityId)
  return previewPartnerMatches(criteria,desk.value.partners)
})
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
        <OpportunityFields v-if="!opportunityId" :form="criteria" />
        <p v-else class="muted">可以打开商机详情补全信息。</p>
      </template>
      <template v-else>
        <p v-if="!result.matches.length" role="status">团队暂未登记匹配的伙伴。</p>
        <PartnerRecommendation v-for="candidate in result.matches" :key="candidate.partnerId" :candidate="candidate" :brief="result.brief" />
        <details v-if="!opportunityId">
          <summary>调整匹配条件</summary>
          <OpportunityFields :form="criteria" />
        </details>
      </template>
    </template>
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
