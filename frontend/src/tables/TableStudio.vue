<script setup>
import { computed, markRaw, onMounted, ref, shallowRef } from 'vue'
import { requestApi } from '../api.js'
import { accountPath, currentSession, saveSession, sessionFromCredentials } from '../auth-session.js'
import { columnName, MAX_COLS, MAX_ROWS } from './cells.js'
import DataGrid from './DataGrid.vue'
import { createTableRepository } from './httpSource.js'
import './studio.css'

const user = ref(null), sheets = shallowRef([]), nextCursor = ref(null)
const selected = shallowRef(null), source = shallowRef(null), grid = ref(null)
const username = ref(''), password = ref(''), busy = ref(false), initializing = ref(true)
const error = ref(''), dialogError = ref(''), dialog = ref(null), editing = ref(null)
const submitBusy = ref(false)
let previousSubmission

const labels = { private: '仅自己', org_read: '组织内可查看', org_edit: '组织内可编辑' }
const kinds = { auto: '通用', text: '纯文本', number: '数值', date: '日期', checkbox: '布尔值' }
const recoveryKey = computed(() => 'partnerledger:table-draft:' + user.value?.organization.id + ':' + user.value?.id + ':' + selected.value?.id)
const defaultColumns = () => Array.from({ length: 8 }, (_, i) => ({ label: '字段 ' + columnName(i), kind: 'auto' }))

async function authorized(path, options) {
  const active = currentSession()
  if (!active) {
    clearSession()
    throw new Error('登录已失效，请重新登录。')
  }
  try {
    const result = await requestApi(path, { ...options, token: active.token })
    if (currentSession() !== active) throw new DOMException('Session changed', 'AbortError')
    return result
  } catch (failure) {
    if (failure.status === 401 && currentSession() === active) clearSession()
    throw failure
  }
}
const repository = createTableRepository(authorized)

