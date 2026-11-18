<script setup>
import { computed } from 'vue'
import UiIcon from './UiIcon.vue'
import UiIconTile from './UiIconTile.vue'

const presets = {
  empty: { icon: 'request', tone: 'neutral' },
  loading: { icon: 'loading', tone: 'brand' },
  denied: { icon: 'permission', tone: 'warning' },
  error: { icon: 'error', tone: 'danger' },
}

const props = defineProps({
  state: { type: String, default: 'empty', validator: (state) => ['empty', 'loading', 'denied', 'error'].includes(state) },
  title: { type: String, required: true },
  description: { type: String, default: '' },
})
const preset = computed(() => presets[props.state] ?? presets.empty)
</script>

<template>
  <div class="ui-state" :role="state === 'error' ? 'alert' : 'status'">
    <UiIcon v-if="state === 'loading'" class="ui-state-spinner" name="loading" :size="24" />
    <UiIconTile v-else :name="preset.icon" :tone="preset.tone" :size="21" />
    <strong>{{ title }}</strong>
    <p v-if="description">{{ description }}</p>
    <slot />
  </div>
</template>

<style scoped>
.ui-state {
  display: grid;
  min-height: var(--ui-state-min-height, 172px);
  justify-items: center;
  align-content: center;
  gap: var(--ui-space-2);
  padding: var(--ui-space-5);
  color: var(--ui-ink);
  text-align: center;
}
.ui-state strong { font-size: var(--ui-text-md); font-weight: var(--ui-weight-semibold); }
.ui-state p { margin: 0; color: var(--ui-ink-muted); font-size: var(--ui-text-sm); }
.ui-state-spinner { color: var(--ui-accent); margin-block: 6px; animation: rotate .85s linear infinite; }
@keyframes rotate { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .ui-state-spinner { animation: none; } }
</style>
