<script setup>
import { computed, ref } from 'vue'
import { UiButton, UiCard, UiState } from '../ui/index.js'
import { renewalDays } from './renewalDates.js'

const props = defineProps({ rows: Array, busy: Boolean, submit: Function })
const range = ref('all')
const search = ref('')
const draft = ref(null)
const date = value => value ? new Date(value).toLocaleDateString('zh-CN', { timeZone: 'Asia/Shanghai' }) : '—'
const sourceLabel = { promotion: '扫码购买', assisted: '代客下单' }
const rows = computed(() => props.rows.map(row => ({ ...row, days: renewalDays(row.due_at) })).filter(row => {
  const text = [row.customer, row.product_name, row.product_version, row.order_id, sourceLabel[row.source]].join(' ').toLowerCase()
  if (!text.includes(search.value.trim().toLowerCase())) return false
  if (range.value === 'all') return true
  if (range.value === 'overdue') return row.days < 0
  return row.days >= 0 && row.days <= Number(range.value)
}).sort((left, right) => left.due_at.localeCompare(right.due_at)))

async function save() {
  const { id, note } = draft.value
  if (await props.submit('renewals/' + id, { note }, '续费跟进已保存。')) draft.value = null
}
</script>

<template>
  <UiCard v-if="draft" :title="'跟进续费 · ' + draft.customer">
    <form class="operations-form" @submit.prevent="save">
      <label>沟通结果与下一步<textarea v-model.trim="draft.note" required maxlength="300" /></label>
      <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">保存跟进</UiButton><UiButton :disabled="busy" @click="draft = null">取消</UiButton></div>
    </form>
  </UiCard>
  <UiCard title="年度授权到期" description="扫码购买和代客下单在确认收款后进入续费台账。">
    <div class="operations-actions">
      <label class="operations-search">搜索订单<input v-model="search" type="search" placeholder="客户、产品、订单号或来源"></label>
      <label class="operations-search">到期范围<select v-model="range"><option value="all">全部</option><option value="7">7 天内</option><option value="30">30 天内</option><option value="90">90 天内</option><option value="overdue">已到期</option></select></label>
    </div>
    <div v-if="rows.length" class="operations-table">
      <table><thead><tr><th scope="col">客户 / 产品</th><th scope="col">订单 / 来源</th><th scope="col">购买日期</th><th scope="col">到期日 / 剩余天数</th><th scope="col">最近跟进</th><th scope="col">操作</th></tr></thead>
        <tbody><tr v-for="row in rows" :key="row.id">
          <td>{{ row.customer }}<small class="operations-meta">{{ row.product_name }} · {{ row.product_version }} · {{ row.quantity }} 份</small></td>
          <td>{{ row.order_id }}<small class="operations-meta">{{ sourceLabel[row.source] }}</small></td>
          <td>{{ date(row.paid_at) }}</td>
          <td>{{ date(row.due_at) }}<small class="operations-meta">{{ row.days === 0 ? '今天到期' : row.days > 0 ? `剩余 ${row.days} 天` : `已逾期 ${-row.days} 天` }}</small></td>
          <td>{{ row.note || '未跟进' }}<small v-if="row.contacted_at" class="operations-meta">{{ date(row.contacted_at) }}</small></td>
          <td><UiButton variant="text" compact :disabled="busy" @click="draft = { ...row }">记录跟进</UiButton></td>
        </tr></tbody>
      </table>
    </div>
    <UiState v-else title="没有符合条件的购买记录" />
  </UiCard>
</template>
