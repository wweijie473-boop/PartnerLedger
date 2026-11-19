<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { UiIcon } from './ui/index.js'

const props = defineProps({
  nav: { type: Array, required: true },
  view: { type: String, required: true },
  organization: { type: String, required: true },
  accountLabel: { type: String, required: true },
  environment: { type: String, required: true },
})
const emit = defineEmits(['navigate', 'account'])
const currentPage = computed(() => props.nav.find(item => item.id === props.view))
const initial = computed(() => [...props.organization][0] ?? '渠')
const mobile = ref(false)
const drawerOpen = ref(false)
const sidebar = ref(null)
const trigger = ref(null)
let breakpoint
let originalOverflow = ''

function closeDrawer() { drawerOpen.value = false }
function navigate(id) { closeDrawer(); emit('navigate', id) }
function account() { closeDrawer(); emit('account') }
function updateViewport() {
  mobile.value = breakpoint.matches
  if (!mobile.value) closeDrawer()
}
function trapFocus(event) {
  if (!drawerOpen.value) return
  if (event.key === 'Escape') { event.preventDefault(); closeDrawer(); return }
  if (event.key !== 'Tab') return
  const controls = [...sidebar.value.querySelectorAll('button, a[href]')].filter(item => item.getClientRects().length)
  const first = controls[0], last = controls[controls.length - 1]
  if (event.shiftKey && (document.activeElement === first || document.activeElement === sidebar.value)) {
    event.preventDefault(); last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault(); first?.focus()
  }
}
watch(drawerOpen, async open => {
  if (open) {
    originalOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    await nextTick()
    sidebar.value?.querySelector('button')?.focus()
  } else {
    document.body.style.overflow = originalOverflow
    await nextTick()
    if (mobile.value) trigger.value?.focus()
  }
})
onMounted(() => {
  breakpoint = window.matchMedia('(max-width: 859px)')
  updateViewport()
  breakpoint.addEventListener('change', updateViewport)
  document.addEventListener('keydown', trapFocus)
})
onUnmounted(() => {
  breakpoint?.removeEventListener('change', updateViewport)
  document.removeEventListener('keydown', trapFocus)
  if (drawerOpen.value) document.body.style.overflow = originalOverflow
})
</script>

<template>
  <div class="app-shell">
    <a href="#workspace-content" class="skip-link">跳转到主要内容</a>
    <button v-if="mobile && drawerOpen" class="nav-backdrop" aria-label="关闭工作台导航" tabindex="-1" @click="closeDrawer"></button>
    <aside ref="sidebar" class="sidebar" :class="{ 'drawer-open': drawerOpen }" :role="mobile ? 'dialog' : undefined"
      :aria-modal="mobile && drawerOpen ? 'true' : undefined" aria-label="工作台导航" :inert="mobile && !drawerOpen" tabindex="-1">
      <div class="sidebar-brand-row">
        <button v-if="mobile" class="nav-close" aria-label="关闭导航" @click="closeDrawer"><UiIcon name="close" /></button>
        <button type="button" class="brand" aria-label="渠智链工作总览" @click="navigate(nav[0].id)">
          <img class="brand-mark" src="/brand/partnerledger-logo.png" width="38" height="38" alt="" />
          <span><strong>渠智链</strong><small>PartnerLedger</small></span>
        </button>
      </div>
      <div class="sidebar-content">
        <span class="nav-label">工作空间</span>
        <nav aria-label="业务导航">
          <button v-for="item in nav" :key="item.id" :class="{ active: view === item.id }"
            :aria-current="view === item.id ? 'page' : undefined" @click="navigate(item.id)">
            <UiIcon :name="item.icon" :size="19" /><span>{{ item.label }}</span>
          </button>
        </nav>
      </div>
      <div class="sidebar-bottom">
        <div class="environment"><UiIcon name="permission" :size="14" /> {{ environment }}</div>
        <button class="account" @click="account">
          <span class="avatar" aria-hidden="true">{{ initial }}</span>
          <span class="account-summary"><strong :title="organization">{{ organization }}</strong><small>{{ accountLabel }}</small></span>
          <UiIcon name="chevron-down" :size="15" />
        </button>
        <a class="workspace-home" href="/">返回平台首页</a>
      </div>
    </aside>
    <main class="main" :inert="mobile && drawerOpen">
      <header class="topbar">
        <div class="workspace-context">
          <button ref="trigger" class="nav-trigger" :aria-expanded="drawerOpen" aria-label="打开工作台导航" @click="drawerOpen = true"><UiIcon name="menu" :size="21" /></button>
          <div class="breadcrumbs"><span>工作台</span><UiIcon name="chevron-right" :size="14" /><strong>{{ currentPage?.label }}</strong></div>
        </div>
        <div class="topbar-actions"><span class="workspace-org" :title="organization">{{ organization }}</span><slot name="actions" /></div>
      </header>
      <div id="workspace-content" class="content" tabindex="-1"><slot /></div>
    </main>
  </div>
</template>
