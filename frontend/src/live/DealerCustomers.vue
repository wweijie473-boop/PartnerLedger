<script setup>
import { computed, ref } from 'vue'
import { UiButton, UiCard, UiState } from '../ui/index.js'

const props = defineProps({ state: Object, busy: Boolean, submit: Function, request: Function, demo: Boolean })
const emit = defineEmits(['navigate'])
const search = ref('')
const draft = ref(null)
const invite = ref(null)
const inviteBusy = ref(false)
const inviteNotice = ref('')
const inviteError = ref('')
const rows = computed(() => props.state.customers.filter(row => [row.name, row.contact, row.note]
  .join(' ').toLowerCase().includes(search.value.trim().toLowerCase())))

const inviteUrl = computed(() => {
  if (!invite.value?.code) return ''
  const url = new URL('/login/customer', location.origin)
  url.searchParams.set('view', 'account')
  url.searchParams.set('bind', invite.value.code)
  return url.href
})

async function generateInvite(refresh = false) {
  if (!props.request || inviteBusy.value) return
  inviteBusy.value = true
  inviteError.value = ''
  inviteNotice.value = ''
  try {
    invite.value = await props.request('/api/channel/v1/link-invites' + (refresh ? '?refresh=1' : ''), { method: 'POST', body: {} })
    inviteNotice.value = refresh ? '已刷新生成新的客户绑定邀请码。' : '已获取专属代理绑定邀请码，发给企业客户确认后即可代客下单与协同建项。'
  } catch (cause) {
    inviteError.value = cause.message || '生成绑定邀请码失败。'
  } finally {
    inviteBusy.value = false
  }
}

async function copyInviteText(text, label) {
  if (!text || !navigator.clipboard) return
  try {
    await navigator.clipboard.writeText(text)
    inviteNotice.value = label + '已复制到剪贴板。'
  } catch {}
}

async function save() {
  const { id, name, contact, note } = draft.value
  const message = props.demo ? '客户档案已保存，可在代客下单时选择该客户。' : '客户档案已保存。'
  if (await props.submit(id ? 'customers/' + id : 'customers', { name, contact, note }, message)) {
    draft.value = null
  }
}
</script>

<template>
  <UiCard v-if="draft" :title="draft.id ? '编辑客户档案' : '新增客户档案'">
    <form class="operations-form" @submit.prevent="save">
      <label>企业名称<input v-model.trim="draft.name" required :maxlength="draft.id && !demo && !draft.prospect ? 120 : 80"
        :readonly="Boolean(draft.id && !demo && !draft.prospect)"></label>
      <label>联系人<input v-model.trim="draft.contact" maxlength="40"></label>
      <label>服务备注<textarea v-autogrow v-model.trim="draft.note" maxlength="200" /></label>
      <div class="operations-actions"><UiButton type="submit" variant="primary" :loading="busy">保存客户</UiButton><UiButton :disabled="busy" @click="draft = null">取消</UiButton></div>
    </form>
  </UiCard>
  <UiCard v-if="!demo && invite" title="企业客户自助绑定邀请" description="企业客户登录「账号与组织」输入此绑定码（或直接打开专属链接）确认后，即刻成为您的已关联客户。">
    <div class="operations-form">
      <label>专属绑定码（7 天有效）<input :value="invite.code" readonly aria-label="专属绑定码"></label>
      <label>客户一键绑定链接<input :value="inviteUrl" readonly aria-label="客户一键绑定链接"></label>
      <p v-if="inviteNotice" class="operations-notice" role="status">{{ inviteNotice }}</p>
      <p v-if="inviteError" class="form-error" role="alert">{{ inviteError }}</p>
      <div class="operations-actions">
        <UiButton compact variant="primary" @click="copyInviteText(invite.code, '绑定码')">复制绑定码</UiButton>
        <UiButton compact @click="copyInviteText(inviteUrl, '客户绑定链接')">复制绑定链接</UiButton>
        <UiButton compact variant="text" :loading="inviteBusy" @click="generateInvite(true)">重置新码</UiButton>
        <UiButton compact variant="text" @click="invite = null">收起</UiButton>
      </div>
    </div>
  </UiCard>
  <UiCard title="客户列表" :description="demo ? '' : '已关联客户可用于代客下单与协同建项；直购或新注册客户可通过绑定邀请码一键建立关联。'">
    <label class="operations-search">搜索客户<input v-model="search" type="search" placeholder="企业、联系人或备注"></label>
    <div class="operations-actions">
      <UiButton variant="primary" :disabled="busy" @click="draft = { name: '', contact: '', note: '' }">新增客户</UiButton>
      <UiButton v-if="!demo && request" :loading="inviteBusy" @click="generateInvite(false)">生成客户绑定码</UiButton>
      <UiButton @click="emit('navigate', 'assisted')">去代客下单</UiButton>
    </div>
    <p v-if="!invite && inviteError" class="form-error" role="alert">{{ inviteError }}</p>
    <div v-if="rows.length" class="operations-table">
      <table><thead><tr><th scope="col">企业</th><th scope="col">联系人</th><th scope="col">服务备注</th><th scope="col">操作</th></tr></thead>
        <tbody><tr v-for="row in rows" :key="row.id">
          <td>{{ row.name }}<small v-if="!demo" class="operations-meta">{{ row.linked ? '已关联' : row.prospect ? '潜在客户' : '直购客户' }}</small></td>
          <td>{{ row.contact || '—' }}</td><td>{{ row.note || '—' }}</td>
          <td>
            <UiButton compact variant="text" :disabled="busy" @click="draft = { ...row }">编辑</UiButton>
            <UiButton v-if="!demo && !row.linked && !row.prospect && request" compact variant="text" :loading="inviteBusy" @click="generateInvite(false)">邀请绑定</UiButton>
          </td>
        </tr></tbody>
      </table>
    </div>
    <UiState v-else title="没有匹配的客户档案" />
  </UiCard>
</template>
