<script setup>
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { UiButton, UiField, UiState } from '../ui/index.js'

const props = defineProps({
  productId: { type: String, required: true },
  request: { type: Function, required: true },
  editable: Boolean,
})

const deploymentNames = { private: '私有化部署', on_premises: '本地部署', cloud: '云部署', saas: 'SaaS' }
const profile = ref(null)
const loading = ref(false)
const loadError = ref('')
const editing = ref(false)
const saving = ref(false)
const saveError = ref('')
const saved = ref(false)
const form = reactive({ category: '', modules: '', modes: [], minimum: '', maximum: '', note: '', sources: [] })
const path = `/api/products/${encodeURIComponent(props.productId)}/knowledge`
const controller = new AbortController()
let nextSourceKey = 0

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    profile.value = await props.request(path, { signal: controller.signal })
  } catch (error) {
    if (!controller.signal.aborted) loadError.value = error.message
  } finally {
    loading.value = false
  }
}

function edit() {
  const item = profile.value
  Object.assign(form, {
    category: item.category, modules: item.modules.join('、'), modes: [...item.deployment_modes],
    minimum: item.license.minimum ?? '', maximum: item.license.maximum ?? '', note: item.license.note,
    sources: item.sources.map(source => ({ ...source, key: nextSourceKey++ })),
  })
  saveError.value = ''
  saved.value = false
  editing.value = true
}

function addSource() {
  form.sources.push({ key: nextSourceKey++, title: '', reference: '', excerpt: '' })
}

async function save() {
  if (saving.value) return
  saving.value = true
  saveError.value = ''
  const body = {
    category: form.category,
    modules: form.modules.split(/[\r\n,，、]+/).map(value => value.trim()).filter(Boolean),
    deployment_modes: form.modes,
    license: { minimum: form.minimum === '' ? null : form.minimum, maximum: form.maximum === '' ? null : form.maximum, note: form.note },
    sources: form.sources.map(({ title, reference, excerpt }) => ({ title, reference, excerpt })),
  }
  try {
    profile.value = await props.request(path, { method: 'PUT', body, signal: controller.signal })
    editing.value = false
    saved.value = true
  } catch (error) {
    if (!controller.signal.aborted) saveError.value = error.message
  } finally {
    saving.value = false
  }
}

onMounted(load)
onUnmounted(() => controller.abort())
</script>

<template>
  <section class="product-knowledge">
    <h3>匹配与授权资料</h3>
    <UiState v-if="loading" state="loading" title="正在读取资料" />
    <UiState v-else-if="loadError" state="error" title="无法读取资料" :description="loadError">
      <UiButton compact @click="load">重试</UiButton>
    </UiState>
    <form v-else-if="editing" class="form-grid" @submit.prevent="save">
      <UiField label="产品分类" v-slot="{ control }"><input v-bind="control" v-model="form.category" maxlength="80" /></UiField>
      <UiField label="支持模块" hint="以顿号、逗号或换行分隔，每项最多 70 字。" v-slot="{ control }">
        <textarea v-bind="control" v-model="form.modules" rows="2" maxlength="2130"></textarea>
      </UiField>
      <fieldset>
        <legend>部署方式</legend>
        <div class="modes">
          <label v-for="(name, code) in deploymentNames" :key="code"><input v-model="form.modes" type="checkbox" :value="code" /> {{ name }}</label>
        </div>
      </fieldset>
      <div class="license-bounds">
        <UiField label="最低授权席位" v-slot="{ control }"><input v-bind="control" v-model.number="form.minimum" type="number" min="1" max="1000000" placeholder="待补充" /></UiField>
        <UiField label="最高授权席位" v-slot="{ control }"><input v-bind="control" v-model.number="form.maximum" type="number" min="1" max="1000000" placeholder="待补充" /></UiField>
      </div>
      <UiField label="授权说明" v-slot="{ control }"><textarea v-bind="control" v-model="form.note" maxlength="500" rows="2"></textarea></UiField>
      <fieldset v-for="(source, index) in form.sources" :key="source.key" class="form-grid">
        <legend>资料来源 {{ index + 1 }}</legend>
        <UiField label="资料名称" required v-slot="{ control }"><input v-bind="control" v-model="source.title" maxlength="120" required /></UiField>
        <UiField label="链接、文件名或页码" required v-slot="{ control }"><input v-bind="control" v-model="source.reference" maxlength="300" required /></UiField>
        <UiField label="摘录" required v-slot="{ control }"><textarea v-bind="control" v-model="source.excerpt" maxlength="500" rows="2" required></textarea></UiField>
        <UiButton compact :disabled="saving" @click="form.sources.splice(index, 1)">移除来源</UiButton>
      </fieldset>
      <div><UiButton compact :disabled="saving || form.sources.length >= 10" @click="addSource">添加资料来源</UiButton></div>
      <p v-if="saveError" class="form-error" role="alert">{{ saveError }}</p>
      <div class="knowledge-actions">
        <UiButton :disabled="saving" @click="editing = false">取消</UiButton>
        <UiButton type="submit" variant="primary" :loading="saving">保存匹配资料</UiButton>
      </div>
    </form>
    <template v-else-if="profile">
      <dl class="details">
        <dt>产品分类</dt><dd>{{ profile.category || '待补充' }}</dd>
        <dt>支持模块</dt><dd>{{ profile.modules.join('、') || '待补充' }}</dd>
        <dt>部署方式</dt><dd>{{ profile.deployment_modes.map(code => deploymentNames[code]).join('、') || '待补充' }}</dd>
        <dt>授权席位</dt><dd>最低 {{ profile.license.minimum ?? '待补充' }} · 最高 {{ profile.license.maximum ?? '待补充' }}</dd>
        <template v-if="profile.license.note"><dt>授权说明</dt><dd>{{ profile.license.note }}</dd></template>
      </dl>
      <ul v-if="profile.sources.length" class="sources" aria-label="资料来源">
        <li v-for="(source, index) in profile.sources" :key="index">
          <strong>{{ source.title }}</strong> · {{ source.reference }}<br />{{ source.excerpt }}
        </li>
      </ul>
      <p v-if="saved" role="status">匹配资料已保存。</p>
      <UiButton v-if="editable" compact @click="edit">编辑匹配资料</UiButton>
    </template>
  </section>
</template>

<style scoped>
.product-knowledge { margin-top: var(--ui-space-5); }
.product-knowledge h3 { margin-bottom: var(--ui-space-3); }
.product-knowledge fieldset { min-width: 0; border: 1px solid var(--ui-border-strong); border-radius: var(--ui-radius-sm); }
.modes, .knowledge-actions { display: flex; flex-wrap: wrap; gap: var(--ui-space-3); }
.license-bounds { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--ui-space-3); }
.sources { overflow-wrap: anywhere; }
</style>
