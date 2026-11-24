<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { UiIcon } from './ui'
import WorkspaceAccount from './WorkspaceAccount.vue'
import { requestApi } from './api.js'
import { accountPath, currentSession } from './auth-session.js'
import { organizationRole } from './live/entrances.js'
import { portalMenus, searchPortal } from './portal-navigation.js'
import './portal-header.css'

const props = defineProps({ homePage: Boolean, account: Object, signedIn: Boolean, workspaceRole: String })
const primary = ref(null)
const searchInput = ref(null)
const query = ref('')
const active = ref('')
const pinned = ref(false)
const normalizedQuery = computed(() => query.value.normalize('NFKC').trim())
const role = computed(() => props.workspaceRole || organizationRole(props.account, props.account?.organization))
const shortcuts = computed(() => searchPortal(normalizedQuery.value, role.value))
const remote = ref([])
const searching = ref(false)
const searchError = ref('')
const searchAgain = ref(0)
let submittedKeyword = ''
const resultIcons = { project: 'request', order: 'order', ticket: 'delivery', product: 'catalog' }
const results = computed(() => /^#?\d+$/.test(normalizedQuery.value)
  ? [...remote.value, ...shortcuts.value]
  : [...shortcuts.value.slice(0, remote.value.length ? 3 : 6), ...remote.value])

watch([active, normalizedQuery, () => props.signedIn, role, searchAgain], ([panel, keyword], _old, onCleanup) => {
  remote.value = []
  searching.value = false
  searchError.value = ''
  if (panel !== 'search' || submittedKeyword !== keyword) submittedKeyword = ''
  const session = currentSession()
  if (panel !== 'search' || !keyword || !session) return
  if ([...keyword].length > 120 || keyword.split(/\s+/).length > 8) {
    searchError.value = '业务搜索最多支持 120 个字符和 8 个关键词。'
    return
  }
  const controller = new AbortController()
  const current = () => !controller.signal.aborted && currentSession() === session
  searching.value = true
  const timer = setTimeout(async () => {
    try {
      const found = await requestApi('/api/search/v1/records?' + new URLSearchParams({ q: keyword }), {
        token: session.token, signal: controller.signal,
      })
      if (!current()) return
      if (!Array.isArray(found?.items) || !found.items.every(item =>
        typeof item?.label === 'string' && typeof item.description === 'string' &&
        typeof item.href === 'string' && /^\/login\/(dealer|customer|vendor)\?/.test(item.href))) {
        throw new Error('搜索服务返回的数据无法识别。')
      }
      remote.value = found.items.map(item => ({ ...item, icon: resultIcons[item.kind] || 'request' }))
      if (submittedKeyword === keyword) {
        submittedKeyword = ''
        openFirstResult()
      }
    } catch (error) {
      if (current()) {
        submittedKeyword = ''
        searchError.value = error.status === 401 ? '登录后可继续搜索业务记录。' : error.message
      }
    } finally {
      if (!controller.signal.aborted) searching.value = false
    }
  }, 180)
  onCleanup(() => {
    clearTimeout(timer)
    controller.abort()
  })
})

function open(id, pin = false) {
  active.value = id
  pinned.value = pin
}

function dismiss() {
  active.value = ''
  pinned.value = false
}

function hover(event, id) {
  if (event.pointerType === 'mouse' && active.value !== id) open(id)
}

function leave(event, id) {
  if (event.pointerType === 'mouse' && active.value === id && !pinned.value &&
      !event.currentTarget.contains(document.activeElement)) dismiss()
}

function toggle(id) {
  if (active.value === id && pinned.value) dismiss()
  else open(id, true)
}

function focusOut(event, id) {
  if (active.value === id && !event.currentTarget.contains(event.relatedTarget)) dismiss()
}

async function focusFirst(id) {
  open(id, true)
  await nextTick()
  document.getElementById('portal-panel-' + id)?.querySelector('a')?.focus()
}

function escape(event) {
  const id = active.value
  if (!id) return
  event.preventDefault()
  if (id === 'search') searchInput.value?.focus()
  else document.getElementById('portal-toggle-' + id)?.focus()
  dismiss()
}

function outside(event) {
  if (active.value && !primary.value?.contains(event.target)) dismiss()
}