function clearSession() {
  saveSession(null)
  user.value = null; sheets.value = []; selected.value = null; source.value = null
}
async function signin() {
  busy.value = true
  error.value = ''
  try {
    const result = await requestApi('/api/auth/login', { method: 'POST', body: { username: username.value.trim(), password: password.value } })
    saveSession(sessionFromCredentials(result))
    password.value = ''
    user.value = result.user
    await loadWorkspace()
  } catch (failure) { error.value = failure.message }
  finally { busy.value = false }
}
async function logout() {
  if (grid.value && !await grid.value.flush()) {
    error.value = '表格仍有待保存改动，请先在表格中处理。'
    return
  }
  try { await authorized('/api/auth/logout', { method: 'POST' }); clearSession() }
  catch (failure) { error.value = failure.message }
}
async function loadList(after) {
  const page = await repository.list(after)
  sheets.value = after ? [...sheets.value, ...page.items] : page.items
  nextCursor.value = page.nextCursor
}
async function loadWorkspace() {
  const preferred = new URLSearchParams(location.search).get('sheet')
  await loadList()
  if (preferred) {
    try {
      const meta = await repository.source(preferred).getSheet()
      openSheet(meta)
      return
    } catch (failure) { error.value = failure.message }
  }
  if (sheets.value.length) openSheet(sheets.value[0])
}
async function more() {
  busy.value = true
  try { await loadList(nextCursor.value) } catch (failure) { error.value = failure.message }
  finally { busy.value = false }
}
function metadata(meta) {
  if (selected.value?.id === meta.id) selected.value = meta
  if (sheets.value.some(item => item.id === meta.id)) sheets.value = sheets.value.map(item => item.id === meta.id ? meta : item)
}
function openSheet(meta) {
  selected.value = meta
  source.value = markRaw(repository.source(meta.id))
  const url = new URL(location.href)
  url.searchParams.set('sheet', meta.id)
  history.replaceState(null, '', url)
}
async function pick(meta) {
  if (selected.value?.id === meta.id) return
  error.value = ''
  busy.value = true
  try {
    if (grid.value && !await grid.value.flush()) throw new Error('请先处理当前表格的未保存改动。')
    openSheet(meta)
  } catch (failure) { error.value = failure.message }
  finally { busy.value = false }
}
async function openDialog(mode) {
  dialogError.value = ''
  previousSubmission = undefined
  error.value = ''
  if (mode === 'create') {
    editing.value = { mode, name: '', rowCount: 1000, visibility: 'private', columns: defaultColumns(), originalWidth: 0, minRows: 1 }
  } else {
    if (!await grid.value.flush()) { error.value = '请先处理当前表格的未保存改动。'; return }
    try {
      const meta = await source.value.getSheet()
      editing.value = { mode, name: meta.name, rowCount: meta.rowCount, visibility: meta.visibility,
        columns: meta.columns.map(column => ({ ...column })), originalWidth: meta.columns.length,
        minRows: meta.rowCount, revision: meta.revision }
    } catch (failure) { error.value = failure.message; return }
  }
  dialog.value.showModal()
}
function addColumn() {
  if (editing.value.columns.length < MAX_COLS) editing.value.columns.push({ label: '字段 ' + columnName(editing.value.columns.length), kind: 'auto' })
}
function closeDialog() { if (!submitBusy.value) dialog.value.close() }
async function submitSettings() {
  const form = editing.value
  const settings = { name: form.name.trim(), rowCount: form.rowCount, visibility: form.visibility,
    columns: form.columns.map(column => ({ label: column.label.trim(), kind: column.kind })) }
  const key = JSON.stringify(settings)
  if (previousSubmission?.key !== key) previousSubmission = { key, id: crypto.randomUUID() }
  submitBusy.value = true
  dialogError.value = ''
  try {
    if (form.mode === 'create') {
      if (grid.value && !await grid.value.flush()) throw new Error('请先处理当前表格的未保存改动。')
      const result = await repository.create({ requestId: previousSubmission.id, ...settings })
      if (!sheets.value.some(item => item.id === result.id)) sheets.value = [result, ...sheets.value]
      openSheet(result)
    } else {
      await source.value.saveChanges({ requestId: previousSubmission.id, revision: form.revision, cells: [], settings })
      await grid.value.refresh()
    }
    dialog.value.close()
  } catch (failure) { dialogError.value = failure.status === 409 ? '表格设置有新版本，请关闭后重新打开并核对。' : failure.message }
  finally { submitBusy.value = false }
}
async function exportSheet() {
  busy.value = true
  error.value = ''
  try {
    if (!await grid.value.flush()) throw new Error('请先保存改动。')
    const active = currentSession()
    if (!active) {
      clearSession()
      throw new Error('登录已失效，请重新登录。')
    }
    const filename = selected.value.name
    const response = await fetch(source.value.exportURL, {
      headers: { Authorization: 'Bearer ' + active.token, Accept: 'text/csv' },
      cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(30000),
    })
    if (currentSession() !== active) return
    if (response.status === 401) { clearSession(); throw new Error('登录已失效，请重新登录。') }
    if (!response.ok) throw new Error('导出失败，导出范围或数据量可能超过限制。')
    const href = URL.createObjectURL(await response.blob())
    const download = document.createElement('a')
    download.href = href
    download.download = filename.replace(/[\\/]/g, '-') + '.csv'
    download.click()
    setTimeout(() => URL.revokeObjectURL(href), 1000)
  } catch (failure) { error.value = failure.message }
  finally { busy.value = false }
}

onMounted(async () => {
  try {
    if (currentSession()) {
      user.value = await authorized('/api/user/profile')
      await loadWorkspace()
    }
  } catch (failure) { error.value = failure.message }
  finally { initializing.value = false }
})
</script>

