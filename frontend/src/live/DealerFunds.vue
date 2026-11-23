<script setup>
import { computed, ref } from 'vue'
import { formatCents, quoteCents } from '../channel/quoteMoney.js'
import { UiButton, UiCard, UiMetricCard, UiState } from '../ui/index.js'

const props = defineProps({ view: String, state: Object, busy: Boolean, submit: Function })
const emit = defineEmits(['navigate'])
const vendor = props.view === 'vendor-finance'
const vendorQueue = ref('transactions')
const section = computed(() => vendor ? vendorQueue.value : props.view.slice('funds-'.length))
const query = ref('')
const draft = ref(null)
const typeNames = { transactions: '交易明细', invoices: '订单开票', claims: '服务费报销' }
const queues = { transactions: '预存入账', invoices: '发票办理', claims: '报销办理' }
const money = amount => amount == null ? '—' : formatCents(BigInt(amount))
const day = time => time ? new Date(time).toLocaleDateString('zh-CN') : '—'

const records = computed(() => {
  const { topups, invoices, orders, claims, payouts } = props.state
  if (section.value === 'transactions') {
    const deposits = topups.map(row => ({ ...row, kind: 'topups', label: '预存款', statusLabel: row.status === 'credited' ? '已入账' : '待厂商确认',
      vendorAction: row.status === 'pending' ? '确认入账' : '' }))
    if (vendor) return deposits
    return [...deposits, ...payouts.map(row => ({ ...row, id: 'payout-' + row.order_id, label: '订单佣金', statusLabel: '已结算' })),
      ...claims.filter(row => row.status === 'paid').map(row => ({ ...row, label: '服务费', statusLabel: '已报销' }))]
      .sort((left, right) => (right.handled_at || right.created_at).localeCompare(left.handled_at || left.created_at))
  }
  if (section.value === 'invoices') {
    return orders.filter(order => order.paid_at).flatMap(order => {
      const invoice = invoices.find(row => row.order_id === order.id)
      if (vendor && !invoice) return []
      return [{ ...invoice, id: invoice?.id ?? order.id, order_id: order.id, kind: 'invoices', label: order.product_name,
        detail: invoice?.title || order.customer, amount_cents: order.total_cents, created_at: invoice?.created_at || order.paid_at,
        statusLabel: invoice?.status === 'issued' ? '已开票' : invoice ? '待开票' : '可申请',
        vendorAction: invoice?.status === 'requested' ? '登记开票' : '', dealerAction: !invoice ? '申请开票' : '' }]
    })
  }
  return claims.map(row => ({ ...row, kind: 'claims', label: '服务费报销', statusLabel: row.status === 'paid' ? '已报销' : '待厂商办理',
    vendorAction: row.status === 'pending' ? '登记报销' : '' }))
})
const rows = computed(() => records.value.filter(row => [row.label, row.detail, row.order_id, row.reference, row.result_reference, row.statusLabel]
  .join(' ').toLowerCase().includes(query.value.trim().toLowerCase())))

function chooseQueue(name) {
  vendorQueue.value = name
  draft.value = null
  query.value = ''
}

function openAmountForm(kind) {
  draft.value = { kind, title: kind === 'topups' ? '登记模拟汇款' : '申请服务费报销', reference: '', amount: '' }
}

function openRowForm(row) {
  draft.value = vendor ? { kind: row.kind, title: row.vendorAction, record: row, reference: '' }
    : { kind: 'invoices', title: '申请订单开票', record: row, invoiceTitle: row.detail }
}

async function send() {
  const form = draft.value
  const key = vendor ? form.kind + '/' + form.record.id + '/complete' : form.kind
  const payload = vendor ? { result_reference: form.reference } : form.kind === 'invoices'
    ? { order_id: form.record.order_id, title: form.invoiceTitle } : { reference: form.reference, amount_cents: Number(quoteCents(form.amount)) }
  if (await props.submit(key, payload, vendor ? '办理结果已登记。' : '申请已提交，等待厂商办理。')) draft.value = null
}
</script>