function destination(href) {
  return !props.homePage && href.startsWith('#') ? '/' + href : href
}

function openFirstResult() {
  if (!results.value.length) return
  const href = destination(results.value[0].href)
  dismiss()
  window.location.assign(href)
}

function submitSearch() {
  if (!normalizedQuery.value || (!results.value.length && !searching.value)) {
    searchInput.value?.focus()
    open('search')
    return
  }
  if (searching.value && !shortcuts.value.length) {
    submittedKeyword = normalizedQuery.value
    return
  }
  openFirstResult()
}

onMounted(() => document.addEventListener('pointerdown', outside))
onUnmounted(() => document.removeEventListener('pointerdown', outside))
</script>

<template>
  <header class="pl-header" @keydown.esc="escape">
    <div ref="primary" class="pl-wrap pl-bar">
      <a class="pl-logo" href="/" aria-label="渠智链首页">
        <img class="brand-mark" src="/brand/partnerledger-logo.png" width="38" height="38" alt="" />
        <b>渠智链</b><span class="pl-product-name">PartnerLedger</span>
      </a>

      <nav class="pl-main-nav" aria-label="主导航">
        <div v-for="menu in portalMenus" :key="menu.id" class="pl-menu"
          @pointerenter="hover($event, menu.id)" @pointerleave="leave($event, menu.id)" @focusout="focusOut($event, menu.id)">
          <button :id="'portal-toggle-' + menu.id" class="pl-nav-button" type="button"
            :aria-expanded="active === menu.id" :aria-controls="'portal-panel-' + menu.id"
            @click="toggle(menu.id)" @keydown.down.prevent="focusFirst(menu.id)">
            {{ menu.label }} <UiIcon name="chevron-down" :size="12" />
          </button>
          <ul v-show="active === menu.id" :id="'portal-panel-' + menu.id" class="pl-popover pl-popover-text">
            <li v-for="link in menu.links" :key="link.href">
              <a :href="destination(link.href)" @click="dismiss">{{ link.label }}</a>
            </li>
          </ul>
        </div>
      </nav>

      <form class="pl-search" role="search" aria-label="站内搜索" @submit.prevent="submitSearch" @focusout="focusOut($event, 'search')">
        <div class="pl-search-field">
          <input ref="searchInput" v-model="query" type="search" placeholder="搜索站内功能" aria-label="搜索站内功能"
            maxlength="120" autocomplete="off" @focus="open('search')" @input="open('search')" @keydown.down.prevent="focusFirst('search')">
          <button type="submit" aria-label="搜索"><UiIcon name="search" :size="17" /></button>
        </div>
        <div v-if="active === 'search'" id="portal-panel-search" class="pl-popover pl-search-panel">
          <p class="pl-search-caption" role="status">{{ normalizedQuery ? (searching ? '正在搜索业务记录…' : results.length ? '搜索结果' : '没有找到相关结果') : '常用功能' }}</p>
          <ul v-if="results.length">
            <li v-for="link in results" :key="link.href">
              <a :href="destination(link.href)" @click="dismiss"><span class="pl-link-icon"><UiIcon :name="link.icon" :size="18" /></span><span><strong>{{ link.label }}</strong><small>{{ link.description }}</small></span></a>
            </li>
          </ul>
          <p v-if="searchError" class="pl-search-empty" role="alert">{{ searchError }}
            <button type="button" class="pl-nav-button" @click="searchAgain++">重试</button>
          </p>
          <p v-else-if="!results.length && !searching" class="pl-search-empty">试试「工单」「资金总览」或业务编号。</p>
        </div>
      </form>

      <nav class="pl-utility" aria-label="关于与帮助">
        <a :href="destination('#about')" @click="dismiss">了解我们</a>
        <a :href="destination('#help')" @click="dismiss">帮助</a>
      </nav>
      <div class="pl-account-slot">
        <slot name="account">
          <WorkspaceAccount v-if="account" :label="account.username" :organization="account.organization.name" :href="accountPath" />
          <a v-else-if="signedIn" class="pl-nav-button pl-nav-cta" :href="accountPath">我的账号</a>
          <a v-else class="pl-nav-button pl-nav-cta" href="/login">登录</a>
        </slot>
      </div>
    </div>
    <slot name="secondary" />
  </header>
</template>
