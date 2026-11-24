<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { requestApi } from '../api.js'
import { UiBadge, UiButton, UiCard, UiMetricCard, UiState } from '../ui/index.js'
import { formatCents, quoteCents } from '../channel/quoteMoney.js'
import { demoStore } from './retailDemo.js'
import {
  clearRetailCheckout, customerOrderLink, pendingRetailOrder, readRetailCatalog, readRetailCheckout,
  retailApi as api, retailCheckoutStorageKey, saveRetailCheckout, withRetailCheckoutLock,
} from './retailRequest.js'

const props = defineProps({ mode: String, title: String, role: String, request: Function, identity: String, demo: Boolean })
const emit = defineEmits(['navigate'])
const params = new URLSearchParams(location.search)
const code = params.get('code') ?? ''
const orderID = !props.demo && props.mode === 'retail-orders' ? params.get('order_id') : null
const promotion = props.mode === 'promotion'
const assisted = props.mode === 'assisted'
const checkout = props.mode === 'checkout'
const pricing = props.mode === 'pricing'
const ledger = ['retail-orders', 'commission', 'manufacturer-receipts', 'manufacturer-settlements'].includes(props.mode)
const vendor = props.role === 'vendor'
const dealer = props.role === 'dealer'
const capable = /^[a-f0-9]{32}$/.test(code)
const storefront = '/retail-store?' + new URLSearchParams({ code })
const checkoutMode = assisted || props.mode === 'retail-orders' && dealer ? 'assisted' : 'checkout'
const orderJournalKey = 'partnerledger-retail-checkout:' + props.identity + ':' + checkoutMode
const recoverable = !props.demo && (assisted || checkout || props.mode === 'retail-orders' && !vendor)
const pendingCheckout = ref(recoverable ? readRetailCheckout(orderJournalKey) : null)
const rejectedCheckout = ref(false)
const controller = new AbortController()
const read = (path) => (props.request ?? requestApi)(path, { signal: controller.signal })
const write = (path, body, method = 'POST') => props.request(api + path, { method, body })
const offers = ref([])
const products = ref([])
const customers = ref([])
const orders = ref([])
const summary = ref(null)
const next = ref(null)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const product = ref(checkout ? params.get('product') ?? '' : '')
const customer = ref('')
const maker = ref('')
const quantity = ref(1)
const unit = ref('')
const rate = ref('50')
const active = ref(true)
const annual = ref(false)
const href = ref('')
const qr = ref('')
const status = ref('')
const actionOrder = ref('')
const reference = ref('')
const labels = {
  confirmation: '待客户确认', payment: '待厂商确认收款', paid: '厂商已确认收款',
  settlement: '待厂商结算', settled: '已登记结算', declined: '客户已拒绝',
}
const tones = { confirmation: 'warning', payment: 'warning', paid: 'success', settlement: 'info', settled: 'success', declined: 'neutral' }
const makers = computed(() => [...new Map(offers.value.map(o => [o.manufacturer_id, o.manufacturer])).entries()])
const selected = computed(() => offers.value.find(o => o.product_id === product.value))
const total = computed(() => selected.value && Number.isInteger(quantity.value) ? selected.value.unit_cents * quantity.value : 0)
const money = value => formatCents(BigInt(value))
const settlementView = ['commission', 'manufacturer-settlements'].includes(props.mode)
const receiptView = props.mode === 'manufacturer-receipts'
const statusOptions = Object.entries(labels).filter(([key]) => {
  if (settlementView) return ['settlement', 'settled'].includes(key)
  return props.role === 'customer' ? !['settlement', 'settled'].includes(key) : key !== 'paid'
})
const localPreview = ['localhost', '127.0.0.1', '[::1]'].includes(location.hostname)
const clipboardAvailable = Boolean(navigator.clipboard)
const canRemind = row => !row.commission?.reminded_at ||
  Date.now() - Date.parse(row.commission.reminded_at) >= 86400000