<template>
  <template v-if="section === 'overview'">
    <div class="operations-metrics">
      <UiMetricCard label="预存款余额" icon="quote" :value="money(state.totals.topup_balance_cents)" interactive @click="emit('navigate', 'funds-transactions')" />
      <UiMetricCard label="待结算佣金" icon="quote" :value="money(state.totals.pending_commission_cents)" interactive @click="emit('navigate', 'commission')" />
      <UiMetricCard label="已结算佣金" icon="check" :value="money(state.totals.settled_commission_cents)" interactive @click="emit('navigate', 'commission')" />
      <UiMetricCard label="可报销服务费" icon="request" :value="money(state.totals.available_service_cents)" interactive @click="emit('navigate', 'funds-claims')" />
    </div>
    <UiCard title="财务办理" description="预存款、订单佣金和服务费分别记账。汇款、开票和报销均使用演示凭证。">
      <div class="operations-actions">
        <UiButton @click="emit('navigate', 'funds-transactions')">查看交易明细</UiButton>
        <UiButton @click="emit('navigate', 'funds-invoices')">查询订单开票</UiButton>
        <UiButton @click="emit('navigate', 'funds-claims')">办理服务费报销</UiButton>
      </div>
    </UiCard>
  </template>
  <template v-else>
    <nav v-if="vendor" class="operations-actions" aria-label="财务办理类别">
      <UiButton v-for="(label, key) in queues" :key="key" :variant="section === key ? 'primary' : 'secondary'"
        :aria-pressed="section === key" @click="chooseQueue(key)">{{ label }}</UiButton>
    </nav>
    <div v-else-if="section === 'claims'" class="operations-metrics">
      <UiMetricCard label="服务费总额" icon="quote" :value="money(state.totals.service_credit_cents)" />
      <UiMetricCard label="可申请" icon="request" :value="money(state.totals.available_service_cents)" />
      <UiMetricCard label="办理中" icon="time" :value="money(state.totals.claimed_service_cents)" />
      <UiMetricCard label="已报销" icon="check" :value="money(state.totals.paid_service_cents)" />
    </div>
    <UiCard v-if="draft" :title="draft.title">
      <form class="operations-form" @submit.prevent="send">
        <label v-if="!vendor && draft.kind === 'invoices'">开票抬头<input v-model.trim="draft.invoiceTitle" maxlength="80" required></label>
        <label v-else>{{ vendor ? '办理凭证编号' : draft.kind === 'topups' ? '模拟汇款凭证号' : '服务费发票号' }}<input v-model.trim="draft.reference" maxlength="80" required></label>
        <label v-if="!vendor && !draft.record">金额（元）<input v-model.trim="draft.amount" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,2})?" placeholder="0.00" required></label>
        <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">{{ vendor ? '确认办理' : '提交申请' }}</UiButton><UiButton :disabled="busy" @click="draft = null">取消</UiButton></div>
      </form>
    </UiCard>
    <UiCard :title="vendor ? queues[section] : typeNames[section]">
      <label class="operations-search">查询<input v-model="query" type="search" placeholder="订单号、凭证、名称或状态"></label>
      <UiButton v-if="!vendor && section === 'transactions'" variant="primary" :disabled="busy" @click="openAmountForm('topups')">登记模拟汇款</UiButton>
      <UiButton v-if="!vendor && section === 'claims'" variant="primary" :disabled="busy || !state.totals.available_service_cents" @click="openAmountForm('claims')">申请服务费报销</UiButton>
      <div v-if="rows.length" class="operations-table">
        <table><thead><tr><th scope="col">项目</th><th scope="col">金额</th><th scope="col">日期</th><th scope="col">凭证</th><th scope="col">状态</th><th scope="col">办理</th></tr></thead>
          <tbody><tr v-for="row in rows" :key="row.id">
            <td>{{ row.label }}<small v-if="row.order_id" class="operations-meta">订单 {{ row.order_id }}{{ row.detail ? ' · ' + row.detail : '' }}</small></td>
            <td>{{ money(row.amount_cents) }}</td><td>{{ day(row.handled_at || row.created_at) }}</td>
            <td>{{ row.reference || '—' }}<small v-if="row.result_reference" class="operations-meta">{{ row.result_reference }}</small></td>
            <td>{{ row.statusLabel }}</td>
            <td><UiButton v-if="vendor ? row.vendorAction : row.dealerAction" compact :disabled="busy" @click="openRowForm(row)">{{ vendor ? row.vendorAction : row.dealerAction }}</UiButton></td>
          </tr></tbody>
        </table>
      </div>
      <UiState v-else :title="section === 'invoices' && !vendor && !query ? '订单完成收款后可申请开票' : '暂无符合条件的记录'" />
    </UiCard>
  </template>
</template>
