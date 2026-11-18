<script setup>
import { computed, onMounted, ref } from 'vue'
import { requestApi } from './api.js'
import WorkspaceShell from './WorkspaceShell.vue'
import ProductCatalog from './live/ProductCatalog.vue'
import RequirementParser from './live/RequirementParser.vue'
import { UiBadge, UiButton, UiCard, UiField, UiIcon, UiState } from './ui/index.js'

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
const view = ref('catalog')
const isManufacturer = computed(() => organization.value?.kind === 'MANUFACTURER')
const kindLabel = computed(() => isManufacturer.value ? '厂家' : '经销商')
const navigation = computed(() => [
  { id: 'catalog', label: '产品目录', icon: 'catalog' },
  ...(!isManufacturer.value ? [{ id: 'requirement', label: 'AI 需求解析', icon: 'request' }] : []),
  { id: 'account', label: '账号与组织', icon: isManufacturer.value ? 'vendor' : 'dealer' },
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
  view.value = 'catalog'
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
    if (profile?.organization?.id !== currentOrg?.id || !profile?.username ||
        !['MANUFACTURER', 'RESELLER'].includes(currentOrg?.kind)) {
      throw new Error('后端未返回可用的账号与组织信息。')
    }
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
  <main v-if="!session" class="login-screen live-login">
    <div class="brand-mark"><UiIcon name="workflow" :size="22" /></div>
    <h1>登录渠智链 AI</h1>
    <p class="muted">使用团队后端开通的账号登录。</p>
    <form class="form-grid" @submit.prevent="signIn">
      <UiField label="用户名" required v-slot="{ control }">
        <input v-bind="control" v-model="username" autocomplete="username" maxlength="64" required />
      </UiField>
      <UiField label="密码" required v-slot="{ control }">
        <input v-bind="control" v-model="password" type="password" autocomplete="current-password" required />
      </UiField>
      <p v-if="loginError" class="form-error" role="alert">{{ loginError }}</p>
      <UiButton type="submit" variant="primary" :loading="signingIn">登录</UiButton>
    </form>
    <div class="live-links"><a href="/demo">打开模拟演示</a><a href="/ui">组件库</a></div>
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
    :account-label="user.username" environment="后端工作台" @navigate="view = $event" @account="view = 'account'">
    <template #actions><UiButton compact variant="text" :icon="isManufacturer ? 'vendor' : 'dealer'" @click="view = 'account'">{{ kindLabel }}</UiButton></template>
    <ProductCatalog v-if="view === 'catalog'" :request="authenticated" :is-manufacturer="isManufacturer" />
    <RequirementParser v-else-if="view === 'requirement' && !isManufacturer" :request="authenticated" />
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
    <footer class="live-links"><a href="/demo">模拟演示</a><a href="/ui">组件库</a></footer>
  </WorkspaceShell>
</template>

<style scoped>
.live-login { max-width: 446px; }
.live-login h1 { font-size: clamp(28px, 4vw, 38px); margin-block: 18px 8px; }
.live-login form { margin-top: var(--ui-space-6); }
.live-login form .ui-button { width: 100%; margin-top: var(--ui-space-2); }
.live-links { display: flex; flex-wrap: wrap; gap: var(--ui-space-4); margin-top: var(--ui-space-5); font-size: var(--ui-text-sm); }
.live-links a { color: var(--ui-accent); text-underline-offset: 4px; }
.live-feedback { margin-top: var(--ui-space-3); }
</style>
