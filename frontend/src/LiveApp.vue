<script setup>
import { computed, onMounted, ref } from 'vue'
import { requestApi } from './api.js'
import WorkspaceShell from './WorkspaceShell.vue'
import ProductCatalog from './live/ProductCatalog.vue'
import RequirementParser from './live/RequirementParser.vue'
import LiveProjects from './live/LiveProjects.vue'
import { entrances, organizationRole } from './live/entrances.js'
import { UiBadge, UiButton, UiCard, UiField, UiIcon, UiState } from './ui/index.js'

const props = defineProps({ role: { type: String, required: true, validator: (value) => Object.hasOwn(entrances, value) } })
const entry = ref(props.role)
const portal = computed(() => entrances[entry.value])
const sessionKey = 'partnerledger-api-session'

function restoreSession() {
  try {
    const saved = JSON.parse(sessionStorage.getItem(sessionKey))
    if (typeof saved?.token === 'string' && saved.expiresAt > Date.now()) return saved
    sessionStorage.removeItem(sessionKey)
  } catch { /* A blocked or unreadable store does not prevent sign-in. */ }
  return null
}

const session = ref(restoreSession())
const user = ref(null)
const organization = ref(null)
const username = ref('')
const password = ref('')
const signingIn = ref(false)
const signingOut = ref(false)
const restoringIdentity = ref(false)
const loginError = ref('')
const identityError = ref('')
const logoutError = ref('')
const view = ref(new URLSearchParams(location.search).has('project') ? 'projects' : 'overview')
const isManufacturer = computed(() => organization.value?.kind === 'MANUFACTURER')
const isDealer = computed(() => organization.value?.kind === 'RESELLER')
const kindLabel = computed(() => portal.value.label)
const navigation = computed(() => [
  { id: 'overview', label: '工作总览', icon: 'overview' },
  { id: 'projects', label: entry.value === 'customer' ? '我的采购项目' : isManufacturer.value ? '渠道询价' : '客户项目', icon: 'request' },
  ...(entry.value !== 'customer' ? [{ id: 'catalog', label: '产品目录', icon: 'catalog' }] : []),
  ...(isDealer.value ? [{ id: 'requirement', label: 'AI 需求解析', icon: 'request' }] : []),
  { id: 'account', label: '账号与组织', icon: portal.value.icon },
])

function setSession(value) {
  session.value = value
  try {
    if (value) sessionStorage.setItem(sessionKey, JSON.stringify(value))
    else sessionStorage.removeItem(sessionKey)
  } catch { /* Keep the active tab usable without browser storage. */ }
}

function clearSession(message = '') {
  setSession(null)
  user.value = null
  organization.value = null
  view.value = 'overview'
  document.title = portal.value.label + '登录 · 渠智链'
  identityError.value = ''
  logoutError.value = ''
  loginError.value = message
}

async function authenticated(path, options) {
  const active = session.value
  if (!active || active.expiresAt <= Date.now()) {
    const message = '登录已失效，请重新登录。'
    clearSession(message)
    throw new Error(message)
  }
  try {
    const data = await requestApi(path, { ...options, token: active.token })
    if (session.value !== active) throw new DOMException('Session changed', 'AbortError')
    return data
  } catch (error) {
    if (error.status === 401 && session.value === active) clearSession(error.message)
    throw error
  }
}

async function loadIdentity() {
  if (restoringIdentity.value || !session.value) return
  restoringIdentity.value = true
  identityError.value = ''
  const active = session.value
  try {
    const [profile, currentOrg] = await Promise.all([
      authenticated('/api/user/profile'), authenticated('/api/org/current'),
    ])
    if (session.value !== active) return
    const accountRole = organizationRole(profile, currentOrg)
    if (!accountRole) throw new Error('账号与组织信息不一致或组织类型暂不支持。')
    if (entry.value !== accountRole) {
      await authenticated('/api/auth/logout', { method: 'POST' })
      clearSession('此账号属于' + entrances[accountRole].label + '组织，请从对应入口登录。')
      return
    }
    document.title = entrances[accountRole].label + '工作台 · 渠智链'
    user.value = profile
    organization.value = currentOrg
  } catch (error) {
    if (session.value === active) identityError.value = error.message
  } finally {
    restoringIdentity.value = false
  }
}

