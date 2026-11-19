<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElDialog } from 'element-plus'
import { UiBadge, UiButton, UiCard, UiField, UiIconTile, UiState } from '../ui/index.js'

const props = defineProps({
  request: { type: Function, required: true },
  isManufacturer: Boolean,
})

const products = ref([])
const searchText = ref('')
const publication = ref('all')
const matches = computed(() => {
  const query = searchText.value.trim().toLocaleLowerCase()
  return products.value.filter(item =>
    (publication.value === 'all' || item.published === (publication.value === 'published')) &&
    [item.name, item.version, item.description].some(value => value?.toLocaleLowerCase().includes(query)))
})
const cursor = ref(null)
const loaded = ref(false)
const loading = ref(false)
const listError = ref(null)
const panel = ref('')
const activeId = ref(null)
const product = ref(null)
const detailError = ref(null)
const detailLoading = ref(false)
const saving = ref(false)
const formError = ref('')
const notice = ref('')
const form = reactive({ name: '', version: '', description: '', published: false })
const pageController = new AbortController()
let detailController

const dialogOpen = computed({
  get: () => panel.value !== '',
  set: (open) => { if (!open && !saving.value) closePanel() },
})
const title = computed(() => panel.value === 'editor' ? (activeId.value ? '编辑产品' : '新增产品') : '产品详情')

async function loadProducts(append = false) {
  if (loading.value || (append && cursor.value === null)) return
  loading.value = true
  listError.value = null
  const params = new URLSearchParams({ limit: '20' })
  if (append) params.set('after', cursor.value)
  try {
    const page = await props.request(`/api/products?${params}`, { signal: pageController.signal })
    const known = new Set(append ? products.value.map((item) => item.id) : [])
    const items = page.items.filter((item) => !known.has(item.id))
    products.value = append ? [...products.value, ...items] : items
    cursor.value = page.next_cursor
    loaded.value = true
  } catch (error) {
    if (error.name !== 'AbortError') listError.value = error
  } finally {
    loading.value = false
  }
}

async function openDetail(id) {
  detailController?.abort()
  const controller = new AbortController()
  detailController = controller
  panel.value = 'detail'
  activeId.value = id
  product.value = null
  detailError.value = null
  detailLoading.value = true
  try {
    const record = await props.request(`/api/products/${encodeURIComponent(id)}`, { signal: controller.signal })
    if (detailController === controller) product.value = record
  } catch (error) {
    if (detailController === controller && error.name !== 'AbortError') detailError.value = error
  } finally {
    if (detailController === controller) detailLoading.value = false
  }
}

function closePanel() {
  detailController?.abort()
  detailController = undefined
  panel.value = ''
}

onMounted(() => loadProducts())
onUnmounted(() => {
  pageController.abort()
  detailController?.abort()
})

function openEditor(item) {
  if (!props.isManufacturer) return
  activeId.value = item?.id ?? null
  Object.assign(form, {
    name: item?.name ?? '', version: item?.version ?? '',
    description: item?.description ?? '', published: item?.published ?? false,
  })
  formError.value = ''
  detailError.value = null
  notice.value = ''
  panel.value = 'editor'
}

