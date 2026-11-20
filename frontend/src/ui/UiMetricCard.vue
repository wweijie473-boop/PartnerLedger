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
  box-shadow: var(--ui-shadow-card);
}
button.ui-metric-card {
  transition: border-color var(--ui-motion-fast) var(--ui-ease-out),
    background-color var(--ui-motion-fast) var(--ui-ease-out);
}
.ui-metric-label { margin-top: var(--ui-space-2); color: var(--ui-ink-secondary); font-size: var(--ui-text-sm); }
.ui-metric-card strong { font-size: 36px; line-height: 1.2; font-weight: 600; font-variant-numeric: tabular-nums; }
.ui-metric-card small { color: var(--ui-ink-muted); font-size: var(--ui-text-xs); }
.ui-metric-arrow {
  position: absolute;
  top: var(--ui-space-5);
  right: var(--ui-space-5);
  color: var(--ui-ink-muted);
  transition: color var(--ui-motion-fast) var(--ui-ease-out),
    translate var(--ui-motion-fast) var(--ui-ease-out);
}
button.ui-metric-card:active:not(:disabled) { background: var(--ui-accent-bg); }
button.ui-metric-card:focus-visible .ui-metric-arrow { color: var(--ui-link); }
@media (hover: hover) and (pointer: fine) {
  button.ui-metric-card:hover:not(:disabled) { border-color: var(--ui-border-strong); background: var(--ui-surface-muted); }
  button.ui-metric-card:hover:not(:disabled) .ui-metric-arrow { color: var(--ui-link); }
}
@media (hover: hover) and (pointer: fine) and (prefers-reduced-motion: no-preference) {
  button.ui-metric-card:hover:not(:disabled) .ui-metric-arrow { translate: 2px -2px; }
}
@media (max-width: 600px) {
  .ui-metric-card { padding: var(--ui-space-3); }
  .ui-metric-card strong { font-size: 23px; }
  .ui-metric-card small { font-size: 12px; }
  .ui-metric-arrow { display: none; }
}
</style>
