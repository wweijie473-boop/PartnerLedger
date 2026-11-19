<script setup>
import { computed } from 'vue'
import { UiBadge, UiCard } from '../ui/index.js'
import { statusTone } from '../ui/statusTone.js'
const props = defineProps({ title: String, subtitle: String, quotes: { type: Array, required: true } })
const versions = computed(() => [...props.quotes].sort((left, right) => right.version - left.version))
const currency = amount => Number(amount).toLocaleString('zh-CN', { style: 'currency', currency: 'CNY' })
const statusNames = { active: '有效', confirmed: '已确认', superseded: '已替代', expired: '已过期', ordered: '订单已锁定' }
</script>
<template>
  <UiCard :title="title" :description="subtitle">
    <p v-if="!versions.length" class="ch-empty-note">尚未出具报价。</p>
    <details v-for="(item, index) in versions" :key="item.id" class="ch-version" :open="index === 0">
      <summary><span><strong>V{{ item.version }}</strong> <UiBadge :tone="statusTone(item.status)">{{ statusNames[item.status] ?? item.status }}</UiBadge></span><strong>{{ currency(item.total) }}</strong></summary>
      <p class="ch-metadata">{{ item.id }} · 有效至 {{ item.validUntil }} · {{ item.deliveryDays }} 天交付</p>
      <div v-for="(line, row) in item.lines" :key="row" class="ch-quote-detail"><strong>{{ line.label }}</strong><small>{{ line.quantity }} × {{ currency(line.unitPrice) }}</small><span>{{ currency(line.quantity * line.unitPrice) }}</span></div>
      <p v-if="item.note" class="ch-version-note">{{ item.note }}</p>
      <p v-if="versions[index + 1]" class="ch-metadata">较上一版：总额{{ Number(item.total) === Number(versions[index + 1].total) ? '不变' : (Number(item.total) > Number(versions[index + 1].total) ? '增加 ' : '减少 ') + currency(Math.abs(item.total - versions[index + 1].total)) }}；交付周期 {{ versions[index + 1].deliveryDays }} → {{ item.deliveryDays }} 天。</p>
    </details>
  </UiCard>
</template>
