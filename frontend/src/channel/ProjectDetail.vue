<script setup>
import { computed } from 'vue'
import { UiBadge, UiButton, UiCard } from '../ui/index.js'
import { actionLabels, projectHandlerLabel } from './domain.js'
import QuoteBook from './QuoteBook.vue'
import UiProjectProgress from '../ui/UiProjectProgress.vue'
import { statusTone } from '../ui/statusTone.js'
const props = defineProps({ project: { type: Object, required: true }, role: String, products: { type: Array, default: () => [] }, opportunities: { type: Array, default: () => [] }, live: Boolean })
defineEmits(['action', 'back', 'capture', 'opportunity'])
const clock = date => new Date(date).toLocaleString('zh-CN', {month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})
const stageOwner = computed(() => projectHandlerLabel(props.project, props.role))
</script>
<template>
  <div class="project-detail-toolbar">
    <UiButton compact icon="back" variant="text" @click="$emit('back')">返回项目列表</UiButton>
    <div v-if="$slots.toolbar" class="workspace-actions"><slot name="toolbar" /></div>
  </div>
  <header v-enter class="ch-project-head">
    <h1>{{ project.title }}</h1>
    <div class="ch-project-meta"><span>编号 {{ project.id }}</span><span v-if="project.customerName">{{ project.customerName }}</span><span v-if="project.dueDate">目标日期 {{ project.dueDate }}</span></div>
  </header>
  <section v-enter="1" class="ch-next" aria-label="当前办理">
    <dl class="project-current">
      <div><dt>当前阶段</dt><dd><UiBadge :tone="statusTone(project.stage)" dot>{{ project.stageLabel }}</UiBadge></dd></div>
      <div><dt>经办方</dt><dd>{{ stageOwner }}</dd></div>
      <div v-if="!project.isTodo || !project.actions.length"><dt>下一步</dt><dd>{{ project.nextStep }}</dd></div>
    </dl>
    <div v-if="project.actions.length" class="ch-buttons" aria-label="可办理操作" role="group">
      <UiButton v-for="(action, index) in project.actions" :key="action"
        :variant="project.isTodo && index === 0 ? 'primary' : 'secondary'" @click="$emit('action', action)">{{ actionLabels[action] }}</UiButton>
    </div>
  </section>
  <UiProjectProgress :role="role" :stage="project.stage" :label="project.stageLabel" />
  <div class="ch-detail-grid">
    <div v-enter:children="2" class="ch-stack">
      <UiCard v-if="project.requirement" title="采购需求">
        <p class="project-copy">{{ project.requirement }}</p>
      </UiCard>
      <UiCard v-if="project.product || project.inquiry" title="产品与询价">
        <dl class="ch-facts">
          <template v-if="project.product"><dt>选用产品</dt><dd>{{ project.productName || products.find(item => item.id === project.product)?.name || project.product }}</dd></template>
          <template v-if="project.inquiry"><dt>询价摘要</dt><dd>{{ project.inquiry.summary }}</dd></template>
        </dl>
      </UiCard>
      <UiCard v-if="role === 'dealer' && !live" title="主单之外的商机" description="原厂没有对应产品，或者客户提到新的需求？留下一条团队可跟进的线索。">
        <p v-if="project.inquiry?.gapNote" class="op-inset">厂商反馈：{{ project.inquiry.gapNote }}</p>
        <div v-if="opportunities.length" class="op-project-list">
          <button v-for="item in opportunities" :key="item.id" class="ch-record" @click="$emit('opportunity', item.id)"><strong>{{ item.note }}</strong><small>{{ item.id }} · 查看商机</small></button>
        </div>
        <UiButton icon="add" @click="$emit('capture')">记录走访与补充需求</UiButton>
      </UiCard>
      <UiCard v-if="live && project.inquiry?.gapNote" title="厂商能力缺口"><p>{{ project.inquiry.gapNote }}</p></UiCard>
      <QuoteBook v-if="project.channelQuotes && (!live || project.channelQuotes.length)" title="厂商渠道报价" subtitle="厂商与代理商之间的采购条件" :quotes="project.channelQuotes" />
      <QuoteBook v-if="project.salesQuotes && (!live || project.salesQuotes.length)" title="对客销售报价" subtitle="代理商向客户提供的软件授权与实施方案" :quotes="project.salesQuotes" />
      <UiCard v-if="project.order" title="订单与履约">
        <dl class="ch-facts"><dt>订单编号</dt><dd>{{ project.order.id }}</dd>
          <template v-if="project.order.channelQuoteId"><dt>渠道报价版本</dt><dd>{{ project.order.channelQuoteId }}</dd></template>
          <template v-if="project.order.salesQuoteId"><dt>对客报价版本</dt><dd>{{ project.order.salesQuoteId }}</dd></template>
          <dt>供货记录</dt><dd>{{ project.supplyNote || '待厂商交接' }}</dd>
          <template v-if="role !== 'vendor'"><dt>实施记录</dt><dd>{{ project.implementationNote || '待代理商实施' }}</dd></template>
        </dl>
      </UiCard>
    </div>
    <UiCard v-enter="2" title="项目时间线">
      <ol v-if="project.timeline.length" class="ch-timeline"><li v-for="(event,index) in [...project.timeline].reverse()" :key="event.at + ':' + index"><time :datetime="event.at">{{ clock(event.at) }}</time><strong>{{ event.label }}</strong><p v-if="event.detail">{{ event.detail }}</p></li></ol>
      <p v-else class="muted">暂无可见的协作记录。</p>
    </UiCard>
  </div>
</template>

<style scoped>
.project-detail-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 4px 12px;
}
.project-detail-toolbar > .ui-button { margin-left: -10px; }
.project-current { display: flex; flex-wrap: wrap; align-items: start; gap: 8px 24px; min-width: 0; }
.project-current dt { color: var(--ui-ink-muted); font-size: 12px; margin-bottom: 2px; }
.project-current dd { margin: 0; overflow-wrap: anywhere; }
.project-copy { white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
