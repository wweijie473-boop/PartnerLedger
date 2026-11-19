<script setup>
import { ref } from 'vue'
import { UiIcon } from './ui'
import { entrances } from './entrances.js'
import './portal.css'

const phase = ref(0)
const phases = [
  { title: '澄清需求', prompt: '企业客户提交采购需求', icon: 'request', note: '采购团队提出财务和库存一体化需求。代理商核对使用人数、模块和实施范围，整理成发给厂商的询价。', rows: [['需求范围', '财务 · 库存管理'], ['下一步', '代理商确认需求并询价']], role: 'customer' },
  { title: '询价与报价', prompt: '代理商编制客户方案', icon: 'quote', note: '厂商提供软件授权和交付周期。代理商比较渠道报价版本，加入培训与实施服务，向客户发出独立报价。', rows: [['厂商 → 代理商', '软件授权、供货周期'], ['代理商 → 客户', '采购明细、实施费用']], role: 'dealer' },
  { title: '交付与验收', prompt: '三方核对交付结果', icon: 'delivery', note: '客户确认报价后，代理商下单；厂商登记授权交付，代理商安排实施。客户按项目记录验收，或说明需要整改的事项。', rows: [['厂商与代理商', '授权交付、实施记录'], ['企业客户', '验收确认或退回整改']], role: 'vendor' },
]
</script>

