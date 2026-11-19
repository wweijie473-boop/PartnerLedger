<script setup>
import { ref } from 'vue'
import PortalArtwork from './PortalArtwork.vue'
import { UiIcon } from './ui'
import { entrances } from './entrances.js'
import './portal.css'

const phase = ref(0)
const phases = [
  { title:'需求与询价', prompt:'把客户需求交给合适的厂商', icon:'request', note:'客户提出采购需求，代理商确认使用人数、软件模块和实施范围，再发起询价。三方从同一项目继续办理。', rows:[['客户 → 代理商','业务需求与补充资料'],['代理商 → 厂商','经核对的询价摘要']], role:'customer' },
  { title:'方案与报价', prompt:'让每一版方案都有据可查', icon:'quote', note:'厂商提供渠道采购条件。代理商核对版本与交付周期，组合实施服务，向客户提供独立的采购方案。', rows:[['渠道采购条件','厂商与代理商之间可见'],['客户采购方案','客户与代理商之间可见']], role:'dealer' },
  { title:'交付与验收', prompt:'从确认订单，到完成项目', icon:'delivery', note:'客户确认方案后，代理商下单。厂商登记授权交付，代理商推进实施，客户确认验收或提出整改事项。', rows:[['供货与实施','厂商交接、代理商实施'],['验收与整改','客户确认结果，继续追踪']], role:'vendor' },
]
</script>

<template>
  <div class="pl-portal">
    <header class="pl-header"><div class="pl-wrap pl-bar">
      <a class="pl-logo" href="/" aria-label="渠智链首页"><span><UiIcon name="workflow" :size="24" /></span><b>渠智链</b><small>PartnerLedger</small></a>
      <nav aria-label="主导航"><a href="#access">工作台</a><a href="#journey">三方协作</a><a href="/tables">表格</a><a class="pl-nav-cta" href="/login/dealer">代理商登录</a></nav>
    </div></header>
    <main>
      <section class="pl-hero" aria-labelledby="portal-title"><div class="pl-wrap pl-intro">
        <div class="pl-hero-copy"><p class="pl-kicker">面向软件渠道的协作平台</p>
          <h1 id="portal-title">每一次客户需求，<br>都有下一步。</h1>
          <p class="pl-subtitle">从客户采购、厂商询价到项目交付，<br class="pl-desktop-break">让代理商的业务与团队经验，在一处连接。</p>
          <div class="pl-hero-actions"><a class="pl-primary" href="#access">选择你的工作台</a><a class="pl-secondary" href="/experience/dealer">体验代理商演示</a></div>
        </div>
        <div class="pl-artwork" aria-hidden="true"><PortalArtwork /></div>
      </div></section>
      <section id="access" class="pl-wrap pl-access-section" aria-labelledby="access-title">
        <div class="pl-section-heading"><p class="pl-kicker">各司其职，一起推进</p><h2 id="access-title">进入你的工作空间</h2></div>
        <div class="pl-access"><a v-for="entry in entrances" :key="entry.role" :href="'/login/' + entry.role" class="pl-entry">
          <div class="pl-entry-top"><span class="pl-tile"><UiIcon :name="entry.icon" :size="25" /></span><span>{{ entry.caption }}</span></div>
          <h3>{{ entry.label }}工作台</h3><p>{{ entry.description }}</p><span class="pl-entry-link">{{ entry.label }}登录</span>
        </a></div>
      </section>
      <section id="journey" class="pl-flow-section"><div class="pl-wrap pl-journey">
        <div><p class="pl-kicker">从同一个项目开始</p><h2>三方协作，<br>每个环节都接得上。</h2>
          <p class="pl-section-note">把原始需求、渠道采购条件和客户方案，交给有权查看的人。</p>
          <div class="pl-phases" aria-label="选择协作阶段"><button v-for="(item,index) in phases" :key="item.title" :aria-pressed="phase === index" @click="phase = index">{{ item.title }}</button></div>
        </div>
        <article class="pl-case" aria-live="polite"><p class="pl-case-label"><UiIcon :name="phases[phase].icon" :size="20" /> ERP 采购 · 示例流程</p><h3>{{ phases[phase].prompt }}</h3><p>{{ phases[phase].note }}</p>
          <dl><div v-for="[label,value] in phases[phase].rows" :key="label"><dt>{{ label }}</dt><dd>{{ value }}</dd></div></dl>
          <a :href="'/experience/' + phases[phase].role">体验{{ entrances.find(entry => entry.role === phases[phase].role).label }}演示</a>
        </article>
      </div></section>
      <section class="pl-wrap pl-opportunity">
        <div class="pl-op-illustration" aria-hidden="true"><span class="pl-op-path"></span><div><UiIcon name="request" :size="28" /><span>一条未承接的需求</span></div><div class="pl-op-match"><UiIcon name="workflow" :size="28" /><span>找到有依据的伙伴</span></div></div>
        <div><p class="pl-kicker">商机拾遗</p><h2>客户额外提的需求，<br>团队里谁做过？</h2><p>原厂暂不支持的接口，也许团队认识的人就能承接。保留附加需求，查看合作依据，找到内部引荐人，继续跟进结果。</p><a class="pl-primary" href="/experience/dealer?view=opportunities">体验商机拾遗</a><small>使用虚构的项目与合作记录演示</small></div>
      </section>
      <section class="pl-start"><div class="pl-wrap"><h2>从一笔项目，开始协作。</h2><p>选择工作台，接着处理属于你的下一步。</p><a class="pl-secondary" href="#access">选择登录入口</a></div></section>
    </main>
    <footer class="pl-wrap pl-footer"><b>渠智链 · PartnerLedger</b><span>软件渠道协作平台</span><a href="/ui">界面组件</a></footer>
  </div>
</template>