async function list(cursor = '') {
  const query = new URLSearchParams()
  if (cursor) query.set('before', cursor)
  if (orderID !== null) query.set('order_id', orderID)
  else if (status.value) query.set('status', status.value)
  if (settlementView) query.set('view', 'commission')
  const result = await read(api + '/orders?' + query)
  orders.value = cursor ? [...orders.value,...result.items] : result.items
  summary.value = result.summary || null
  next.value = result.next_cursor
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    if (ledger) await list()
    else if (pricing) {
      const [configured, owned] = await Promise.all([read(api + '/manufacturer-offers'),readRetailCatalog(read, '/api/products?limit=100')])
      offers.value = configured.items
      products.value = owned
    } else if (checkout) {
      if (!capable) throw new Error('推广链接无效。')
      const catalog = await readRetailCatalog(read, api + '/store/' + code)
      if (params.has('product') && !catalog.some(item => item.product_id === params.get('product'))) {
        offers.value = []
        throw new Error('链接所选产品已下架，请返回厂商商城重新选择。')
      }
      offers.value = catalog
      if (!selected.value) product.value = catalog[0]?.product_id ?? ''
    } else {
      offers.value = await readRetailCatalog(read, api + '/offers')
      maker.value = makers.value[0]?.[0] ?? ''
      if (assisted) customers.value = (await read('/api/channel/v1/organization-links')).items
    }
  } catch (cause) {
    if (!controller.signal.aborted) error.value = cause.message
  } finally {
    loading.value = false
  }
}
function refreshOnFocus() {
  refreshPendingCheckout()
  if (!busy.value && !loading.value) load()
}
function refreshPendingCheckout() {
  if (!recoverable) return
  const current = readRetailCheckout(orderJournalKey)
  if (pendingCheckout.value?.request_id !== current?.request_id) rejectedCheckout.value = false
  pendingCheckout.value = current
}
function checkoutStorageChanged(event) {
  if (event.key === null || event.key === retailCheckoutStorageKey(orderJournalKey)) refreshPendingCheckout()
}
onMounted(() => {
  load()
  if (ledger) window.addEventListener('focus', refreshOnFocus)
  if (recoverable) window.addEventListener('storage', checkoutStorageChanged)
})
onUnmounted(() => {
  controller.abort()
  window.removeEventListener('focus', refreshOnFocus)
  window.removeEventListener('storage', checkoutStorageChanged)
})

async function run(task) {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  error.value = ''
  try { await task() } catch (cause) { if (!controller.signal.aborted) error.value = cause.message }
  finally { busy.value = false }
}
function generate() {
  run(async () => {
    const result = await write('/promotion', { manufacturer_id:maker.value })
    const url = new URL(props.demo ? demoStore : '/retail-store', location.origin)
    url.searchParams.set('code', result.code)
    const { default: qrCode } = await import('qrcode')
    const image = await qrCode.toDataURL(url.href, { width:240, margin:2 })
    href.value = url.href
    qr.value = image
  })
}
function copyCustomerLink(id) {
  run(async () => {
    await navigator.clipboard.writeText(new URL(customerOrderLink(id), location.origin).href)
    notice.value = '客户确认链接已复制。'
  })
}
function copyLink() {
  run(async () => {
    if (!navigator.clipboard) {
      const field = document.querySelector('input[aria-label="推广链接"]')
      field?.select()
      notice.value = '链接已选中，可以手动复制。'
      return
    }
    await navigator.clipboard.writeText(href.value)
    notice.value = '推广链接已复制。'
  })
}
function choosePrice(id) {
  const row = offers.value.find(item => item.product_id === id)
  unit.value = row ? (row.unit_cents / 100).toFixed(2) : ''
  rate.value = row ? String(row.commission_bps / 100) : '50'
  active.value = row?.active ?? true
  annual.value = row?.annual ?? false
}
function savePrice() {
  run(async () => {
    const moneyFormat = /^\d+(\.\d{1,2})?$/
    if (!moneyFormat.test(unit.value) || !moneyFormat.test(rate.value)) throw new Error('单价和比例最多支持两位小数。')
    const unitCents = Number(quoteCents(unit.value))
    const rateBps = Number(quoteCents(rate.value))
    if (!(unitCents > 0 && unitCents <= 100000000 && rateBps <= 10000)) throw new Error('请填写有效单价和 0–100% 的分佣比例。')
    const row = await write('/manufacturer-offers/' + product.value, {
      unit_cents: unitCents, commission_bps: rateBps, active: active.value, annual: annual.value,
    }, 'PUT')
    offers.value = [...offers.value.filter(o=>o.product_id!==row.product_id),row]
    notice.value = '订购规则已保存。'
  })
}
function submitOrder() {
  run(async () => {
    const item = selected.value
    if (!item || !Number.isInteger(quantity.value) || quantity.value < 1 || quantity.value > 10000) return
    const draft = { product_id:item.product_id, quantity:quantity.value, expected_unit_cents:item.unit_cents,
      ...(assisted ? { customer_id:customer.value } : { code }) }
    if (props.demo) {
      await write('/orders', pendingRetailOrder(orderJournalKey, draft, item))
      sessionStorage.removeItem(orderJournalKey)
      emit('navigate', 'retail-orders')
      return
    }
    await withRetailCheckoutLock(orderJournalKey, async () => {
      refreshPendingCheckout()
      if (pendingCheckout.value) return
      const saved = saveRetailCheckout(orderJournalKey, draft, item)
      pendingCheckout.value = saved
      await sendSavedOrder(saved)
    })
  })
}

