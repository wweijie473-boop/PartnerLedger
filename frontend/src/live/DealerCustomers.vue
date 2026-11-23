<script setup>
import { computed, ref } from 'vue'
import { UiButton, UiCard, UiState } from '../ui/index.js'

const props = defineProps({ state: Object, busy: Boolean, submit: Function })
const emit = defineEmits(['navigate'])
const search = ref('')
const draft = ref(null)
const rows = computed(() => props.state.customers.filter(row => [row.name, row.contact, row.note]
  .join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))

async function save() {
  const { id, name, contact, note } = draft.value
  if (await props.submit(id ? 'customers/' + id : 'customers', { name, contact, note }, '客户档案已保存，可在代客下单时选择该客户。')) {
    draft.value = null
  }
}
</script>

<template>
  <UiCard v-if="draft" :title="draft.id ? '编辑客户档案' : '新增客户档案'">
    <form class="operations-form" @submit.prevent="save">
      <label>企业名称<input v-model.trim="draft.name" required maxlength="80"></label>
      <label>联系人<input v-model.trim="draft.contact" maxlength="40"></label>
      <label>服务备注<textarea v-model.trim="draft.note" maxlength="200" /></label>
      <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">保存客户</UiButton><UiButton :disabled="busy" @click="draft = null">取消</UiButton></div>
    </form>
  </UiCard>
  <UiCard title="客户列表">
    <label class="operations-search">搜索客户<input v-model="search" type="search" placeholder="企业、联系人或备注"></label>
    <div class="operations-actions">
      <UiButton variant="primary" :disabled="busy" @click="draft = { name: '', contact: '', note: '' }">新增客户</UiButton>
      <UiButton @click="emit('navigate', 'assisted')">去代客下单</UiButton>
    </div>
    <div v-if="rows.length" class="operations-table">
      <table><thead><tr><th scope="col">企业</th><th scope="col">联系人</th><th scope="col">服务备注</th><th scope="col">操作</th></tr></thead>
        <tbody><tr v-for="row in rows" :key="row.id"><td>{{ row.name }}</td><td>{{ row.contact || '—' }}</td><td>{{ row.note || '—' }}</td><td><UiButton compact variant="text" :disabled="busy" @click="draft = { ...row }">编辑</UiButton></td></tr></tbody>
      </table>
    </div>
    <UiState v-else title="没有匹配的客户档案" />
  </UiCard>
</template>
