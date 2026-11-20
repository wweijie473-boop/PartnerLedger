<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import PortalHeader from './PortalHeader.vue'
import WorkspaceAccount from './WorkspaceAccount.vue'
import { useSessionAccount } from './useSessionAccount.js'
import { UiButton, UiIcon } from './ui/index.js'
import './workspace.css'

const props = defineProps({
  nav: { type: Array, required: true },
  view: { type: String, required: true },
  organization: { type: String, required: true },
  accountLabel: { type: String, required: true },
  environment: { type: String, required: true },
  demo: Boolean,
})
const emit = defineEmits(['navigate', 'account'])
const { account, signedIn } = props.demo ? useSessionAccount() : { account: null, signedIn: false }
const tabs = ref(null)
const content = ref(null)
let headingAnimation

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
onBeforeUnmount(() => headingAnimation?.cancel())
watch(() => props.view, () => {
  revealActiveTab()
  headingAnimation?.cancel()
  const heading = content.value?.querySelector('h1')
  if (!heading?.animate || !window.matchMedia('(prefers-reduced-motion: no-preference)').matches) return

  const styles = getComputedStyle(heading)
  const duration = styles.getPropertyValue('--ui-motion-standard').trim()
  headingAnimation = heading.animate(
    [{ opacity: 0.65, translate: '0 4px' }, { opacity: 1, translate: '0 0' }],
    {
      duration: Number.parseFloat(duration) * (duration.endsWith('ms') ? 1 : 1000) || 240,
      easing: styles.getPropertyValue('--ui-ease-out').trim() || 'ease-out',
    },
  )
}, { flush: 'post' })
</script>

<template>
  <div class="workspace-shell">
    <a href="#workspace-content" class="skip-link">跳转到主要内容</a>
    <PortalHeader :account="account" :signed-in="signedIn">
      <template v-if="!demo || !signedIn" #account>
        <WorkspaceAccount :label="accountLabel" :organization="organization" @click="emit('account')" />
      </template>
      <template #secondary>
        <div class="workspace-subbar">
          <div class="pl-wrap workspace-subbar-inner" :class="{ 'with-actions': $slots.actions || (demo && signedIn) }">
            <nav ref="tabs" class="workspace-tabs" aria-label="工作台栏目">
              <button v-for="item in nav" :key="item.id" type="button"
                :aria-current="view === item.id ? 'page' : undefined" @click="emit('navigate', item.id)">
                <UiIcon :name="item.icon" :size="16" />
                <span>{{ item.label }}</span>
              </button>
            </nav>
            <div class="workspace-context">
              <slot name="actions" />
              <UiButton v-if="demo && signedIn" variant="text" compact @click="emit('account')">切换演示身份</UiButton>
              <span class="workspace-environment"><UiIcon name="permission" :size="14" />{{ environment }}</span>
            </div>
          </div>
        </div>
      </template>
    </PortalHeader>
    <main id="workspace-content" ref="content" class="pl-wrap workspace-content" tabindex="-1">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.workspace-shell {
  min-height: 100vh;
  background: var(--ui-surface);
}
.workspace-subbar {
  border-top: 1px solid var(--ui-border);
  background: var(--ui-surface);
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
  min-height: 40px;
  flex: none;
  align-items: center;
  gap: 6px;
  padding: 0 clamp(9px, 1vw, 14px);
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--ui-ink-secondary);
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  transition: background-color var(--ui-motion-fast) var(--ui-ease-out),
    color var(--ui-motion-fast) var(--ui-ease-out);
}
.workspace-tabs button:active { background: var(--ui-accent-bg); color: var(--ui-link); }
.workspace-tabs button:focus-visible { outline-offset: -3px; }
.workspace-tabs button[aria-current="page"] { color: var(--ui-link); }
.workspace-tabs button::after {
  content: '';
  position: absolute;
  inset: auto 10px 0;
  height: 2px;
  border-radius: 2px;
  background: var(--ui-accent);
  pointer-events: none;
  opacity: 0;
  scale: 0.4 1;
  transition: opacity var(--ui-motion-fast) var(--ui-ease-out),
    scale var(--ui-motion-standard) var(--ui-ease-out);
}
.workspace-tabs button[aria-current="page"]::after { opacity: 1; scale: 1; }
@media (hover: hover) and (pointer: fine) {
  .workspace-tabs button:hover { background: var(--ui-surface-hover); color: var(--ui-link); }
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
  padding-block: clamp(18px, 2vw, 28px) 24px;
  scroll-margin-top: 108px;
}
@media (max-width: 760px) {
  .workspace-subbar-inner { gap: 8px; }
  .workspace-subbar-inner.with-actions { grid-template-columns: minmax(0, 1fr); }
  .with-actions .workspace-context { flex-wrap: wrap; justify-content: space-between; padding-bottom: 8px; }
  .workspace-content { scroll-margin-top: 16px; }
}
@media (max-width: 540px) {
  .workspace-content { width: calc(100% - 32px); }
}
</style>
