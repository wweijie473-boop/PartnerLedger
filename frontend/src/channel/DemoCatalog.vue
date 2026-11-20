<script setup>
import { computed, ref } from 'vue'
import { ElDialog } from 'element-plus'
import { UiBadge, UiButton, UiCard, UiField, UiIconTile, UiState } from '../ui/index.js'
const props = defineProps({ items: Array, role: String, save: Function })
const search = ref('')
const filter = ref('')
const visible = computed(() => props.items.filter(item => (!filter.value || (item.published ? 'published' : 'draft') === filter.value) && [item.name, item.deployment, item.modules?.join('、')].filter(Boolean).join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))
const form = ref(null)
const busy = ref(false)
const error = ref('')
function edit(item) {
  error.value = ''
  form.value = { id: item?.id, name: item?.name || '', moduleText: item?.modules?.join('、') || '', deployment: item?.deployment || '', license: item?.license || '', published: item?.published || false }
}
async function submit(){
  if(busy.value) return
  busy.value = true; error.value = ''
  try {
    const {moduleText,...value} = form.value
    await props.save({...value, modules:moduleText.split(/[、，,\n]/).map(part => part.trim()).filter(Boolean)})
    form.value = null
  } catch (cause) { error.value = cause.message }
  finally { busy.value = false }
}
</script>
<template>
  <section v-enter="0" class="page-heading"><div><p class="ch-kicker">PRODUCT LIBRARY</p><h1>软件目录</h1><p class="muted">{{ role === 'vendor' ? '维护授权模式、部署选项和可供代理商询价的产品。' : '按业务模块筛选可询价的软件产品。' }}</p></div><UiButton v-if="role === 'vendor'" icon="add" variant="primary" @click="edit()">新增产品</UiButton></section>
  <div v-enter="1" class="ch-tools"><input v-model="search" type="search" aria-label="搜索产品" placeholder="搜索产品或模块" /><select v-if="role === 'vendor'" v-model="filter" aria-label="产品发布状态"><option value="">全部状态</option><option value="published">已发布</option><option value="draft">草稿</option></select></div>
  <div v-enter="2" class="catalog-grid"><UiCard v-for="item in visible" :key="item.id"><div class="ch-product-top"><UiIconTile name="product" /><UiBadge>{{ item.published ? '已发布' : '草稿' }}</UiBadge></div><h2 class="ch-product-name">{{ item.name }}</h2><p class="ch-metadata">{{ item.id }}</p><dl class="ch-facts"><dt>模块</dt><dd>{{ item.modules?.join('、') }}</dd><dt>部署</dt><dd>{{ item.deployment }}</dd><dt>授权</dt><dd>{{ item.license }}</dd></dl><template v-if="role === 'vendor'" #footer><UiButton compact @click="edit(item)">编辑产品</UiButton></template></UiCard></div>
  <UiState v-if="!visible.length" title="没有匹配的产品" description="调整搜索词或筛选条件。" />
  <ElDialog :model-value="!!form" :title="form?.id ? '编辑产品' : '新增产品'" width="min(92vw, 530px)" :close-on-click-modal="false" @update:model-value="value => { if (!value && !busy) form = null }">
    <form v-if="form" class="form-grid" @submit.prevent="submit">
      <UiField v-for="(label,key) in {name:'产品名称', moduleText:'业务模块（用顿号分隔）',deployment:'部署方式',license:'授权模式'}" :key="key" :label="label" required v-slot="{control}"><input v-bind="control" v-model="form[key]" required maxlength="200" /></UiField>
      <label class="ch-checkbox"><input v-model="form.published" type="checkbox" /> 发布至代理商目录</label>
      <p v-if="error" role="alert" class="form-error">{{ error }}</p><div class="modal-actions"><UiButton :disabled="busy" @click="form = null">取消</UiButton><UiButton type="submit" :loading="busy" variant="primary">保存产品</UiButton></div>
    </form>
  </ElDialog>
</template>
