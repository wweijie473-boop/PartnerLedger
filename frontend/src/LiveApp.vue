<script setup>
import { computed, onMounted, ref } from 'vue'
import { requestApi } from './api.js'
import WorkspaceShell from './WorkspaceShell.vue'
import ProductCatalog from './live/ProductCatalog.vue'
import RequirementParser from './live/RequirementParser.vue'
import LiveProjects from './live/LiveProjects.vue'
import LiveVisits from './live/LiveVisits.vue'
import LiveOpportunities from './live/LiveOpportunities.vue'
import { entrances, organizationKinds, organizationRole } from './live/entrances.js'
import { UiBadge, UiButton, UiCard, UiField, UiIcon, UiState } from './ui/index.js'

const props = defineProps({
  role: { type: String, default: '', validator: (value) => value === '' || Object.hasOwn(entrances, value) },
  mode: { type: String, default: 'login' },
})
const entry = ref(props.role)
const mode = ref(props.mode)
const portal = computed(() => entrances[entry.value])
const isRegister = computed(() => mode.value === 'register')
const accessTitle = computed(() => (portal.value?.label || '') + (isRegister.value ? '注册' : '登录'))
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
const organizationName = ref('')
const confirmPassword = ref('')
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
const navigation = computed(() => [
  { id: 'overview', label: '工作总览', icon: 'overview' },
  { id: 'projects', label: entry.value === 'customer' ? '我的采购项目' : isManufacturer.value ? '渠道询价' : '客户项目', icon: 'request' },
  ...(entry.value !== 'customer' ? [{ id: 'catalog', label: '产品目录', icon: 'catalog' }] : []),
  ...(isDealer.value ? [{ id: 'requirement', label: 'AI 需求解析', icon: 'request' }] : []),
  ...(isDealer.value ? [{ id: 'visits', label: '记拜访', icon: 'request' }] : []),
  ...(isDealer.value ? [{ id: 'opportunities', label: '商机拾遗', icon: 'workflow' }] : []),
  { id: 'account', label: '账号与组织', icon: portal.value?.icon || 'customer' },
])

function accessPath(nextMode, role = entry.value) {
  return '/' + nextMode + (role ? '/' + role : '')
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
    if (entry.value && entry.value !== accountRole && !isRegister.value) {
      await authenticated('/api/auth/logout', { method: 'POST' })
      clearSession('此账号属于' + entrances[accountRole].label + '组织，请从对应入口登录。')
      return
    }
    if (!entry.value || isRegister.value) {
      entry.value = accountRole
      mode.value = 'login'
      const url = new URL(location.href)
      url.pathname = accessPath('login')
      history.replaceState(null, '', url)
    }
    document.title = entrances[accountRole].label + '工作台 · 渠智链'
    user.value = profile
    organization.value = currentOrg
    if (!navigation.value.some(item => item.id === view.value)) view.value = 'overview'
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
    history.replaceState(null, '', accessPath('login'))
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
function navigate(target) {
  navigationRevision.value += 1
  view.value = target
  captureProject.value = ''
  const url = new URL(location.href)
  url.search = ''
  url.searchParams.set('view',target)
  history.replaceState(null,'',url)
}
function openProject(id) {
  navigate('projects')
  const url = new URL(location.href)
  url.searchParams.set('project',id)
  history.replaceState(null,'',url)
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
        <form class="form-grid" aria-labelledby="access-title" @submit.prevent="isRegister ? signUp() : signIn()">
          <fieldset class="access-roles" :disabled="signingIn">
            <legend>选择身份</legend>
            <div>
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
        <a :href="accessPath(isRegister ? 'login' : 'register')">{{ isRegister ? '已有账号？登录' : '没有账号？注册' }}</a>
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

  <WorkspaceShell v-else :nav="navigation" :view="view" :organization="organization.name"
    :account-label="user.username" environment="正式工作台" @navigate="navigate" @account="navigate('account')">
    <template #actions><UiButton compact variant="text" :icon="portal.icon" @click="navigate('account')">{{ kindLabel }}</UiButton></template>
    <template v-if="view === 'overview'">
      <section class="page-heading"><div><p class="access-kicker">{{ organization.name }}</p><h1>{{ kindLabel }}工作台</h1>
        <p class="muted">{{ isManufacturer ? '接收渠道询价，回复资料问题并记录能力缺口。' : isDealer ? '接收客户需求、发起厂商询价并跟进双方回复。' : '提交采购需求，向经办代理商补充资料并查看进度。' }}</p></div></section>
      <LiveProjects :key="navigationRevision" overview :request="authenticated" :role="entry" :organization-id="organization.id" @open-project="openProject" />
      <h2 class="live-tools-heading">常用工作</h2>
      <div class="live-tiles">
        <UiCard v-if="entry !== 'customer'" title="产品目录" :description="isManufacturer ? '新增、编辑与发布本组织的软件产品。' : '查看产品版本、说明与厂商发布信息。'">
          <template #actions><UiIcon name="catalog" :size="20" /></template>
          <UiButton end-icon="forward" @click="navigate('catalog')">打开目录</UiButton>
        </UiCard>
        <UiCard v-if="isDealer" title="需求整理" description="录入客户描述，核对授权数、模块和部署方式。">
          <template #actions><UiIcon name="request" :size="20" /></template>
          <UiButton end-icon="forward" @click="navigate('requirement')">整理需求</UiButton>
        </UiCard>
        <UiCard v-if="isDealer" title="记拜访" description="留下一句话，核对主采购事项与附加需求，同一账号可继续整理已同步草稿。">
          <UiButton @click="navigate('visits')">记录本次拜访</UiButton>
        </UiCard>
        <UiCard v-if="isDealer" title="商机拾遗" description="查看团队共享的商机和伙伴，跟进引荐结果、介绍费条件和自己的未读候选。">
          <UiButton @click="navigate('opportunities')">打开商机台账</UiButton>
        </UiCard>
        <UiCard title="项目协作" description="在关联项目中查看进度，处理需求、询价、补充资料和能力缺口。">
          <UiButton @click="navigate('projects')">打开项目</UiButton>
        </UiCard>
      </div>
    </template>
    <LiveProjects v-else-if="view === 'projects'" :key="navigationRevision" :request="authenticated" :role="entry" :organization-id="organization.id" @capture-opportunity="id => openOpportunity('',id)" />
    <LiveVisits v-else-if="view === 'visits' && isDealer" :request="authenticated" :scope="organization.id + ':' + user.id" @project="openProject" @opportunity="id => openOpportunity(id)" />
    <LiveOpportunities v-else-if="view === 'opportunities' && isDealer" :key="navigationRevision" :request="authenticated" :scope="organization.id + ':' + user.id" :project-id="captureProject" :capture="!!captureProject" @project="openProject" />
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
.live-links a { color: var(--ui-link); text-underline-offset: 4px; }
.live-feedback { margin-top: var(--ui-space-3); }
</style>
