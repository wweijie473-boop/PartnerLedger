<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import PortalHeader from './PortalHeader.vue'
import { UiIcon } from './ui/index.js'

const props = defineProps({
  nav: { type: Array, required: true },
  view: { type: String, required: true },
  organization: { type: String, required: true },
  accountLabel: { type: String, required: true },
  environment: { type: String, required: true },
})
const emit = defineEmits(['navigate', 'account'])
const initial = computed(() => [...props.organization][0] ?? '渠')
const tabs = ref(null)

function revealActiveTab() {
  const nav = tabs.value
  const active = nav?.querySelector('[aria-current="page"]')
  if (!active) return
  const viewport = nav.getBoundingClientRect()
  const item = active.getBoundingClientRect()
  if (item.left < viewport.left) nav.scrollLeft += item.left - viewport.left
  else if (item.right > viewport.right) nav.scrollLeft += item.right - viewport.right
}

onMounted(revealActiveTab)
watch(() => props.view, revealActiveTab, { flush: 'post' })
</script>

<template>
  <div class="workspace-shell">
    <a href="#workspace-content" class="skip-link">跳转到主要内容</a>
    <PortalHeader>
      <template #account>
        <button type="button" class="workspace-account"
          :aria-label="'账号：' + accountLabel + '，' + organization"
          :title="accountLabel + ' · ' + organization" @click="emit('account')">
          <span class="workspace-avatar" aria-hidden="true">{{ initial }}</span>
          <span class="workspace-account-text">
            <strong>{{ accountLabel }}</strong>
            <small>{{ organization }}</small>
          </span>
          <UiIcon name="chevron-right" :size="14" />
        </button>
      </template>
      <template #secondary>
        <div class="workspace-subbar">
          <div class="pl-wrap workspace-subbar-inner" :class="{ 'with-actions': $slots.actions }">
            <nav ref="tabs" class="workspace-tabs" aria-label="工作台栏目">
              <button v-for="item in nav" :key="item.id" type="button"
                :aria-current="view === item.id ? 'page' : undefined" @click="emit('navigate', item.id)">
                <UiIcon :name="item.icon" :size="17" />
                <span>{{ item.label }}</span>
              </button>
            </nav>
            <div class="workspace-context">
              <slot name="actions" />
              <span class="workspace-environment"><UiIcon name="permission" :size="14" />{{ environment }}</span>
            </div>
          </div>
        </div>
      </template>
    </PortalHeader>
    <main id="workspace-content" class="pl-wrap workspace-content" tabindex="-1">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.workspace-shell {
  min-height: 100vh;
  background: var(--ui-canvas);
}
.workspace-account {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px 4px 4px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-md);
  background: var(--ui-surface);
  color: var(--ui-ink);
  text-align: left;
}
.workspace-account:hover { background: var(--ui-surface-hover); }
.workspace-avatar {
  display: grid;
  width: 30px;
  height: 30px;
  flex: none;
  place-items: center;
  border-radius: 50%;
  background: var(--ui-accent-bg);
  color: var(--ui-link);
  font-size: 13px;
  font-weight: 600;
}
.workspace-account-text { min-width: 0; max-width: clamp(84px, 13vw, 136px); }
.workspace-account-text > * {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.25;
}
.workspace-account-text strong { font-size: 13px; font-weight: 600; }
.workspace-account-text small { font-size: 11px; color: var(--ui-ink-muted); }
.workspace-account > svg { color: var(--ui-ink-muted); }
.workspace-subbar {
  border-top: 1px solid var(--ui-border);
  background: var(--ui-surface-muted);
}
.workspace-subbar-inner {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
}
.workspace-tabs {
  display: flex;
  align-items: stretch;
  min-width: 0;
  overflow-x: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--ui-border-strong) transparent;
}
.workspace-tabs button {
  position: relative;
  display: inline-flex;
  min-height: 46px;
  flex: none;
  align-items: center;
  gap: 8px;
  padding: 0 clamp(10px, 1.2vw, 17px);
  border: 0;
  border-radius: var(--ui-radius-sm) var(--ui-radius-sm) 0 0;
  background: transparent;
  color: var(--ui-ink-secondary);
  font-size: 14px;
  white-space: nowrap;
}
.workspace-tabs button:hover { background: var(--ui-surface-hover); color: var(--ui-link); }
.workspace-tabs button:focus-visible { outline-offset: -3px; }
.workspace-tabs button[aria-current="page"] {
  background: var(--ui-accent-bg);
  color: var(--ui-link);
  font-weight: 600;
}
.workspace-tabs button[aria-current="page"]::after {
  content: '';
  position: absolute;
  inset: auto 10px 0;
  height: 2px;
  border-radius: 2px;
  background: var(--ui-accent);
}
.workspace-context, .workspace-environment {
  display: flex;
  align-items: center;
  gap: 8px;
}
.workspace-environment {
  color: var(--ui-ink-muted);
  font-size: 12px;
  white-space: nowrap;
}
.workspace-content {
  padding-block: clamp(24px, 3vw, 40px) 32px;
  scroll-margin-top: 112px;
}
@media (max-width: 760px) {
  .workspace-account { min-height: 44px; }
  .workspace-account-text { max-width: min(125px, 29vw); }
  .workspace-subbar-inner { gap: 8px; }
  .workspace-subbar-inner.with-actions { grid-template-columns: minmax(0, 1fr); }
  .with-actions .workspace-context { justify-content: space-between; padding-bottom: 8px; }
  .workspace-content { scroll-margin-top: 16px; }
}
@media (max-width: 540px) {
  .workspace-content { width: calc(100% - 32px); }
}
</style>
