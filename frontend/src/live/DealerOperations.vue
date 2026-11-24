<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { UiButton, UiCard, UiMetricCard, UiState } from '../ui/index.js'
import { formatCents } from '../channel/quoteMoney.js'
import { readRetailCatalog, retailApi } from './retailRequest.js'
import DealerCustomers from './DealerCustomers.vue'
import DealerRenewals from './DealerRenewals.vue'
import DealerFunds from './DealerFunds.vue'
import DealerTickets from './DealerTickets.vue'
import './dealer-operations.css'

const props = defineProps({ view: String, title: String, role: String, identity: String, request: Function, demo: Boolean })
const emit = defineEmits(['navigate'])
const controller = new AbortController()
const loading = ref(true)
const busy = ref(false)
const state = ref(null)
const error = ref('')
const notice = ref('')
const pendingOnly = ref(false)
const operations = retailApi + '/operations'
const applications = computed(() => {
  if (!state.value) return []
  const { topups, invoices, claims } = state.value
  return [
    ...topups.map(row => ({ ...row, type: '预存入账', target: 'funds-transactions', state: row.status === 'credited' ? '已入账' : '待处理' })),
    ...invoices.map(row => ({ ...row, type: '订单开票', reference: '订单 ' + row.order_id, target: 'funds-invoices', state: row.status === 'issued' ? '已开票' : '待处理' })),
    ...claims.map(row => ({ ...row, type: '服务费报销', target: 'funds-claims', state: row.status === 'paid' ? '已报销' : '待处理' })),
  ].filter(row => !pendingOnly.value || !row.handled_at).sort((left, right) => right.created_at.localeCompare(left.created_at))
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const read = path => props.request(path, { signal: controller.signal })
    if (props.demo) {
      state.value = await read(operations)
    } else if (props.view === 'business-overview') {
      state.value = { totals: await read(operations + '/overview') }
    } else {
      state.value = { [props.view]: await readRetailCatalog(read, operations + '/' + props.view) }
    }
  } catch (cause) {
    if (!controller.signal.aborted) error.value = cause.message
  } finally {
    loading.value = false
  }
}

async function submit(path, body, message) {
  if (busy.value) return false
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const method = props.demo || path === 'customers' ? 'POST' : 'PUT'
    await props.request(operations + '/' + path, { method, body, signal: controller.signal })
    if (!controller.signal.aborted) {
      notice.value = message
      await load()
    }
    return true
  } catch (cause) {
    if (!controller.signal.aborted) error.value = cause.message
    return false
  } finally {
    busy.value = false
  }
}

function refreshOnFocus() {
  if (!busy.value && !loading.value) load()
}

onMounted(() => {
  load()
  window.addEventListener('focus', refreshOnFocus)
})
onUnmounted(() => {
  controller.abort()
  window.removeEventListener('focus', refreshOnFocus)
})
</script>

<template>
  <div class="dealer-operations">
    <header class="page-heading"><h1>{{ title }}</h1><UiButton compact variant="text" :disabled="loading || busy" @click="load">刷新</UiButton></header>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="operations-notice" role="status">{{ notice }}</p>
    <UiState v-if="!state && loading" state="loading" title="正在读取运营记录" />
    <UiState v-else-if="!state" state="error" title="无法读取运营记录"><UiButton @click="load">重试</UiButton></UiState>
    <template v-else-if="view === 'business-overview'">
      <div class="metric-grid">
        <UiMetricCard label="客户档案" icon="customer" :value="state.totals.customers" interactive @click="emit('navigate', 'customers')" />
        <UiMetricCard label="直购订单" icon="order" :value="state.totals.orders" :description="'推广订单 ' + state.totals.promotion_orders + ' 笔'" interactive @click="emit('navigate', 'retail-orders')" />
        <UiMetricCard label="30 天内到期" icon="time" :value="state.totals.renewals_30_days" interactive @click="emit('navigate', 'renewals')" />
        <UiMetricCard label="待结算佣金" icon="quote" :value="formatCents(BigInt(state.totals.pending_commission_cents))" interactive @click="emit('navigate', 'commission')" />
        <UiMetricCard v-if="demo" label="未关闭工单" icon="request" :value="state.totals.open_tickets" interactive @click="emit('navigate', 'service-tickets')" />
      </div>
      <UiCard title="日常办理">
        <div class="operations-actions"><UiButton @click="emit('navigate', 'promotion')">推广获客</UiButton><UiButton @click="emit('navigate', 'assisted')">代客下单</UiButton><UiButton @click="emit('navigate', demo ? 'funds-overview' : 'commission')">{{ demo ? '资金往来' : '佣金结算' }}</UiButton></div>
      </UiCard>
    </template>
    <DealerCustomers v-else-if="view === 'customers'" :state="state" :busy="busy" :submit="submit" :demo="demo" @navigate="emit('navigate', $event)" />
    <DealerRenewals v-else-if="view === 'renewals'" :rows="state.renewals" :busy="busy" :submit="submit" />
    <UiCard v-else-if="view === 'application-queue'" title="财务申请记录">
      <label class="operations-check"><input v-model="pendingOnly" type="checkbox"> 仅看待处理申请</label>
      <div v-if="applications.length" class="operations-table">
        <table><thead><tr><th scope="col">申请类型</th><th scope="col">订单或凭证</th><th scope="col">提交时间</th><th scope="col">状态</th><th scope="col">详情</th></tr></thead>
          <tbody><tr v-for="item in applications" :key="item.id"><td>{{ item.type }}</td><td>{{ item.reference }}</td>
            <td>{{ new Date(item.created_at).toLocaleDateString('zh-CN') }}</td><td>{{ item.state }}</td>
            <td><UiButton variant="text" compact @click="emit('navigate', item.target)">查看明细</UiButton></td>
          </tr></tbody>
        </table>
      </div>
      <UiState v-else title="暂无符合条件的申请" />
    </UiCard>
    <DealerTickets v-else-if="view === 'service-tickets' || view === 'vendor-tickets'" :state="state" :role="role" :identity="identity" :busy="busy" :submit="submit" />
    <DealerFunds v-else :state="state" :view="view" :busy="busy" :submit="submit" @navigate="emit('navigate', $event)" />
  </div>
</template>
