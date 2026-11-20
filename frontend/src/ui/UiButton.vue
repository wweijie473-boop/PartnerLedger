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
  border-radius: var(--ui-radius-sm);
  background: var(--ui-surface);
  color: var(--ui-ink);
  font-size: var(--ui-text-sm);
  font-weight: var(--ui-weight-semibold);
  white-space: nowrap;
  transition: background-color var(--ui-motion-fast) var(--ui-ease-out),
    border-color var(--ui-motion-fast) var(--ui-ease-out),
    translate var(--ui-motion-fast) var(--ui-ease-out);
}
.ui-button[data-variant='primary'] { background: var(--ui-accent); border-color: var(--ui-accent); color: var(--ui-on-accent); }
.ui-button[data-variant='text'] { background: transparent; border-color: transparent; color: var(--ui-link); }
.ui-button[data-variant='danger'] { border-color: var(--ui-danger-ink); color: var(--ui-danger-ink); }
.ui-button.is-compact { padding: 4px 10px; min-height: 32px; font-size: var(--ui-text-sm); }
.ui-button:disabled { cursor: default; opacity: .55; }
.ui-button-spinner { animation: rotate .85s linear infinite; }
@keyframes rotate { to { transform: rotate(360deg); } }
@media (hover: hover) and (pointer: fine) {
  .ui-button:hover:not(:disabled) { background: var(--ui-surface-hover); }
  .ui-button[data-variant='primary']:hover:not(:disabled) { background: var(--ui-accent-hover); border-color: var(--ui-accent-hover); }
  .ui-button[data-variant='danger']:hover:not(:disabled) { background: var(--ui-danger-bg); }
}
@media (prefers-reduced-motion: no-preference) {
  .ui-button:active:not(:disabled) { translate: 0 1px; }
}
@media (max-width: 540px) { .ui-button, .ui-button.is-compact { min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .ui-button { transition: none; } .ui-button-spinner { animation: none; } }
</style>