function openSavedOrder(result, saved) {
  if (typeof result?.id !== 'string' || !/^[1-9]\d*$/.test(result.id)) throw new Error('无法识别订单编号，请重新核对。')
  pendingCheckout.value = clearRetailCheckout(orderJournalKey, saved.request_id)
  emit('navigate', 'retail-orders', { order_id: result.id })
}

async function sendSavedOrder(saved) {
  try {
    openSavedOrder(await write('/orders', saved), saved)
  } catch (cause) {
    if (['retail_conflict', 'not_found', 'invalid_request'].includes(cause.code)) {
      const existing = await lookupSavedOrder(saved)
      if (existing) {
        openSavedOrder(existing, saved)
        return
      }
      rejectedCheckout.value = true
    }
    throw cause
  }
}

async function lookupSavedOrder(saved) {
  try {
    return await read(api + '/orders/requests/' + encodeURIComponent(saved.request_id))
  } catch (cause) {
    if (cause.status === 404 && cause.code === 'not_found') return null
    throw cause
  }
}

function recoverOrder() {
  run(() => withRetailCheckoutLock(orderJournalKey, async () => {
    refreshPendingCheckout()
    const saved = pendingCheckout.value
    if (!saved) return
    const result = await lookupSavedOrder(saved)
    if (result) openSavedOrder(result, saved)
    else if (!rejectedCheckout.value) await sendSavedOrder(saved)
  }))
}

function discardRejectedOrder() {
  run(() => withRetailCheckoutLock(orderJournalKey, async () => {
    refreshPendingCheckout()
    if (!rejectedCheckout.value || !pendingCheckout.value) return
    const saved = pendingCheckout.value
    const result = await lookupSavedOrder(saved)
    if (result) {
      openSavedOrder(result, saved)
      return
    }
    pendingCheckout.value = clearRetailCheckout(orderJournalKey, saved.request_id)
    rejectedCheckout.value = false
    await load()
  }))
}
function advance(row, action, refValue) {
  run(async () => {
    await write('/orders/' + row.id + '/' + action, refValue ? { reference:refValue } : {})
    if (controller.signal.aborted) return
    reference.value = ''
    actionOrder.value = ''
    notice.value = { remind:'已向厂商登记催结算提醒。', receipt:'已登记厂商收款。', settle:'已登记佣金结算。', confirm:'已确认代下订单。', decline:'已拒绝代下订单。' }[action]
    await list()
  })
}
</script>

