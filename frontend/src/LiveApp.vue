<script setup>
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref } from 'vue'
import { requestApi } from './api.js'
import { currentSession, saveSession, sessionFromCredentials } from './auth-session.js'
import WorkspaceShell from './WorkspaceShell.vue'
import WorkspaceShortcuts from './WorkspaceShortcuts.vue'
import DealerPage from './DealerPage.vue'
import { workspacePages, workspaceShortcuts } from './workspaceNavigation.js'
import ProductCatalog from './live/ProductCatalog.vue'
import AiWorkspace from './live/AiWorkspace.vue'
import RoleAiWorkspace from './live/RoleAiWorkspace.vue'
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
const aiProject = ref(entryParams.get('project') || '')
const navigationRevision = ref(0)
const isManufacturer = computed(() => organization.value?.kind === 'MANUFACTURER')
const isDealer = computed(() => organization.value?.kind === 'RESELLER')
const isCustomer = computed(() => organization.value?.kind === 'CUSTOMER')
const kindLabel = computed(() => portal.value?.label || '')
const RetailDesk = defineAsyncComponent(() => import('./live/RetailDesk.vue'))
const DealerOperations = defineAsyncComponent(() => import('./live/DealerOperations.vue'))
const navigation = computed(() => workspacePages(entry.value, { directView: entryParams.get('view') }))
const currentPage = computed(() => navigation.value.find(page => page.id === view.value))
const shortcuts = computed(() => workspaceShortcuts(navigation.value, entry.value))

const badgeState = ref({ total: 0, pages: {}, items: [] })
const linkedOrgs = ref([])
const bindCode = ref(entryParams.get('bind') || '')
const bindPreview = ref(null)
const bindBusy = ref(false)
const bindNotice = ref('')
const bindError = ref('')

function accessPath(nextMode, role = entry.value) {
  return '/' + nextMode + (role ? '/' + role : '')
}

function customerRetailIntent() {
  const params = new URLSearchParams(location.search)
  if (params.get('view') === 'checkout' && params.has('code')) return 'checkout'
  if (params.get('view') === 'retail-orders' && params.has('order_id')) return 'retail-orders'
  if (params.has('bind')) return 'account'
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
    loadBadges()
    loadLinkedOrganizations()
    if (accountRole === 'customer' && bindCode.value) previewInviteCode()
  } catch (error) {
    if (session.value === active) identityError.value = error.message
  } finally {
    restoringIdentity.value = false
  }
}

async function loadBadges() {
  if (!session.value || !user.value) return
  try {
    const res = await authenticated('/api/workspace/badges')
    if (res && typeof res.pages === 'object' && Array.isArray(res.items)) {
      badgeState.value = { total: Number(res.total) || 0, pages: res.pages, items: res.items }
    }
  } catch {}
}

async function loadLinkedOrganizations() {
  if (!session.value || !organization.value || organization.value.kind === 'MANUFACTURER') return
  try {
    const res = await authenticated('/api/channel/v1/organization-links')
    if (Array.isArray(res?.items)) linkedOrgs.value = res.items
  } catch {}
}

async function previewInviteCode() {
  if (!bindCode.value.trim() || bindBusy.value) return
  bindBusy.value = true
  bindError.value = ''
  bindNotice.value = ''
  bindPreview.value = null
  try {
    bindPreview.value = await authenticated('/api/channel/v1/link-invites/preview?code=' + encodeURIComponent(bindCode.value.trim()))
  } catch (cause) {
    bindError.value = cause.message || '未找到该代理绑定码。'
  } finally {
    bindBusy.value = false
  }
}

async function confirmBindInvite() {
  if (!bindCode.value.trim() || bindBusy.value) return
  bindBusy.value = true
  bindError.value = ''
  bindNotice.value = ''
  try {
    const res = await authenticated('/api/channel/v1/organization-links', {
      method: 'POST',
      body: { code: bindCode.value.trim() },
    })
    bindNotice.value = `已与代理商「${res.reseller?.name || ''}」建立正式协作关联，现在可直接发起采购项目！`
    bindPreview.value = null
    bindCode.value = ''
    const url = new URL(location.href)
    url.searchParams.delete('bind')
    history.replaceState(null, '', url)
    await loadLinkedOrganizations()
  } catch (cause) {
    bindError.value = cause.message || '绑定失败，请核对邀请码后重试。'
  } finally {
    bindBusy.value = false
  }
}

