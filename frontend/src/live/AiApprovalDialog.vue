<script setup>
import { computed } from 'vue'
import { ElDialog } from 'element-plus'
import { UiBadge, UiButton, UiField, UiIcon } from '../ui/index.js'

const props = defineProps({
  open: Boolean,
  approval: Object,
  error: String,
  working: String,
  demo: Boolean,
  projects: { type: Array, default: () => [] },
  inquiryProjects: { type: Array, default: () => [] },
  skillLabels: { type: Object, required: true },
  inquiryStepsPreview: { type: Function, required: true },
  formatMoneyCents: { type: Function, required: true },
})

const emit = defineEmits([
  'close',
  'decline',
  'approve',
  'refresh-matches',
  'apply-markup',
  'refresh-visit',
  'open-projects',
])

const requirementFields = [
  { key: 'enterprise_type', label: '企业类型', max: 120 },
  { key: 'employee_count', label: '员工人数', numeric: true },
  { key: 'required_modules', label: '所需模块', hint: '多个模块用顿号或逗号分隔。' },
  { key: 'deployment_mode', label: '部署方式', max: 80 },
  { key: 'license_count', label: '授权用户数', numeric: true },
  { key: 'version', label: '指定版本', max: 100 },
]

const fitLabels = {
  matched: ['符合已知条件', 'success'],
  needs_confirmation: ['待核实', 'warning'],
  mismatch: ['有条件不符', 'danger'],
}
const checkLabels = { met: '符合', unknown: '待核实', missing: '不符' }

const steps = computed(() => {
  const item = props.approval
  if (!item) return []
  return item.kind === 'inquiry_workflow' ? props.inquiryStepsPreview(item) : (item.stepsPreview ?? [])
})

const eligibleMatches = computed(() => {
  if (props.approval?.kind !== 'inquiry_workflow') return []
  return (props.approval.matches ?? []).filter(item => item.status !== 'mismatch')
})

const salesQuoteTotals = computed(() => {
  const item = props.approval
  if (item?.kind !== 'sales_quote') return null
  const channelTotal = Number(item.channelQuote?.total ?? 0)
  const salesCents = (item.lines ?? []).reduce((sum, line) => {
    const qty = Number(line.quantity) || 0
    const cents = Math.round((Number(line.unitPrice) || 0) * 100)
    return sum + qty * cents
  }, 0)
  const salesTotal = salesCents / 100
  return {
    channel: channelTotal.toFixed(2),
    sales: salesTotal.toFixed(2),
    margin: (salesTotal - channelTotal).toFixed(2),
  }
})

const selectedAssistedOffer = computed(() => {
  const item = props.approval
  if (item?.kind !== 'assisted_order') return null
  return item.offers.find(offer => String(offer.product_id) === String(item.productId)) ?? null
})

const assistedTotalText = computed(() => {
  const offer = selectedAssistedOffer.value
  const qty = Number(props.approval?.quantity) || 0
  if (!offer || qty < 1) return '¥0.00'
  return props.formatMoneyCents(BigInt(offer.unit_cents) * BigInt(qty))
})

const primaryLabel = computed(() => {
  const item = props.approval
  if (!item) return '同意并自动执行'
  if (item.kind === 'send_draft' || (item.kind === 'inquiry_workflow' && item.autoSend)) return '同意并发送询价'
  if (item.kind === 'inquiry_workflow' && item.intent === 'match_products') return '同意并匹配产品'
  return '同意并自动执行'
})

function markDirty() {
  if (props.approval?.kind === 'inquiry_workflow') {
    props.approval.fieldsDirty = true
  }
}

function onInvoiceOrderChange() {
  const item = props.approval
  if (item?.kind !== 'apply_invoice') return
  const chosen = item.orders.find(order => String(order.id) === String(item.orderId))
  if (chosen?.customer) item.invoiceTitle = chosen.customer
}
</script>

