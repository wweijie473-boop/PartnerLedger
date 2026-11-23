<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { dealerSections } from './dealerNavigation.js'
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
  dealer: Boolean,
})
const emit = defineEmits(['navigate', 'account'])
const { account, signedIn } = props.demo ? useSessionAccount() : { account: null, signedIn: false }
const dealerLogo = computed(() => props.demo && props.dealer ? '/brand/zhufei-logo.png' : '')
const tabs = ref(null)
const section = computed(() => dealerSections.find(item => item.views.includes(props.view)) || dealerSections[0])
const childrenOf = group => group.views.map(id => props.nav.find(item => item.id === id)).filter(Boolean)
const sections = computed(() => dealerSections.filter(group => childrenOf(group).length))
const shortcuts = computed(() => ['visits','opportunities','orders','finance','tickets']
  .map(id => props.nav.find(item => item.id === id)).filter(Boolean))

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
    <PortalHeader :account="account" :signed-in="signedIn">
      <template v-if="!demo || !signedIn" #account>
        <WorkspaceAccount :label="accountLabel" :organization="organization" :logo="dealerLogo" @click="emit('account')" />
      </template>
      <template v-if="!dealer" #secondary>
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
    <div :class="{ 'dealer-layout': dealer }">
    <aside v-if="dealer" class="dealer-rail">
      <header>
        <span v-if="dealerLogo" class="dealer-logo">
          <img :src="dealerLogo" width="26" height="26" alt="" />
        </span>
        <UiIcon v-else name="dealer" :size="24" />
        <div><strong>代理商运营中心</strong><small>{{ organization }}</small></div>
      </header>
      <nav aria-label="代理商业务栏目">
        <button v-for="group in sections" :key="group.id" type="button"
          :aria-current="section.id === group.id ? 'true' : undefined" @click="emit('navigate', childrenOf(group)[0].id)">
          <UiIcon :name="group.icon" :size="17" />{{ group.label }}
        </button>
      </nav>
    </aside>
    <main id="workspace-content" class="pl-wrap workspace-content" tabindex="-1">
      <template v-if="dealer">
        <div class="dealer-location"><span>代理商运营中心　/　<strong>{{ section.label }}</strong></span><span>{{ environment }}</span></div>
        <nav class="dealer-children" aria-label="栏目功能">
          <button v-for="item in childrenOf(section)" :key="item.id" type="button" :aria-current="item.id === view ? 'page' : undefined" @click="emit('navigate', item.id)">{{ item.label }}</button>
        </nav>
        <div v-if="$slots.actions || (demo && signedIn)" class="workspace-context"><slot name="actions" /><UiButton v-if="demo && signedIn" @click="emit('account')">切换演示身份</UiButton></div>
        <nav v-if="view === 'overview' && shortcuts.length" class="dealer-quick" aria-label="业务快捷入口">
          <button v-for="item in shortcuts" :key="item.id" @click="emit('navigate', item.id)">
            <UiIcon :name="item.icon" :size="20" />
            {{ item.label }}
          </button>
        </nav>
      </template>
      <slot />
    </main>
    </div>
  </div>
</template>

<style scoped>
.dealer-layout { display:grid;grid-template-columns:210px minmax(0,1fr);min-height:calc(100vh - 55px);background:#f4f7fb }
.dealer-rail { background:#192c46;color:#e5efff;padding:18px 10px }
.dealer-rail header {display:flex;align-items:center;gap:10px;padding:6px 8px 22px}
.dealer-logo {
  display:grid;
  flex:none;
  align-self:flex-start;
  width:30px;
  height:30px;
  place-items:center;
  overflow:hidden;
  border-radius:6px;
  background:#fff;
}
.dealer-logo img {
  object-fit:contain;
  /* Center the artwork within the source image's uneven white margins. */
  transform:translate(2.4px,.9px);
}
.dealer-rail strong,.dealer-rail small {display:block}.dealer-rail small {color:#b0c2dc;margin-top:4px;overflow-wrap:anywhere}
.dealer-rail nav {display:grid;gap:3px}.dealer-rail button,.dealer-children button {display:flex;align-items:center;gap:10px;border:0;background:transparent;color:inherit;text-align:left}
.dealer-rail button {padding:10px 14px;border-radius:5px;font-weight:500}.dealer-rail button:hover {background:#273f60}.dealer-rail button[aria-current] {background:#1467ac;color:white}
.dealer-layout main {width:100%;max-width:1580px;min-width:0;padding:16px clamp(14px,2vw,32px);box-sizing:border-box}
.dealer-location {display:flex;justify-content:space-between;gap:12px;color:var(--ui-ink-muted);font-size:12px;margin:0 0 10px}
.dealer-quick {display:flex;gap:8px;flex-wrap:wrap;margin-bottom:20px}
.dealer-quick button {flex:1 1 100px;display:flex;align-items:center;justify-content:center;gap:8px;padding:12px 8px;background:#fff;border:1px solid var(--ui-border);border-radius:7px;color:#174e79}
.dealer-quick button:hover {border-color:#588cba;background:#f5faff}
.dealer-layout :deep(.ui-metric-card) {background:#fff;border-radius:10px;border:1px solid var(--ui-border)}
.dealer-layout :deep(.workspace-section),.dealer-layout :deep(.ch-ledger) {background:#fff;border:1px solid var(--ui-border);border-radius:10px;padding:12px}
.dealer-children {display:flex;gap:12px;flex-wrap:wrap;background:white;border-bottom:1px solid var(--ui-border);padding:0 14px;margin-bottom:18px}
.dealer-children button {min-height:40px;border-bottom:2px solid transparent}.dealer-children button[aria-current] {color:var(--ui-link);border-bottom-color:var(--ui-accent)}
@media (max-width:760px) {
  .dealer-layout {display:block}.dealer-rail {padding:8px}.dealer-rail header {display:none}.dealer-rail nav {display:flex;overflow-x:auto}.dealer-rail button {flex:none;padding:8px 10px}
}
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
