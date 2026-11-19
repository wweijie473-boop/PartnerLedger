<script setup>
import { ref } from 'vue'
import { UiBadge, UiButton, UiCard, UiField, UiIcon, UiIconTile, UiMetricCard, UiProgress, UiState } from './ui/index.js'

const palette = [
  ['主操作与标题', '--ui-accent'], ['正文', '--ui-ink-secondary'], ['次要文字', '--ui-ink-muted'],
  ['页面底色', '--ui-canvas'], ['卡片表面', '--ui-surface'], ['分隔线', '--ui-border'],
]
const tones = [
  ['neutral', '未提交'], ['info', '交付中'], ['success', '已完成'], ['warning', '待确认'], ['danger', '操作失败'],
]
const iconGroups = [
  { title: '业务与导航', names: ['overview', 'workflow', 'catalog', 'product', 'request', 'quote', 'order', 'delivery', 'dealer', 'vendor', 'activity'] },
  { title: '动作与反馈', names: ['add', 'search', 'back', 'forward', 'open-detail', 'chevron-right', 'chevron-down', 'sign-out', 'check', 'time', 'info', 'permission', 'error', 'loading'] },
]
const sections = [
  ['colors', '配色'], ['icons', '图标'], ['cards', '卡片'], ['controls', '控件'], ['states', '界面状态'],
]
const stateOptions = [
  { id: 'empty', label: '空数据', title: '暂无询价记录', description: '新提交的询价会显示在这里。' },
  { id: 'loading', label: '加载中', title: '正在加载询价记录' },
  { id: 'denied', label: '无权限', title: '你没有权限查看这些记录', description: '请联系所属组织的管理员。' },
  { id: 'error', label: '失败', title: '询价加载失败', description: '请稍后重试。' },
]
const activeState = ref(stateOptions[0])
const selectedProduct = ref('std')
const name = ref('制造企业 ERP 升级')
const amount = ref('')
const saveStatus = ref('')
const showLicense = ref(false)

function colorValue(token) {
  return getComputedStyle(document.documentElement).getPropertyValue(token).trim().toUpperCase()
}
</script>