onMounted(loadIdentity)

async function signIn() {
  if (signingIn.value) return
  loginError.value = ''
  if (!username.value.trim() || !password.value) {
    loginError.value = '请输入用户名和密码。'
    return
  }
  signingIn.value = true
  try {
    const credentials = await requestApi('/api/auth/login', {
      method: 'POST', body: { username: username.value.trim(), password: password.value },
    })
    if (!credentials?.access_token || credentials.token_type !== 'Bearer' ||
        !Number.isFinite(credentials.expires_in) || credentials.expires_in <= 0) {
      throw new Error('后端未返回可用的登录会话。')
    }
    setSession({ token: credentials.access_token, expiresAt: Date.now() + credentials.expires_in * 1000 })
    await loadIdentity()
  } catch (error) {
    loginError.value = error.message
  } finally {
    password.value = ''
    signingIn.value = false
  }
}

async function signOut() {
  if (signingOut.value) return
  signingOut.value = true
  logoutError.value = ''
  try {
    await authenticated('/api/auth/logout', { method: 'POST' })
    clearSession()
  } catch (error) {
    if (session.value) logoutError.value = error.message
  } finally {
    signingOut.value = false
  }
}
</script>

<template>
  <main v-if="!session" class="access-page">
    <header class="access-header">
      <a class="access-brand" href="/"><span class="brand-mark"><UiIcon name="workflow" :size="20" /></span>渠智链 AI</a>
      <a href="/" class="access-back"><UiIcon name="back" :size="15" /> 返回平台首页</a>
    </header>
    <div class="access-grid">
      <section class="access-story">
        <p class="access-kicker">PartnerLedger / {{ portal.english }}</p>
        <h1>{{ portal.label }}登录</h1>
        <p class="access-intro">{{ portal.description }}</p>
        <ol class="access-steps">
          <li v-for="(step, index) in portal.steps" :key="step"><span>0{{ index + 1 }}</span>{{ step }}</li>
        </ol>
      </section>
      <UiCard class="access-panel" title="使用组织账号登录" description="请输入管理员为你开通的账号。">
        <template #actions><UiIcon :name="portal.icon" :size="22" /></template>
        <form class="form-grid" @submit.prevent="signIn">
          <UiField label="用户名" required v-slot="{ control }">
            <input v-bind="control" v-model="username" autocomplete="username" maxlength="64" required />
          </UiField>
          <UiField label="密码" required v-slot="{ control }">
            <input v-bind="control" v-model="password" type="password" autocomplete="current-password" required />
          </UiField>
          <p v-if="loginError" class="form-error" role="alert">{{ loginError }}</p>
          <UiButton type="submit" variant="primary" end-icon="forward" :loading="signingIn">登录</UiButton>
        </form>
        <template #footer>
          <div class="access-demo"><span>先了解业务流程？</span><a :href="'/experience/' + entry">体验{{ portal.label }}演示 <UiIcon name="open-detail" :size="14" /></a></div>
        </template>
      </UiCard>
    </div>
  </main>

  <main v-else-if="!user || !organization" class="login-screen live-login">
    <UiCard>
      <UiState v-if="identityError" state="error" title="暂时无法加载账号信息" :description="identityError">
        <UiButton @click="loadIdentity">重试</UiButton>
        <UiButton variant="text" :loading="signingOut" @click="signOut">退出登录</UiButton>
      </UiState>
      <UiState v-else state="loading" title="正在读取账号与组织" />
      <p v-if="logoutError" class="form-error" role="alert">{{ logoutError }}</p>
    </UiCard>
  </main>

  <WorkspaceShell v-else :nav="navigation" :view="view" :organization="organization.name"
    :account-label="user.username" environment="正式工作台" @navigate="view = $event" @account="view = 'account'">
    <template #actions><UiButton compact variant="text" :icon="portal.icon" @click="view = 'account'">{{ kindLabel }}</UiButton></template>
    <template v-if="view === 'overview'">
      <section class="page-heading"><div><p class="access-kicker">{{ organization.name }}</p><h1>{{ kindLabel }}工作台</h1>
        <p class="muted">{{ isManufacturer ? '接收渠道询价，回复资料问题并记录能力缺口。' : isDealer ? '接收客户需求、发起厂商询价并跟进双方回复。' : '提交采购需求，向经办代理商补充资料并查看进度。' }}</p></div></section>
      <div class="live-tiles">
        <UiCard v-if="entry !== 'customer'" title="产品目录" :description="isManufacturer ? '新增、编辑与发布本组织的软件产品。' : '查看产品版本、说明与厂商发布信息。'">
          <template #actions><UiIcon name="catalog" :size="20" /></template>
          <UiButton end-icon="forward" @click="view = 'catalog'">打开目录</UiButton>
        </UiCard>
        <UiCard v-if="isDealer" title="需求整理" description="录入客户描述，核对授权数、模块和部署方式。">
          <template #actions><UiIcon name="request" :size="20" /></template>
          <UiButton end-icon="forward" @click="view = 'requirement'">整理需求</UiButton>
        </UiCard>
        <UiCard v-if="isDealer" title="商机拾遗" description="从附加需求找到做过类似项目的伙伴，跟进引荐结果。当前仅开放独立演示。">
          <template #actions><UiBadge>演示</UiBadge></template>
          <a class="access-inline-link" href="/experience/dealer?view=opportunities">查看商机演示</a>
        </UiCard>
        <UiCard title="项目协作" description="在关联项目中查看进度，处理需求、询价、补充资料和能力缺口。">
          <UiButton @click="view = 'projects'">打开项目</UiButton>
        </UiCard>
      </div>
    </template>
    <LiveProjects v-else-if="view === 'projects'" :request="authenticated" :role="entry" :organization-id="organization.id" />
    <ProductCatalog v-else-if="view === 'catalog' && entry !== 'customer'" :request="authenticated" :is-manufacturer="isManufacturer" />
    <RequirementParser v-else-if="view === 'requirement' && isDealer" :request="authenticated" />
    <template v-else>
      <section class="page-heading"><div><h1>账号与组织</h1></div></section>
      <UiCard title="当前账号">
        <template #actions><UiBadge tone="brand">{{ kindLabel }}</UiBadge></template>
        <dl class="details">
          <dt>用户名</dt><dd>{{ user.username }}</dd>
          <dt>所属组织</dt><dd>{{ organization.name }}</dd>
          <dt>组织编号</dt><dd>{{ organization.id }}</dd>
        </dl>
        <p v-if="logoutError" class="form-error live-feedback" role="alert">{{ logoutError }}</p>
        <template #footer><UiButton icon="sign-out" :loading="signingOut" @click="signOut">退出登录</UiButton></template>
      </UiCard>
    </template>
    <footer class="live-links"><a href="/">平台首页</a><a :href="'/experience/' + entry">{{ kindLabel }}演示</a></footer>
  </WorkspaceShell>
</template>

<style scoped src="./live/access.css"></style>
<style scoped>
.live-login { max-width: 446px; }
.live-links { display: flex; flex-wrap: wrap; gap: var(--ui-space-4); margin-top: var(--ui-space-5); font-size: var(--ui-text-sm); }
.live-links a { color: var(--ui-accent); text-underline-offset: 4px; }
.live-feedback { margin-top: var(--ui-space-3); }
</style>