<template>
  <div class="studio-page">
    <header class="studio-top"><div class="studio-top-inner">
      <a href="/" class="studio-brand">渠智链</a><span class="studio-crumb">表格工作台</span>
      <template v-if="user">
        <a class="studio-identity" :href="accountPath">{{ user.organization.name }} · {{ user.username }}</a>
        <button type="button" @click="logout">退出</button>
      </template>
    </div></header>
    <p v-if="initializing" class="table-message" role="status">正在核验会话…</p>
    <main v-else-if="!user" class="studio-auth">
      <h1>登录表格工作台</h1>
      <p>使用你的 PartnerLedger 账号管理表格。</p>
      <form @submit.prevent="signin">
        <label>用户名<input v-model="username" autocomplete="username" required /></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <p v-if="error" role="alert" class="studio-alert">{{ error }}</p>
        <button type="submit" class="studio-primary" :disabled="busy">{{ busy ? '正在登录…' : '登录' }}</button>
      </form>
    </main>
    <main v-else class="studio-content">
      <aside class="studio-list">
        <h2>我的表格与共享表格</h2>
        <button type="button" class="studio-primary" :disabled="busy" @click="openDialog('create')">新建表格</button>
        <nav v-if="sheets.length" aria-label="表格列表">
          <button v-for="item in sheets" :key="item.id" type="button" :title="item.name" :disabled="busy"
            :aria-current="selected?.id === item.id ? 'true' : undefined" @click="pick(item)">{{ item.name }}</button>
        </nav>
        <button v-if="nextCursor" type="button" :disabled="busy" @click="more">加载更多</button>
      </aside>
      <article class="studio-work">
        <p v-if="error" role="alert" class="studio-alert">{{ error }}</p>
        <template v-if="selected">
          <div class="studio-title"><div><h1>{{ selected.name }}</h1><p>{{ labels[selected.visibility] }}</p></div>
            <div class="studio-actions">
              <button type="button" v-if="selected.isOwner" :disabled="busy" @click="openDialog('settings')">表格设置</button>
              <button type="button" :disabled="busy" title="导出前 10,000 行内有数据的区域" @click="exportSheet">导出 CSV</button>
            </div>
          </div>
          <DataGrid :key="selected.id" ref="grid" :source="source" :recovery-key="recoveryKey" @metadata="metadata" />
        </template>
        <div v-else class="studio-empty"><div class="studio-title"><h1>表格工作台</h1></div><p>创建空白表格，或从左侧打开共享表格。</p></div>
      </article>
    </main>
    <dialog ref="dialog" class="studio-dialog" @cancel.prevent="closeDialog">
      <form v-if="editing" class="studio-form" @submit.prevent="submitSettings">
        <h2>{{ editing.mode === 'create' ? '新建表格' : '表格设置' }}</h2>
        <label>表格名称<input v-model="editing.name" maxlength="120" required /></label>
        <label>可见范围<select v-model="editing.visibility"><option v-for="(label, id) in labels" :key="id" :value="id">{{ label }}</option></select></label>
        <label>行数<input v-model.number="editing.rowCount" type="number" :min="editing.minRows" :max="MAX_ROWS" required /></label>
        <div class="studio-fields">
          <div v-for="(column, i) in editing.columns" :key="i" class="studio-column-edit">
            <span>{{ columnName(i) }}</span>
            <input :aria-label="columnName(i) + '列名称'" v-model="column.label" maxlength="80" required />
            <select :aria-label="columnName(i) + '列类型'" v-model="column.kind" :disabled="i < editing.originalWidth">
              <option v-for="(label, id) in kinds" :key="id" :value="id">{{ label }}</option>
            </select>
          </div>
        </div>
        <div class="studio-actions">
          <button type="button" :disabled="editing.columns.length >= MAX_COLS" @click="addColumn">添加列</button>
          <button type="button" :disabled="editing.columns.length <= Math.max(1, editing.originalWidth)" @click="editing.columns.pop()">移除最后一列</button>
        </div>
        <p v-if="dialogError" role="alert" class="studio-alert">{{ dialogError }}</p>
        <footer><button type="button" :disabled="submitBusy" @click="closeDialog">取消</button>
          <button type="submit" class="studio-primary" :disabled="submitBusy">{{ editing.mode === 'create' ? '创建表格' : '保存设置' }}</button></footer>
      </form>
    </dialog>
  </div>
</template>