<template>
  <div class="ui-gallery">
    <header class="gallery-top">
      <div class="gallery-top-inner">
        <a href="/" class="gallery-back"><UiIcon name="back" :size="16" /> 返回平台首页</a>
        <span class="gallery-brand"><UiIcon name="workflow" :size="16" /> 渠智链 AI</span>
      </div>
    </header>
    <main class="gallery-content">
      <div class="gallery-heading">
        <h1>UI 组件库</h1>
        <p>渠智链 AI 的基础样式和可复用组件。</p>
      </div>
      <nav class="gallery-nav" aria-label="组件目录">
        <a v-for="[id, label] in sections" :key="id" :href="`#${id}`">{{ label }}</a>
      </nav>

      <UiCard id="colors" title="黑白配色" description="用黑色突出主操作，白色和灰阶区分页面层级。状态由文字和图标表达。">
        <ul class="swatches">
          <li v-for="[label, token] in palette" :key="token">
            <div class="swatch" :style="{ background: `var(${token})` }"></div>
            <strong>{{ label }}</strong><code>{{ colorValue(token) }}</code><code>{{ token }}</code>
          </li>
        </ul>
        <div class="gallery-line tone-samples">
          <UiBadge v-for="[tone, label] in tones" :key="tone" :tone="tone" dot>{{ label }}</UiBadge>
        </div>
      </UiCard>

      <UiCard id="icons" title="图标" description="用业务名称引用图标。同一种操作在各页面使用同一个图标。">
        <div v-for="group in iconGroups" :key="group.title" class="icon-group">
          <h3>{{ group.title }}</h3>
          <ul class="icon-list">
            <li v-for="name in group.names" :key="name"><UiIconTile :name="name" /><code>{{ name }}</code></li>
          </ul>
        </div>
      </UiCard>

      <UiCard id="cards" title="卡片" description="统计卡适合总览，内容卡适合产品详情和业务区块。">
        <div class="gallery-metrics">
          <UiMetricCard label="询价记录" :value="16" description="已提交的需求" icon="request" tone="brand" />
          <UiMetricCard label="待确认报价" :value="4" description="等待经销商确认" icon="quote" tone="warning" />
          <UiMetricCard label="交付中订单" :value="7" description="已进入交付" icon="order" tone="info" />
        </div>
        <div class="gallery-example-grid">
          <UiCard title="渠智 ERP · 专业版" heading="h3" description="财务、采购、库存、销售">
            <template #actions><UiBadge tone="success" dot>可询价</UiBadge></template>
            <dl class="gallery-details"><dt>部署方式</dt><dd>私有部署 / 云部署</dd><dt>授权模式</dt><dd>按授权用户</dd></dl>
            <p v-show="showLicense" id="sample-license" class="license-note">按确认的授权人数计费，报价时可同时选择实施服务。</p>
            <template #footer>
              <UiButton variant="text" end-icon="chevron-down" aria-controls="sample-license"
                :aria-expanded="showLicense" @click="showLicense = !showLicense">
                {{ showLicense ? '收起授权说明' : '展开授权说明' }}
              </UiButton>
            </template>
          </UiCard>
          <UiCard title="交付进度" heading="h3" description="授权、部署、迁移、交接" muted>
            <div class="gallery-progress"><strong>50%</strong><span>已完成 2 / 4 个节点</span></div>
            <UiProgress :value="50" label="交付进度" />
          </UiCard>
        </div>
      </UiCard>

      <UiCard id="controls" title="按钮与表单" description="每个操作区保留一个主要按钮。输入项提供明确的标签和错误提示。">
        <div class="gallery-line button-samples">
          <UiButton variant="primary" icon="add" @click="saveStatus = '已预览主操作'">新建需求</UiButton>
          <UiButton @click="saveStatus = '已预览次要操作'">保存草稿</UiButton>
          <UiButton variant="text" end-icon="forward" @click="saveStatus = '已预览文字操作'">查看明细</UiButton>
          <UiButton variant="danger" @click="saveStatus = '已预览危险操作'">撤回询价</UiButton>
          <UiButton variant="primary" loading>提交中</UiButton>
          <UiButton disabled>不可用</UiButton>
        </div>
        <div class="gallery-fields">
          <UiField label="需求名称" required v-slot="{ control }"><input v-bind="control" v-model="name" /></UiField>
          <UiField label="ERP 产品" v-slot="{ control }"><select v-bind="control" v-model="selectedProduct"><option value="std">标准版</option><option value="pro">专业版</option></select></UiField>
          <UiField label="报价金额" required :error="Number(amount) > 0 ? '' : '请输入大于 0 的报价金额。'" v-slot="{ control }">
            <input v-bind="control" v-model="amount" type="number" placeholder="0.00" />
          </UiField>
        </div>
        <p v-if="saveStatus" role="status" class="gallery-feedback">{{ saveStatus }}</p>
      </UiCard>

      <UiCard id="states" title="界面状态" description="列表和卡片共用加载、空数据、无权限和失败反馈。">
        <div class="gallery-line state-options" role="group" aria-label="预览界面状态">
          <UiButton v-for="state in stateOptions" :key="state.id" compact :variant="activeState.id === state.id ? 'primary' : 'secondary'"
            :aria-pressed="activeState.id === state.id" @click="activeState = state">{{ state.label }}</UiButton>
        </div>
        <div class="state-preview">
          <UiState :state="activeState.id" :title="activeState.title" :description="activeState.description">
            <UiButton v-if="activeState.id === 'error'" compact @click="activeState = stateOptions[0]">重试</UiButton>
          </UiState>
        </div>
      </UiCard>
    </main>
  </div>
</template>

