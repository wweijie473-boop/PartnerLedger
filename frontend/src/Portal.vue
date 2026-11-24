<script setup>
import { computed, ref } from 'vue'
import PortalArtwork from './PortalArtwork.vue'
import PortalHeader from './PortalHeader.vue'
import { UiIcon } from './ui'
import { entrances } from './entrances.js'
import { portalMenus } from './portal-navigation.js'
import { accountPath } from './auth-session.js'
import { useSessionAccount } from './useSessionAccount.js'
import { retailDemoEnabled, retailDemoLocation } from './live/retailDemo.js'
import { entrance as vEnter } from './ui/entrance.js'
import './portal.css'

const phase = ref(0)
const hero = ref(null)
const changedPhase = ref(false)
const { account, signedIn } = useSessionAccount()
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

function showPhase(index) {
  if (phase.value === index) return
  changedPhase.value = true
  phase.value = index
}
</script>

<template>
  <div id="top" class="pl-portal">
    <PortalHeader home-page :account="account" :signed-in="signedIn" />
    <main>
      <section class="pl-hero" aria-labelledby="portal-title">
        <div ref="hero" v-enter class="pl-wrap pl-intro">
          <div class="pl-hero-copy">
            <p class="pl-kicker">软件渠道协作平台</p>
            <h1 id="portal-title">软件采购，<br />从谈需求到验收</h1>
            <p class="pl-subtitle">客户提需求，代理商整理方案，厂商确认供货。三方在同一个项目里查看需求、报价和交付进度。</p>
            <div class="pl-hero-actions">
              <a class="pl-primary" href="/experience/dealer">体验代理商工作台 <UiIcon name="forward" :size="15" /></a>
              <a v-if="retailDemoEnabled" class="pl-secondary" :href="retailDemoLocation('dealer', 'promotion')">体验厂商直购与佣金</a>
              <a class="pl-secondary" href="#journey">了解三方协作</a>
            </div>
          </div>
          <div class="pl-artwork"><PortalArtwork /></div>
        </div>
      </section>
      <nav class="pl-section-bar" aria-label="首页栏目">
        <div class="pl-wrap pl-section-links">
          <a v-enter:link="0" href="#journey"><span>三方协作</span></a>
          <a v-enter:link="1" href="#roles"><span>角色工作台</span></a>
          <a v-enter:link="2" href="#opportunity"><span>商机拾遗</span></a>
        </div>
      </nav>
      <section id="journey" v-enter:after="() => hero" class="pl-flow-section" tabindex="-1"><div class="pl-wrap pl-journey">
        <div><h2>一套 ERP 是怎么采购的？</h2>
          <p class="pl-section-note">以一家企业的 ERP 采购为例，看看需求怎么交给厂商、采购方案怎么确认，以及上线后由谁验收。</p>
          <div class="pl-phases" aria-label="选择协作阶段"><button v-for="(item,index) in phases" :key="item.title" :aria-pressed="phase === index" @click="showPhase(index)">{{ item.title }}</button></div>
        </div>
        <article class="pl-case" aria-live="polite">
          <div :key="currentPhase.title" class="pl-case-content" :class="{ 'pl-case-change': changedPhase }">
            <p class="pl-case-label"><UiIcon :name="currentPhase.icon" :size="20" />{{ currentPhase.title }}</p>
            <h3>{{ currentPhase.prompt }}</h3>
            <p>{{ currentPhase.note }}</p>
            <dl><div v-for="[label,value] in currentPhase.rows" :key="label"><dt>{{ label }}</dt><dd>{{ value }}</dd></div></dl>
            <a :href="'/experience/' + exampleRole.role">体验{{ exampleRole.label }}工作台</a>
          </div>
        </article>
      </div></section>
      <section id="roles" class="pl-wrap pl-role-section" tabindex="-1" aria-labelledby="portal-roles-title">
        <header v-enter>
          <h2 id="portal-roles-title">三个工作台，各自从哪里开始？</h2>
          <p>可以直接打开演示，用示例项目看看每种身份的日常工作。</p>
        </header>
        <ul v-enter:children class="pl-role-grid">
          <li v-for="entry in entrances" :key="entry.role">
            <span class="pl-role-icon"><UiIcon :name="entry.icon" :size="21" /></span>
            <h3>{{ entry.label }}</h3>
            <p>{{ entry.description }}</p>
            <a :href="'/experience/' + entry.role">体验{{ entry.label }}工作台 <UiIcon name="forward" :size="14" /></a>
          </li>
        </ul>
      </section>
      <section id="opportunity" class="pl-wrap pl-opportunity" tabindex="-1" aria-labelledby="portal-opportunity-title">
        <div v-enter class="pl-op-illustration" aria-hidden="true">
          <span class="pl-op-path"></span>
          <div><UiIcon name="request" :size="28" /><span>客户还想接入仓储系统</span></div>
          <div class="pl-op-match"><UiIcon name="workflow" :size="28" /><span>同事合作过类似的项目</span></div>
        </div>
        <div v-enter="1">
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
        <div v-enter:children class="pl-footer-grid">
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
            <p v-if="signedIn">在账号与组织里查看当前账号信息。</p>
            <p v-else>有账号就登录，在页面上选身份。需要为新组织开户，可以从注册开始。</p>
            <ul>
              <li v-if="signedIn"><a :href="accountPath">账号与组织</a></li>
              <li v-else><a href="/login">登录</a> · <a href="/register">注册</a></li>
              <li><a href="/experience/dealer">免登录体验</a></li>
              <li><a href="/ui">界面组件</a></li>
            </ul>
          </section>
        </div>
        <div v-enter class="pl-footer-bottom"><span>© {{ new Date().getFullYear() }} 渠智链</span><a href="#top">返回顶部 <span aria-hidden="true">↑</span></a></div>
      </div>
    </footer>
  </div>
</template>
