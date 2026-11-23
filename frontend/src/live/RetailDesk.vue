<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { requestApi } from '../api.js'
import { UiButton, UiCard, UiState } from '../ui/index.js'
import { formatCents, quoteCents } from '../channel/quoteMoney.js'
import { demoStore } from './retailDemo.js'
import { pendingRetailRequest } from './retailRequest.js'

const props = defineProps({ mode: String, role: String, request: Function, identity: String, demo: Boolean })
const emit = defineEmits(['navigate'])
const api = '/api/retail/v1'
const params = new URLSearchParams(location.search)
const code = params.get('code') ?? ''
const isPublic = props.mode === 'store'
const promotion = props.mode === 'promotion'
const assisted = props.mode === 'assisted'
const checkout = props.mode === 'checkout'
const pricing = props.mode === 'pricing'
const ledger = ['retail-orders', 'commission', 'manufacturer-receipts', 'manufacturer-settlements'].includes(props.mode)
const vendor = props.role === 'vendor'
const dealer = props.role === 'dealer'
const capable = /^[a-f0-9]{32}$/.test(code)
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
const href = ref('')
const qr = ref('')
const status = ref('')
const actionOrder = ref('')
const reference = ref('')
const title = {
  store: '厂商订购专区', promotion: '推广获客', assisted: '帮客户下单', checkout: '确认厂商订单',
  pricing: '订购价格与佣金规则', 'retail-orders': '厂商直收订单', commission: '佣金结算',
  'manufacturer-receipts': '直收订单与收款', 'manufacturer-settlements': '代理商分佣',
}[props.mode]
const labels = {
  confirmation: '待客户确认', payment: '待厂商确认收款', paid: '厂商已确认收款',
  settlement: '待厂商结算', settled: '已登记结算', declined: '客户已拒绝',
}
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
const canRemind = row => !row.commission?.reminded_at ||
  Date.now() - Date.parse(row.commission.reminded_at) >= 86400000

