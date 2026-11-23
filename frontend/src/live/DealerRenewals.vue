<script setup>
import { computed, ref } from 'vue'
import { UiButton, UiCard, UiState } from '../ui/index.js'

const props = defineProps({ rows: Array, busy: Boolean, submit: Function })
const dayMs = 86400000
const range = ref('30')
const draft = ref(null)
const rows = computed(() => props.rows.filter(row => {
  const remaining = Date.parse(row.due_at) - Date.now()
  if (range.value === 'all') return true
  if (range.value === 'overdue') return remaining < 0
  return remaining >= 0 && remaining <= Number(range.value) * dayMs
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
  <UiCard title="年度授权到期" description="年度样例产品在确认收款后进入续费台账。">
    <label class="operations-search">到期范围<select v-model="range"><option value="7">7 天内</option><option value="30">30 天内</option><option value="90">90 天内</option><option value="overdue">已到期</option><option value="all">全部</option></select></label>
    <div v-if="rows.length" class="operations-table">
      <table><thead><tr><th scope="col">客户 / 产品</th><th scope="col">到期日</th><th scope="col">最近跟进</th><th scope="col">操作</th></tr></thead>
        <tbody><tr v-for="row in rows" :key="row.id">
          <td>{{ row.customer }}<small class="operations-meta">{{ row.product_name }}{{ row.order_id ? ' · 订单 ' + row.order_id : '' }}</small></td>
          <td>{{ new Date(row.due_at).toLocaleDateString('zh-CN') }}</td>
          <td>{{ row.note || '未跟进' }}<small v-if="row.contacted_at" class="operations-meta">{{ new Date(row.contacted_at).toLocaleDateString('zh-CN') }}</small></td>
          <td><UiButton variant="text" compact :disabled="busy" @click="draft = { ...row }">记录跟进</UiButton></td>
        </tr></tbody>
      </table>
    </div>
    <UiState v-else title="当前范围没有到期记录" />
  </UiCard>
</template>
