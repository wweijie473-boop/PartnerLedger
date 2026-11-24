<script setup>
import { computed, ref } from 'vue'
import { formatCents, quoteCents } from '../channel/quoteMoney.js'
import { UiBadge, UiButton, UiCard, UiMetricCard, UiState } from '../ui/index.js'

const props = defineProps({ view: String, state: Object, busy: Boolean, submit: Function, demo: Boolean })
const emit = defineEmits(['navigate'])
const vendor = props.view === 'vendor-finance'
const vendorQueue = ref('transactions')
const section = computed(() => vendor ? vendorQueue.value : props.view.slice('funds-'.length))
const query = ref('')
const draft = ref(null)
const formError = ref('')
const typeNames = { transactions: '交易明细', invoices: '订单开票', claims: '服务费报销' }
const queues = { transactions: '预存入账', invoices: '发票办理', claims: '报销办理' }
const money = amount => amount == null ? '—' : formatCents(BigInt(amount))
const day = time => time ? new Date(time).toLocaleDateString('zh-CN') : '—'
const allowanceRemaining = row => BigInt(row.limit_cents) - BigInt(row.pending_cents) - BigInt(row.paid_cents)
const partners = computed(() => props.state.partners ?? [])
const eligiblePartners = computed(() => draft.value?.kind === 'claims'
  ? partners.value.filter(row => allowanceRemaining(row) > 0n) : partners.value)
const vendorQueues = computed(() => props.demo ? queues : { ...queues, allowances: '服务费额度' })
const canClaim = computed(() => BigInt(props.state.totals.available_service_cents) > 0n)

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
  if (section.value === 'allowances') return []
  return claims.map(row => ({ ...row, kind: 'claims', label: '服务费报销', statusLabel: row.status === 'paid' ? '已报销' : '待厂商办理',
    vendorAction: row.status === 'pending' ? '登记报销' : '' }))
})
const rows = computed(() => records.value.filter(row => [row.label, row.detail, row.partner, row.order_id, row.reference, row.result_reference, row.statusLabel]
  .join(' ').toLowerCase().includes(query.value.trim().toLowerCase())))

function chooseQueue(name) {
  vendorQueue.value = name
  draft.value = null
  query.value = ''
  formError.value = ''
}

function openAmountForm(kind) {
  formError.value = ''
  const title = kind === 'topups' ? (props.demo ? '登记模拟汇款' : '提交预存入账申请') : '申请服务费报销'
  draft.value = { kind, title, reference: '', amount: '', partnerID: '' }
  if (eligiblePartners.value.length === 1) draft.value.partnerID = eligiblePartners.value[0].id
}

function openRowForm(row) {
  formError.value = ''
  draft.value = vendor ? { kind: row.kind, title: row.vendorAction, record: row, reference: '' }
    : { kind: 'invoices', title: '申请订单开票', record: row, invoiceTitle: row.detail }
}

function editAllowance(row) {
  formError.value = ''
  const cents = BigInt(row.limit_cents)
  draft.value = { kind: 'allowances', title: '设置额度 · ' + row.name, record: row,
    amount: String(cents / 100n) + '.' + String(cents % 100n).padStart(2, '0') }
}

async function send() {
  const form = draft.value
  const allowance = form.kind === 'allowances'
  formError.value = ''
  let cents
  if (allowance || (!vendor && !form.record)) {
    if (!/^\d{1,7}(\.\d{1,2})?$/.test(form.amount) || (cents = quoteCents(form.amount)) > 100000000n || (!allowance && cents === 0n)) {
      formError.value = '请输入有效金额，最高 100 万元。'
      return
    }
  }
  if (allowance) {
    if (await props.submit('allowances/' + form.record.id, { limit_cents: Number(cents) }, '服务费额度已保存。')) draft.value = null
    return
  }
  const suffix = vendor ? form.kind + '/' + form.record.id + '/complete' : form.kind
  const payload = vendor ? { result_reference: form.reference } : form.kind === 'invoices'
    ? { order_id: form.record.order_id, title: form.invoiceTitle } : { reference: form.reference, amount_cents: Number(cents),
      ...(!props.demo ? { manufacturer_id: form.partnerID } : {}) }
  if (await props.submit((props.demo ? '' : 'finance/') + suffix, payload, vendor ? '办理结果已登记。' : '申请已提交，等待厂商办理。')) draft.value = null
}
</script>