async function saveProduct() {
  if (!props.isManufacturer || saving.value) return
  formError.value = ''
  const body = { name: form.name.trim(), version: form.version.trim(), description: form.description.trim(), published: form.published }
  if (!body.name || !body.version) {
    formError.value = '请填写产品名称和版本。'
    return
  }
  saving.value = true
  const id = activeId.value
  try {
    const saved = await props.request(id ? `/api/products/${encodeURIComponent(id)}` : '/api/products', {
      method: id ? 'PUT' : 'POST', body, signal: pageController.signal,
    })
    const index = products.value.findIndex((item) => item.id === saved.id)
    if (index < 0) products.value.unshift(saved)
    else products.value.splice(index, 1, saved)
    product.value = saved
    activeId.value = saved.id
    panel.value = 'detail'
    notice.value = id ? '产品修改已保存。' : '产品已创建。'
    if (!loaded.value) await loadProducts()
  } catch (error) {
    if (error.name !== 'AbortError') formError.value = error.message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <section class="page-heading">
    <div><h1>产品目录</h1><p class="muted">{{ isManufacturer ? '维护本组织产品及其发布状态。' : '查看厂商已发布的软件和版本。' }}</p></div>
    <UiButton v-if="isManufacturer" variant="primary" icon="add" :disabled="loading" @click="openEditor()">新增产品</UiButton>
  </section>

  <p v-if="notice" class="catalog-feedback" role="status">{{ notice }}</p>
  <div v-if="products.length" class="catalog-filters">
    <label>查找已加载产品 <input v-model="searchText" type="search" placeholder="名称、版本或说明" /></label>
    <label v-if="isManufacturer">发布状态 <select v-model="publication"><option value="all">全部</option><option value="published">已发布</option><option value="draft">未发布</option></select></label>
  </div>
  <UiCard v-if="!loaded && (loading || !listError)" flush><UiState state="loading" title="正在加载产品目录" /></UiCard>
  <UiCard v-else-if="listError && !loaded" flush>
    <UiState :state="listError.status === 403 ? 'denied' : 'error'" title="无法加载产品目录" :description="listError.message">
      <UiButton compact @click="loadProducts()">重试</UiButton>
    </UiState>
  </UiCard>
  <UiCard v-else-if="!products.length" flush>
    <UiState title="暂无可见产品" :description="isManufacturer ? '创建本组织的第一个软件产品。' : '厂商发布产品后会显示在这里。'" />
  </UiCard>
  <UiCard v-else-if="!matches.length" flush><UiState title="已加载产品中没有匹配项" :description="cursor !== null ? '可加载更多产品，或调整筛选条件。' : '请调整搜索词或筛选条件。'" /></UiCard>
  <section v-else class="catalog-grid" aria-label="产品目录">
    <UiCard v-for="item in matches" :key="item.id" as="article" :title="item.name" :description="`版本 ${item.version}`">
      <template #actions><UiIconTile name="product" /></template>
      <p class="catalog-description muted">{{ item.description || '暂无产品说明' }}</p>
      <UiBadge v-if="isManufacturer" class="catalog-status" :tone="item.published ? 'success' : 'neutral'" dot>
        {{ item.published ? '已发布' : '未发布' }}
      </UiBadge>
      <template #footer><UiButton variant="text" end-icon="forward" @click="openDetail(item.id)">查看产品</UiButton></template>
    </UiCard>
  </section>
  <div v-if="loaded && cursor !== null" class="catalog-more">
    <UiButton :loading="loading" @click="loadProducts(true)">{{ listError ? '重试加载' : '加载更多' }}</UiButton>
    <p v-if="listError" class="form-error" role="alert">{{ listError.message }}</p>
  </div>

  <ElDialog v-model="dialogOpen" :title="title" width="min(584px, calc(100vw - 28px))" align-center destroy-on-close
    :show-close="!saving" :close-on-press-escape="!saving" :close-on-click-modal="false">
    <template v-if="panel === 'detail'">
      <UiState v-if="detailLoading" state="loading" title="正在读取产品" />
      <UiState v-else-if="detailError" :state="[403, 404].includes(detailError.status) ? 'denied' : 'error'"
        title="无法查看产品" :description="detailError.message">
        <UiButton compact @click="openDetail(activeId)">重试</UiButton>
      </UiState>
      <template v-else-if="product">
        <div class="detail-heading">
          <h2>{{ product.name }}</h2>
          <UiBadge v-if="isManufacturer" :tone="product.published ? 'success' : 'neutral'" dot>{{ product.published ? '已发布' : '未发布' }}</UiBadge>
        </div>
        <dl class="details">
          <dt>版本</dt><dd>{{ product.version }}</dd>
          <dt>产品说明</dt><dd>{{ product.description || '暂无' }}</dd>
        </dl>
        <div v-if="isManufacturer" class="modal-actions"><UiButton variant="primary" @click="openEditor(product)">编辑产品</UiButton></div>
      </template>
    </template>

    <form v-else-if="panel === 'editor'" class="form-grid" @submit.prevent="saveProduct">
      <UiField label="产品名称" required v-slot="{ control }"><input v-bind="control" v-model="form.name" maxlength="150" required /></UiField>
      <UiField label="版本" required v-slot="{ control }"><input v-bind="control" v-model="form.version" maxlength="100" required /></UiField>
      <UiField label="产品说明" v-slot="{ control }"><textarea v-bind="control" v-model="form.description" maxlength="2000" rows="5"></textarea></UiField>
      <label class="publish-option"><input v-model="form.published" type="checkbox" /> 发布给代理商</label>
      <p v-if="formError" class="form-error" role="alert">{{ formError }}</p>
      <div class="modal-actions">
        <UiButton :disabled="saving" @click="closePanel">取消</UiButton>
        <UiButton type="submit" variant="primary" :loading="saving">保存产品</UiButton>
      </div>
    </form>
  </ElDialog>
</template>

<style scoped>
.catalog-description { display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; overflow-wrap: anywhere; min-height: 3em; font-size: var(--ui-text-sm); line-height: 1.5; }
.catalog-status { margin-top: var(--ui-space-3); }
.catalog-filters, .catalog-filters label { display: flex; align-items: center; flex-wrap: wrap; gap: var(--ui-space-2); font-size: var(--ui-text-sm); }
.catalog-filters { gap: var(--ui-space-4); margin-bottom: var(--ui-space-4); }
.catalog-filters :is(input, select) { border: 1px solid var(--ui-border-strong); border-radius: var(--ui-radius-sm); padding: 7px 9px; color: var(--ui-ink); background: var(--ui-surface); max-width: 100%; }
.catalog-feedback, .catalog-more { margin-block: var(--ui-space-4); font-size: var(--ui-text-sm); }
.catalog-more { display: flex; align-items: center; gap: var(--ui-space-3); }
.publish-option { display: flex; align-items: center; gap: var(--ui-space-2); font-size: var(--ui-text-sm); }
</style>
