<script setup>
import { computed, onMounted, ref } from 'vue'
import { formatCents } from '../channel/quoteMoney.js'
import { UiButton, UiCard, UiIcon, UiState } from '../ui/index.js'
import { requestRetailDemo } from './retailDemo.js'
import { pendingRetailRequest } from './retailRequest.js'

const params = new URLSearchParams(location.search)
const code = params.get('code') || ''
const order = params.get('order') || ''
const valid = /^[a-f\d]{32}$/.test(order || code)
const endpoint = order ? '/customer-orders/' + order : '/store/' + code
const products = ref([])
const receipt = ref(null)
const selected = ref(null)
const quantity = ref(1)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const paymentError = ref('')
const amount = computed(() => Number.isInteger(quantity.value) && quantity.value > 0 && quantity.value <= 10000
  ? quantity.value * (selected.value?.unit_cents ?? 0) : 0)
const money = cents => formatCents(BigInt(cents))

onMounted(async () => {
  try {
    if (!valid) throw new Error('请从代理商提供的二维码或客户确认链接进入。')
    const result = await requestRetailDemo(endpoint)
    if (order) receipt.value = result
    else products.value = result.items
  } catch (cause) {
    error.value = cause.message
  } finally {
    loading.value = false
  }
})

function choose(product) {
  selected.value = product
  quantity.value = 1
  paymentError.value = ''
}

async function pay() {
  if (busy.value) return
  busy.value = true
  paymentError.value = ''
  try {
    if (order) {
      receipt.value = await requestRetailDemo(endpoint + '/pay', { method: 'POST', body: {} })
    } else if (selected.value && amount.value) {
      const body = { product_id: selected.value.product_id, quantity: quantity.value, expected_unit_cents: selected.value.unit_cents }
      const storageKey = 'partnerledger-retail-demo-checkout:' + code
      const requestId = pendingRetailRequest(storageKey, body)
      const result = await requestRetailDemo(endpoint + '/orders', { method: 'POST', body: { ...body, request_id: requestId } })
      sessionStorage.removeItem(storageKey)
      location.replace(result.checkout_path)
    }
  } catch (cause) {
    paymentError.value = cause.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="demo-store">
    <header class="store-header">
      <div class="store-wrap store-brand">
        <UiIcon name="catalog" :size="24" />
        <span>畅捷通 · 软件订购</span>
        <span class="store-tag">模拟交易</span>
      </div>
    </header>
    <main class="store-wrap">
      <UiState v-if="loading" state="loading" title="正在读取订购信息" />
      <UiState v-else-if="error" state="error" :title="error" />
      <UiCard v-else-if="receipt" class="checkout-card" :title="receipt.paid_at ? '模拟付款成功' : '确认代客订单'">
        <dl class="checkout-details">
          <dt>订单编号</dt><dd>{{ receipt.id }}</dd>
          <dt>订购客户</dt><dd>{{ receipt.customer }}</dd>
          <dt>订购产品</dt><dd>{{ receipt.product_name }} · {{ receipt.product_version }}</dd>
          <dt>数量</dt><dd>{{ receipt.quantity }} 份</dd>
          <dt>收款方</dt><dd>{{ receipt.manufacturer }}</dd>
          <dt>{{ receipt.paid_at ? '已付金额' : '应付金额' }}</dt><dd><strong>{{ money(receipt.total_cents) }}</strong></dd>
        </dl>
        <p v-if="paymentError" class="form-error" role="alert">{{ paymentError }}</p>
        <UiButton v-if="!receipt.paid_at" variant="primary" :loading="busy" @click="pay">确认并模拟付款</UiButton>
        <p v-else class="store-paid" role="status"><UiIcon name="check" :size="18" /> 款项已计入厂商演示收款。</p>
      </UiCard>
      <UiCard v-else-if="selected" class="checkout-card" title="核对订单">
        <form class="checkout-form" @submit.prevent="pay">
          <dl class="checkout-details">
            <dt>订购产品</dt><dd>{{ selected.name }} · {{ selected.version }}</dd>
            <dt>收款方</dt><dd>{{ selected.manufacturer }}</dd>
            <dt>单价</dt><dd>{{ money(selected.unit_cents) }}</dd>
          </dl>
          <label>订购数量 <input v-model.number="quantity" type="number" min="1" max="10000" step="1" required></label>
          <p class="checkout-total">应付金额 <strong>{{ money(amount) }}</strong></p>
          <p v-if="paymentError" class="form-error" role="alert">{{ paymentError }}</p>
          <div class="checkout-actions">
            <UiButton variant="primary" type="submit" :loading="busy" :disabled="!amount">模拟付款</UiButton>
            <UiButton :disabled="busy" @click="selected = null">返回选购</UiButton>
          </div>
        </form>
      </UiCard>
      <template v-else>
        <section class="store-intro">
          <h1>企业软件订购</h1>
          <p>选择财务管理、进销存或企业 ERP。订购费用由厂商收取。</p>
        </section>
        <div class="store-catalog">
          <article v-for="item in products" :key="item.product_id" class="store-product">
            <span class="product-category">{{ item.category }}</span>
            <h2>{{ item.name }}</h2>
            <p class="product-version">{{ item.version }}</p>
            <p>{{ item.summary }}</p>
            <div class="product-action"><strong>{{ money(item.unit_cents) }} <small>／份</small></strong>
              <UiButton variant="primary" @click="choose(item)">订购{{ item.name }}</UiButton></div>
          </article>
        </div>
      </template>
    </main>
    <footer class="store-wrap store-footer">PartnerLedger 演示 · 软件和价格均为样例</footer>
  </div>
</template>

<style scoped>
.demo-store { min-height:100vh; color:#122a3a; background:#f2f7f8; }
.store-wrap { width:min(1080px, calc(100% - 36px)); margin-inline:auto; }
.store-header { border-bottom:1px solid #d9e6eb; background:white; }
.store-brand { min-height:66px; display:flex; align-items:center; gap:10px; font-size:18px; font-weight:700; }
.store-tag { margin-left:auto; font-size:13px; color:#295e59; font-weight:600; }
.store-intro { margin-block:26px; padding:clamp(24px,6vw,62px); border-radius:16px; background:#123245; color:#fff; }
.store-intro h1 { margin:0; font-size:clamp(30px,5vw,52px); letter-spacing:.03em; }
.store-intro p { margin:12px 0 0; color:#d4e9ec; }
.store-catalog { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(280px,100%),1fr)); gap:16px; }
.store-product { display:flex; flex-direction:column; gap:10px; padding:23px; border:1px solid #d9e6eb; border-radius:12px; background:white; }
.store-product h2,.store-product p { margin:0; }
.product-category { color:#20665f; font-size:13px; font-weight:700; }
.product-version,.store-footer { color:#4e656f; font-size:13px; }
.product-action { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:12px; margin-top:auto; padding-top:16px; }
.product-action strong,.checkout-total strong { font-size:22px; }
.product-action small { font-size:12px; color:#4e656f; font-weight:400; }
.checkout-card { max-width:640px; margin:28px auto; }
.checkout-details { display:grid; grid-template-columns:auto 1fr; gap:10px 18px; margin:14px 0 22px; }
.checkout-details dd { margin:0; }
.checkout-details dt { color:#4e656f; }
.checkout-form input { width:80px; margin-left:8px; padding:6px; }
.checkout-actions,.store-paid { display:flex; align-items:center; gap:10px; }
.store-paid { color:#236454; }
.store-footer { padding-block:24px; }
</style>