<template>
  <template v-if="section === 'overview'">
    <div class="metric-grid workspace-finance-metrics">
      <UiMetricCard :label="demo ? '预存款余额' : '已入账预存款'" icon="quote" :value="money(state.totals.topup_balance_cents)" interactive @click="emit('navigate', 'funds-transactions')" />
      <UiMetricCard label="待结算佣金" icon="quote" :value="money(state.totals.pending_commission_cents)" interactive @click="emit('navigate', 'commission')" />
      <UiMetricCard label="已结算佣金" icon="check" :value="money(state.totals.settled_commission_cents)" interactive @click="emit('navigate', 'commission')" />
      <UiMetricCard label="可报销服务费" icon="request" :value="money(state.totals.available_service_cents)" interactive @click="emit('navigate', 'funds-claims')" />
    </div>
    <UiCard title="财务办理" :description="demo ? '预存款、订单佣金和服务费分别记账。汇款、开票和报销均使用演示凭证。' : '预存款、订单佣金和服务费分别记账。厂商在完成线下办理后登记凭证。'">
      <div class="operations-actions">
        <UiButton @click="emit('navigate', 'funds-transactions')">查看交易明细</UiButton>
        <UiButton @click="emit('navigate', 'funds-invoices')">查询订单开票</UiButton>
        <UiButton @click="emit('navigate', 'funds-claims')">办理服务费报销</UiButton>
      </div>
    </UiCard>
  </template>
  <template v-else>
    <nav v-if="vendor" class="operations-actions" aria-label="财务办理类别">
      <UiButton v-for="(label, key) in vendorQueues" :key="key" :variant="section === key ? 'primary' : 'secondary'"
        :aria-pressed="section === key" @click="chooseQueue(key)">{{ label }}</UiButton>
    </nav>
    <div v-else-if="section === 'claims'" class="metric-grid workspace-finance-metrics">
      <UiMetricCard label="服务费总额" icon="quote" :value="money(state.totals.service_credit_cents)" />
      <UiMetricCard label="可申请" icon="request" :value="money(state.totals.available_service_cents)" />
      <UiMetricCard label="办理中" icon="time" :value="money(state.totals.claimed_service_cents)" />
      <UiMetricCard label="已报销" icon="check" :value="money(state.totals.paid_service_cents)" />
    </div>
    <UiCard v-if="draft" :title="draft.title">
      <form class="operations-form" @submit.prevent="send">
        <label v-if="!demo && !vendor && !draft.record">办理厂商
          <select v-model="draft.partnerID" required><option value="">选择厂商</option>
            <option v-for="item in eligiblePartners" :key="item.id" :value="item.id">
              {{ item.name }}{{ draft.kind === 'claims' ? ' · 可申请 ' + money(allowanceRemaining(item)) : '' }}
            </option>
          </select>
        </label>
        <label v-if="!vendor && draft.kind === 'invoices'">开票抬头<input v-model.trim="draft.invoiceTitle" maxlength="80" required></label>
        <label v-else-if="draft.kind !== 'allowances'">{{ vendor ? '办理凭证编号' : draft.kind === 'topups' ? (demo ? '模拟汇款凭证号' : '汇款凭证号') : '服务费发票号' }}<input v-model.trim="draft.reference" maxlength="80" required></label>
        <label v-if="draft.kind === 'allowances' || (!vendor && !draft.record)">金额（元）<input v-model.trim="draft.amount" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,2})?" maxlength="10" placeholder="0.00" required></label>
        <p v-if="formError" class="form-error" role="alert">{{ formError }}</p>
        <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">{{ draft.kind === 'allowances' ? '保存额度' : vendor ? '确认办理' : '提交申请' }}</UiButton><UiButton :disabled="busy" @click="draft = null">取消</UiButton></div>
      </form>
    </UiCard>
    <UiCard v-if="section === 'allowances'" title="合作代理商额度" description="每个代理商的额度单独计算，已提交的申请会占用额度。">
      <div v-if="partners.length" class="operations-table">
        <table><thead><tr><th scope="col">代理商</th><th scope="col">额度</th><th scope="col">剩余可申请</th><th scope="col">办理</th></tr></thead>
          <tbody><tr v-for="row in partners" :key="row.id"><td>{{ row.name }}</td><td>{{ money(row.limit_cents) }}</td>
            <td>{{ money(allowanceRemaining(row)) }}</td><td><UiButton compact :disabled="busy" @click="editAllowance(row)">设置额度</UiButton></td></tr></tbody>
        </table>
      </div>
      <UiState v-else title="暂无有直购订单的合作代理商" />
    </UiCard>
    <section v-else class="workspace-section" aria-labelledby="fund-records">
      <header class="workspace-section-heading">
        <h2 id="fund-records">{{ vendor ? queues[section] : typeNames[section] }}</h2>
        <div class="workspace-record-controls">
          <input v-model="query" type="search" aria-label="查询财务记录" placeholder="订单号、凭证、名称或状态">
          <UiButton v-if="!vendor && section === 'transactions' && (demo || partners.length)" variant="primary" :disabled="busy" @click="openAmountForm('topups')">{{ demo ? '登记模拟汇款' : '提交预存入账申请' }}</UiButton>
          <UiButton v-if="!vendor && section === 'claims'" variant="primary" :disabled="busy || !canClaim" @click="openAmountForm('claims')">申请服务费报销</UiButton>
        </div>
      </header>
      <div v-if="rows.length" class="table-scroll">
        <table class="workspace-record-table" role="table">
          <thead role="rowgroup"><tr role="row"><th scope="col" role="columnheader">项目</th><th scope="col" role="columnheader">金额</th><th scope="col" role="columnheader">日期</th><th scope="col" role="columnheader">凭证</th><th scope="col" role="columnheader">状态</th><th scope="col" role="columnheader">办理</th></tr></thead>
          <tbody role="rowgroup"><tr v-for="row in rows" :key="section === 'invoices' ? row.order_id : row.id" role="row">
            <td role="cell">{{ row.label }}<small v-if="row.order_id" class="operations-meta">订单 {{ row.order_id }}{{ row.detail ? ' · ' + row.detail : '' }}</small><small v-if="row.partner" class="operations-meta">{{ row.partner }}</small></td>
            <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">金额</span>{{ money(row.amount_cents) }}</td>
            <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">日期</span>{{ day(row.handled_at || row.created_at) }}</td>
            <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">凭证</span>{{ row.reference || '—' }}<small v-if="row.result_reference" class="operations-meta">{{ row.result_reference }}</small></td>
            <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">状态</span><UiBadge :tone="row.handled_at ? 'success' : (row.vendorAction || row.dealerAction ? 'warning' : 'info')" dot>{{ row.statusLabel }}</UiBadge></td>
            <td role="cell" class="workspace-record-action"><UiButton v-if="vendor ? row.vendorAction : row.dealerAction" compact :disabled="busy" @click="openRowForm(row)">{{ vendor ? row.vendorAction : row.dealerAction }}</UiButton></td>
          </tr></tbody>
        </table>
      </div>
      <UiState v-else :title="section === 'invoices' && !vendor && !query ? '订单完成收款后可申请开票' : '暂无符合条件的记录'" />
    </section>
  </template>
</template>
