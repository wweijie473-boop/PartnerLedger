<script setup>
import { computed, ref } from 'vue'
import PortalArtwork from './PortalArtwork.vue'
import PortalHeader from './PortalHeader.vue'
import { UiIcon } from './ui'
import { entrances } from './entrances.js'
import { portalMenus } from './portal-navigation.js'
import './portal.css'

const phase = ref(0)
const phases = [
  {
    title: '需求与询价', prompt: '先把采购需求说清楚', icon: 'request', role: 'customer',
    note: '一家企业想用 ERP 把仓库和财务的数据接起来。代理商先确认使用人数、需要的模块和部署方式，再把整理好的需求发给厂商询价。',
    rows: [['客户补充资料', '仓储和财务的业务场景'], ['代理商发出询价', '25 个授权、模块和部署要求']],
  },
  {
    title: '方案与报价', prompt: '把软件和服务列进一份方案', icon: 'quote', role: 'dealer',
    note: '厂商给出软件授权价格和交付周期。代理商把这些条件和实施、培训服务一起核算，给客户一份可以逐项确认的采购方案。',
    rows: [['渠道报价', '厂商和代理商查看'], ['采购方案', '代理商和客户查看']],
  },
  {
    title: '交付与验收', prompt: '授权到了，接下来就是实施和验收', icon: 'delivery', role: 'vendor',
    note: '客户确认方案后，代理商下单，厂商登记授权交付。实施和培训结束后，客户按项目记录验收，有问题可以退回整改。',
    rows: [['供货与实施', '登记授权、记录实施结果'], ['验收与整改', '确认验收或写明待处理的问题']],
  },
]
const currentPhase = computed(() => phases[phase.value])
const exampleRole = computed(() => entrances.find(entry => entry.role === currentPhase.value.role))
</script>

<template>
  <div id="top" class="pl-portal">
    <PortalHeader />
    <main>
      <section class="pl-hero" aria-labelledby="portal-title"><div class="pl-wrap pl-intro">
        <div class="pl-hero-copy"><p class="pl-kicker">软件渠道协作平台</p>
          <h1 id="portal-title">软件采购，从谈需求到验收</h1>
          <p class="pl-subtitle">客户提出采购需求，代理商整理方案，厂商确认供货。需求、报价和交付记录都留在同一个项目里，三方随时能查看进度。</p>
          <div class="pl-hero-actions"><a class="pl-primary" href="#journey">看看三方怎么配合</a><a class="pl-secondary" href="/experience/dealer">体验代理商工作台</a></div>
        </div>
        <div class="pl-artwork" aria-hidden="true"><PortalArtwork /></div>
      </div></section>
      <section id="journey" class="pl-flow-section" tabindex="-1"><div class="pl-wrap pl-journey">
        <div><h2>一套 ERP 是怎么采购的？</h2>
          <p class="pl-section-note">以一家企业的 ERP 采购为例，看看需求怎么交给厂商、采购方案怎么确认，以及上线后由谁验收。</p>
          <div class="pl-phases" aria-label="选择协作阶段"><button v-for="(item,index) in phases" :key="item.title" :aria-pressed="phase === index" @click="phase = index">{{ item.title }}</button></div>
        </div>
        <article class="pl-case" aria-live="polite">
          <p class="pl-case-label"><UiIcon :name="currentPhase.icon" :size="20" />{{ currentPhase.title }}</p>
          <h3>{{ currentPhase.prompt }}</h3>
          <p>{{ currentPhase.note }}</p>
          <dl><div v-for="[label,value] in currentPhase.rows" :key="label"><dt>{{ label }}</dt><dd>{{ value }}</dd></div></dl>
          <a :href="'/experience/' + exampleRole.role">体验{{ exampleRole.label }}工作台</a>
        </article>
      </div></section>
      <section class="pl-wrap pl-role-section" aria-labelledby="portal-roles-title">
        <h2 id="portal-roles-title">三个工作台，各自从哪里开始？</h2>
        <p>可以直接打开演示，用示例项目看看每种身份的日常工作。</p>
        <ul class="pl-role-grid">
          <li v-for="entry in entrances" :key="entry.role">
            <span class="pl-role-icon"><UiIcon :name="entry.icon" :size="21" /></span>
            <h3>{{ entry.label }}</h3>
            <p>{{ entry.description }}</p>
            <a :href="'/experience/' + entry.role">体验{{ entry.label }}工作台 <UiIcon name="forward" :size="14" /></a>
          </li>
        </ul>
      </section>
      <section class="pl-wrap pl-opportunity" aria-labelledby="portal-opportunity-title">
        <div class="pl-op-illustration" aria-hidden="true">
          <span class="pl-op-path"></span>
          <div><UiIcon name="request" :size="28" /><span>客户还想接入仓储系统</span></div>
          <div class="pl-op-match"><UiIcon name="workflow" :size="28" /><span>同事合作过类似的项目</span></div>
        </div>
        <div>
          <p class="pl-kicker">商机拾遗</p>
          <h2 id="portal-opportunity-title">客户多问了一句，也值得跟进</h2>
          <p>比如客户买 ERP 时，还想接入现有的仓储系统，原厂暂时做不了。把需求记进商机拾遗，就能翻看团队以往的合作记录，找做过类似项目的同事帮忙引荐，再记录后续进展。</p>
          <a class="pl-primary" href="/experience/dealer?view=opportunities">体验商机拾遗</a>
          <small>演示使用虚构项目和合作记录</small>
        </div>
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
            <p>渠智链为软件采购和渠道合作提供工作台。询过什么价、客户确认了哪版方案、项目走到哪一步，都可以回到项目里查。</p>
          </section>
          <nav v-for="menu in portalMenus" :key="menu.id" :aria-labelledby="'portal-footer-' + menu.id">
            <h2 :id="'portal-footer-' + menu.id">{{ menu.label }}</h2>
            <ul><li v-for="link in menu.links" :key="link.href"><a :href="link.href">{{ link.label }}</a></li></ul>
          </nav>
          <section id="help" tabindex="-1" aria-labelledby="portal-help-title">
            <h2 id="portal-help-title">帮助</h2>
            <p>有账号就登录，在页面上选身份。需要为新组织开户，可以从注册开始。</p>
            <ul><li><a href="/login">登录</a> · <a href="/register">注册</a></li><li><a href="/experience/dealer">免登录体验</a></li><li><a href="/ui">界面组件</a></li></ul>
          </section>
        </div>
        <div class="pl-footer-bottom"><span>© {{ new Date().getFullYear() }} 渠智链</span><a href="#top">返回顶部 <span aria-hidden="true">↑</span></a></div>
      </div>
    </footer>
  </div>
</template>
