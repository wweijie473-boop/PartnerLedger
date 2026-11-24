<script setup>
import { computed, ref } from 'vue'
import { UiBadge, UiButton, UiCard, UiIcon, UiState } from '../ui/index.js'
import { pendingRetailRequest } from './retailRequest.js'

const props = defineProps({ state: Object, role: String, identity: String, busy: Boolean, submit: Function, demo: Boolean })
const vendor = props.role === 'vendor'
const linkedTicket = ref(props.demo ? '' : new URLSearchParams(location.search).get('ticket'))
const newTicket = ref(null)
const response = ref(null)
const status = ref('')
const labels = { open: '待厂商处理', resolved: '待代理商确认', closed: '已关闭' }
const tones = { open: 'warning', resolved: 'info', closed: 'success' }
const labelsByRole = { dealer: '代理商', vendor: '厂商' }
const rows = computed(() => props.state.tickets.filter(row =>
  (!linkedTicket.value || row.id === linkedTicket.value) && (!status.value || row.status === status.value)))
const time = value => new Date(value).toLocaleString('zh-CN')

function create() {
  newTicket.value = { subject: '', category: '产品使用', order_id: '', detail: '',
    manufacturer_id: props.state.partners?.length === 1 ? props.state.partners[0].id : '' }
}

async function submitTicket() {
  const { manufacturer_id, ...demoBody } = newTicket.value
  const { order_id, ...fields } = demoBody
  const body = props.demo ? demoBody : { ...fields, ...(order_id ? { order_id } : { manufacturer_id }) }
  const key = (props.demo ? 'partnerledger-demo-ticket:' : 'partnerledger-live-ticket:') + props.identity
  const id = pendingRetailRequest(key, body)
  if (await props.submit('tickets', { ...body, request_id: id }, '工单已提交，等待厂商处理。')) {
    sessionStorage.removeItem(key)
    newTicket.value = null
    status.value = ''
  }
}

async function sendReply() {
  const item = response.value
  const body = props.demo ? { reply: item.reply } : { reply: item.reply, revision: item.revision }
  if (await props.submit('tickets/' + item.id + '/' + (vendor ? 'resolve' : 'reopen'), body, vendor ? '处理结果已提交。' : '补充说明已提交。')) {
    response.value = null
    status.value = ''
  }
}

function closeTicket(row) {
  return props.submit('tickets/' + row.id + '/close', props.demo ? {} : { revision: row.revision }, '工单已关闭。')
}

function showAllTickets() {
  linkedTicket.value = ''
  const url = new URL(location.href)
  url.searchParams.delete('ticket')
  history.replaceState(history.state, '', url)
}
</script>

<template>
  <UiCard v-if="!vendor && newTicket" title="提交服务工单">
    <form class="operations-form" @submit.prevent="submitTicket">
      <label>工单标题<input v-model.trim="newTicket.subject" required maxlength="80"></label>
      <label>问题类型<select v-model="newTicket.category"><option>产品使用</option><option>授权与交付</option><option>订单与结算</option></select></label>
      <label>关联订单<select v-model="newTicket.order_id"><option value="">无需关联</option>
        <option v-for="order in state.orders" :key="order.id" :value="order.id">订单 {{ order.id }} · {{ order.product_name }}</option>
      </select></label>
      <label v-if="!demo && !newTicket.order_id">办理厂商
        <select v-model="newTicket.manufacturer_id" required><option value="">选择已有直购订单的厂商</option>
          <option v-for="partner in state.partners" :key="partner.id" :value="partner.id">{{ partner.name }}</option>
        </select>
      </label>
      <label>问题描述<textarea v-autogrow v-model.trim="newTicket.detail" maxlength="500" required /></label>
      <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">提交工单</UiButton><UiButton :disabled="busy" @click="newTicket = null">取消</UiButton></div>
    </form>
  </UiCard>
  <section class="workspace-section" aria-labelledby="ticket-records">
    <header class="workspace-section-heading">
      <h2 id="ticket-records">工单列表</h2>
      <div class="workspace-record-controls">
        <UiButton v-if="linkedTicket" @click="showAllTickets">查看全部工单</UiButton>
        <label class="ticket-filter">工单状态 <select v-model="status"><option value="">全部</option><option v-for="(label, key) in labels" :key="key" :value="key">{{ label }}</option></select></label>
        <UiButton v-if="!vendor && !newTicket" variant="primary" :disabled="busy || (!demo && !state.partners.length)" @click="create">新建工单</UiButton>
      </div>
    </header>
    <details v-for="row in rows" :key="row.id" class="ticket-record" :open="row.id === linkedTicket || undefined">
      <summary>
        <span><strong>{{ row.subject }}</strong><small>{{ row.category }}<template v-if="row.partner"> · {{ row.partner }}</template><template v-if="row.order_id"> · 订单 {{ row.order_id }}</template></small></span>
        <UiBadge :tone="tones[row.status]" dot>{{ labels[row.status] }}</UiBadge>
        <UiIcon name="chevron-down" :size="14" />
      </summary>
      <div class="ticket-body">
      <p>{{ row.detail }}</p>
      <small class="operations-meta">代理商提交 · {{ time(row.created_at) }}</small>
      <ul v-if="row.replies.length" class="operations-thread">
        <li v-for="(reply, index) in row.replies" :key="index"><strong>{{ labelsByRole[reply.role] }}</strong> {{ reply.text }}<small class="operations-meta">{{ time(reply.time) }}</small></li>
      </ul>
      <form v-if="response?.id === row.id" class="operations-form" @submit.prevent="sendReply">
        <label>{{ vendor ? '处理结果' : '补充说明' }}<textarea v-autogrow v-model.trim="response.reply" required maxlength="500" /></label>
        <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">{{ vendor ? '提交处理结果' : '提交补充说明' }}</UiButton><UiButton :disabled="busy" @click="response = null">取消</UiButton></div>
      </form>
      <div v-else-if="vendor ? row.status === 'open' : row.status === 'resolved'" class="operations-actions">
        <UiButton v-if="!vendor" :disabled="busy" @click="closeTicket(row)">确认关闭</UiButton>
        <UiButton v-if="vendor || demo || row.revision &lt; 20" :disabled="busy" @click="response = { id: row.id, revision: row.revision, reply: '' }">{{ vendor ? '填写处理结果' : '补充并重新打开' }}</UiButton>
      </div>
      </div>
    </details>
    <UiState v-if="!rows.length" title="暂无符合条件的工单"
      :description="!demo && !vendor && !state.partners.length ? '完成首次直购下单后，可选择合作厂商提交工单。' : ''" />
  </section>
</template>

<style scoped>
.ticket-filter { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; }
.ticket-filter select { width: auto; }
.ticket-record { border-top: 1px solid var(--ui-border); }
.ticket-record summary {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  padding: 8px 6px;
  list-style: none;
  cursor: pointer;
}
.ticket-record summary::-webkit-details-marker { display: none; }
.ticket-record summary:hover, .ticket-record[open] summary { background: var(--ui-surface-muted); }
.ticket-record summary > span:first-child { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.ticket-record summary strong { display: block; font-weight: 500; }
.ticket-record summary small { display: block; color: var(--ui-ink-muted); }
.ticket-record[open] summary > svg { transform: rotate(180deg); }
.ticket-body { padding: 10px 6px 12px; overflow-wrap: anywhere; }
.ticket-body > p { white-space: pre-wrap; }
.ticket-body .operations-actions { margin-top: 8px; }
</style>