<template>
  <ElDialog
    :model-value="open"
    :title="approval?.title || '人工审批'"
    width="min(94vw, 700px)"
    :close-on-click-modal="false"
    @update:model-value="value => { if (!value) emit('close') }"
  >
    <form v-if="approval" class="ai-approval" @submit.prevent="emit('approve')">
      <div class="ai-result-head">
        <strong>{{ skillLabels[approval.skillId] || approval.title }}</strong>
        <UiBadge tone="warning">待人工审批</UiBadge>
      </div>

      <section v-if="steps.length" class="ai-pipeline-preview" aria-label="批准后自动执行步骤">
        <p class="ai-pipeline-title">批准后 AI 将自动连续执行以下 {{ steps.length }} 步：</p>
        <ol class="ai-pipeline-steps">
          <li v-for="(step, index) in steps" :key="index">{{ step }}</li>
        </ol>
      </section>

      <!-- 1. 采购询价 / 产品匹配打包审批 -->
      <template v-if="approval.kind === 'inquiry_workflow'">
        <details v-if="approval.source">
          <summary>查看需求原文</summary>
          <p class="ai-original">{{ approval.source }}</p>
        </details>

        <template v-if="approval.intent !== 'match_products'">
          <UiField label="关联项目" v-slot="{ control }">
            <select v-bind="control" v-model="approval.projectId" required>
              <option v-if="approval.canAutoCreate" value="__create__">自动新建采购项目（一步到位）</option>
              <option v-if="!approval.canAutoCreate" value="">选择待询价项目</option>
              <option v-for="project in inquiryProjects" :key="project.id" :value="project.id">
                已有项目：{{ project.title }} · {{ project.id }}
              </option>
            </select>
          </UiField>

          <div v-if="approval.projectId === '__create__'" class="ai-fields">
            <UiField label="新项目名称" v-slot="{ control }">
              <input v-bind="control" v-model="approval.newProjectTitle" maxlength="120" required placeholder="输入采购项目名称">
            </UiField>
            <UiField v-if="!demo" label="关联企业客户" v-slot="{ control }">
              <select v-bind="control" v-model="approval.customerId" required>
                <option value="">选择已关联客户</option>
                <option v-for="client in approval.customers" :key="client.id" :value="client.id">{{ client.name }}</option>
              </select>
            </UiField>
          </div>

          <UiButton
            v-if="!demo && !approval.canAutoCreate && !inquiryProjects.length"
            compact
            variant="text"
            @click="emit('open-projects')"
          >
            前往项目协作查看关联客户
          </UiButton>
        </template>

        <div class="ai-fields">
          <UiField v-for="field in requirementFields" :key="field.key" :label="field.label" :hint="field.hint" v-slot="{ control }">
            <input
              v-if="field.numeric"
              v-bind="control"
              v-model="approval.fields[field.key]"
              type="number"
              min="1"
              max="1000000"
              placeholder="待确认"
              @input="markDirty"
            >
            <input
              v-else
              v-bind="control"
              v-model="approval.fields[field.key]"
              :maxlength="field.max"
              placeholder="待确认"
              @input="markDirty"
            >
          </UiField>
        </div>

        <aside v-if="approval.questions?.length" class="ai-follow-up">
          <strong><UiIcon name="info" :size="16" /> 建议向客户核实</strong>
          <ul><li v-for="(question, index) in approval.questions" :key="index">{{ question }}</li></ul>
        </aside>

        <section v-if="approval.intent !== 'match_products'" class="ai-packaged-candidates">
          <div class="ai-result-head">
            <strong>候选产品与匹配依据</strong>
            <UiButton compact variant="text" :disabled="Boolean(working)" @click="emit('refresh-matches')">
              {{ approval.fieldsDirty ? '按修改后的条件重新匹配' : '刷新匹配' }}
            </UiButton>
          </div>
          <p v-if="!eligibleMatches.length" class="ai-muted-hint">
            {{ approval.matches?.length ? '当前条件暂无符合产品，调整上方字段后点击重新匹配。' : '填写需求字段后，同意时将自动匹配并锁定候选产品。' }}
          </p>
          <div v-else class="ai-candidate-list">
            <label
              v-for="item in eligibleMatches"
              :key="item.product.id"
              class="ai-candidate-option"
              :class="{ 'is-selected': String(approval.selectedProductId) === String(item.product.id) }"
            >
              <input
                v-model="approval.selectedProductId"
                type="radio"
                name="ai-candidate-product"
                :value="String(item.product.id)"
                :aria-label="`${item.product.name}${item.product.version ? ' ' + item.product.version : ''}`"
              >
              <div class="ai-candidate-body">
                <div class="ai-result-head">
                  <strong>{{ item.product.name }} {{ item.product.version || '' }}</strong>
                  <UiBadge :tone="fitLabels[item.status]?.[1]">{{ fitLabels[item.status]?.[0] ?? '待核实' }}</UiBadge>
                </div>
                <ul v-if="item.checks?.length" class="ai-checks">
                  <li v-for="(check, idx) in item.checks" :key="idx">
                    <span :data-check="check.status">{{ checkLabels[check.status] }}</span>
                    <span>{{ check.value ? check.value + ' · ' : '' }}{{ check.detail }}</span>
                  </li>
                </ul>
              </div>
            </label>
          </div>

          <label class="ai-checkbox-row">
            <input v-model="approval.autoSend" type="checkbox">
            <span>保存询价草稿后<strong>立即正式发送给所属软件厂商</strong>（不勾选则仅保存内部草稿，厂商暂不可见）</span>
          </label>
        </section>
      </template>

      <!-- 2. 已有草稿直接发送厂商询价 -->
      <template v-else-if="approval.kind === 'send_draft'">
        <p>同意后，以下脱敏询价内容将正式提交给产品所属厂商，项目进入待报价阶段。</p>
        <dl class="ai-send-details">
          <div><dt>项目</dt><dd>{{ approval.project.title }}</dd></div>
          <div><dt>产品</dt><dd>{{ approval.product.name }} {{ approval.product.version }}</dd></div>
          <div v-if="approval.product.manufacturerName || approval.product.manufacturer_id">
            <dt>厂商</dt><dd>{{ approval.product.manufacturerName || '#' + approval.product.manufacturer_id }}</dd>
          </div>
          <div><dt>询价摘要</dt><dd>{{ approval.summary }}</dd></div>
        </dl>
      </template>

      <!-- 3. 对客方案报价打包审批 -->
      <template v-else-if="approval.kind === 'sales_quote'">
        <dl class="ai-send-details">
          <div><dt>目标项目</dt><dd>{{ approval.project.title }}（{{ approval.project.customerName || '关联客户' }}）</dd></div>
          <div><dt>渠道成本</dt><dd>¥{{ salesQuoteTotals?.channel }}（引用 {{ approval.channelQuote.id }}，客户不可见）</dd></div>
          <div><dt>对客合计</dt><dd><strong>¥{{ salesQuoteTotals?.sales }}</strong>（预计毛利 ¥{{ salesQuoteTotals?.margin }}）</dd></div>
        </dl>

        <div class="ai-fields">
          <UiField label="整体加价比例 (%)" v-slot="{ control }">
            <div class="ai-inline-action">
              <input v-bind="control" v-model.number="approval.markupPercent" type="number" min="0" max="500" step="0.5">
              <UiButton compact @click="emit('apply-markup')">重算单价</UiButton>
            </div>
          </UiField>
          <UiField label="交付周期（天）" v-slot="{ control }">
            <input v-bind="control" v-model.number="approval.deliveryDays" type="number" min="1" max="3650" required>
          </UiField>
          <UiField label="报价有效期至" v-slot="{ control }">
            <input v-bind="control" v-model="approval.validUntil" type="date" required>
          </UiField>
          <UiField label="方案备注" v-slot="{ control }">
            <input v-bind="control" v-model="approval.note" maxlength="300" placeholder="面向客户的交付与服务说明">
          </UiField>
        </div>

        <div class="ai-quote-lines">
          <div v-for="(line, index) in approval.lines" :key="index" class="ai-quote-line">
            <UiField :label="`明细 ${index + 1}`" v-slot="{ control }">
              <input v-bind="control" v-model="line.label" maxlength="120" required>
            </UiField>
            <UiField label="数量" v-slot="{ control }">
              <input v-bind="control" v-model.number="line.quantity" type="number" min="1" max="1000000" required>
            </UiField>
            <UiField :label="`对客单价（进价 ¥${line.channelUnitPrice}）`" v-slot="{ control }">
              <input v-bind="control" v-model="line.unitPrice" inputmode="decimal" required>
            </UiField>
          </div>
        </div>
      </template>

      <!-- 4. 代客确认正式下单 -->
      <template v-else-if="approval.kind === 'place_order'">
        <dl class="ai-send-details">
          <div><dt>项目</dt><dd>{{ approval.project.title }}</dd></div>
          <div><dt>企业客户</dt><dd>{{ approval.project.customerName || '已关联客户' }}</dd></div>
          <div><dt>已确认报价</dt><dd>{{ approval.salesQuote.id }} · ¥{{ Number(approval.salesQuote.total).toFixed(2) }}（有效期至 {{ approval.salesQuote.validUntil }}）</dd></div>
          <div><dt>上游渠道单</dt><dd>{{ approval.channelQuote.id }} · ¥{{ Number(approval.channelQuote.total).toFixed(2) }}（有效期至 {{ approval.channelQuote.validUntil }}）</dd></div>
        </dl>
      </template>

      <!-- 5. 提交项目实施验收 -->
      <template v-else-if="approval.kind === 'submit_acceptance'">
        <dl class="ai-send-details">
          <div><dt>项目</dt><dd>{{ approval.project.title }}</dd></div>
          <div v-if="approval.project.supplyNote"><dt>供货记录</dt><dd>{{ approval.project.supplyNote }}</dd></div>
        </dl>
        <UiField label="实施完成与交接说明" required v-slot="{ control }">
          <textarea v-bind="control" v-model="approval.note" rows="3" maxlength="1000" required />
        </UiField>
      </template>

      <!-- 6. 记拜访并转团队商机 -->
      <template v-else-if="approval.kind === 'visit_opportunity'">
        <UiField label="关联客户项目" v-slot="{ control }">
          <select v-bind="control" v-model="approval.projectId" required>
            <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.title }} · {{ project.id }}</option>
          </select>
        </UiField>
        <UiField label="提取出的附加商机需求" hint="不会改动主项目与主订单" required v-slot="{ control }">
          <textarea v-bind="control" v-model="approval.additionalNote" rows="2" maxlength="500" required />
        </UiField>
        <div class="ai-fields ai-fields-three">
          <UiField label="能力类型" v-slot="{ control }">
            <select v-bind="control" v-model="approval.criteria.category" required @change="emit('refresh-visit')">
              <option value="">选择类型</option>
              <option v-for="(label, key) in approval.categoryOptions" :key="key" :value="key">{{ label }}</option>
            </select>
          </UiField>
          <UiField label="源系统" v-slot="{ control }">
            <select v-bind="control" v-model="approval.criteria.source" @change="emit('refresh-visit')">
              <option value="">不限 / 待定</option>
              <option v-for="(label, key) in approval.systemOptions" :key="key" :value="key">{{ label }}</option>
            </select>
          </UiField>
          <UiField label="目标系统" v-slot="{ control }">
            <select v-bind="control" v-model="approval.criteria.target" @change="emit('refresh-visit')">
              <option value="">不限 / 无</option>
              <option v-for="(label, key) in approval.systemOptions" :key="key" :value="key">{{ label }}</option>
            </select>
          </UiField>
        </div>
        <aside class="ai-follow-up">
          <strong><UiIcon name="workflow" :size="16" /> 团队可引荐伙伴预匹配（{{ approval.partners?.length || 0 }} 人）</strong>
          <ul v-if="approval.partners?.length">
            <li v-for="partner in approval.partners.slice(0, 3)" :key="partner.partnerId || partner.id">
              {{ partner.name }}（内部引荐人：{{ partner.owner }}）
            </li>
          </ul>
          <p v-else class="ai-muted-hint">当前条件下暂无现成登记伙伴，沉淀商机后可作为能力缺口由团队跟进。</p>
        </aside>
      </template>

      <!-- 7. 代客直购下单 -->
      <template v-else-if="approval.kind === 'assisted_order'">
        <div class="ai-fields">
          <UiField label="关联企业客户" v-slot="{ control }">
            <select v-bind="control" v-model="approval.customerId" required>
              <option v-for="client in approval.customers" :key="client.id" :value="String(client.id)">{{ client.name }}</option>
            </select>
          </UiField>
          <UiField label="直购上架软件" v-slot="{ control }">
            <select v-bind="control" v-model="approval.productId" required>
              <option v-for="offer in approval.offers" :key="offer.product_id" :value="String(offer.product_id)">
                {{ offer.name }} · {{ formatMoneyCents(offer.unit_cents) }}{{ offer.annual ? '/年' : '' }}
              </option>
            </select>
          </UiField>
          <UiField label="订购份数" v-slot="{ control }">
            <input v-bind="control" v-model.number="approval.quantity" type="number" min="1" max="10000" required>
          </UiField>
        </div>
        <dl class="ai-send-details">
          <div><dt>所属厂商</dt><dd>{{ selectedAssistedOffer?.manufacturer || '合作厂商' }}</dd></div>
          <div><dt>订单总额</dt><dd><strong>{{ assistedTotalText }}</strong>（客户确认并付款后自动计入本组织佣金台账）</dd></div>
        </dl>
      </template>

      <!-- 8. 催办待结佣金 -->
      <template v-else-if="approval.kind === 'remind_commission'">
        <div class="ai-candidate-list">
          <label v-for="order in approval.orders" :key="order.id" class="ai-candidate-option is-selected">
            <input v-model="approval.selectedOrderIds" type="checkbox" :value="String(order.id)">
            <div class="ai-candidate-body">
              <div class="ai-result-head">
                <strong>订单 #{{ order.id }} · {{ order.product_name }}</strong>
                <UiBadge tone="info">待结佣金 {{ formatMoneyCents(order.commission?.cents ?? 0) }}</UiBadge>
              </div>
              <p>{{ order.customer }} · {{ order.manufacturer }}</p>
            </div>
          </label>
        </div>
      </template>

      <!-- 9. 申请订单开票 -->
      <template v-else-if="approval.kind === 'apply_invoice'">
        <UiField label="选择已收款直购订单" v-slot="{ control }">
          <select v-bind="control" v-model="approval.orderId" required @change="onInvoiceOrderChange">
            <option v-for="order in approval.orders" :key="order.id" :value="String(order.id)">
              订单 #{{ order.id }} · {{ order.product_name }} · {{ formatMoneyCents(order.total_cents) }}（{{ order.customer }}）
            </option>
          </select>
        </UiField>
        <UiField label="发票抬头" required v-slot="{ control }">
          <input v-bind="control" v-model="approval.invoiceTitle" maxlength="80" required>
        </UiField>
      </template>

      <!-- 10. 提交厂商服务工单 -->
      <template v-else-if="approval.kind === 'create_ticket'">
        <div class="ai-fields">
          <UiField label="工单标题" required v-slot="{ control }">
            <input v-bind="control" v-model="approval.subject" maxlength="80" required>
          </UiField>
          <UiField label="问题类型" v-slot="{ control }">
            <select v-bind="control" v-model="approval.category">
              <option value="产品使用">产品使用</option>
              <option value="授权与交付">授权与交付</option>
              <option value="订单与结算">订单与结算</option>
            </select>
          </UiField>
          <UiField label="关联直购订单" v-slot="{ control }">
            <select v-bind="control" v-model="approval.orderId">
              <option value="">无需关联具体订单</option>
              <option v-for="order in approval.orders" :key="order.id" :value="String(order.id)">
                订单 #{{ order.id }} · {{ order.product_name }}
              </option>
            </select>
          </UiField>
          <UiField v-if="!approval.demo && !approval.orderId" label="合作软件厂商" required v-slot="{ control }">
            <select v-bind="control" v-model="approval.manufacturerId" required>
              <option value="">选择合作厂商</option>
              <option v-for="partner in approval.partners" :key="partner.id" :value="String(partner.id)">{{ partner.name }}</option>
            </select>
          </UiField>
        </div>
        <UiField label="问题描述" required v-slot="{ control }">
          <textarea v-bind="control" v-model="approval.detail" rows="3" maxlength="500" required />
        </UiField>
      </template>

      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <div class="ai-approval-actions">
        <UiButton :disabled="Boolean(working)" @click="emit('decline')">不同意</UiButton>
        <UiButton type="submit" variant="primary" icon="check" :loading="Boolean(working)">{{ primaryLabel }}</UiButton>
      </div>
    </form>
  </ElDialog>
</template>
