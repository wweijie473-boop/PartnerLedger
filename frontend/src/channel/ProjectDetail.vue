<script setup>
import { UiBadge, UiButton, UiCard, UiIcon } from '../ui/index.js'
import { actionLabels } from './domain.js'
import QuoteBook from './QuoteBook.vue'
import UiProjectProgress from '../ui/UiProjectProgress.vue'
import { statusTone } from '../ui/statusTone.js'
defineProps({ project: { type: Object, required: true }, role: String, products: { type: Array, default: () => [] }, opportunities: { type: Array, default: () => [] }, live: Boolean })
defineEmits(['action', 'back', 'capture', 'opportunity'])
const clock = date => new Date(date).toLocaleString('zh-CN', {month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false})
</script>
<template>
  <UiButton compact icon="back" variant="text" @click="$emit('back')">返回工作列表</UiButton>
  <header class="ch-project-head"><p class="ch-kicker">PROJECT FILE &nbsp; / &nbsp; {{ project.id }}</p><h1>{{ project.title }}</h1>
    <div class="ch-project-meta"><UiBadge :tone="statusTone(project.stage)" dot>{{ project.stageLabel }}</UiBadge><span v-if="project.customerName">{{ project.customerName }}</span><span v-if="project.dueDate">目标日期 {{ project.dueDate }}</span></div>
  </header>
  <UiProjectProgress :role="role" :stage="project.stage" :label="project.stageLabel" />
  <section class="ch-next"><div><p class="ch-kicker">下一步</p><h2>{{ project.nextStep }}</h2></div><div class="ch-buttons"><UiButton v-for="(action,index) in project.actions" :key="action" :variant="index ? 'secondary' : 'primary'" @click="$emit('action', action)">{{ actionLabels[action] }}</UiButton></div></section>
  <div class="ch-detail-grid">
    <div class="ch-stack">
      <UiCard title="业务与需求">
        <dl class="ch-facts">
          <template v-if="project.requirement"><dt>原始需求</dt><dd>{{ project.requirement }}</dd></template>
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
      <QuoteBook v-if="!live && project.channelQuotes" title="厂商渠道报价" subtitle="厂商与代理商之间的采购条件" :quotes="project.channelQuotes" />
      <QuoteBook v-if="!live && project.salesQuotes" title="对客销售报价" subtitle="代理商向客户提供的软件授权与实施方案" :quotes="project.salesQuotes" />
      <UiCard v-if="project.order" title="订单与履约">
        <dl class="ch-facts"><dt>订单编号</dt><dd>{{ project.order.id }}</dd>
          <template v-if="project.order.channelQuoteId"><dt>渠道报价版本</dt><dd>{{ project.order.channelQuoteId }}</dd></template>
          <template v-if="project.order.salesQuoteId"><dt>对客报价版本</dt><dd>{{ project.order.salesQuoteId }}</dd></template>
          <dt>供货记录</dt><dd>{{ project.supplyNote || '待厂商交接' }}</dd>
          <template v-if="role !== 'vendor'"><dt>实施记录</dt><dd>{{ project.implementationNote || '待代理商实施' }}</dd></template>
        </dl>
      </UiCard>
    </div>
    <UiCard title="项目时间线" description="当前身份可见的协作记录">
      <ol class="ch-timeline"><li v-for="(event,index) in [...project.timeline].reverse()" :key="event.at + ':' + index"><time>{{ clock(event.at) }}</time><strong>{{ event.label }}</strong><p v-if="event.detail">{{ event.detail }}</p></li></ol>
    </UiCard>
  </div>
</template>
