<script setup>
import { computed, reactive, ref } from 'vue'
import { UiButton, UiField } from '../ui/index.js'
import { actionLabels, roles } from './domain.js'
import { formatCents, lineCents, quoteCents } from './quoteMoney.js'

const props = defineProps({
  action: { type: String, required: true },
  project: Object,
  products: { type: Array, default: () => [] },
  inquiryDraft: Object,
  role: String,
  busy: Boolean,
  error: String,
})
const emit = defineEmits(['submit', 'cancel'])
const quoteAction = ['channelQuote', 'salesQuote'].includes(props.action)
const quotes = props.action === 'channelQuote' ? props.project?.channelQuotes : props.project?.salesQuotes
const lastQuote = [...(quotes ?? [])].sort((left, right) => right.version - left.version)[0]
const future = new Date()
future.setDate(future.getDate() + 30)
const toDate = date => date.toLocaleDateString('sv-SE', { timeZone: 'Asia/Shanghai' })
const prepared = props.action === 'submitInquiry' ? props.inquiryDraft : null
const form = reactive({
  title: '', requirement: '', customerName: props.role === 'dealer' ? roles.customer.company : '', dueDate: '', summary: prepared?.summary ?? props.project?.inquiry?.summary ?? '',
  product: prepared?.productId || props.project?.product || '', note: '', validUntil: toDate(future),
  deliveryDays: quoteAction && lastQuote ? lastQuote.deliveryDays : 15,
  lines: quoteAction && lastQuote ? lastQuote.lines.map(line => ({ ...line })) : [{ label: '软件用户授权', quantity: 1, unitPrice: '' }],
})
const today = toDate(new Date())
const approved = ref(false)
const helpers = ref(null)
const notes = {
  flagGap: ['暂不支持哪项附加需求？','请描述能力缺口。代理商可另行寻找合作伙伴，原询价继续处理。'],
  requestInfo: ['需要补充哪些信息？', '列出需要确认的模块、授权数量或部署条件。'],
  replyInfo: ['补充资料', '针对待补充的问题填写答复。'],
  reviseChannel: ['渠道报价修改要求', '说明需要厂商调整的授权、价格或服务项。'],
  reviseSales: ['对客报价修改要求', '说明需要代理商调整的项目。'],
  supply: ['供货或授权交接记录', '填写已完成的授权数量与交接说明，无需填写授权密钥。'],
  implement: ['实施与送验说明', '记录已部署的模块、培训情况和待验收事项。'],
  rectify: ['整改要求', '说明未通过验收的事项及预期结果。'],
  accept: ['验收意见（选填）', '记录验收结果或补充说明。'],
}
const noteSpec = computed(() => notes[props.action])
const amount = computed(() => form.lines.reduce((sum, line) => sum + lineCents(line), 0n))
const submitText = computed(() => ({
  create: props.role === 'customer' ? '提交需求' : '建立客户项目', submitInquiry: '确认并提交询价',
  channelQuote: '发布渠道报价', salesQuote: '发送对客报价', confirmSales: '确认此版本报价',
  placeOrder: '确认生成订单',
}[props.action] || actionLabels[props.action] || '提交'))

function suggestFields() {
  const text = props.project?.requirement || ''
  const licenses = text.match(/(?:授权|账号|许可|点位)\s*[：:]?\s*(\d+)[个套席位人]?|(\d+)\s*(?:个|名|位|席)?\s*(?:授权|账号|许可|点位)/)
  const modules = ['财务','采购','销售','库存','生产','人事','审批','客户管理','门店'].filter(word => text.includes(word))
  const deployment = ['私有化部署','本地部署','云部署','SaaS'].find(word => text.includes(word))
  helpers.value = { licenses: licenses?.[1] || licenses?.[2] || '', modules: modules.join('、'), deployment: deployment || '' }
}
function applyFields() {
  form.summary = [['授权数量', helpers.value.licenses], ['所需模块', helpers.value.modules], ['部署方式', helpers.value.deployment]]
    .filter(([, value]) => String(value).trim()).map(([label, value]) => label + '：' + value).join('；')
  helpers.value = null
  approved.value = false
}
function submit() {
  if (props.busy) return
  let payload
  if (props.action === 'create') {
    payload = { title: form.title.trim(), requirement: form.requirement.trim(), ...(form.dueDate && { dueDate: form.dueDate }), ...(form.customerName.trim() && { customerName: form.customerName.trim() }) }
  } else if (props.action === 'submitInquiry') {
    if (!approved.value) return
    payload = { summary: form.summary.trim(), product: form.product }
  } else if (quoteAction) {
    payload = { lines: form.lines.map(line => ({ label: line.label.trim(), quantity: Number(line.quantity), unitPrice: Number(line.unitPrice) })), validUntil: form.validUntil, deliveryDays: Number(form.deliveryDays), note: form.note.trim() }
  } else if (props.action === 'confirmSales' || props.action === 'placeOrder') {
    payload = { quoteId: lastQuote?.id }
  } else payload = noteSpec.value ? { note: form.note.trim() } : {}
  emit('submit', payload)
}
</script>

