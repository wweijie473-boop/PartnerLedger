<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { UiIcon } from './ui'
import WorkspaceAccount from './WorkspaceAccount.vue'
import { accountPath } from './auth-session.js'
import { portalMenus, searchPortal } from './portal-navigation.js'
import './portal-header.css'

const props = defineProps({ homePage: Boolean, account: Object, signedIn: Boolean })
const primary = ref(null)
const searchInput = ref(null)
const query = ref('')
const active = ref('')
const pinned = ref(false)
const results = computed(() => searchPortal(query.value))

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

function submitSearch() {
  if (!query.value.trim() || !results.value.length) {
    searchInput.value?.focus()
    open('search')
    return
  }
  const href = destination(results.value[0].href)
  dismiss()
  window.location.assign(href)
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
          <ul v-show="active === menu.id" :id="'portal-panel-' + menu.id" class="pl-popover"
            :class="{ 'pl-popover-text': menu.id === 'solutions' }">
            <li v-for="link in menu.links" :key="link.href">
              <a :href="destination(link.href)" @click="dismiss">
                <template v-if="menu.id === 'solutions'">{{ link.label }}</template>
                <template v-else>
                  <span class="pl-link-icon"><UiIcon :name="link.icon" :size="18" /></span>
                  <span><strong>{{ link.label }}</strong><small>{{ link.description }}</small></span>
                </template>
              </a>
            </li>
          </ul>
        </div>
      </nav>

      <form class="pl-search" role="search" aria-label="站内搜索" @submit.prevent="submitSearch" @focusout="focusOut($event, 'search')">
        <div class="pl-search-field">
          <input ref="searchInput" v-model="query" type="search" placeholder="搜索站内功能" aria-label="搜索站内功能"
            autocomplete="off" @focus="open('search')" @input="open('search')" @keydown.down.prevent="focusFirst('search')">
          <button type="submit" aria-label="搜索"><UiIcon name="search" :size="17" /></button>
        </div>
        <div v-if="active === 'search'" id="portal-panel-search" class="pl-popover pl-search-panel">
          <p class="pl-search-caption" role="status">{{ query.trim() ? (results.length ? '搜索结果' : '没有找到相关功能') : '常用功能' }}</p>
          <ul v-if="results.length">
            <li v-for="link in results" :key="link.href">
              <a :href="destination(link.href)" @click="dismiss"><span class="pl-link-icon"><UiIcon :name="link.icon" :size="18" /></span><span><strong>{{ link.label }}</strong><small>{{ link.description }}</small></span></a>
            </li>
          </ul>
          <p v-else class="pl-search-empty">试试「询价」「拜访」或「表格」。</p>
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
