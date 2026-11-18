<script setup>
import UiIcon from './UiIcon.vue'

defineProps({
  variant: { type: String, default: 'secondary', validator: (value) => ['primary', 'secondary', 'text', 'danger'].includes(value) },
  type: { type: String, default: 'button' },
  icon: { type: String, default: '' },
  endIcon: { type: String, default: '' },
  compact: Boolean,
  loading: Boolean,
  disabled: Boolean,
})
</script>

<template>
  <button class="ui-button" :type="type" :data-variant="variant" :class="{ 'is-compact': compact }"
    :disabled="disabled || loading" :aria-busy="loading || undefined">
    <UiIcon v-if="loading" class="ui-button-spinner" name="loading" :size="16" />
    <UiIcon v-else-if="icon" :name="icon" :size="16" />
    <slot />
    <UiIcon v-if="endIcon" :name="endIcon" :size="15" />
  </button>
</template>

<style scoped>
.ui-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  width: fit-content;
  min-height: var(--ui-control-height);
  padding: 7px 16px;
  border: 1px solid var(--ui-border-strong);
  border-radius: var(--ui-radius-pill);
  background: var(--ui-surface);
  color: var(--ui-ink);
  font-size: var(--ui-text-sm);
  font-weight: var(--ui-weight-semibold);
  white-space: nowrap;
  transition: background-color .16s ease, border-color .16s ease;
}
.ui-button:hover:not(:disabled) { background: var(--ui-surface-hover); }
.ui-button[data-variant='primary'] { background: var(--ui-accent); border-color: var(--ui-accent); color: var(--ui-on-accent); }
.ui-button[data-variant='primary']:hover:not(:disabled) { background: var(--ui-accent-hover); border-color: var(--ui-accent-hover); }
.ui-button[data-variant='text'] { background: transparent; border-color: transparent; color: var(--ui-accent); }
.ui-button[data-variant='danger'] { border-color: var(--ui-danger-ink); color: var(--ui-danger-ink); }
.ui-button[data-variant='danger']:hover:not(:disabled) { background: var(--ui-danger-bg); }
.ui-button.is-compact { padding: 4px 9px; min-height: 30px; font-size: var(--ui-text-xs); }
.ui-button:disabled { cursor: default; opacity: .55; }
.ui-button-spinner { animation: rotate .85s linear infinite; }
@keyframes rotate { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .ui-button { transition: none; } .ui-button-spinner { animation: none; } }
</style>