<template>
    <header class="page-heading"><h1>{{ title }}</h1><UiButton compact variant="text" :disabled="loading || busy" @click="load">刷新</UiButton></header>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
    <UiButton v-if="orderID !== null" compact @click="emit('navigate', 'retail-orders')">查看全部订单</UiButton>
    <p v-if="checkout && capable"><a :href="storefront">返回厂商商城</a></p>
    <UiCard v-if="pendingCheckout" title="待核对订单提交">
      <p class="muted">
        原订单商品编号 {{ pendingCheckout.product_id }} · {{ pendingCheckout.quantity }} 份 · 单价 {{ money(pendingCheckout.expected_unit_cents) }}
      </p>
      <p v-if="rejectedCheckout" role="status">服务端未接受原请求。可清除记录，重新核对当前订购信息。</p>
      <p v-else>本次核对沿用原订单的价格、数量和请求编号。</p>
      <div class="retail-actions">
        <UiButton variant="primary" :loading="busy" @click="recoverOrder">{{ rejectedCheckout ? '再次查询原订单' : '核对并重试原请求' }}</UiButton>
        <UiButton v-if="rejectedCheckout" :disabled="busy" @click="discardRejectedOrder">清除记录并重新核对</UiButton>
      </div>
    </UiCard>
    <UiState v-if="loading" state="loading" title="正在读取业务记录" />

    <UiCard v-else-if="promotion" title="厂商专属推广码">
      <p class="muted">{{ demo ? '客户扫码进入厂商演示官网。付款归厂商，订单自动归属当前代理商。' : '客户扫码进入本平台的厂商订购专区。通过该入口提交的订单会归属当前代理商。' }}</p>
      <form class="retail-form" @submit.prevent="generate">
        <label>推广厂商<select v-model="maker" required :disabled="busy" @change="href=''; qr=''"><option value="">选择厂商</option><option v-for="[id,name] in makers" :key="id" :value="id">{{ name }}</option></select></label>
        <UiButton type="submit" variant="primary" :disabled="!maker" :loading="busy">生成推广二维码</UiButton>
      </form>
      <UiState v-if="!makers.length && !error" title="当前没有厂商上架直购产品" />
      <div v-if="qr" class="retail-qr">
        <img :src="qr" width="240" height="240" alt="扫码查看厂商订购产品">
        <div><a :href="href" target="_blank" rel="noopener">预览推广页面</a><input :value="href" readonly aria-label="推广链接"><UiButton @click="copyLink">复制链接</UiButton> <a :href="qr" download="partnerledger-promotion.png">保存二维码</a><p v-if="localPreview" class="muted">{{ demo ? '当前二维码仅供本机预览。手机扫码请从同一网络可访问的地址打开演示，再生成二维码。' : '当前链接供本机预览。对外推广请从已部署的渠智链地址生成二维码。' }}</p></div>
      </div>
    </UiCard>

    <UiCard v-else-if="pricing" title="厂商直购配置" description="下单时锁定产品版本、价格、分佣比例和授权周期。">
      <form class="retail-form" @submit.prevent="savePrice">
        <label>产品<select v-model="product" required @change="choosePrice(product)"><option value="">选择已发布产品</option><option v-for="item in products" :key="item.id" :value="item.id">{{ item.name }} · {{ item.version }}{{ item.published ? '' : '（未发布）' }}</option></select></label>
        <template v-if="product">
          <label>订购单价（元）<input v-model="unit" required inputmode="decimal"></label>
          <label>代理商分佣（%）<input v-model="rate" required inputmode="decimal"></label>
          <label class="retail-check"><input v-model="active" type="checkbox"> 开放直购</label>
          <label class="retail-check"><input v-model="annual" type="checkbox"> 年度授权，收款后进入续费台账</label>
          <UiButton type="submit" variant="primary" :loading="busy">保存规则</UiButton>
        </template>
      </form>
      <UiButton v-if="!products.length" @click="emit('navigate','catalog')">前往产品目录</UiButton>
    </UiCard>

    <UiCard v-else-if="assisted || checkout" :title="assisted ? '创建代客订单' : '核对订购信息'">
      <p class="muted">{{ demo ? '选定客户和产品后，在订单列表打开客户确认链接，交给客户确认并模拟付款。' : assisted ? '创建订单后，把确认链接发给关联客户。客户登录后确认订单并向厂商付款。' : '提交后请按厂商提供的方式付款，由厂商核对并登记收款。' }}</p>
      <form v-if="!pendingCheckout && offers.length && (!assisted || customers.length)" class="retail-form" @submit.prevent="submitOrder">
        <label v-if="assisted">关联客户<select v-model="customer" required><option value="">选择客户</option><option v-for="item in customers" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label>订购产品<select v-model="product" required><option value="">选择产品</option><option v-for="item in offers" :key="item.product_id" :value="item.product_id">{{ item.manufacturer }} · {{ item.name }} · {{ item.version }} · {{ money(item.unit_cents) }}</option></select></label>
        <label>订购数量<input v-model.number="quantity" required type="number" min="1" max="10000" step="1"></label>
        <p v-if="selected">订单金额 <strong>{{ money(total) }}</strong><span v-if="selected.annual"> · 年度授权</span></p>
        <UiButton type="submit" variant="primary" :loading="busy">{{ assisted ? '提交客户确认' : '确认并下单' }}</UiButton>
      </form>
      <UiState v-else-if="!pendingCheckout" :title="assisted && !customers.length ? '暂无关联客户' : '该入口暂无可订购产品'" />
    </UiCard>

    <template v-else-if="ledger">
      <p v-if="vendor || dealer" class="muted">{{ vendor && receiptView ? (demo ? '客户完成模拟付款后，订单归入厂商收款，同时生成代理商佣金。' : '核对客户已付金额和凭证后登记收款，订单佣金会进入代理商结算台账。') : vendor ? '核对代理商、佣金金额和催办记录后，登记转账凭证。' : '厂商确认收款后可查看待结算金额，每笔订单每 24 小时可催结算一次。' }}</p>
      <section v-if="summary" class="metric-grid workspace-finance-metrics" aria-label="结算概况">
        <UiMetricCard label="待结算佣金" :value="money(summary.settlement_cents)" icon="quote" />
        <UiMetricCard label="已结算佣金" :value="money(summary.settled_cents)" icon="check" />
        <UiMetricCard label="已催办待结算" :value="summary.reminded + ' 笔'" icon="time" />
      </section>
      <section class="workspace-section" aria-labelledby="retail-records">
        <header class="workspace-section-heading">
          <h2 id="retail-records">{{ settlementView ? '佣金明细' : '订单列表' }}</h2>
          <label v-if="orderID === null" class="retail-filter">订单状态 <select v-model="status" :disabled="busy" @change="load"><option value="">全部</option><option v-for="[value,label] in statusOptions" :key="value" :value="value">{{ label }}</option></select></label>
        </header>
      <div v-if="orders.length" class="table-scroll"><table class="workspace-record-table" role="table">
        <thead role="rowgroup"><tr role="row"><th scope="col" role="columnheader">订单和产品</th><th scope="col" role="columnheader">交易方</th><th scope="col" role="columnheader">来源</th><th scope="col" role="columnheader">金额</th><th v-if="vendor || dealer" scope="col" role="columnheader">佣金</th><th scope="col" role="columnheader">状态与办理</th></tr></thead>
        <tbody role="rowgroup"><tr v-for="row in orders" :key="row.id" role="row">
          <td role="cell"><strong>{{ row.product_name }}</strong><small>订单 {{ row.id }} · {{ row.product_version }} · {{ row.quantity }} 份</small></td>
          <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">交易方</span>{{ vendor ? row.reseller : row.manufacturer }}<small v-if="vendor || dealer">{{ row.customer }}</small></td>
          <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">来源</span>{{ row.source === 'assisted' ? '帮客户下单' : '推广获客' }}</td>
          <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">金额</span>{{ money(row.total_cents) }}</td>
          <td v-if="vendor || dealer" role="cell"><span class="workspace-mobile-label" aria-hidden="true">佣金</span><template v-if="row.commission && row.paid_at">{{ money(row.commission.cents) }} <small>{{ row.commission.rate_bps / 100 }}%</small></template><span v-else>{{ row.status === 'declined' ? '—' : '待收款' }}</span></td>
          <td class="retail-state workspace-record-action" role="cell">
            <UiBadge :tone="tones[row.status]" dot>{{ labels[row.status] }}</UiBadge>
            <small v-if="row.commission?.reminder_count">催结算 {{ row.commission.reminder_count }} 次 · 最近 {{ new Date(row.commission.reminded_at).toLocaleString() }}</small>
            <small v-if="row.commission?.settled_at">结算 {{ new Date(row.commission.settled_at).toLocaleString() }}</small>
            <small v-if="vendor && row.receipt_ref">{{ demo ? '模拟收款凭证' : '收款凭证' }} {{ row.receipt_ref }}</small>
            <small v-if="row.payout_ref">{{ demo ? '模拟转账凭证' : '转账凭证' }} {{ row.payout_ref }}</small>
            <div v-if="dealer && row.source === 'assisted' && row.status === 'confirmation'" class="retail-actions">
              <a :href="demo ? row.checkout_path : customerOrderLink(row.id)" target="_blank" rel="noopener">打开客户确认链接</a>
              <UiButton v-if="!demo && clipboardAvailable" compact variant="text" :disabled="busy" @click="copyCustomerLink(row.id)">复制链接</UiButton>
            </div>
            <small v-if="role === 'customer' && row.status === 'payment'">请按厂商提供的方式付款，完成后由厂商确认收款。</small>
            <div v-if="role === 'customer' && row.status === 'confirmation'" class="retail-actions"><UiButton compact :disabled="busy" @click="advance(row,'confirm')">确认订单</UiButton> <UiButton compact :disabled="busy" @click="advance(row,'decline')">拒绝</UiButton></div>
            <UiButton v-if="dealer && settlementView && row.status === 'settlement'" compact class="retail-actions" :disabled="busy || !canRemind(row)" @click="advance(row,'remind')">{{ canRemind(row) ? '催结算' : '已催结算' }}</UiButton>
            <template v-if="vendor && ((receiptView && row.status === 'payment') || (settlementView && row.status === 'settlement'))">
              <div v-if="actionOrder !== row.id" class="retail-actions">
                <UiButton compact :disabled="busy" @click="actionOrder=row.id; reference=''">{{ row.status === 'payment' ? '登记收款' : '登记结算' }}</UiButton>
              </div>
              <form v-else class="retail-actions" @submit.prevent="advance(row,row.status === 'payment' ? 'receipt' : 'settle',reference)">
                <label>{{ row.status === 'payment' ? '收款凭证编号' : '转账凭证编号' }}<input v-model="reference" required maxlength="100"></label>
                <UiButton compact type="submit" :loading="busy">确认登记</UiButton> <UiButton compact :disabled="busy" @click="actionOrder=''">取消</UiButton>
              </form>
            </template>
          </td>
        </tr></tbody>
      </table></div>
      <UiState v-else :title="orderID === null ? '当前没有符合条件的订单' : '订单不存在或当前组织无权查看'" />
      <UiButton v-if="next" :loading="busy" @click="run(() => list(next))">加载更多订单</UiButton>
      </section>
    </template>
</template>

<style scoped>
.retail-form { display:grid; gap:15px; max-width:520px; margin-block:16px; }
.retail-form label,.retail-actions label { display:grid; gap:6px; }
.retail-form .retail-check { display:flex; align-items:center; }
.retail-form button { justify-self:start; }
input:not([type=checkbox]),select { width:100%; min-width:0; padding:7px 10px; border:1px solid var(--ui-border-strong); border-radius:6px; background:var(--ui-surface); color:var(--ui-ink); }
.retail-filter { display:flex; align-items:center; gap:6px; font-size:13px; }
.retail-filter select { width:auto; }
.retail-qr { display:flex; flex-wrap:wrap; align-items:center; gap:20px; }
.retail-qr input { display:block; margin-block:10px; }
.retail-actions { margin-top:6px; }
.retail-state { min-width: 170px; }
@media (max-width: 760px) { .retail-state { min-width: 0; } }
</style>
