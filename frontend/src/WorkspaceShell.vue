<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import PortalHeader from './PortalHeader.vue'
import WorkspaceAccount from './WorkspaceAccount.vue'
import { useSessionAccount } from './useSessionAccount.js'
import { groupWorkspacePages } from './workspaceNavigation.js'
import { UiButton, UiIcon } from './ui/index.js'
import { vPageFade } from './ui/pageFade.js'
import './workspace.css'

const props = defineProps({
  nav: { type: Array, required: true },
  view: { type: String, required: true },
  detailKey: { type: String, default: '' },
  organization: { type: String, required: true },
  accountLabel: { type: String, required: true },
  environment: { type: String, required: true },
  role: { type: String, default: '' },
  demo: Boolean,
})
const emit = defineEmits(['navigate', 'account'])
const { account, signedIn } = props.demo ? useSessionAccount() : { account: null, signedIn: false }
const dealerLogo = computed(() => props.demo && props.role === 'dealer' ? '/brand/zhufei-logo.png' : '')
const groups = computed(() => groupWorkspacePages(props.nav, props.role))
const currentPage = computed(() => props.nav.find(page => page.id === props.view))
const currentGroup = computed(() => groups.value.find(group => group.id === (currentPage.value?.group || props.view)))
const topGroups = computed(() => groups.value.filter(group => group.pinned || group.id === currentGroup.value?.id))
const groupPages = computed(() => currentGroup.value?.pages ?? [])
const showGroupPages = computed(() => groupPages.value.length > 1 || currentPage.value?.direct)
const menuOpen = ref(false)
const menu = ref(null)
const menuTrigger = ref(null)
const directory = ref(null)
const content = ref(null)

async function toggleMenu() {
  menuOpen.value = !menuOpen.value
  if (menuOpen.value) {
    await nextTick()
    if (directory.value) directory.value.scrollTop = 0
  }
}

function dismissMenu(returnFocus = false) {
  menuOpen.value = false
  if (returnFocus) menuTrigger.value?.focus()
}

function leaveMenu(event) {
  if (!menu.value?.contains(event.relatedTarget)) dismissMenu()
}
function outsideMenu(event) {
  if (menuOpen.value && !menu.value?.contains(event.target)) dismissMenu()
}

function escapeMenu(event) {
  if (!menuOpen.value) return
  event.preventDefault()
  event.stopPropagation()
  dismissMenu(true)
}

async function navigateFromMenu(id) {
  emit('navigate', id)
  dismissMenu()
  await nextTick()
  content.value?.focus({ preventScroll: true })
}

watch(() => props.view, () => dismissMenu())
onMounted(() => document.addEventListener('pointerdown', outsideMenu))
onUnmounted(() => document.removeEventListener('pointerdown', outsideMenu))
</script>

<template>
  <div class="workspace-shell">
    <a href="#workspace-content" class="skip-link">跳转到主要内容</a>
    <PortalHeader :account="account" :signed-in="signedIn" :workspace-role="demo ? '' : role">
      <template v-if="!demo || !signedIn" #account>
        <WorkspaceAccount :label="accountLabel" :organization="organization" :logo="dealerLogo" @click="emit('account')" />
      </template>
      <template #secondary>
        <div class="workspace-subbar">
          <div class="pl-wrap workspace-subbar-inner">
            <nav class="workspace-tabs" aria-label="工作台栏目">
              <button v-for="group in topGroups" :key="group.id" type="button"
                :class="{ 'mobile-tab': group.featured || group === currentGroup || group.id === (role ? 'overview' : 'home'), 'workspace-featured-tab': group.featured }"
                :aria-current="group === currentGroup ? (showGroupPages ? 'true' : 'page') : undefined"
                @click="emit('navigate', group.pages[0].id)">
                <UiIcon v-if="group.featured" :name="group.icon" :size="19" />
                <span>{{ group.label }}</span>
              </button>
            </nav>
            <div ref="menu" class="workspace-menu" @focusout="leaveMenu" @keydown.esc="escapeMenu">
              <button ref="menuTrigger" type="button" class="workspace-menu-trigger"
                :aria-expanded="menuOpen" aria-controls="workspace-directory" @click="toggleMenu">
                全部功能 <UiIcon name="chevron-down" :size="13" />
              </button>
              <nav v-show="menuOpen" id="workspace-directory" ref="directory" class="workspace-directory" aria-label="全部工作台功能">
                <div class="workspace-directory-groups">
                  <section v-for="group in groups" :key="group.id" class="workspace-menu-group">
                    <h2 v-if="role && (group.pages.length > 1 || group.label !== group.pages[0].label)">{{ group.label }}</h2>
                    <button v-for="page in group.pages" :key="page.id" type="button"
                      :aria-current="view === page.id ? 'page' : undefined" @click="navigateFromMenu(page.id)">
                      {{ page.label }}
                    </button>
                  </section>
                </div>
              </nav>
            </div>
            <span class="workspace-environment"><UiIcon name="permission" :size="14" />{{ environment }}</span>
          </div>
        </div>
      </template>
    </PortalHeader>
    <main id="workspace-content" ref="content" class="pl-wrap workspace-content" tabindex="-1">
      <nav v-if="showGroupPages" class="workspace-subpages" :class="{ 'has-picker': groupPages.length > 4 }" aria-label="当前栏目功能">
        <div class="workspace-page-links">
          <button v-for="page in groupPages" :key="page.id" type="button"
            :aria-current="view === page.id ? 'page' : undefined" @click="emit('navigate', page.id)">{{ page.label }}</button>
          <span v-if="currentPage?.direct" aria-current="page">{{ currentPage.label }}</span>
        </div>
        <label v-if="groupPages.length > 4" class="workspace-page-picker">
          栏目功能
          <select :value="view" @change="emit('navigate', $event.target.value)">
            <option v-for="page in groupPages" :key="page.id" :value="page.id">{{ page.label }}</option>
            <option v-if="currentPage?.direct" :value="view" disabled>{{ currentPage.label }}</option>
          </select>
        </label>
      </nav>
      <div v-if="$slots.actions || (demo && signedIn)" class="workspace-actions workspace-context">
        <slot name="actions" />
        <UiButton v-if="demo && signedIn" variant="text" compact @click="emit('account')">切换演示身份</UiButton>
      </div>
      <div v-page-fade="view + ':' + detailKey" class="workspace-view"><slot /></div>
    </main>
  </div>