<template>
  <form class="form-grid ch-form" @submit.prevent="submit">
    <template v-if="action === 'create'">
      <UiField label="项目名称" required v-slot="{ control }"><input v-bind="control" v-model="form.title" required maxlength="100" placeholder="例如：生产基地 ERP 升级" /></UiField>
      <UiField v-if="role === 'dealer'" label="已关联客户组织" hint="当前演示代理商已获此客户授权，可代录需求并交由其确认报价。" v-slot="{ control }"><input v-bind="control" :value="form.customerName" readonly /></UiField>
      <UiField label="业务需求" required v-slot="{ control }"><textarea v-autogrow v-bind="control" v-model="form.requirement" required maxlength="2000" rows="5" placeholder="描述业务场景、所需模块、预计授权数和部署方式。" /></UiField>
      <UiField label="目标日期（选填）" v-slot="{ control }"><input v-bind="control" v-model="form.dueDate" type="date" /></UiField>
    </template>
    <template v-else-if="action === 'submitInquiry'">
      <p v-if="prepared" class="ch-callout">已带入询价草稿，请核对产品和可发给厂商的摘要。</p>
      <UiField label="询价产品" required v-slot="{ control }"><select v-bind="control" v-model="form.product" required><option value="" disabled>选择已发布产品</option><option v-for="item in products.filter(item => item.published)" :key="item.id" :value="item.id">{{ item.name }}</option></select></UiField>
      <div class="ch-assist"><p>仅发送询价所需字段，请勿包含客户联系人或内部预算。</p><UiButton v-if="!helpers" compact icon="request" @click="suggestFields">从原始需求整理字段</UiButton>
        <template v-else><p>本地规则仅摘取明确提及的字段。核对后可生成摘要，未知信息请留空。</p>
          <div class="ch-input-grid">
            <label>授权数量<input v-model="helpers.licenses" type="number" min="1" placeholder="待确认" /></label>
            <label>所需模块<input v-model="helpers.modules" placeholder="待确认" /></label>
            <label>部署方式<input v-model="helpers.deployment" placeholder="待确认" /></label>
          </div><UiButton compact @click="applyFields">用已核对字段填写摘要</UiButton>
        </template>
      </div>
      <UiField label="对厂商的询价摘要" required v-slot="{ control }"><textarea v-autogrow v-bind="control" v-model="form.summary" required rows="4" maxlength="3000" @input="approved = false" /></UiField>
      <label class="ch-checkbox"><input v-model="approved" type="checkbox" required /> 已核对产品与摘要，确认发送给厂商。</label>
    </template>
    <template v-else-if="quoteAction">
      <p class="ch-callout">{{ action === 'channelQuote' ? '此报价只发送给代理商。' : '此报价将发给客户，请独立填写销售单价与对客服务内容。' }}{{ lastQuote ? ' 发布后将保留旧版记录。' : '' }}</p>
      <fieldset class="ch-line-set"><legend>报价明细 · 人民币</legend>
        <div v-for="(line, index) in form.lines" :key="index" class="ch-quote-row">
          <label>项目 {{ index + 1 }}<input v-model="line.label" required maxlength="100" placeholder="授权或服务项" /></label>
          <label>数量<input v-model="line.quantity" required type="number" min="1" max="1000000" step="1" /></label>
          <label>单价（元）<input v-model="line.unitPrice" required type="number" min="0" max="99999999.99" step="0.01" /></label>
          <UiButton compact :disabled="form.lines.length < 2" :aria-label="'移除明细 ' + (index + 1)" @click="form.lines.splice(index, 1)">移除</UiButton>
        </div>
        <div class="ch-line-foot"><UiButton compact icon="add" @click="form.lines.push({label:'', quantity:1, unitPrice:''})">添加明细</UiButton><strong>合计 {{ formatCents(amount) }}</strong></div>
      </fieldset>
      <div class="form-row">
        <UiField label="有效至" required v-slot="{ control }"><input v-bind="control" v-model="form.validUntil" type="date" :min="today" required /></UiField>
        <UiField label="交付周期（天）" required v-slot="{ control }"><input v-bind="control" v-model="form.deliveryDays" type="number" min="1" max="3650" required /></UiField>
      </div>
      <UiField label="方案与服务说明" v-slot="{ control }"><textarea v-autogrow v-bind="control" v-model="form.note" maxlength="2000" rows="3" /></UiField>
    </template>
    <template v-else-if="action === 'confirmSales' || action === 'placeOrder'">
      <p class="ch-callout">{{ action === 'confirmSales' ? '确认后由代理商单独下单。' : '将按已确认的对客报价生成唯一订单。' }}</p>
      <dl v-if="lastQuote" class="details"><dt>报价版本</dt><dd>{{ lastQuote.id }} · V{{ lastQuote.version }}</dd><dt>总金额</dt><dd>{{ formatCents(quoteCents(lastQuote.total)) }}</dd><dt>有效至</dt><dd>{{ lastQuote.validUntil }}</dd></dl>
    </template>
    <UiField v-else-if="noteSpec" :label="noteSpec[0]" :required="action !== 'accept'" v-slot="{ control }"><textarea v-autogrow v-bind="control" v-model="form.note" :required="action !== 'accept'" :placeholder="noteSpec[1]" maxlength="2000" rows="4" /></UiField>
    <p v-else-if="action === 'withdrawInquiry'" class="ch-callout">撤回后厂商不能继续处理这笔询价。你可以核对内容并重新提交。</p>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <div class="modal-actions"><UiButton :disabled="busy" @click="$emit('cancel')">取消</UiButton><UiButton type="submit" variant="primary" :loading="busy">{{ submitText }}</UiButton></div>
  </form>
</template>
