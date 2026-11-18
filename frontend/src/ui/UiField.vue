<script setup>
import { computed, useId } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  hint: { type: String, default: '' },
  error: { type: String, default: '' },
  required: Boolean,
})

const id = useId()
const control = computed(() => ({
  id,
  'aria-describedby': props.error ? `${id}-error` : props.hint ? `${id}-hint` : undefined,
  'aria-invalid': props.error ? 'true' : undefined,
  'aria-required': props.required ? 'true' : undefined,
}))
</script>

<template>
  <div class="ui-field">
    <label :for="id">{{ label }}<span v-if="required" aria-hidden="true"> *</span></label>
    <slot :control="control" />
    <p v-if="error" :id="`${id}-error`" class="ui-field-error">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-hint`">{{ hint }}</p>
  </div>
</template>

<style scoped>
.ui-field { display: grid; gap: 6px; min-width: 0; }
.ui-field label { color: var(--ui-ink); font-size: var(--ui-text-sm); font-weight: var(--ui-weight-semibold); }
.ui-field label span, .ui-field .ui-field-error { color: var(--ui-danger-ink); }
.ui-field p { margin: 0; color: var(--ui-ink-muted); font-size: var(--ui-text-xs); }
:slotted(input), :slotted(select), :slotted(textarea) {
  width: 100%;
  min-width: 0;
  min-height: var(--ui-control-height);
  padding: 7px var(--ui-space-3);
  border: 1px solid var(--ui-border-strong);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-surface);
  color: var(--ui-ink);
  font: inherit;
  font-size: var(--ui-text-md);
}
:slotted(textarea) { resize: vertical; }
:slotted([aria-invalid='true']) { border-color: var(--ui-danger-ink); }
:slotted(:disabled) { background: var(--ui-surface-muted); color: var(--ui-ink-muted); }
</style>
