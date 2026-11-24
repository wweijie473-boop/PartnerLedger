<script setup>
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref } from 'vue'
import { requestApi } from './api.js'
import { currentSession, saveSession, sessionFromCredentials } from './auth-session.js'
import WorkspaceShell from './WorkspaceShell.vue'
import WorkspaceShortcuts from './WorkspaceShortcuts.vue'
import DealerPage from './DealerPage.vue'
import { workspacePages, workspaceShortcuts } from './workspaceNavigation.js'
import ProductCatalog from './live/ProductCatalog.vue'
import RequirementParser from './live/RequirementParser.vue'
import LiveProjects from './live/LiveProjects.vue'
import LiveVisits from './live/LiveVisits.vue'
import LiveOpportunities from './live/LiveOpportunities.vue'
import { entrances, organizationKinds, organizationRole } from './live/entrances.js'
import { UiButton, UiCard, UiField, UiIcon, UiState } from './ui/index.js'

const props = defineProps({
  role: { type: String, default: '', validator: (value) => value === '' || Object.hasOwn(entrances, value) },
  mode: { type: String, default: 'login' },
})
const entry = ref(props.role)
const mode = ref(props.mode)
const portal = computed(() => entrances[entry.value])
const isRegister = computed(() => mode.value === 'register')
const accessTitle = computed(() => (portal.value?.label || '') + (isRegister.value ? '注册' : '登录'))
const session = ref(currentSession())
const user = ref(null)
const organization = ref(null)
const username = ref('')
const password = ref('')
const organizationName = ref('')
const confirmPassword = ref('')
const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
let typingAnimation
let requestedRole
const signingIn = ref(false)
const signingOut = ref(false)
const restoringIdentity = ref(false)
const loginError = ref('')
const accessNotice = ref('')
const identityError = ref('')
const logoutError = ref('')
const entryParams = new URLSearchParams(location.search)
const view = ref(entryParams.get('view') || (entryParams.has('project') ? 'projects' : 'overview'))
const captureProject = ref(entryParams.get('captureProject') || '')
const navigationRevision = ref(0)
const isManufacturer = computed(() => organization.value?.kind === 'MANUFACTURER')
const isDealer = computed(() => organization.value?.kind === 'RESELLER')
const kindLabel = computed(() => portal.value?.label || '')
const RetailDesk = defineAsyncComponent(() => import('./live/RetailDesk.vue'))
const navigation = computed(() => workspacePages(entry.value, { directView: entryParams.get('view') }))
const currentPage = computed(() => navigation.value.find(page => page.id === view.value))
const shortcuts = computed(() => workspaceShortcuts(navigation.value, entry.value))

function accessPath(nextMode, role = entry.value) {
  return '/' + nextMode + (role ? '/' + role : '')
}

function customerRetailIntent() {
  const params = new URLSearchParams(location.search)
  if (params.get('view') === 'checkout' && params.has('code')) return 'checkout'
  if (params.get('view') === 'retail-orders' && params.has('order_id')) return 'retail-orders'
  return ''
}

function identityLink(nextMode) {
  const path = accessPath(nextMode)
  return entry.value === 'customer' && customerRetailIntent() ? path + location.search : path
}

function selectRole(role) {
  entry.value = role
  loginError.value = ''
  accessNotice.value = ''
  const url = new URL(location.href)
  url.pathname = accessPath(mode.value)
  history.replaceState(null, '', url)
  document.title = accessTitle.value + ' · 渠智链'
}

function animateTyping(event) {
  if (event.isComposing || reducedMotion.matches || !event.target.matches('.ui-field input')) return
  typingAnimation?.cancel()
  typingAnimation = event.target.animate([
    { backgroundColor: '#edf6ff' },
    { backgroundColor: '#ffffff' },
  ], { duration: 200, easing: 'ease-out' })
}

function setSession(value) {
  session.value = value
  saveSession(value)
}

function clearSession(message = '') {
  setSession(null)
  requestedRole = undefined
  user.value = null
  organization.value = null
  view.value = 'overview'
  document.title = accessTitle.value + ' · 渠智链'
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
    if (requestedRole && requestedRole !== accountRole) {
      await authenticated('/api/auth/logout', { method: 'POST' })
      clearSession('此账号属于' + entrances[accountRole].label + '组织，请从对应入口登录。')
      return
    }
    requestedRole = undefined
    if (entry.value !== accountRole || isRegister.value) {
      entry.value = accountRole
      mode.value = 'login'
      const url = new URL(location.href)
      url.pathname = accessPath('login')
      history.replaceState(null, '', url)
    }
    document.title = entrances[accountRole].label + '工作台 · 渠智链'
    user.value = profile
    organization.value = currentOrg
    const retailIntent = accountRole === 'customer' && customerRetailIntent()
    if (retailIntent) view.value = retailIntent
    if (!navigation.value.some(item => item.id === view.value)) view.value = 'overview'
  } catch (error) {
    if (session.value === active) identityError.value = error.message
  } finally {
    restoringIdentity.value = false
  }
}

