<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch, watchEffect } from 'vue'
import { clipboardTSV, columnName, internalClipboard, keyFor, MAX_CHANGES, parseAddress, readClipboard, validateCell } from './cells.js'
import { displayCell } from './formulas.js'
import { createSaveQueue } from './saveQueue.js'
import { createWindowCache } from './windowCache.js'
import GridSearch from './GridSearch.vue'
import './grid.css'

const props = defineProps({
  source: { type: Object, required: true },
  recoveryKey: { type: String, default: '' },
  readOnly: Boolean,
})
const emit = defineEmits(['metadata', 'dirty-change'])
const rowHeight = 32, headerHeight = 40, indexWidth = 54, cellWidth = 148
const sheet = shallowRef(null)
const grid = ref(null), editorElement = ref(null), searchDialog = ref(null)
const viewport = ref({ x: 0, y: 0, width: 800, height: 420 })
const selected = ref({ row: 0, col: 0 }), anchor = ref(selected.value)
const editor = ref(null), bar = ref(''), barFocused = ref(false)
const editingError = ref(''), readError = ref(''), storageError = ref(false)
const loading = ref(true), applying = ref(false)
const epoch = ref(0)
const state = shallowRef({ dirty: false, running: false, error: null, overlay: new Map() })
const undoStack = shallowRef([]), redoStack = shallowRef([])
let cache, queue, observer, refreshing = false, closed = false, cachedRecovery

const editable = computed(() => sheet.value?.canEdit && !props.readOnly)
const canvas = computed(() => sheet.value && {
  width: indexWidth + sheet.value.columns.length * cellWidth + 'px',
  height: headerHeight + sheet.value.rowCount * rowHeight + 'px',
})
function indices(start, length, capacity, size, overscan) {
  const first = Math.max(0, Math.floor(start / size) - overscan)
  const last = Math.min(capacity, Math.ceil((start + length) / size) + overscan)
  return Array.from({ length: last - first }, (_, i) => i + first)
}
const rows = computed(() => sheet.value ? indices(viewport.value.y, viewport.value.height, sheet.value.rowCount, rowHeight, 5) : [])
const columns = computed(() => sheet.value ? indices(viewport.value.x, viewport.value.width, sheet.value.columns.length, cellWidth, 2) : [])
const address = computed(() => columnName(selected.value.col) + (selected.value.row + 1))
const bounds = computed(() => ({
  top: Math.min(selected.value.row, anchor.value.row), bottom: Math.max(selected.value.row, anchor.value.row),
  left: Math.min(selected.value.col, anchor.value.col), right: Math.max(selected.value.col, anchor.value.col),
}))
const status = computed(() => {
  if (state.value.error?.status === 409) return '保存冲突'
  if (state.value.error) return '待重试'
  if (state.value.running) return '正在保存'
  return state.value.dirty ? '待保存' : '已保存'
})
const domID = (r, c) => 'cell-' + sheet.value.id + '-' + r + '-' + c
const activeID = computed(() => sheet.value && rows.value.includes(selected.value.row) && columns.value.includes(selected.value.col)
  ? domID(selected.value.row, selected.value.col) : undefined)

