<script setup>
import { UiIcon } from './ui/index.js'

defineProps({ pages: { type: Array, required: true } })
defineEmits(['navigate'])
</script>

<template>
  <section v-if="pages.length" class="workspace-section" aria-labelledby="workspace-shortcuts">
    <header class="workspace-section-heading"><h2 id="workspace-shortcuts">常用工作</h2></header>
    <ul class="workspace-shortcuts">
      <li v-for="page in pages" :key="page.id">
        <button type="button" @click="$emit('navigate', page.id)">
          <UiIcon :name="page.icon" :size="16" />
          <span>{{ page.label }}</span>
          <UiIcon name="chevron-right" :size="14" />
        </button>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.workspace-shortcuts {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(150px, 100%), 1fr));
  gap: 0 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.workspace-shortcuts button {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 38px;
  height: 100%;
  padding: 6px;
  border: 0;
  border-top: 1px solid var(--ui-border);
  background: transparent;
  color: var(--ui-link);
  text-align: left;
}
.workspace-shortcuts button > span { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.workspace-shortcuts button:hover, .workspace-shortcuts button:focus-visible { background: var(--ui-accent-bg); }
@media (max-width: 540px) { .workspace-shortcuts button { min-height: 44px; } }
</style>