async function list(cursor = '') {
  const query = new URLSearchParams()
  if (cursor) query.set('before', cursor)
  if (status.value) query.set('status', status.value)
  if (settlementView) query.set('view', 'commission')
  const result = await read(api + '/orders?' + query)
  orders.value = cursor ? [...orders.value,...result.items] : result.items
  summary.value = result.summary || null
  next.value = result.next_cursor
}
async function readCatalog(path) {
  const items = []
  const cursors = new Set()
  let cursor = ''
  do {
    const page = await read(path + (cursor ? (path.includes('?') ? '&' : '?') + 'after=' + encodeURIComponent(cursor) : ''))
    items.push(...page.items)
    cursor = page.next_cursor
    if (cursor && cursors.has(cursor)) throw new Error('产品目录分页无效，请刷新。')
    cursors.add(cursor)
  } while (cursor)
  return items
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    if (ledger) await list()
    else if (pricing) {
      const [configured, owned] = await Promise.all([read(api + '/manufacturer-offers'),readCatalog('/api/products?limit=100')])
      offers.value = configured.items
      products.value = owned
    } else if (isPublic || checkout) {
      if (!capable) throw new Error('推广链接无效。')
      offers.value = await readCatalog(api + '/store/' + code)
      if (checkout && !selected.value && offers.value.length) product.value = offers.value[0].product_id
    } else {
      offers.value = await readCatalog(api + '/offers')
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
  if (!busy.value && !loading.value) load()
}
onMounted(() => {
  load()
  if (props.demo && ledger) window.addEventListener('focus', refreshOnFocus)
})
onUnmounted(() => {
  controller.abort()
  window.removeEventListener('focus', refreshOnFocus)
})

async function run(task) {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  error.value = ''
  try { await task() } catch (cause) { if (!controller.signal.aborted) error.value = cause.message }
  finally { busy.value = false }
}
function buyLink(item) {
  return '/login/customer?' + new URLSearchParams({ view:'checkout', code, product:item.product_id })
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
}
function savePrice() {
  run(async () => {
    const moneyFormat = /^\d+(\.\d{1,2})?$/
    if (!moneyFormat.test(unit.value) || !moneyFormat.test(rate.value)) throw new Error('单价和比例最多支持两位小数。')
    const unitCents = Number(quoteCents(unit.value))
    const rateBps = Number(quoteCents(rate.value))
    if (!(unitCents > 0 && unitCents <= 100000000 && rateBps <= 10000)) throw new Error('请填写有效单价和 0–100% 的分佣比例。')
    const row = await write('/manufacturer-offers/' + product.value, { unit_cents:unitCents, commission_bps:rateBps, active:active.value },'PUT')
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
    // Preserve the request ID on a lost response, including after a reload in this tab.
    const key = 'partnerledger-retail-checkout:' + props.identity + ':' + props.mode
    await write('/orders', { ...draft, request_id:pendingRetailRequest(key, draft) })
    sessionStorage.removeItem(key)
    emit('navigate','retail-orders')
  })
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
  <main v-if="isPublic" class="content store-page">
    <header v-enter class="page-heading"><div><a href="/">渠智链官网</a><h1>{{ title }}</h1><p class="muted">选择产品，以企业客户身份登录后确认订购。</p></div></header>
    <UiState v-if="loading" state="loading" title="正在读取厂商产品" />
    <UiState v-else-if="error" state="error" :title="error" />
    <div v-else-if="offers.length" v-enter:children class="store-products">
      <UiCard v-for="item in offers" :key="item.product_id" :title="item.name + ' · ' + item.version" :description="item.manufacturer">
        <p class="store-price">{{ money(item.unit_cents) }} <span>／份</span></p><a :href="buyLink(item)">登录并订购 →</a>
      </UiCard>
    </div>
    <UiState v-else title="厂商暂未上架可订购产品" />
  </main>

  <template v-else>
    <header v-enter class="page-heading"><h1>{{ title }}</h1><UiButton compact :disabled="loading || busy" @click="load">刷新</UiButton></header>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <p v-if="notice" role="status">{{ notice }}</p>
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

    <UiCard v-else-if="pricing" title="厂商直购配置" description="正式下单时会锁定产品版本、价格和分佣比例。比例默认显示 50%，保存后生效。">
      <form class="retail-form" @submit.prevent="savePrice">
        <label>产品<select v-model="product" required @change="choosePrice(product)"><option value="">选择已发布产品</option><option v-for="item in products" :key="item.id" :value="item.id">{{ item.name }} · {{ item.version }}{{ item.published ? '' : '（未发布）' }}</option></select></label>
        <template v-if="product">
          <label>订购单价（元）<input v-model="unit" required inputmode="decimal"></label>
          <label>代理商分佣（%）<input v-model="rate" required inputmode="decimal"></label>
          <label class="retail-check"><input v-model="active" type="checkbox"> 开放直购</label>
          <UiButton type="submit" variant="primary" :loading="busy">保存规则</UiButton>
        </template>
      </form>
      <UiButton v-if="!products.length" @click="emit('navigate','catalog')">前往产品目录</UiButton>
    </UiCard>

    <UiCard v-else-if="assisted || checkout" :title="assisted ? '创建代客订单' : '核对订购信息'">
      <p class="muted">{{ demo ? '选定客户和产品后，在订单列表打开客户确认链接，交给客户确认并模拟付款。' : assisted ? '订单将提交给关联客户确认，客户随后向厂商付款。' : '提交后请按厂商提供的方式付款，由厂商核对并登记收款。' }}</p>
      <form v-if="offers.length && (!assisted || customers.length)" class="retail-form" @submit.prevent="submitOrder">
        <label v-if="assisted">关联客户<select v-model="customer" required><option value="">选择客户</option><option v-for="item in customers" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label>订购产品<select v-model="product" required><option value="">选择产品</option><option v-for="item in offers" :key="item.product_id" :value="item.product_id">{{ item.manufacturer }} · {{ item.name }} · {{ item.version }} · {{ money(item.unit_cents) }}</option></select></label>
        <label>订购数量<input v-model.number="quantity" required type="number" min="1" max="10000" step="1"></label>
        <p v-if="selected">订单金额 <strong>{{ money(total) }}</strong></p>
        <UiButton type="submit" variant="primary" :loading="busy">{{ assisted ? '提交客户确认' : '确认并下单' }}</UiButton>
      </form>
      <UiState v-else :title="assisted && !customers.length ? '暂无关联客户' : '该入口暂无可订购产品'" />
    </UiCard>

    <UiCard v-else-if="ledger" :title="settlementView ? '厂商直收订单佣金' : '订单列表'">
      <p v-if="vendor || dealer" class="muted">{{ demo && vendor ? (receiptView ? '客户完成模拟付款后，订单归入厂商收款，同时生成代理商佣金。' : '核对代理商、佣金金额和催办记录，登记模拟转账凭证。') : vendor ? '核对实际收款或转账凭证后，再登记对应结果。' : '厂商确认收款后可查看待结算金额，每笔订单每 24 小时可催结算一次。' }}</p>
      <dl v-if="summary" class="retail-summary">
        <div><dt>待结算佣金</dt><dd>{{ money(summary.settlement_cents) }}</dd></div>
        <div><dt>已结算佣金</dt><dd>{{ money(summary.settled_cents) }}</dd></div>
        <div><dt>已催办待结算</dt><dd>{{ summary.reminded }} 笔</dd></div>
      </dl>
      <label class="retail-filter">状态 <select v-model="status" :disabled="busy" @change="load"><option value="">全部</option><option v-for="[value,label] in statusOptions" :key="value" :value="value">{{ label }}</option></select></label>
      <div v-if="orders.length" class="retail-table"><table>
        <thead><tr><th>订单和产品</th><th>交易方</th><th>来源</th><th>金额</th><th v-if="vendor || dealer">佣金</th><th>状态与办理</th></tr></thead>
        <tbody><tr v-for="row in orders" :key="row.id">
          <td><strong>{{ row.product_name }}</strong><small>订单 {{ row.id }} · {{ row.product_version }} · {{ row.quantity }} 份</small></td>
          <td>{{ vendor ? row.reseller : row.manufacturer }}<small v-if="vendor || dealer">{{ row.customer }}</small></td>
          <td>{{ row.source === 'assisted' ? '帮客户下单' : '推广获客' }}</td>
          <td>{{ money(row.total_cents) }}</td>
          <td v-if="vendor || dealer"><template v-if="row.commission && (!demo || row.paid_at)">{{ money(row.commission.cents) }} <small>{{ row.commission.rate_bps / 100 }}%</small></template><span v-else>待收款</span></td>
          <td>
            <strong>{{ labels[row.status] }}</strong>
            <small v-if="row.commission?.reminder_count">催结算 {{ row.commission.reminder_count }} 次 · 最近 {{ new Date(row.commission.reminded_at).toLocaleString() }}</small>
            <small v-if="row.commission?.settled_at">结算 {{ new Date(row.commission.settled_at).toLocaleString() }}</small>
            <small v-if="demo && vendor && row.receipt_ref">模拟收款凭证 {{ row.receipt_ref }}</small>
            <small v-if="demo && row.payout_ref">模拟转账凭证 {{ row.payout_ref }}</small>
            <a v-if="demo && dealer && row.checkout_path && row.status === 'confirmation'" :href="row.checkout_path" target="_blank" rel="noopener">打开客户确认链接</a>
            <div v-if="role === 'customer' && row.status === 'confirmation'" class="retail-actions"><UiButton compact :disabled="busy" @click="advance(row,'confirm')">确认订单</UiButton> <UiButton compact :disabled="busy" @click="advance(row,'decline')">拒绝</UiButton></div>
            <UiButton v-if="dealer && (!demo || settlementView) && row.status === 'settlement'" compact class="retail-actions" :disabled="busy || !canRemind(row)" @click="advance(row,'remind')">{{ canRemind(row) ? '催结算' : '已催结算' }}</UiButton>
            <template v-if="vendor && (!demo || settlementView) && (row.status === 'payment' || row.status === 'settlement')">
              <UiButton v-if="actionOrder !== row.id" compact class="retail-actions" :disabled="busy" @click="actionOrder=row.id; reference=''">{{ row.status === 'payment' ? '登记收款' : '登记结算' }}</UiButton>
              <form v-else class="retail-actions" @submit.prevent="advance(row,row.status === 'payment' ? 'receipt' : 'settle',reference)">
                <label>{{ row.status === 'payment' ? '收款凭证编号' : '转账凭证编号' }}<input v-model="reference" required maxlength="100"></label>
                <UiButton compact type="submit" :loading="busy">确认登记</UiButton> <UiButton compact :disabled="busy" @click="actionOrder=''">取消</UiButton>
              </form>
            </template>
          </td>
        </tr></tbody>
      </table></div>
      <UiState v-else title="当前没有符合条件的订单" />
      <UiButton v-if="next" :loading="busy" @click="run(() => list(next))">加载更多订单</UiButton>
    </UiCard>
  </template>
</template>

<style scoped>
.store-page { max-width:1120px; margin-inline:auto; }
.store-products { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(250px,100%),1fr)); gap:16px; }
.store-price { font-size:22px; margin-block:8px 12px; }
.store-price span { font-size:14px; color:var(--ui-ink-muted); }
.retail-form { display:grid; gap:15px; max-width:520px; margin-block:16px; }
.retail-form label,.retail-actions label { display:grid; gap:6px; }
.retail-form .retail-check { display:flex; align-items:center; }
.retail-form button { justify-self:start; }
input:not([type=checkbox]),select { width:100%; min-width:0; padding:7px 10px; border:1px solid var(--ui-border-strong); border-radius:6px; background:var(--ui-surface); color:var(--ui-ink); }
.retail-filter { display:flex; align-items:center; gap:10px; margin:12px 0; }
.retail-filter select { width:auto; }
.retail-qr { display:flex; flex-wrap:wrap; align-items:center; gap:20px; }
.retail-qr input { display:block; margin-block:10px; }
.retail-table { overflow:auto; margin-block:12px; }
.retail-table table { width:100%; min-width:680px; }
.retail-table small { display:block; color:var(--ui-ink-muted); }
.retail-actions { margin-top:6px; }
.retail-summary { display:flex; flex-wrap:wrap; gap:10px; margin-block:14px; }
.retail-summary div { flex:1 1 145px; padding:12px 14px; background:var(--ui-surface-hover); border:1px solid var(--ui-border); border-radius:7px; }
.retail-summary dt { font-size:13px; color:var(--ui-ink-muted); }
.retail-summary dd { margin:4px 0 0; font-weight:700; }
</style>