onMounted(() => {
  loadIdentity()
  window.addEventListener('popstate', restoreWorkspaceView)
})
onUnmounted(() => window.removeEventListener('popstate', restoreWorkspaceView))

async function signIn() {
  if (signingIn.value) return
  loginError.value = ''
  accessNotice.value = ''
  if (!entry.value || !username.value.trim() || !password.value) {
    loginError.value = '请选择身份并输入用户名和密码。'
    return
  }
  signingIn.value = true
  try {
    const credentials = await requestApi('/api/auth/login', {
      method: 'POST', body: { username: username.value.trim(), password: password.value },
    })
    const active = sessionFromCredentials(credentials)
    requestedRole = entry.value
    setSession(active)
    await loadIdentity()
  } catch (error) {
    loginError.value = error.message
  } finally {
    password.value = ''
    signingIn.value = false
  }
}

async function signUp() {
  if (signingIn.value) return
  loginError.value = ''
  const size = new TextEncoder().encode(password.value).length
  if (!entry.value || !organizationName.value.trim() || !/^[a-z0-9][a-z0-9_.-]{2,63}$/i.test(username.value.trim())) {
    loginError.value = '请选择身份、填写组织名称，并使用 3–64 位字母、数字、点、下划线或连字符作为用户名。'
    return
  }
  if (size < 12 || size > 72) {
    loginError.value = '密码长度须为 12–72 字节。'
    return
  }
  if (password.value !== confirmPassword.value) {
    loginError.value = '两次输入的密码不一致。'
    return
  }
  signingIn.value = true
  try {
    await requestApi('/api/auth/register', {
      method: 'POST',
      body: {
        username: username.value.trim(),
        password: password.value,
        organization_name: organizationName.value.trim(),
        organization_kind: organizationKinds[entry.value],
      },
    })
    password.value = ''
    confirmPassword.value = ''
    mode.value = 'login'
    history.replaceState(null, '', identityLink('login'))
    document.title = accessTitle.value + ' · 渠智链'
    accessNotice.value = '注册成功，请用刚创建的账号登录。'
  } catch (error) {
    loginError.value = error.message
  } finally {
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
function restoreWorkspaceView() {
  const params = new URLSearchParams(location.search)
  const target = params.get('view') || (params.has('project') ? 'projects' : 'overview')
  const next = navigation.value.some(item => item.id === target) ? target : 'overview'
  if (next !== view.value) {
    navigationRevision.value += 1
    view.value = next
    captureProject.value = params.get('captureProject') || ''
  }
}
function navigate(target, params = {}) {
  navigationRevision.value += 1
  view.value = target
  captureProject.value = ''
  const url = new URL(location.href)
  const checkout = target === 'checkout'
    ? { code: entryParams.get('code') ?? '', product: entryParams.get('product') ?? '' }
    : {}
  url.search = new URLSearchParams({ view: target, ...checkout, ...params })
  history.replaceState(null, '', url)
  window.scrollTo(0, 0)
}
function openProject(id, scope = 'all') {
  navigate('projects', { project: id, scope })
}
function openOpportunity(id = '', projectId = '') {
  navigate('opportunities')
  captureProject.value = projectId
  const url = new URL(location.href)
  if (id) url.searchParams.set('op',id)
  if (projectId) url.searchParams.set('captureProject',projectId)
  history.replaceState(null,'',url)
}
</script>

<template>
  <main v-if="!session" class="access-page">
    <header class="access-header">
      <a class="access-brand" href="/">
        <img class="brand-mark" src="/brand/partnerledger-logo.png" width="30" height="30" alt="" />
        渠智链 AI
      </a>
      <a href="/" class="access-back"><UiIcon name="back" :size="14" /> 返回首页</a>
    </header>
    <section class="access-form-wrap" aria-labelledby="access-title">
      <h1 id="access-title">{{ accessTitle }}</h1>
      <UiCard class="access-panel">
        <form class="form-grid" aria-labelledby="access-title" @input="animateTyping" @submit.prevent="isRegister ? signUp() : signIn()">
          <fieldset class="access-roles" :disabled="signingIn">
            <legend>选择身份</legend>
            <div class="access-role-options">
              <label v-for="(identity, role) in entrances" :key="role">
                <input type="radio" name="role" :value="role" :checked="entry === role" required @change="selectRole(role)">
                <span>{{ identity.label }}</span>
              </label>
            </div>
          </fieldset>
          <UiField v-if="isRegister" label="组织名称" hint="注册会创建一个独立组织。" required v-slot="{ control }">
            <input v-bind="control" v-model="organizationName" autocomplete="organization" maxlength="150" required />
          </UiField>
          <UiField label="用户名" required v-slot="{ control }">
            <input v-bind="control" v-model="username" autocomplete="username" maxlength="64" autocapitalize="none" required />
          </UiField>
          <UiField label="密码" :hint="isRegister ? '12–72 字节' : ''" required v-slot="{ control }">
            <input v-bind="control" v-model="password" type="password" :autocomplete="isRegister ? 'new-password' : 'current-password'" maxlength="72" required />
          </UiField>
          <UiField v-if="isRegister" label="确认密码" required v-slot="{ control }">
            <input v-bind="control" v-model="confirmPassword" type="password" autocomplete="new-password" maxlength="72" required />
          </UiField>
          <p v-if="loginError" class="form-error" role="alert">{{ loginError }}</p>
          <p v-if="accessNotice" class="access-notice" role="status">{{ accessNotice }}</p>
          <div class="access-actions"><UiButton type="submit" variant="primary" compact :loading="signingIn">{{ isRegister ? '注册' : '登录' }}</UiButton></div>
        </form>
      </UiCard>
      <div class="access-links">
        <a :href="identityLink(isRegister ? 'login' : 'register')">{{ isRegister ? '已有账号？登录' : '没有账号？注册' }}</a>
        <a :href="'/experience/' + (entry || 'dealer')">体验演示</a>
      </div>
    </section>
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

  <WorkspaceShell v-else :role="entry" :nav="navigation" :view="view" :organization="organization.name"
    :account-label="user.username" environment="正式工作台" @navigate="navigate" @account="navigate('account')">
    <RetailDesk v-if="currentPage?.source === 'retail'" :key="navigationRevision + ':' + view" :mode="view" :title="currentPage.label" :role="entry" :request="authenticated" :identity="user.id" @navigate="navigate" />
    <DealerPage v-else-if="currentPage?.source === 'utility'" :key="view" :view="view" :title="currentPage.label" :request="authenticated" @navigate="navigate" @project="openProject" />
    <template v-else-if="view === 'overview'">
      <LiveProjects :key="navigationRevision" overview :request="authenticated" :role="entry" :organization-id="organization.id" :user-id="user.id"
        @open-project="openProject" @open-list="(scope, create) => navigate('projects', create ? { scope, create: '1' } : { scope })">
        <template #heading>
          <div>
            <p class="workspace-kicker">{{ organization.name }}</p>
            <h1>{{ kindLabel }}工作台</h1>
            <p class="muted">{{ isManufacturer ? '接收渠道询价，回复资料问题并记录能力缺口。' : isDealer ? '接收客户需求、发起厂商询价并跟进双方回复。' : '提交采购需求，向经办代理商补充资料并查看进度。' }}</p>
          </div>
        </template>
      </LiveProjects>
      <WorkspaceShortcuts :pages="shortcuts" @navigate="navigate" />
    </template>
    <LiveProjects v-else-if="view === 'projects'" :key="navigationRevision" :request="authenticated" :role="entry" :organization-id="organization.id" :user-id="user.id" @capture-opportunity="id => openOpportunity('',id)" />
    <LiveVisits v-else-if="view === 'visits' && isDealer" :request="authenticated" :scope="organization.id + ':' + user.id" @project="openProject" @opportunity="id => openOpportunity(id)" />
    <LiveOpportunities v-else-if="view === 'opportunities' && isDealer" :key="navigationRevision" :request="authenticated" :scope="organization.id + ':' + user.id" :project-id="captureProject" :capture="!!captureProject" @project="openProject" />
    <ProductCatalog v-else-if="view === 'catalog' && entry !== 'customer'" :request="authenticated" :is-manufacturer="isManufacturer" />
    <RequirementParser v-else-if="view === 'requirement' && isDealer" :request="authenticated"
      @open-inquiry="id => openProject(id)" @open-projects="navigate('projects')" />
    <template v-else>
      <header class="page-heading"><h1>账号与组织</h1></header>
      <section class="workspace-section live-account" aria-labelledby="current-account">
        <header class="workspace-section-heading">
          <h2 id="current-account">当前账号</h2>
          <UiButton icon="sign-out" variant="text" :loading="signingOut" @click="signOut">退出登录</UiButton>
        </header>
        <dl class="live-account-facts">
          <div><dt>用户名</dt><dd>{{ user.username }}</dd></div>
          <div><dt>账号类型</dt><dd>{{ kindLabel }}</dd></div>
          <div><dt>所属组织</dt><dd>{{ organization.name }}</dd></div>
          <div><dt>组织编号</dt><dd>{{ organization.id }}</dd></div>
        </dl>
        <p v-if="logoutError" class="form-error live-feedback" role="alert">{{ logoutError }}</p>
      </section>
    </template>
    <footer class="live-links"><a href="/">平台首页</a><a :href="'/experience/' + entry">{{ kindLabel }}演示</a></footer>
  </WorkspaceShell>
</template>

<style scoped src="./live/access.css"></style>
<style scoped>
.live-login { max-width: 446px; }
.live-links { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 16px; font-size: 13px; }
.live-links a { text-decoration: none; }
.live-links a:hover { text-decoration: underline; }
.live-account { max-width: 720px; }
.live-account-facts > div {
  display: grid;
  grid-template-columns: minmax(80px, 18ch) minmax(0, 1fr);
  gap: 12px;
  padding: 8px 0;
  border-top: 1px solid var(--ui-border);
}
.live-account-facts dt { color: var(--ui-ink-muted); }
.live-account-facts dd { margin: 0; overflow-wrap: anywhere; }
.live-feedback { margin-block: 8px; }
</style>