function recover() {
  if (!props.recoveryKey) return null
  try {
    const draft = JSON.parse(sessionStorage.getItem(props.recoveryKey))
    if (!Number.isSafeInteger(draft?.revision) || !Array.isArray(draft.groups)) return null
    const groups = [...draft.groups, ...(draft.batch ? [draft.batch.cells] : [])]
    if (groups.some(group => !Array.isArray(group) || group.length === 0 || group.length > MAX_CHANGES ||
      group.some(c => !Number.isInteger(c?.row) || !Number.isInteger(c?.col) || typeof c?.input !== 'string'))) return null
    return draft
  } catch { storageError.value = true; return null }
}
function changed(value) {
  const wasDirty = state.value.dirty
  state.value = value
  if (wasDirty !== value.dirty) emit('dirty-change', value.dirty)
  if (!value.dirty && !value.running && value.observed > value.revision) void refresh()
  if (!props.recoveryKey) return
  try {
    const next = JSON.stringify(value.recovery)
    if (cachedRecovery === next) return
    if (value.recovery) sessionStorage.setItem(props.recoveryKey, next)
    else sessionStorage.removeItem(props.recoveryKey)
    cachedRecovery = next
    storageError.value = false
  } catch { storageError.value = true }
}
function connect(meta) {
  cache?.close()
  sheet.value = meta
  cache = createWindowCache({
    source: props.source, sheet: meta,
    onData: () => { epoch.value++ },
    onError: error => {
      readError.value = error.message
      if ([401, 403, 404].includes(error.status)) {
        sheet.value = { ...sheet.value, canEdit: false }
        cache.invalidate()
        cache.close()
      }
    },
    onRemote: revision => {
      if (!state.value.dirty && (editor.value || barFocused.value || applying.value)) return
      queue.observe(revision)
      if (!state.value.dirty) void refresh()
    },
  })
  epoch.value++
  emit('metadata', meta)
}
async function initialize() {
  loading.value = true
  readError.value = ''
  try {
    const meta = await props.source.getSheet()
    if (closed) return
    queue = createSaveQueue({
      revision: meta.revision, save: props.source.saveChanges, recover: meta.canEdit && !props.readOnly ? recover() : null,
      onChange: changed, onSaved: (cells, revision) => cache.confirm(cells, revision),
    })
    const initial = queue.state()
    if (meta.canEdit && !props.readOnly) changed(initial)
    connect(meta)
  } catch (error) { readError.value = error.message }
  finally { loading.value = false }
}
async function refresh(keep) {
  if (!sheet.value) return initialize()
  if (refreshing || state.value.running || keep === undefined &&
    (state.value.dirty || editor.value || barFocused.value || applying.value)) return false
  refreshing = true
  const recoveringRead = Boolean(readError.value)
  try {
    const meta = await props.source.getSheet()
    if (closed) return false
    readError.value = ''
    if (keep && (!meta.canEdit || props.readOnly)) {
      editingError.value = '当前账号已无编辑权限，改动仍保存在当前标签页。'
      return false
    }
    if (keep === undefined && !recoveringRead && meta.revision === state.value.revision &&
      meta.revision === sheet.value.revision && meta.canEdit === sheet.value.canEdit) return true
    if (keep === false) { editor.value = null; barFocused.value = false; editingError.value = '' }
    connect(meta)
    queue.reconcile(meta.revision, Boolean(keep))
    undoStack.value = []; redoStack.value = []
    return true
  } catch (error) { readError.value = error.message; return false }
  finally { refreshing = false }
}

