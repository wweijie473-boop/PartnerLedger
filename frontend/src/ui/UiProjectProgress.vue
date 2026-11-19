<script setup>
import { computed } from 'vue'
import UiIcon from './UiIcon.vue'

const props = defineProps({ role: String, stage: String, label: String })
const schemes = {
  dealer: {
    steps: ['核对需求', '厂商询价', '客户方案', '交付与验收'],
    groups: [['requested', 'customer_info'], ['inquired', 'vendor_info', 'channel_revision', 'channel_quoted'], ['sales_quoted', 'sales_revision', 'confirmed'], ['ordered', 'supplied', 'acceptance', 'rectification', 'completed']],
  },
  customer: {
    steps: ['提交需求', '确认方案', '跟踪履约', '项目验收'],
    groups: [['requested', 'customer_info', 'processing'], ['sales_quoted', 'sales_revision', 'confirmed'], ['ordered', 'supplied', 'rectification'], ['acceptance', 'completed']],
  },
  vendor: {
    steps: ['处理询价', '渠道报价', '授权与供货'],
    groups: [['inquired', 'vendor_info'], ['channel_quoted', 'channel_revision', 'channel_wait'], ['ordered', 'shipped', 'supplied', 'completed']],
  },
}
const scheme = computed(() => schemes[props.role])
const index = computed(() => scheme.value?.groups.findIndex(group => group.includes(props.stage)) ?? -1)
</script>

<template>
  <ol v-if="scheme && index >= 0" class="project-progress" aria-label="项目协作进度">
    <li v-for="(step, position) in scheme.steps" :key="step" :class="{ done: position < index || stage === 'completed', current: position === index }" :aria-current="position === index ? 'step' : undefined">
      <span class="project-progress-mark"><UiIcon v-if="position < index || stage === 'completed'" name="check" :size="14" /><span v-else></span></span>
      <span>{{ step }}<small v-if="position === index">{{ label }}</small></span>
    </li>
  </ol>
</template>

<style scoped>
.project-progress { display: grid; grid-auto-flow: column; grid-auto-columns: 1fr; list-style: none; padding: 18px 24px; margin: 0 0 20px; border: 1px solid var(--ui-border); border-radius: 10px; background: var(--ui-surface); }
.project-progress li { position: relative; display: flex; align-items: start; gap: 10px; color: var(--ui-ink-muted); font-size: 14px; }
.project-progress li:not(:last-child)::after { content: ''; position: absolute; top: 10px; left: 28px; right: 12px; height: 1px; background: var(--ui-border); }
.project-progress li > span:last-child { position: relative; z-index: 1; background: var(--ui-surface); padding-right: 10px; }
.project-progress-mark { position: relative; z-index: 2; display: grid; place-items: center; width: 22px; height: 22px; border: 1px solid var(--ui-border-strong); border-radius: 50%; background: var(--ui-surface); flex-shrink: 0; }
.project-progress-mark > span { width: 5px; height: 5px; border-radius: 50%; background: var(--ui-border-strong); }
.current { font-weight: 600; color: var(--ui-link) !important; }
.current .project-progress-mark { border-color: var(--ui-accent); background: var(--ui-accent-bg); }
.current .project-progress-mark > span { background: var(--ui-accent); }
.done .project-progress-mark { border-color: var(--ui-success-ink); background: var(--ui-success-bg); color: var(--ui-success-ink); }
small { display: block; font-weight: 400; font-size: 12px; padding-top: 3px; color: var(--ui-ink-muted); }
@media(max-width:540px) { .project-progress { padding: 16px; grid-auto-flow: row; grid-template-columns: 1fr 1fr; gap: 16px; } .project-progress li::after { display: none; } }
</style>
