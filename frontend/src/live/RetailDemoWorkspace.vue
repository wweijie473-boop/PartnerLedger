<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import WorkspaceShell from '../WorkspaceShell.vue'
import { UiButton, UiCard } from '../ui/index.js'
import RetailDesk from './RetailDesk.vue'
import { demoWorkspace, requestRetailDemo } from './retailDemo.js'

const sessionKey = 'partnerledger-retail-demo-session-v1'
const workspace = {
  dealer: {
    name: '天津主飞（演示代理商）', label: '代理商工作台',
    nav: [
      { id: 'promotion', label: '推广获客二维码', icon: 'workflow' },
      { id: 'assisted', label: '代客下单', icon: 'order' },
      { id: 'retail-orders', label: '直购订单', icon: 'order' },
      { id: 'commission', label: '佣金结算', icon: 'quote' },
    ],
  },
  vendor: {
    name: '畅捷通（演示厂商）', label: '厂商工作台',
    nav: [
      { id: 'manufacturer-receipts', label: '直购订单与收款', icon: 'order' },
      { id: 'manufacturer-settlements', label: '代理商佣金结算', icon: 'quote' },
    ],
  },
}
const role = ref('dealer')
const view = ref('promotion')
const current = computed(() => workspace[role.value])
const session = ref(null)
const loading = ref(true)
const error = ref('')

function restoreLocation() {
  const params = new URLSearchParams(location.search)
  role.value = params.get('role') === 'vendor' ? 'vendor' : 'dealer'
  const selected = params.get('view')
  view.value = current.value.nav.some(item => item.id === selected) ? selected : current.value.nav[0].id
}

function navigate(target, nextRole = role.value) {
  const params = new URLSearchParams({ role: nextRole, view: target })
  const url = demoWorkspace + '?' + params
  if (location.pathname + location.search !== url) history.pushState(null, '', url)
  restoreLocation()
}

async function restoreSession() {
  try {
    const saved = JSON.parse(localStorage.getItem(sessionKey))
    if (!saved?.dealer || !saved?.vendor || saved.expires <= Date.now()) return
    await requestRetailDemo('/session', { token: saved[role.value] })
    session.value = saved
  } catch (cause) {
    if (cause.status && cause.status !== 401) error.value = cause.message
  } finally {
    loading.value = false
  }
}

async function start() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    session.value = await requestRetailDemo('/sessions', { method: 'POST', body: {} })
    localStorage.setItem(sessionKey, JSON.stringify(session.value))
  } catch (cause) {
    error.value = cause.message
  } finally {
    loading.value = false
  }
}

function request(path, options) {
  const endpoint = path === '/api/channel/v1/organization-links' ? '/organization-links' : path.slice('/api/retail/v1'.length)
  return requestRetailDemo(endpoint, { ...options, token: session.value[role.value] })
}

function switchRole() {
  const next = role.value === 'dealer' ? 'vendor' : 'dealer'
  navigate(workspace[next].nav[0].id, next)
}

restoreLocation()
onMounted(() => {
  restoreSession()
  window.addEventListener('popstate', restoreLocation)
})
onUnmounted(() => window.removeEventListener('popstate', restoreLocation))
</script>

<template>
  <WorkspaceShell :nav="current.nav" :view="view" :organization="current.name" :account-label="current.label"
    environment="模拟交易" :dealer="role === 'dealer'" @navigate="navigate" @account="switchRole">
    <template #actions>
      <UiButton compact @click="switchRole">{{ role === 'dealer' ? '切换到厂商后台' : '切换到代理商中台' }}</UiButton>
    </template>
    <RetailDesk v-if="session" :key="role + ':' + view" :mode="view" :role="role" :request="request"
      :identity="session.dealer" demo @navigate="navigate" />
    <UiCard v-else title="厂商直购与佣金演示" description="同一场演示里，客户付款计入厂商收款，代理商按订单取得 50% 佣金。">
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <UiButton variant="primary" :loading="loading" @click="start">开启演示</UiButton>
    </UiCard>
  </WorkspaceShell>
</template>