function raw(row, col) {
  void epoch.value
  const key = keyFor(row, col)
  if (state.value.overlay.has(key)) return state.value.overlay.get(key)
  const value = cache?.get(row, col)
  if (value === undefined && !readError.value && state.value.error?.status !== 409) void cache?.load(row, col)
  return value
}
function display(row, col) { return displayCell(row, col, raw, sheet.value.rowCount, sheet.value.columns) }
function inRange(row, col) {
  const { top, bottom, left, right } = bounds.value
  return row >= top && row <= bottom && col >= left && col <= right
}
function measure() {
  if (!grid.value) return
  const el = grid.value
  viewport.value = { x: el.scrollLeft, y: el.scrollTop, width: el.clientWidth, height: el.clientHeight }
}
function select(row, col, extend = false, scroll = true) {
  if (!sheet.value) return
  const point = {
    row: Math.max(0, Math.min(sheet.value.rowCount - 1, row)),
    col: Math.max(0, Math.min(sheet.value.columns.length - 1, col)),
  }
  selected.value = point
  if (!extend) anchor.value = point
  if (!scroll || !grid.value) return
  const el = grid.value
  const x = indexWidth + cellWidth * point.col, y = headerHeight + rowHeight * point.row
  if (x < el.scrollLeft + indexWidth) el.scrollLeft = x - indexWidth
  else if (x + cellWidth > el.scrollLeft + el.clientWidth) el.scrollLeft = x + cellWidth - el.clientWidth
  if (y < el.scrollTop + headerHeight) el.scrollTop = y - headerHeight
  else if (y + rowHeight > el.scrollTop + el.clientHeight) el.scrollTop = y + rowHeight - el.clientHeight
  measure()
}
function focusGrid() { nextTick(() => grid.value?.focus({ preventScroll: true })) }
function jump(event) {
  const position = parseAddress(event.target.value)
  if (!position || position.row >= sheet.value.rowCount || position.col >= sheet.value.columns.length) {
    editingError.value = '请输入此表范围内的单元格地址。'
    return
  }
  editingError.value = ''
  select(position.row, position.col)
  event.target.value = address.value
  focusGrid()
}
async function beginEdit(row, col, initial) {
  if (!editable.value || applying.value || state.value.error?.status === 409) return
  const loaded = raw(row, col)
  if (loaded === undefined) {
    await cache.load(row, col)
    if (raw(row, col) === undefined || selected.value.row !== row || selected.value.col !== col) return
  }
  editor.value = { row, col, input: initial ?? raw(row, col) }
  editingError.value = ''
  await nextTick()
  editorElement.value?.focus()
  if (initial === undefined) editorElement.value?.select()
}
async function apply(cells, record = true) {
  if (!editable.value || applying.value || state.value.error?.status === 409) return false
  applying.value = true
  editingError.value = ''
  try {
    const before = [], after = []
    for (const cell of cells) {
      if (!sheet.value.columns[cell.col] || cell.row >= sheet.value.rowCount) throw new Error('所选区域超出表格范围。')
      const invalid = validateCell(sheet.value.columns[cell.col].kind, cell.input)
      if (invalid) throw new Error(columnName(cell.col) + (cell.row + 1) + '：' + invalid)
    }
    await Promise.all(cells.map(cell => cache.load(cell.row, cell.col)))
    for (const cell of cells) {
      const previous = raw(cell.row, cell.col)
      if (previous === undefined) throw new Error('等待单元格加载后再编辑。')
      if (previous !== cell.input) {
        before.push({ ...cell, input: previous })
        after.push(cell)
      }
    }
    if (!after.length) return true
    if (state.value.error?.status === 409) return false
    stage(after, before, record)
    return true
  } catch (error) { editingError.value = error.message; return false }
  finally { applying.value = false }
}
function stage(after, before, record) {
  queue.add(after)
  if (record) {
    undoStack.value = [...undoStack.value.slice(-29), { before, after }]
    redoStack.value = []
  }
}
async function locateMatch(match, revision) {
  if (state.value.revision !== revision) await refresh()
  select(match.row, match.col)
  focusGrid()
}
async function replaceMatches(matches, revision) {
  if (!editable.value) throw new Error('当前表格为只读。')
  if (!await flush()) throw new Error('请先处理表格中尚未保存的改动。')
  if (state.value.revision !== revision) await refresh()
  if (state.value.revision !== revision || state.value.dirty) throw new Error('表格已有新版本，请重新预览替换。')
  if (!editable.value) throw new Error('当前表格为只读。')

  const before = [], after = []
  for (const { row, col, input, proposed } of matches) {
    const kind = sheet.value.columns[col]?.kind
    if (!kind || row < 0 || row >= sheet.value.rowCount || typeof proposed !== 'string' || validateCell(kind, proposed)) {
      throw new Error('替换结果不符合表格列设置。')
    }
    before.push({ row, col, input })
    after.push({ row, col, input: proposed })
  }
  stage(after, before, true)
  select(after[0].row, after[0].col)
  await queue.flush()
}
async function finishEdit(move = '') {
  const cell = editor.value
  if (!cell || applying.value) return
  if (await apply([{ ...cell }])) {
    editor.value = null
    if (move === 'down') select(cell.row + 1, cell.col)
    if (move === 'right') select(cell.row, cell.col + 1)
    if (move === 'left') select(cell.row, cell.col - 1)
    if (move) focusGrid()
  }
}
async function finishBar() {
  if (!barFocused.value) return
  barFocused.value = false
  if (bar.value !== raw(selected.value.row, selected.value.col)) await apply([{ ...selected.value, input: bar.value }])
}
function editorKey(event) {
  if (event.isComposing) return
  if (event.key === 'Escape') { event.preventDefault(); editor.value = null; focusGrid() }
  if (event.key === 'Enter' && !event.altKey || event.key === 'Tab') {
    event.preventDefault()
    void finishEdit(event.key === 'Enter' ? 'down' : event.shiftKey ? 'left' : 'right')
  }
}
async function undo(redo = false) {
  const source = redo ? redoStack : undoStack, destination = redo ? undoStack : redoStack
  const group = source.value.at(-1)
  if (!group || !await apply(redo ? group.after : group.before, false)) return
  destination.value = [...destination.value, group]
  source.value = source.value.slice(0, -1)
}
function selection(transform) {
  const { top, bottom, left, right } = bounds.value
  if ((bottom - top + 1) * (right - left + 1) > MAX_CHANGES) throw new Error('单次操作最多 250 个单元格。')
  const result = []
  for (let row = top; row <= bottom; row++) {
    const items = []
    for (let col = left; col <= right; col++) items.push(transform(row, col))
    result.push(items)
  }
  return result
}
function copy(event) {
  if (editor.value || event.target !== grid.value) return
  try {
    const matrix = selection((row, col) => {
      const value = raw(row, col)
      if (value === undefined) throw new Error('复制区域仍在加载。')
      return value
    })
    event.clipboardData.setData('text/plain', clipboardTSV(matrix))
    event.clipboardData.setData(internalClipboard, JSON.stringify(matrix))
    event.preventDefault()
  } catch (error) { editingError.value = error.message }
}
function paste(event) {
  if (!editable.value || editor.value || event.target !== grid.value) return
  event.preventDefault()
  try {
    const { top, left } = bounds.value
    const edits = readClipboard(event.clipboardData).flatMap((line, y) =>
      line.map((input, x) => ({ row: top + y, col: left + x, input })))
    void apply(edits)
  } catch (error) { editingError.value = error.message }
}
function key(event) {
  if (event.isComposing || event.target !== grid.value) return
  const shortcut = event.key.toLowerCase()
  if (searchDialog.value && (event.ctrlKey || event.metaKey) && ['f', 'h'].includes(shortcut)) {
    event.preventDefault()
    void searchDialog.value.open(shortcut === 'h')
    return
  }
  const { row, col } = selected.value
  const directions = { ArrowUp: [-1, 0], ArrowDown: [1, 0], ArrowLeft: [0, -1], ArrowRight: [0, 1] }
  if (directions[event.key]) {
    event.preventDefault()
    const [y, x] = directions[event.key]
    select(row + y, col + x, event.shiftKey)
  } else if (event.key === 'Tab') {
    const delta = event.shiftKey ? -1 : 1
    if (col + delta >= 0 && col + delta < sheet.value.columns.length) {
      event.preventDefault()
      select(row, col + delta)
    }
  } else if (event.key === 'Enter' || event.key === 'F2') {
    event.preventDefault()
    void beginEdit(row, col)
  } else if (event.key === 'Delete' || event.key === 'Backspace') {
    event.preventDefault()
    try { void apply(selection((r, c) => ({ row: r, col: c, input: '' })).flat()) }
    catch (error) { editingError.value = error.message }
  } else if ((event.ctrlKey || event.metaKey) && ['z', 'y'].includes(event.key.toLowerCase())) {
    event.preventDefault()
    void undo(event.shiftKey || event.key.toLowerCase() === 'y')
  } else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
    event.preventDefault()
    void beginEdit(row, col, event.key)
  }
}
function clickCell(event, row, col) {
  if (editor.value) return
  select(row, col, event.shiftKey, false)
  focusGrid()
}
function beforeUnload(event) {
  if (state.value.dirty || editor.value || barFocused.value && bar.value !== raw(selected.value.row, selected.value.col)) {
    event.preventDefault()
    event.returnValue = ''
  }
}
function onOnline() { if (state.value.dirty) void queue?.retry() }
function onFocus() { if (!state.value.dirty && sheet.value && !editor.value && !barFocused.value) void refresh() }
async function flush() {
  if (editor.value) await finishEdit()
  if (barFocused.value) await finishBar()
  return !editor.value && !applying.value && (await queue?.flush() ?? true)
}
defineExpose({ flush, refresh })

