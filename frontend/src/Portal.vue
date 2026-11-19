<script setup>
import { ref } from 'vue'
import PortalArtwork from './PortalArtwork.vue'
import PortalHeader from './PortalHeader.vue'
import { UiIcon } from './ui'
import { entrances } from './entrances.js'
import { portalMenus } from './portal-navigation.js'
import './portal.css'

const phase = ref(0)
const phases = [
  { title:'需求与询价', prompt:'把客户需求交给合适的厂商', icon:'request', note:'客户提出采购需求，代理商确认使用人数、软件模块和实施范围，再发起询价。三方从同一项目继续办理。', rows:[['客户 → 代理商','业务需求与补充资料'],['代理商 → 厂商','经核对的询价摘要']], role:'customer' },
  { title:'方案与报价', prompt:'让每一版方案都有据可查', icon:'quote', note:'厂商提供渠道采购条件。代理商核对版本与交付周期，组合实施服务，向客户提供独立的采购方案。', rows:[['渠道采购条件','厂商与代理商之间可见'],['客户采购方案','客户与代理商之间可见']], role:'dealer' },
  { title:'交付与验收', prompt:'从确认订单，到完成项目', icon:'delivery', note:'客户确认方案后，代理商下单。厂商登记授权交付，代理商推进实施，客户确认验收或提出整改事项。', rows:[['供货与实施','厂商交接、代理商实施'],['验收与整改','客户确认结果，继续追踪']], role:'vendor' },
]
</script>

<template>
  <div id="top" class="pl-portal">
    <PortalHeader />
    <main>
      <section class="pl-hero" aria-labelledby="portal-title"><div class="pl-wrap pl-intro">
        <div class="pl-hero-copy"><p class="pl-kicker">面向软件渠道的协作平台</p>
          <h1 id="portal-title">每一次客户需求，<br>都有下一步。</h1>
          <p class="pl-subtitle">从客户采购、厂商询价到项目交付，<br class="pl-desktop-break">让代理商的业务与团队经验，在一处连接。</p>
          <div class="pl-hero-actions"><a class="pl-primary" href="#journey">了解协作流程</a><a class="pl-secondary" href="/experience/dealer">体验代理商演示</a></div>
        </div>
        <div class="pl-artwork" aria-hidden="true"><PortalArtwork /></div>
      </div></section>
      <section id="journey" class="pl-flow-section" tabindex="-1"><div class="pl-wrap pl-journey">
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
    </main>
    <footer class="pl-footer">
      <div class="pl-wrap">
        <div class="pl-footer-grid">
          <section id="about" class="pl-footer-about" tabindex="-1" aria-labelledby="portal-about-title">
            <a class="pl-logo" href="#top" aria-label="返回渠智链首页顶部">
              <img class="brand-mark" src="/brand/partnerledger-logo.png" width="38" height="38" alt="" />
              <b>渠智链</b><small>PartnerLedger</small>
            </a>
            <h2 id="portal-about-title">了解我们</h2>
            <p>渠智链连接企业客户、代理商和软件厂商，在同一项目中协作采购、询价与交付。</p>
          </section>
          <nav v-for="menu in portalMenus" :key="menu.id" :aria-labelledby="'portal-footer-' + menu.id">
            <h2 :id="'portal-footer-' + menu.id">{{ menu.label }}</h2>
            <ul><li v-for="link in menu.links" :key="link.href"><a :href="link.href">{{ link.label }}</a></li></ul>
          </nav>
          <section id="help" tabindex="-1" aria-labelledby="portal-help-title">
            <h2 id="portal-help-title">帮助</h2>
            <p>已有账号可在顶部选择登录身份。</p>
            <ul><li><a href="/experience/dealer">免登录体验</a></li><li><a href="/ui">界面组件</a></li></ul>
          </section>
        </div>
        <div class="pl-footer-bottom"><span>© {{ new Date().getFullYear() }} 渠智链</span><a href="#top">返回顶部 <span aria-hidden="true">↑</span></a></div>
      </div>
    </footer>
  </div>
</template>
