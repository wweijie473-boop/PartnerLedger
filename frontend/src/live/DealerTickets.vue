<script setup>
import { computed, ref } from 'vue'
import { UiButton, UiCard, UiState } from '../ui/index.js'
import { pendingRetailRequest } from './retailRequest.js'

const props = defineProps({ state: Object, role: String, identity: String, busy: Boolean, submit: Function })
const vendor = props.role === 'vendor'
const newTicket = ref(null)
const response = ref(null)
const status = ref('')
const labels = { open: '待厂商处理', resolved: '待代理商确认', closed: '已关闭' }
const labelsByRole = { dealer: '代理商', vendor: '厂商' }
const rows = computed(() => props.state.tickets.filter(row => !status.value || row.status === status.value))
const time = value => new Date(value).toLocaleString('zh-CN')

function create() {
  newTicket.value = { subject: '', category: '产品使用', order_id: '', detail: '' }
}

async function submitTicket() {
  const body = newTicket.value
  const key = 'partnerledger-demo-ticket:' + props.identity
  const id = pendingRetailRequest(key, body)
  if (await props.submit('tickets', { ...body, request_id: id }, '工单已提交，等待厂商处理。')) {
    sessionStorage.removeItem(key)
    newTicket.value = null
    status.value = ''
  }
}

async function sendReply() {
  const item = response.value
  if (await props.submit('tickets/' + item.id + '/' + (vendor ? 'resolve' : 'reopen'), { reply: item.reply }, vendor ? '处理结果已提交。' : '补充说明已提交。')) {
    response.value = null
    status.value = ''
  }
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
      <label>问题描述<textarea v-model.trim="newTicket.detail" maxlength="500" required /></label>
      <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">提交工单</UiButton><UiButton :disabled="busy" @click="newTicket = null">取消</UiButton></div>
    </form>
  </UiCard>
  <div class="operations-actions">
    <label class="operations-search">工单状态<select v-model="status"><option value="">全部</option><option v-for="(label, key) in labels" :key="key" :value="key">{{ label }}</option></select></label>
    <UiButton v-if="!vendor && !newTicket" variant="primary" :disabled="busy" @click="create">新建工单</UiButton>
  </div>
  <div v-if="rows.length" class="operations-list">
    <UiCard v-for="row in rows" :key="row.id" :title="row.subject"
      :description="row.category + (row.order_id ? ' · 订单 ' + row.order_id : '') + ' · ' + labels[row.status]">
      <p class="operations-thread">{{ row.detail }}</p>
      <small class="operations-meta">代理商提交 · {{ time(row.created_at) }}</small>
      <ul v-if="row.replies.length" class="operations-thread">
        <li v-for="(reply, index) in row.replies" :key="index"><strong>{{ labelsByRole[reply.role] }}</strong> {{ reply.text }}<small class="operations-meta">{{ time(reply.time) }}</small></li>
      </ul>
      <form v-if="response?.id === row.id" class="operations-form" @submit.prevent="sendReply">
        <label>{{ vendor ? '处理结果' : '补充说明' }}<textarea v-model.trim="response.reply" required maxlength="500" /></label>
        <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">{{ vendor ? '提交处理结果' : '提交补充说明' }}</UiButton><UiButton :disabled="busy" @click="response = null">取消</UiButton></div>
      </form>
      <div v-else-if="vendor ? row.status === 'open' : row.status === 'resolved'" class="operations-actions">
        <UiButton v-if="!vendor" :disabled="busy" @click="submit('tickets/' + row.id + '/close', {}, '工单已关闭。')">确认关闭</UiButton>
        <UiButton :disabled="busy" @click="response = { id: row.id, reply: '' }">{{ vendor ? '填写处理结果' : '补充并重新打开' }}</UiButton>
      </div>
    </UiCard>
  </div>
  <UiState v-else title="暂无符合条件的工单" />
</template>
