<script setup>
import UiIcon from './UiIcon.vue'

defineProps({
  name: { type: String, required: true },
  tone: { type: String, default: 'neutral' },
  size: { type: Number, default: 18 },
})
</script>

<template>
  <span class="ui-icon-tile" :data-tone="tone"><UiIcon :name="name" :size="size" /></span>
</template>

<style scoped>
.ui-icon-tile {
  display: inline-grid;
  place-items: center;
  inline-size: var(--ui-tile-size, 36px);
  block-size: var(--ui-tile-size, 36px);
  flex: none;
  border-radius: var(--ui-radius-md);
  color: var(--ui-tone-ink);
  background: var(--ui-tone-bg);
}
</style>
