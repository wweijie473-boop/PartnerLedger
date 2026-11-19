<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { columnName, keyFor } from './cells.js'

const props = defineProps({
  search: { type: Function, required: true },
  prepare: { type: Function, required: true },
  replace: { type: Function, required: true },
  canEdit: Boolean,
})
const emit = defineEmits(['locate'])
const dialog = ref(null), input = ref(null)
const text = ref(''), replacement = ref(''), replacing = ref(false)
const matchCase = ref(false), wholeCell = ref(false), includeFormulas = ref(false)
const page = ref(null), scanned = ref(0), picked = ref(new Set())
const error = ref(''), busy = ref(false), submitting = ref(false)
const readyToReplace = computed(() => page.value?.matches.filter(match => picked.value.has(keyFor(match.row, match.col)) &&
  typeof match.proposed === 'string' && match.proposed !== match.input) ?? [])
let controller
const bytes = new TextEncoder()

function reset() {
  controller?.abort()
  controller = null
  page.value = null
  picked.value = new Set()
  scanned.value = 0
  error.value = ''
  busy.value = false
}
watch([text, replacement, replacing, matchCase, wholeCell, includeFormulas], reset, { flush: 'sync' })
onBeforeUnmount(reset)

async function open(offerReplace) {
  replacing.value = offerReplace && props.canEdit
  if (!dialog.value.open) dialog.value.showModal()
  await nextTick()
  input.value?.focus()
  input.value?.select()
}
function close() {
  if (!submitting.value) dialog.value.close()
}
async function find(more = false) {
  if (busy.value || submitting.value) return
  if (!text.value || bytes.encode(text.value).length > 160) {
    error.value = '请输入 1–160 字节的查找内容。'
    return
  }
  if (replacing.value && bytes.encode(replacement.value).length > 1000) {
    error.value = '替换文字超过 1000 字节。'
    return
  }
  if (!more) reset()
  const request = new AbortController()
  controller = request
  busy.value = true
  error.value = ''
  try {
    const query = { text: text.value, matchCase: matchCase.value, wholeCell: wholeCell.value, includeFormulas: includeFormulas.value }
    if (replacing.value) query.replacement = replacement.value
    if (more) {
      query.after = page.value.nextCursor
      query.revision = page.value.revision
    }
    if (!await props.prepare()) throw new Error('请先处理表格中尚未保存的改动。')
    const cursors = new Set(query.after ? [query.after] : [])
    while (!request.signal.aborted) {
      const result = await props.search(query, request.signal)
      if (request.signal.aborted) return
      if (result.nextCursor && (result.scanned < 1 || cursors.has(result.nextCursor))) {
        throw new Error('查找分页未能继续，请重新查找。')
      }
      page.value = result
      scanned.value += result.scanned
      if (result.matches.length || !result.nextCursor) {
        picked.value = new Set(result.matches.filter(match => typeof match.proposed === 'string' && match.proposed !== match.input)
          .map(match => keyFor(match.row, match.col)))
        break
      }
      cursors.add(result.nextCursor)
      query.after = result.nextCursor
      query.revision = result.revision
    }
  } catch (failure) {
    if (!request.signal.aborted) {
      if (failure.status === 409) {
        page.value = null
        picked.value = new Set()
        scanned.value = 0
        error.value = '表格已有新版本，请重新查找。'
      } else error.value = failure.message
    }
  } finally {
    if (controller === request) { controller = null; busy.value = false }
  }
}
function pick(event, match) {
  const next = new Set(picked.value)
  const key = keyFor(match.row, match.col)
  if (event.target.checked) next.add(key)
  else next.delete(key)
  picked.value = next
}
function locate(match) {
  const revision = page.value.revision
  dialog.value.close()
  emit('locate', match, revision)
}
async function replacePicked() {
  if (!props.canEdit || !readyToReplace.value.length || submitting.value) return
  submitting.value = true
  error.value = ''
  const matches = readyToReplace.value
  const revision = page.value.revision
  try {
    await props.replace(matches, revision)
    dialog.value.close()
  } catch (failure) { error.value = failure.message }
  finally { submitting.value = false }
}

defineExpose({ open })
</script>

<template>
  <dialog ref="dialog" class="grid-search" @cancel.prevent="close" @close="reset">
    <form class="grid-search-form" @submit.prevent="find()">
      <header><h2>{{ replacing ? '查找和替换' : '在表格中查找' }}</h2><button type="button" :disabled="submitting" @click="close">关闭</button></header>
      <label>查找内容 <input ref="input" v-model="text" :disabled="submitting" maxlength="160" autocomplete="off" required /></label>
      <label v-if="canEdit" class="grid-search-option"><input type="checkbox" v-model="replacing" :disabled="submitting" /> 启用替换</label>
      <label v-if="replacing">替换为 <input v-model="replacement" :disabled="submitting" maxlength="1000" autocomplete="off" placeholder="留空表示删除匹配内容" /></label>
      <div class="grid-search-options">
        <label class="grid-search-option"><input type="checkbox" v-model="matchCase" :disabled="submitting" /> 区分大小写</label>
        <label class="grid-search-option"><input type="checkbox" v-model="wholeCell" :disabled="submitting" /> 匹配整个单元格</label>
        <label class="grid-search-option"><input type="checkbox" v-model="includeFormulas" :disabled="submitting" /> 包含公式原文</label>
      </div>
      <div class="grid-search-actions">
        <button type="submit" :disabled="busy || submitting">{{ busy ? '正在查找…' : '查找' }}</button>
        <button v-if="page?.nextCursor" type="button" :disabled="busy || submitting" @click="find(true)">继续查找</button>
      </div>
      <p v-if="error" class="grid-search-error" role="alert">{{ error }}</p>
      <template v-if="page">
        <p class="grid-search-summary" role="status">本页 {{ page.matches.length }} 格匹配 · 已扫描 {{ scanned.toLocaleString() }} 个非空单元格<span v-if="page.nextCursor"> · 还有内容可查找</span></p>
        <ul v-if="page.matches.length" class="grid-search-results" aria-label="查找结果">
          <li v-for="match in page.matches" :key="keyFor(match.row, match.col)">
            <input v-if="replacing" type="checkbox" :checked="picked.has(keyFor(match.row, match.col))"
              :disabled="typeof match.proposed !== 'string' || match.proposed === match.input || busy || submitting"
              :aria-label="'选中 ' + columnName(match.col) + (match.row + 1)" @change="pick($event, match)" />
            <button type="button" :disabled="submitting" @click="locate(match)">{{ columnName(match.col) }}{{ match.row + 1 }}</button>
            <span class="grid-search-value" :title="match.input">{{ match.input }}</span>
            <span v-if="replacing" aria-hidden="true">→</span>
            <span v-if="replacing" class="grid-search-value" :class="{ invalid: match.invalid }" :title="match.proposed">
              {{ match.invalid ? '不符合列类型或长度限制' : match.proposed === '' ? '清空' : match.proposed }}
            </span>
          </li>
        </ul>
        <button v-if="replacing" type="button" :disabled="!readyToReplace.length || busy || submitting" @click="replacePicked">
          {{ submitting ? '正在替换…' : '替换选中的 ' + readyToReplace.length + ' 格' }}
        </button>
      </template>
    </form>
  </dialog>
</template>