<style scoped>
.ui-gallery { font-family: var(--ui-font); color: var(--ui-ink); background: var(--ui-canvas); min-height: 100vh; }
.gallery-top { position: sticky; top: 0; z-index: 5; border-bottom: 1px solid var(--ui-border); background: var(--ui-header-glass); backdrop-filter: blur(18px); }
.gallery-top-inner, .gallery-content { max-width: 1120px; margin: 0 auto; padding-inline: clamp(16px, 4vw, 32px); }
.gallery-top-inner { display: flex; min-height: 58px; align-items: center; justify-content: space-between; gap: 12px; }
.gallery-back, .gallery-brand { display: inline-flex; align-items: center; gap: 6px; font-size: var(--ui-text-sm); font-weight: var(--ui-weight-semibold); }
.gallery-back { color: var(--ui-accent); text-decoration: none; }
.gallery-back:hover { text-decoration: underline; }
.gallery-content { display: grid; gap: var(--ui-space-5); padding-block: clamp(28px, 5vw, 58px) var(--ui-space-8); }
.gallery-content > section { scroll-margin-top: 76px; }
.gallery-heading h1 { margin: 0 0 var(--ui-space-2); font-size: var(--ui-text-display); letter-spacing: -.045em; }
.gallery-heading p { margin: 0; color: var(--ui-ink-muted); font-size: var(--ui-text-md); }
.gallery-nav { display: flex; flex-wrap: wrap; gap: var(--ui-space-2) var(--ui-space-4); padding-block: var(--ui-space-2); margin: 0; }
.gallery-nav a { padding: 6px 12px; border: 1px solid var(--ui-border); border-radius: var(--ui-radius-pill); background: var(--ui-surface); color: var(--ui-accent); font-size: var(--ui-text-sm); text-decoration: none; }
.gallery-nav a:hover { background: var(--ui-surface-hover); }
.gallery-line { display: flex; gap: var(--ui-space-2); flex-wrap: wrap; align-items: center; }
.swatches, .icon-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(132px, 1fr)); list-style: none; padding: 0; margin: 0; gap: var(--ui-space-3); }
.swatches { grid-template-columns: repeat(6, minmax(0, 1fr)); }
.swatches li { display: grid; gap: var(--ui-space-1); font-size: var(--ui-text-sm); }
.swatches .swatch { height: 48px; margin-bottom: var(--ui-space-1); border: 1px solid var(--ui-border); border-radius: var(--ui-radius-md); }
.ui-gallery code { font-family: var(--ui-font-mono); font-size: 11px; color: var(--ui-ink-muted); overflow-wrap: anywhere; }
.tone-samples { margin-top: var(--ui-space-4); }
.icon-group + .icon-group { margin-top: var(--ui-space-4); padding-top: var(--ui-space-4); border-top: 1px solid var(--ui-border); }
.icon-group h3 { margin: 0 0 var(--ui-space-3); font-size: var(--ui-text-sm); }
.icon-list { grid-template-columns: repeat(auto-fit, minmax(112px, 1fr)); }
.icon-list li { display: flex; align-items: center; gap: 7px; min-width: 0; }
.icon-list .ui-icon-tile { --ui-tile-size: 30px; }
.gallery-metrics, .gallery-example-grid, .gallery-fields { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--ui-space-3); }
.gallery-example-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); margin-top: var(--ui-space-3); }
.gallery-details { display: grid; grid-template-columns: max-content 1fr; gap: var(--ui-space-2) var(--ui-space-3); margin: 0; font-size: var(--ui-text-sm); }
.gallery-details dt, .gallery-progress span { color: var(--ui-ink-muted); }
.gallery-details dd { margin: 0; }
.license-note { margin-top: var(--ui-space-3); color: var(--ui-ink-muted); font-size: var(--ui-text-sm); }
.gallery-progress { display: flex; justify-content: space-between; align-items: baseline; gap: var(--ui-space-2); margin-bottom: 8px; font-size: var(--ui-text-sm); }
.gallery-progress strong { font-size: 28px; }
.gallery-fields { margin-top: var(--ui-space-5); }
.gallery-feedback { margin-top: var(--ui-space-3); font-size: var(--ui-text-sm); color: var(--ui-accent); }
.state-preview { margin-top: var(--ui-space-3); border: 1px solid var(--ui-border); border-radius: var(--ui-radius-md); background: var(--ui-surface-muted); }
@media (max-width: 950px) { .swatches { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 620px) {
  .gallery-example-grid, .gallery-fields { grid-template-columns: minmax(0, 1fr); }
  .gallery-metrics { gap: 7px; }
}
@media (max-width: 420px) { .swatches { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
