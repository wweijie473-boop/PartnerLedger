<script setup>
defineProps({
  as: { type: String, default: 'section' },
  title: { type: String, default: '' },
  description: { type: String, default: '' },
  heading: { type: String, default: 'h2' },
  flush: Boolean,
  muted: Boolean,
})
</script>

<template>
  <component :is="as" class="ui-card" :class="{ 'is-flush': flush, 'is-muted': muted }">
    <header v-if="title || description || $slots.actions" class="ui-card-header">
      <div>
        <component :is="heading" v-if="title" class="ui-card-title">{{ title }}</component>
        <p v-if="description" class="ui-card-description">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="ui-card-actions"><slot name="actions" /></div>
    </header>
    <div v-if="$slots.default" class="ui-card-body"><slot /></div>
    <footer v-if="$slots.footer" class="ui-card-footer"><slot name="footer" /></footer>
  </component>
</template>

<style scoped>
.ui-card {
  --card-space: var(--ui-card-padding, clamp(16px, 2.1vw, 24px));
  min-width: 0;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-lg);
  background: var(--ui-surface);
  box-shadow: var(--ui-shadow-card);
}
.ui-card.is-muted { background: var(--ui-surface-muted); }
.ui-card-header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--ui-space-3); padding: var(--card-space) var(--card-space) 0; }
.ui-card-title { margin: 0; font-size: var(--ui-text-lg); line-height: 1.45; font-weight: var(--ui-weight-semibold); }
.ui-card-description { color: var(--ui-ink-muted); margin: var(--ui-space-1) 0 0; font-size: var(--ui-text-sm); line-height: 1.6; }
.ui-card-actions { display: flex; align-items: center; gap: var(--ui-space-2); }
.ui-card-body { padding: var(--card-space); }
.ui-card-header + .ui-card-body { padding-top: var(--ui-space-4); }
.ui-card-footer { border-top: 1px solid var(--ui-border); padding: var(--ui-space-3) var(--card-space); }
.ui-card.is-flush .ui-card-header { padding-bottom: var(--ui-space-4); }
.ui-card.is-flush .ui-card-body { padding: 0; border-radius: 0 0 var(--ui-radius-lg) var(--ui-radius-lg); overflow: hidden; }
</style>
