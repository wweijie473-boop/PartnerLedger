<script setup>
import UiIcon from './UiIcon.vue'
import UiIconTile from './UiIconTile.vue'

defineProps({
  label: { type: String, required: true },
  value: { type: [Number, String], required: true },
  description: { type: String, default: '' },
  icon: { type: String, default: '' },
  tone: { type: String, default: 'neutral' },
  interactive: Boolean,
})
</script>

<template>
  <component :is="interactive ? 'button' : 'article'" :type="interactive ? 'button' : undefined" class="ui-metric-card">
    <UiIconTile v-if="icon" :name="icon" :tone="tone" />
    <UiIcon v-if="interactive" class="ui-metric-arrow" name="open-detail" :size="16" />
    <span class="ui-metric-label">{{ label }}</span>
    <strong>{{ value }}</strong>
    <small v-if="description">{{ description }}</small>
  </component>
</template>

<style scoped>
.ui-metric-card {
  position: relative;
  min-width: 0;
  display: grid;
  justify-items: start;
  align-content: start;
  gap: 5px;
  padding: clamp(16px, 2.2vw, 26px);
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  color: var(--ui-ink);
  text-align: left;
}
button.ui-metric-card:hover { border-color: var(--ui-border-strong); background: var(--ui-surface-muted); }
.ui-metric-label { margin-top: var(--ui-space-2); color: var(--ui-ink-secondary); font-size: var(--ui-text-sm); }
.ui-metric-card strong { font-size: clamp(28px, 3vw, 42px); line-height: 1.05; letter-spacing: -.045em; font-variant-numeric: tabular-nums; }
.ui-metric-card small { color: var(--ui-ink-muted); font-size: var(--ui-text-xs); }
.ui-metric-arrow { position: absolute; top: var(--ui-space-5); right: var(--ui-space-5); color: var(--ui-ink-muted); }
@media (max-width: 600px) {
  .ui-metric-card { padding: var(--ui-space-3); }
  .ui-metric-card strong { font-size: 23px; }
  .ui-metric-card small { font-size: 11px; }
  .ui-metric-arrow { display: none; }
}
</style>
