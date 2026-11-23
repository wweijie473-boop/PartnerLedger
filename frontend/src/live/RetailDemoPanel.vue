<script setup>
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { UiButton, UiCard } from '../ui/index.js'
import { demoCompanies } from '../demoCompanies.js'
import RetailDesk from './RetailDesk.vue'
import { requestRetailDemo, retailDemoCounterpart } from './retailDemo.js'
import { retailApi } from './retailRequest.js'

const props = defineProps({ role: String, view: String })
const emit = defineEmits(['navigate'])
const DealerOperations = defineAsyncComponent(() => import('./DealerOperations.vue'))
const operations = computed(() => props.view.startsWith('funds-') || ['business-overview', 'customers', 'renewals', 'application-queue', 'service-tickets', 'vendor-finance', 'vendor-tickets'].includes(props.view))
const sessionKey = 'partnerledger-retail-demo-session-v1'
const session = ref(null)
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const saved = JSON.parse(localStorage.getItem(sessionKey))
    if (!saved?.dealer || !saved?.vendor) return
    await requestRetailDemo('/session', { token: saved[props.role] })
    session.value = saved
  } catch (cause) {
    if (cause.status !== undefined) error.value = cause.message
  } finally {
    loading.value = false
  }
})

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

async function request(path, options) {
  const endpoint = path === '/api/channel/v1/organization-links' ? '/organization-links' : path.slice(retailApi.length)
  const active = session.value
  try {
    return await requestRetailDemo(endpoint, { ...options, token: active[props.role] })
  } catch (cause) {
    if (cause.status === 401 && session.value === active) {
      error.value = cause.message
      session.value = null
    }
    throw cause
  }
}
</script>

<template>
  <div class="retail-demo-context">
    <span>{{ operations ? '运营演示' : '直购演示' }} · {{ demoCompanies[role] }}</span>
    <a :href="retailDemoCounterpart(role, view)">{{ role === 'dealer' ? '切换到厂商后台' : '返回代理商中台' }}</a>
  </div>
  <DealerOperations v-if="session && operations" :key="view" :view="view" :role="role" :request="request"
    :identity="session.dealer" @navigate="emit('navigate', $event)" />
  <RetailDesk v-else-if="session" :key="view" :mode="view" :role="role" :request="request"
    :identity="session.dealer" demo @navigate="emit('navigate', $event)" />
  <UiCard v-else title="渠道运营演示" description="使用共享演示数据办理客户、订单、资金和工单业务。">
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <UiButton variant="primary" :loading="loading" @click="start">开启演示</UiButton>
  </UiCard>
</template>

<style scoped>
.retail-demo-context { display:flex; flex-wrap:wrap; justify-content:space-between; gap:8px; margin:0 0 12px; color:var(--ui-ink-muted); font-size:13px; }
</style>