watch(grid, element => { if (element) { observer?.observe(element); measure() } })
watchEffect(() => { if (!barFocused.value) bar.value = raw(selected.value.row, selected.value.col) ?? '' })
onMounted(() => {
  observer = new ResizeObserver(measure)
  window.addEventListener('beforeunload', beforeUnload)
  window.addEventListener('online', onOnline)
  window.addEventListener('focus', onFocus)
  void initialize()
})
onBeforeUnmount(() => {
  closed = true
  observer?.disconnect()
  queue?.stop()
  cache?.close()
  window.removeEventListener('beforeunload', beforeUnload)
  window.removeEventListener('online', onOnline)
  window.removeEventListener('focus', onFocus)
})
</script>

<template>
  <div class="data-grid">
    <p v-if="loading" role="status" class="table-message">正在加载表格…</p>
    <div v-else-if="!sheet" class="table-message" role="alert">{{ readError }} <button @click="initialize">重新加载</button></div>
    <template v-else>
      <div class="grid-toolbar">
        <button type="button" :disabled="!undoStack.length || !editable" @click="undo()">撤销</button>
        <button type="button" :disabled="!redoStack.length || !editable" @click="undo(true)">重做</button>
        <button type="button" :disabled="!editable || !state.dirty || state.running || state.error?.status === 409" @click="queue.retry()">立即保存</button>
        <button v-if="source.search" type="button" @click="searchDialog.open(false)">查找</button>
        <span class="grid-save-status" :class="{ warning: Boolean(state.error) }" role="status">{{ editable ? status : '只读' }}</span>
      </div>
      <div v-if="state.error || readError || editingError || storageError" class="grid-notices" role="alert">
        <span v-if="state.error">{{ state.error.status === 409 ? '检测到其他更新。你的编辑仍在当前标签页。' : state.error.message }}</span>
        <span v-if="readError">{{ readError }}</span><span v-if="editingError">{{ editingError }}</span>
        <span v-if="storageError">无法保存浏览器恢复草稿，请在离开前确认已同步。</span>
        <button v-if="state.error?.status === 409" type="button" @click="refresh(true)">在最新版重放我的改动</button>
        <button v-if="state.error" type="button" @click="refresh(false)">放弃本地改动并刷新</button>
        <button v-if="readError && !state.dirty" type="button" @click="refresh()">重新加载</button>
      </div>
      <div class="grid-formula">
        <input class="grid-address" aria-label="转到单元格" :value="address" @keydown.enter.prevent="jump" />
        <span class="grid-fx" aria-hidden="true">ƒx</span>
        <textarea v-model="bar" rows="1" aria-label="单元格内容" :readonly="!editable"
          @focus="barFocused = true" @blur="finishBar" @keydown.enter.exact.prevent="finishBar(); focusGrid()" />
      </div>
      <div ref="grid" role="grid" tabindex="0" class="grid-scroll" :aria-label="sheet.name"
        :aria-rowcount="sheet.rowCount + 1" :aria-colcount="sheet.columns.length + 1" :aria-activedescendant="activeID"
        @scroll.passive="measure" @keydown="key" @copy="copy" @paste="paste">
        <div class="grid-canvas" :style="canvas">
          <div class="grid-head" role="row" aria-rowindex="1">
            <div class="grid-corner" role="columnheader" aria-colindex="1">#</div>
            <div v-for="col in columns" :key="col" class="grid-column" role="columnheader" :aria-colindex="col + 2"
              :style="{ left: indexWidth + col * cellWidth + 'px' }" :title="sheet.columns[col].label">
              <span>{{ columnName(col) }}</span>{{ sheet.columns[col].label }}
            </div>
          </div>
          <div v-for="row in rows" :key="row" class="grid-row" role="row" :aria-rowindex="row + 2"
            :style="{ top: headerHeight + row * rowHeight + 'px' }">
            <div class="grid-index" role="rowheader" aria-colindex="1">{{ row + 1 }}</div>
            <div v-for="col in columns" :id="domID(row, col)" :key="col" class="grid-cell" role="gridcell"
              :aria-label="columnName(col) + (row + 1)" :aria-colindex="col + 2" :aria-selected="inRange(row, col)"
              :class="{ selected: inRange(row, col), active: row === selected.row && col === selected.col }"
              :style="{ left: indexWidth + col * cellWidth + 'px' }"
              @pointerdown="clickCell($event, row, col)" @pointerenter="$event.buttons === 1 && !editor && select(row, col, true, false)"
              @dblclick="beginEdit(row, col)">
              <textarea v-if="editor?.row === row && editor?.col === col" :ref="element => { if (element) editorElement = element }"
                v-model="editor.input" rows="1" :aria-label="'编辑 ' + address" @keydown.stop="editorKey" @blur="finishEdit()" />
              <template v-else>{{ display(row, col) }}</template>
            </div>
          </div>
        </div>
      </div>
      <div class="grid-footer"><span>{{ sheet.rowCount.toLocaleString() }} 行 · {{ sheet.columns.length }} 列</span>
        <span>双击编辑 · Shift 扩选 · 支持从 Excel 粘贴</span></div>
      <GridSearch v-if="source.search" ref="searchDialog" :search="source.search" :prepare="flush" :replace="replaceMatches"
        :can-edit="editable" @locate="locateMatch" />
    </template>
  </div>
</template>
