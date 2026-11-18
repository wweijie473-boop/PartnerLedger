<script setup>
import UiIcon from './UiIcon.vue'

const statusIcons = { info: 'info', success: 'check', warning: 'time', danger: 'error' }
defineProps({ tone: { type: String, default: 'neutral' }, dot: Boolean })
</script>

<template>
  <span class="ui-badge" :data-tone="tone">
    <UiIcon v-if="dot && statusIcons[tone]" :name="statusIcons[tone]" :size="13" />
    <span v-else-if="dot" class="ui-badge-dot" aria-hidden="true"></span>
    <slot />
  </span>
</template>

<style scoped>
.ui-badge {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  gap: 5px;
  padding: 3px var(--ui-space-2);
  border-radius: var(--ui-radius-pill);
  color: var(--ui-tone-ink);
  background: var(--ui-tone-bg);
  font-size: var(--ui-text-xs);
  font-weight: var(--ui-weight-medium);
  line-height: 1.4;
  white-space: nowrap;
}
.ui-badge-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
</style>
