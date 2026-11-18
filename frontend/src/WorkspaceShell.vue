<script setup>
import { computed } from 'vue'
import { UiIcon } from './ui/index.js'

const props = defineProps({
  nav: { type: Array, required: true },
  view: { type: String, required: true },
  organization: { type: String, required: true },
  accountLabel: { type: String, required: true },
  environment: { type: String, required: true },
})
defineEmits(['navigate', 'account'])
const currentPage = computed(() => props.nav.find((item) => item.id === props.view))
const initial = computed(() => [...props.organization][0] ?? '渠')
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <button type="button" class="brand" aria-label="渠智链 AI 首页" @click="$emit('navigate', nav[0].id)">
        <span class="brand-mark"><UiIcon name="workflow" :size="22" /></span>
        <span><strong>渠智链 AI</strong><small>PARTNERLEDGER</small></span>
      </button>
      <div class="sidebar-content">
        <span class="nav-label">渠道协作</span>
        <nav aria-label="工作台导航">
          <button v-for="item in nav" :key="item.id" :class="{ active: view === item.id }"
            :aria-current="view === item.id ? 'page' : undefined" @click="$emit('navigate', item.id)">
            <UiIcon :name="item.icon" :size="19" /><span>{{ item.label }}</span>
            <UiIcon v-if="view === item.id" name="chevron-right" :size="14" class="nav-arrow" />
          </button>
        </nav>
      </div>
      <div class="sidebar-bottom">
        <div class="environment"><span></span> {{ environment }}</div>
        <button class="account" @click="$emit('account')">
          <span class="avatar" aria-hidden="true">{{ initial }}</span>
          <span class="account-summary"><strong :title="organization">{{ organization }}</strong><small>{{ accountLabel }}</small></span>
          <UiIcon name="chevron-down" :size="15" />
        </button>
      </div>
    </aside>

    <main class="main">
      <header class="topbar">
        <div class="breadcrumbs"><span>工作台</span><UiIcon name="chevron-right" :size="15" /><strong>{{ currentPage?.label }}</strong></div>
        <div class="topbar-actions"><slot name="actions" /></div>
      </header>
      <div class="content"><slot /></div>
    </main>
  </div>
</template>

<style scoped>
.account-summary { min-width: 0; }
.account-summary strong, .account-summary small { overflow: hidden; text-overflow: ellipsis; }
</style>