onMounted(() => {
  loadIdentity()
  window.addEventListener('popstate', restoreWorkspaceView)
  window.addEventListener('focus', loadBadges)
})
onUnmounted(() => {
  window.removeEventListener('popstate', restoreWorkspaceView)
  window.removeEventListener('focus', loadBadges)
})

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
  const projectId = next === 'requirement' ? params.get('project') || '' : ''
  if (next !== view.value || (next === 'requirement' && aiProject.value !== projectId)) {
    navigationRevision.value += 1
    view.value = next
    aiProject.value = projectId
    captureProject.value = params.get('captureProject') || ''
  }
}
function navigate(target, params = {}) {
  navigationRevision.value += 1
  view.value = target
  captureProject.value = ''
  aiProject.value = target === 'requirement' ? params.project || '' : ''
  const url = new URL(location.href)
  const checkout = target === 'checkout'
    ? { code: entryParams.get('code') ?? '', product: entryParams.get('product') ?? '' }
    : {}
  url.search = new URLSearchParams({ view: target, ...checkout, ...params })
  history.replaceState(null, '', url)
  window.scrollTo(0, 0)
  loadBadges()
  if (target === 'account') loadLinkedOrganizations()
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
    :account-label="user.username" environment="正式工作台" :badges="badgeState.pages" @navigate="navigate" @account="navigate('account')">
    <RetailDesk v-if="currentPage?.source === 'retail'" :key="navigationRevision + ':' + view" :mode="view" :title="currentPage.label" :role="entry" :request="authenticated" :identity="user.id" @navigate="navigate" />
    <DealerOperations v-else-if="currentPage?.source === 'dealer-operations'" :key="navigationRevision + ':' + view"
      :view="view" :title="currentPage.label" :role="entry" :identity="user.id" :request="authenticated" @navigate="navigate" />
    <DealerPage v-else-if="currentPage?.source === 'utility'" :key="view" :view="view" :title="currentPage.label" :request="authenticated" @navigate="navigate" @project="openProject" />
    <template v-else-if="view === 'overview'">
      <section v-if="badgeState.items.length" class="live-todo-banner" aria-label="跨端协同待办提醒">
        <div class="live-todo-head">
          <UiIcon name="workflow" :size="18" />
          <strong>跨端协同待办（{{ badgeState.total }}）</strong>
        </div>
        <div class="live-todo-pills">
          <button v-for="alert in badgeState.items" :key="alert.view" type="button" class="live-todo-pill" @click="navigate(alert.view)">
            <span>{{ alert.description }}</span>
            <UiIcon name="forward" :size="14" />
          </button>
        </div>
      </section>
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
    <LiveProjects v-else-if="view === 'projects'" :key="navigationRevision" :request="authenticated" :role="entry"
      :organization-id="organization.id" :user-id="user.id" @capture-opportunity="id => openOpportunity('',id)"
      @open-opportunity="id => openOpportunity(id)" @prepare-inquiry="id => navigate('requirement', { project: id })" />
    <LiveVisits v-else-if="view === 'visits' && isDealer" :request="authenticated" :scope="organization.id + ':' + user.id" @project="openProject" @opportunity="id => openOpportunity(id)" />
    <LiveOpportunities v-else-if="view === 'opportunities' && isDealer" :key="navigationRevision" :request="authenticated" :scope="organization.id + ':' + user.id" :project-id="captureProject" :capture="!!captureProject" @project="openProject" />
    <ProductCatalog v-else-if="view === 'catalog' && entry !== 'customer'" :request="authenticated" :is-manufacturer="isManufacturer" />
    <AiWorkspace v-else-if="view === 'requirement' && isDealer" :key="navigationRevision" :request="authenticated"
      :initial-project="aiProject" :organization-id="organization.id" :user-id="user.id"
      @open-project="id => openProject(id)" @open-projects="navigate('projects')"
      @open-opportunity="(id, projectId) => openOpportunity(id, projectId)"
      @navigate="(target, params) => navigate(target, params)" />
    <RoleAiWorkspace v-else-if="view === 'ai-assistant' && (isManufacturer || isCustomer)" :key="navigationRevision"
      :role="entry" :request="authenticated" :organization-id="organization.id" :user-id="user.id" :organization-name="organization.name"
      @open-project="id => openProject(id)" @navigate="(target, params) => navigate(target, params)" @badges-changed="loadBadges" />
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

      <section v-if="isCustomer || isDealer" class="workspace-section live-account" aria-labelledby="linked-orgs-heading" style="margin-top: 20px;">
        <header class="workspace-section-heading">
          <h2 id="linked-orgs-heading">{{ isCustomer ? '已绑定的合作代理商' : '已建立代理关联的企业客户' }}</h2>
          <UiButton v-if="isDealer" compact variant="text" @click="navigate('customers')">前往客户档案生成绑定码</UiButton>
        </header>
        <ul v-if="linkedOrgs.length" class="live-linked-list">
          <li v-for="org in linkedOrgs" :key="org.id">
            <strong>{{ org.name }}</strong>
            <span class="muted">组织编号 #{{ org.id }}</span>
          </li>
        </ul>
        <p v-else class="muted">{{ isCustomer ? '当前尚未绑定代理商，输入下方代理商服务绑定码即可建立协作。' : '当前暂无已关联企业客户，可在「客户档案」生成绑定邀请码发给客户。' }}</p>

        <div v-if="isCustomer" class="live-bind-box">
          <h3>通过绑定码关联代理商</h3>
          <form class="live-bind-form" @submit.prevent="previewInviteCode">
            <input v-model.trim="bindCode" placeholder="输入代理商绑定码，例如 PL-A8F3K9D2" maxlength="16" aria-label="代理商服务绑定码" />
            <UiButton type="submit" compact :loading="bindBusy" :disabled="!bindCode.trim()">核对绑定码</UiButton>
          </form>
          <div v-if="bindPreview" class="live-bind-preview">
            <p>
              识别到二级代理商：<strong>{{ bindPreview.reseller?.name }}</strong>（编号 #{{ bindPreview.reseller?.id }}）
              <span v-if="bindPreview.already_linked" class="muted"> · 已建立关联</span>
            </p>
            <UiButton v-if="!bindPreview.already_linked" variant="primary" compact :loading="bindBusy" @click="confirmBindInvite">确认建立代理协作关系</UiButton>
          </div>
          <p v-if="bindNotice" class="access-notice" role="status">{{ bindNotice }}</p>
          <p v-if="bindError" class="form-error" role="alert">{{ bindError }}</p>
        </div>
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
.live-todo-banner {
  display: grid;
  gap: 10px;
  margin-bottom: 18px;
  padding: 12px 14px;
  border: 1px solid #d5e4ff;
  border-radius: var(--ui-radius-md);
  background: linear-gradient(180deg, #f5f9ff 0%, #eef5ff 100%);
}
.live-todo-head {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #15315e;
  font-size: 14px;
}
.live-todo-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.live-todo-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 11px;
  border: 1px solid #bfd6ff;
  border-radius: 999px;
  background: #ffffff;
  color: var(--ui-link);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
}
.live-todo-pill:hover { background: var(--ui-accent-bg); border-color: var(--ui-accent); }
.live-linked-list {
  display: grid;
  gap: 8px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}
.live-linked-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--ui-border);
  border-radius: var(--ui-radius-sm);
}
.live-bind-box {
  display: grid;
  gap: 10px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--ui-border);
}
.live-bind-box h3 { margin: 0; font-size: 14px; font-weight: 600; }
.live-bind-form { display: flex; flex-wrap: wrap; gap: 8px; }
.live-bind-form input {
  flex: 1 1 220px;
  min-width: 0;
  padding: 7px 10px;
  border: 1px solid var(--ui-border-strong);
  border-radius: var(--ui-radius-sm);
}
.live-bind-preview {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--ui-accent-border);
  border-radius: var(--ui-radius-sm);
  background: var(--ui-accent-bg);
}
.live-bind-preview p { margin: 0; }
</style>
