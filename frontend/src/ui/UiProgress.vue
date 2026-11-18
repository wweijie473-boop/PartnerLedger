<script setup>
defineProps({
  value: { type: Number, required: true },
  label: { type: String, required: true },
})
</script>

<template>
  <progress class="ui-progress" :value="value" max="100" :aria-label="label">{{ value }}%</progress>
</template>

<style scoped>
.ui-progress {
  appearance: none;
  display: block;
  inline-size: 100%;
  block-size: 7px;
  overflow: hidden;
  border: 0;
  border-radius: var(--ui-radius-sm);
  background: var(--ui-neutral-bg);
  color: var(--ui-accent);
}
.ui-progress::-webkit-progress-bar { background: var(--ui-neutral-bg); }
.ui-progress::-webkit-progress-value { border-radius: var(--ui-radius-sm); background: var(--ui-accent); }
.ui-progress::-moz-progress-bar { border-radius: var(--ui-radius-sm); background: var(--ui-accent); }
</style>