</template>

<style scoped>
.workspace-shell { min-height: 100vh; background: var(--ui-surface); }
.workspace-subbar { border-top: 1px solid var(--ui-border); background: var(--ui-surface); }
.workspace-subbar-inner {
  position: relative;
  display: flex;
  align-items: center;
  gap: 4px 12px;
}
.workspace-tabs {
  display: flex;
  align-items: stretch;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}
.workspace-tabs button, .workspace-menu-trigger, .workspace-subpages :is(button, span) {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 40px;
  padding: 0 clamp(8px, 1vw, 14px);
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--ui-ink-secondary);
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}
.workspace-tabs button[aria-current], .workspace-subpages [aria-current] {
  border-bottom-color: var(--ui-accent);
  color: var(--ui-link);
}
.workspace-menu { flex: none; }
.workspace-menu-trigger {
  min-height: 32px;
  margin-block: 3px;
  border-bottom: 0;
  border-radius: var(--ui-radius-sm);
  background: var(--ui-accent-hover);
  color: var(--ui-on-accent);
  font-weight: 600;
}
.workspace-menu-trigger:hover, .workspace-menu-trigger[aria-expanded="true"] { background: var(--ui-link); }
.workspace-tabs button:hover, .workspace-subpages button:hover {
  background: var(--ui-surface-hover); color: var(--ui-link);
}
.workspace-tabs .workspace-featured-tab {
  color: var(--ui-link);
  background: var(--ui-accent-bg);
  font-weight: 650;
}
.workspace-tabs .workspace-featured-tab:hover { background: var(--ui-accent-border); }
.workspace-directory {
  position: absolute;
  z-index: 2;
  inset: 100% 0 auto auto;
  width: min(440px, 100%);
  max-height: min(62vh, 480px);
  overflow-y: auto;
  padding: 12px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-surface);
  box-shadow: var(--ui-shadow-dialog);
}
.workspace-directory-groups { columns: 2 140px; column-gap: 12px; }
.workspace-menu-group { break-inside: avoid; padding-bottom: 8px; }
.workspace-menu-group h2 { margin: 0 0 4px; color: var(--ui-ink-muted); font-size: 12px; font-weight: 500; }
.workspace-menu-group button {
  display: block;
  width: 100%;
  min-height: 34px;
  padding: 4px 6px;
  border: 0;
  background: transparent;
  color: var(--ui-ink);
  text-align: left;
  overflow-wrap: anywhere;
}
.workspace-menu-group button:hover, .workspace-menu-group button[aria-current] { background: var(--ui-accent-bg); color: var(--ui-link); }
.workspace-environment {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--ui-ink-muted);
  font-size: 12px;
  white-space: nowrap;
}
.workspace-content { padding-block: clamp(18px, 2vw, 28px) 24px; scroll-margin-top: 120px; }
.workspace-subpages { margin-bottom: 16px; border-bottom: 1px solid var(--ui-border); }
.workspace-page-links { display: flex; flex-wrap: wrap; }
.workspace-page-links :is(button, span) { min-height: 36px; }
.workspace-page-picker { display: none; }
.workspace-context { justify-content: end; margin-bottom: 8px; }
@media (max-width: 860px) {
  .workspace-subbar-inner { flex-wrap: wrap; gap: 0 8px; }
  .workspace-tabs button:not(.mobile-tab) { display: none; }
  .workspace-tabs button, .workspace-menu-trigger { min-height: 44px; }
  .workspace-menu-group button { min-height: 40px; }
  .workspace-subpages button { min-height: 44px; }
}
@media (max-width: 540px) {
  .workspace-content { width: calc(100% - 32px); scroll-margin-top: 16px; }
  .workspace-environment { order: 1; flex-basis: 100%; padding-block: 0 5px; }
  .workspace-subpages.has-picker .workspace-page-links { display: none; }
  .workspace-page-picker {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 7px;
    color: var(--ui-ink-muted);
    font-size: 13px;
  }
  .workspace-page-picker select {
    min-width: 0;
    min-height: 40px;
    padding-inline: 8px;
    border: 1px solid var(--ui-border-strong);
    border-radius: var(--ui-radius-sm);
    background: var(--ui-surface);
    color: var(--ui-ink);
  }
}
</style>