<template>
  <div class="pl-portal">
    <header class="pl-header"><div class="pl-wrap pl-bar">
      <a class="pl-logo" href="/" aria-label="渠智链首页"><span><UiIcon name="workflow" :size="23"/></span><b>渠智链</b><i></i><small>PartnerLedger</small></a>
      <nav aria-label="主导航"><a href="#access">工作台入口</a><a href="/tables">表格</a><a href="#journey">协作流程</a></nav>
    </div></header>
    <main class="pl-wrap">
      <section class="pl-intro" aria-labelledby="portal-title">
        <div class="pl-hero-copy"><p class="pl-kicker">软件渠道协作平台</p>
          <h1 id="portal-title">让软件渠道协作，<br>回到同一个项目。</h1>
          <p class="pl-subtitle">从客户需求、厂家询价到项目交付，让软件代理商在一处跟进每笔业务。</p>
        </div>
        <div class="pl-visual" aria-hidden="true">
          <svg viewBox="0 0 340 190" fill="none" focusable="false">
            <defs>
              <pattern id="portal-grid" width="12" height="12" patternUnits="userSpaceOnUse">
                <circle cx="1" cy="1" r=".8" fill="#a5b8ca"/>
              </pattern>
              <radialGradient id="portal-grid-fade">
                <stop offset="35%" stop-color="white"/>
                <stop offset="100%" stop-color="white" stop-opacity="0"/>
              </radialGradient>
              <mask id="portal-grid-mask">
                <rect width="340" height="190" fill="url(#portal-grid-fade)"/>
              </mask>
              <radialGradient id="portal-layer-shadow">
                <stop stop-color="#172f49" stop-opacity=".22"/>
                <stop offset="100%" stop-color="#172f49" stop-opacity="0"/>
              </radialGradient>
            </defs>
            <rect width="340" height="190" fill="url(#portal-grid)" mask="url(#portal-grid-mask)"/>
            <ellipse cx="170" cy="154" rx="106" ry="25" fill="url(#portal-layer-shadow)"/>
            <g v-for="signal in [
              { route: 'M28 51h47l43 21', x: 28, y: 51, delay: '.2s' },
              { route: 'M35 143h42l40-20', x: 35, y: 143, delay: '.8s' },
              { route: 'M311 79h-28l-43 24', x: 311, y: 79, delay: '1.4s' },
            ]" :key="signal.route" :style="{ '--signal-delay': signal.delay }">
              <path :d="signal.route" stroke="#a4b6c8" stroke-width="1.25" stroke-linejoin="round"/>
              <path :d="signal.route" pathLength="1" class="pl-visual-signal"/>
              <circle :cx="signal.x" :cy="signal.y" r="6" fill="#f1f7fc" stroke="#96b3ce"/>
              <circle :cx="signal.x" :cy="signal.y" r="2" fill="#2378bd"/>
            </g>
            <g stroke-linejoin="round" stroke-width="1.6">
              <g class="pl-visual-layer" style="--layer-delay: .82s">
                <path d="M170 58 272 103 170 153 68 107Z" fill="#f7fbff" stroke="#94a9bc"/>
                <path d="m68 107 0 7 102 50 102-53v-8l-102 50Z" fill="#d3e0ec" stroke="#94a9bc"/>
              </g>
              <g class="pl-visual-layer" style="--layer-delay: 1.15s">
                <path d="m170 40 102 45-102 50L68 89Z" fill="#deebf5" stroke="#809bb1"/>
                <path d="m68 89 0 7 102 50 102-53v-8l-102 50Z" fill="#a8bfd2" stroke="#809bb1"/>
              </g>
              <g class="pl-visual-layer" style="--layer-delay: 1.5s">
                <path d="m170 19 102 45-102 50L68 68Z" fill="#172b42" stroke="#172b42"/>
                <path d="m68 68 0 7 102 50 102-53v-8l-102 50Z" fill="#344f6b" stroke="#172b42"/>
                <path d="m125 68 44-21 44 19-43 21Z" fill="#25435f" stroke="#85a9c7"/>
                <path d="m147 58 43 19m-42 0 44-21" stroke="#85a9c7"/>
                <circle class="pl-visual-beacon" cx="169" cy="67" r="3" fill="#9bd6ff"/>
              </g>
            </g>
          </svg>
        </div>
      </section>
      <section id="access" aria-label="选择工作台" class="pl-access">
        <a v-for="entry in entrances" :key="entry.role" :href="'/login/' + entry.role" class="pl-entry">
          <div class="pl-entry-top"><span class="pl-tile"><UiIcon :name="entry.icon" :size="23"/></span><span>{{ entry.caption }}</span></div>
          <h2>{{ entry.label }}登录</h2><p>{{ entry.description }}</p>
          <span class="pl-entry-link">进入工作台</span>
        </a>
      </section>
      <aside class="pl-opportunity"><div><p class="pl-kicker">代理商工作台 · 商机拾遗</p><h2>原厂做不了的接口，团队里谁做过？</h2><p>把走访时听到的附加需求单独记下来，用过往案例找到引荐人。主单照常交付，额外的机会也有人跟进。</p></div><a href="/experience/dealer?view=opportunities">体验商机拾遗演示</a></aside>
      <section id="journey" class="pl-journey">
        <div><p class="pl-kicker">协作流程 · 示例项目</p><h2>一次 ERP 采购，<br>三方如何配合。</h2>
          <div class="pl-phases" aria-label="选择示例阶段"><button v-for="(item, index) in phases" :key="item.title" :aria-pressed="phase === index" @click="phase=index">{{ item.title }}</button></div>
        </div>
        <article class="pl-case" aria-live="polite"><p class="pl-kicker"><UiIcon :name="phases[phase].icon" :size="16"/> 企业 ERP 采购</p><h3>{{ phases[phase].prompt }}</h3><p>{{ phases[phase].note }}</p>
          <dl><div v-for="[label,value] in phases[phase].rows" :key="label"><dt>{{ label }}</dt><dd>{{ value }}</dd></div></dl>
          <a :href="'/experience/' + phases[phase].role">体验{{ entrances.find(entry => entry.role === phases[phase].role).label }}演示</a>
        </article>
      </section>
    </main>
    <footer class="pl-wrap pl-footer"><b>PartnerLedger</b><span>渠智链 · 软件渠道协作平台</span></footer>
  </div>
</template>
