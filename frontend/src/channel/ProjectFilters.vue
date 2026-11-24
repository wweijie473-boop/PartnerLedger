<script setup>
import { projectScopeLabels } from './domain.js'

defineProps({
  scope: String,
  counts: Object,
  keyword: String,
  stage: String,
  stages: { type: Array, required: true },
})
defineEmits(['scope', 'keyword', 'stage'])
</script>

<template>
  <header class="workspace-section-heading">
    <div class="project-scopes" role="group" aria-label="项目范围">
      <button v-for="(label, key) in projectScopeLabels" :key="key" type="button"
        :aria-pressed="scope === key" @click="$emit('scope', key)">{{ label }} <span>{{ counts[key] }}</span></button>
    </div>
    <div class="project-filters">
      <input :value="keyword" type="search" placeholder="搜索项目或编号" aria-label="搜索项目" maxlength="120" @input="$emit('keyword', $event)">
      <select :value="stage" aria-label="项目状态" @change="$emit('stage', $event)">
        <option value="" :selected="!stage">全部状态</option>
        <option v-for="option in stages" :key="option.id" :value="option.id" :selected="stage === option.id">{{ option.label }}</option>
      </select>
      <slot />
    </div>
  </header>
</template>

<style scoped>
.project-filters, .project-scopes { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; font-size: 13px; }
.project-scopes { gap: 2px 4px; }
.project-scopes button {
  min-height: var(--ui-control-height);
  padding: 4px 7px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--ui-ink-secondary);
  white-space: nowrap;
}
.project-scopes button:hover { background: var(--ui-accent-bg); }
.project-scopes button[aria-pressed="true"] { border-bottom-color: var(--ui-accent); color: var(--ui-link); font-weight: 600; }
.project-scopes button span { margin-left: 2px; font-variant-numeric: tabular-nums; }
.project-filters :is(input, select) {
  min-height: var(--ui-control-height);
  min-width: 0;
  max-width: 100%;
  padding: 4px 8px;
  border: 1px solid var(--ui-border-strong);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-surface);
  color: var(--ui-ink);
}
@media (max-width: 540px) {
  .project-filters { width: 100%; }
  .project-filters input { flex: 1 1 125px; }
}
</style>
