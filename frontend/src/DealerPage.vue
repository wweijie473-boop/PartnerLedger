<script setup>
import { computed, ref, watchEffect } from 'vue'
import { createHttpProjectRepository } from './channel/httpProjects.js'
import ProjectList from './channel/ProjectList.vue'
import { formatCents, lineCents, quoteCents } from './channel/quoteMoney.js'
import { retailDemoEnabled } from './live/retailDemo.js'
import { UiButton, UiState } from './ui/index.js'

const props = defineProps({ view: String, title: String, projects: Array, request: Function, organization: String, account: String })
const emit = defineEmits(['project', 'navigate', 'account'])
const remote = ref([])
const loading = ref(false)
const error = ref('')
const retry = ref(0)
const search = ref('')
const price = ref('')
const quantity = ref(1)
const queues = ['orders', 'finance', 'support', 'tickets']
watchEffect(onCleanup => {
  const view = props.view
  retry.value
  search.value = ''
  remote.value = []
  error.value = ''
  loading.value = false
  if (!props.request || !queues.includes(view)) return
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  loading.value = true
  createHttpProjectRepository(props.request).list({ signal: controller.signal })
    .then(items => { if (!controller.signal.aborted) remote.value = items })
    .catch(cause => { if (!controller.signal.aborted) error.value = cause.message })
    .finally(() => { if (!controller.signal.aborted) loading.value = false })
})
const rows = computed(() => (props.projects ?? remote.value).filter(p => {
  if (props.view === 'tickets') return p.isTodo
  if (props.view === 'support') return ['ordered','shipped','supplied','acceptance','rectification','completed'].includes(p.stage)
  return p.order || (props.view === 'orders' && p.stage === 'confirmed')
}).filter(p => [p.id, p.title, p.customerName].join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))
function lockedAmount(quotes, id) {
  const quote = quotes?.find(q => q.id === id)
  return quote ? formatCents(quoteCents(quote.total)) : '—'
}
const estimate = computed(() => /^\d{1,8}(\.\d{1,2})?$/.test(String(price.value)) && Number.isSafeInteger(quantity.value) && quantity.value > 0
  ? formatCents(lineCents({ unitPrice: price.value, quantity: quantity.value })) : '—')
const lessons = [
  ['拜访时怎么记附加需求？','记下客户要解决的问题、现用产品及对接对象。进入商机拾遗查看团队伙伴的能力与合作记录，再联系引荐人核实。','visits','打开记拜访'],
  ['如何从选型走到订购？','先确认部署方式和授权数量，再核对渠道报价与对客方案。客户确认后，经办人在项目详情办理下单。','orders','查看订购项目'],
  ['交付时要核对什么？','核对订单、供货记录和实施结果，再提交验收。遇到整改要求时回到同一项目补充记录。','support','查看实施与验收'],
]
const endpoints = [
  ['组织身份','GET /api/org/current','读取已登录账号的当前组织。'],
  ['已发布产品','GET /api/products','按当前身份查询可见的软件产品。'],
  ['项目工作台','GET /api/channel/v1/projects/workspace','按范围和进度查询本组织可见的项目。'],
]
</script>
<template>
  <header class="page-heading"><h1>{{ title }}</h1></header>
  <section v-if="queues.includes(view)" class="dealer-panel">
    <p v-if="view === 'finance'">按订单锁定的报价版本核对渠道采购额与对客销售额，实际收付款以财务凭证为准。</p>
    <p v-if="view === 'tickets'">按项目当前阶段列出需要本方处理的事项，打开项目即可办理。</p>
    <p v-if="view === 'support' && !request && retailDemoEnabled"><UiButton @click="emit('navigate', 'service-tickets')">打开服务工单</UiButton></p>
    <label class="dealer-search">查找项目 <input v-model="search" placeholder="项目名称、客户或编号" type="search"></label>
    <UiState v-if="loading" state="loading" title="正在读取本组织的项目" />
    <UiState v-else-if="error" state="error" title="读取失败" :description="error"><UiButton @click="retry++">重试</UiButton></UiState>
    <ProjectList v-else-if="rows.length && view !== 'finance'" :projects="rows" show-customer @open="project => emit('project', project.id)" />
    <div v-else-if="rows.length" class="table-scroll">
      <table class="workspace-record-table" role="table">
        <thead role="rowgroup"><tr role="row"><th scope="col" role="columnheader">项目 / 客户</th><th scope="col" role="columnheader">当前阶段</th><th scope="col" role="columnheader">渠道采购额</th><th scope="col" role="columnheader">对客销售额</th></tr></thead>
        <tbody role="rowgroup"><tr v-for="project in rows" :key="project.id" role="row">
          <td role="cell"><button class="dealer-inline" @click="emit('project', project.id)">{{ project.title }}</button><small>{{ project.customerName }} · {{ project.id }}</small></td>
          <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">当前阶段</span>{{ project.stageLabel }}</td>
          <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">渠道采购额</span>{{ lockedAmount(project.channelQuotes, project.order?.channelQuoteId) }}</td>
          <td role="cell"><span class="workspace-mobile-label" aria-hidden="true">对客销售额</span>{{ lockedAmount(project.salesQuotes, project.order?.salesQuoteId) }}</td>
        </tr></tbody>
      </table>
    </div>
    <UiState v-else :title="search ? '没有匹配的项目' : '当前没有相关项目'" />
  </section>
  <section v-else-if="view === 'staff'" class="dealer-panel"><h2>{{ organization }}</h2><p>{{ account }}</p><UiButton @click="emit('account')">查看身份</UiButton></section>
  <section v-else-if="view === 'academy'" class="dealer-panel"><details v-for="[title, body, target, label] in lessons" :key="title"><summary>{{ title }}</summary><p>{{ body }}</p><UiButton @click="emit('navigate',target)">{{ label }}</UiButton></details></section>
  <section v-else-if="view === 'platform'" class="dealer-panel"><h2>渠智链接口</h2><p>调用以下只读接口需要本平台的登录凭证，返回范围由当前组织和角色决定。</p><details v-for="[title, path, purpose] in endpoints" :key="path"><summary>{{ title }}</summary><code>{{ path }}</code><p>{{ purpose }}</p></details></section>
  <section v-else-if="view === 'tools'" class="dealer-panel"><h2>授权金额试算</h2><div class="dealer-inputs"><label>授权单价（元）<input v-model="price" inputmode="decimal" placeholder="0.00"></label><label>数量<input v-model.number="quantity" type="number" min="1" step="1"></label></div><p>预计总额　<strong>{{ estimate }}</strong></p><a href="/tables">打开表格工作台 →</a></section>
</template>
<style scoped>
.dealer-panel { padding-block:12px; border-block:1px solid var(--ui-border) }
.dealer-panel h2 { font-size:16px;font-weight:500 }.dealer-panel p {margin:8px 0 12px;color:var(--ui-ink-secondary)}
.dealer-search,.dealer-inputs label {display:grid;gap:6px;max-width:320px;font-weight:500;margin-bottom:16px}
.dealer-inputs {display:flex;flex-wrap:wrap;gap:16px;margin-top:16px}
input {min-width:0;border:1px solid var(--ui-border-strong);padding:8px 12px;border-radius:6px}
.dealer-inline {background:none;border:0;padding:0;color:var(--ui-link);text-align:left}
details {padding:14px 0;border-bottom:1px solid var(--ui-border)} summary {cursor:pointer;font-weight:600} code {display:block;margin-top:12px;overflow-wrap:anywhere}
</style>
