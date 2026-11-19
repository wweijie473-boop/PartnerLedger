import { COL_BLOCK, ROW_BLOCK, keyFor } from './cells.js'

export function createWindowCache({ source, sheet, onData, onRemote, onError }) {
  let revision = sheet.revision
  let latest = revision
  let stopped = false
  let active = 0
  const blocks = new Map()
  const tasks = new Map()
  const waiting = []
  const controller = new AbortController()

  function bounds(row, col) {
    const r = Math.floor(row / ROW_BLOCK) * ROW_BLOCK, c = Math.floor(col / COL_BLOCK) * COL_BLOCK
    return { row: r, col: c, height: Math.min(ROW_BLOCK, sheet.rowCount - r), width: Math.min(COL_BLOCK, sheet.columns.length - c) }
  }
  function fill() {
    while (!stopped && active < 6 && waiting.length) {
      const { id, area, resolve } = waiting.shift()
      active++
      source.getWindow(area, controller.signal).then(data => {
        if (stopped) return
        if (data.revision > revision) {
          if (data.revision > latest) { latest = data.revision; onRemote(latest) }
          return
        }
        if (data.revision < revision) return
        blocks.set(id, new Map(data.cells.map(cell => [keyFor(cell.row, cell.col), cell.input])))
        if (blocks.size > 48) blocks.delete(blocks.keys().next().value)
        onData()
      }).catch(error => {
        if (!stopped && error.name !== 'AbortError') onError(error)
      }).finally(() => {
        active--
        tasks.delete(id)
        resolve()
        fill()
      })
    }
  }
  function load(row, col) {
    if (stopped || latest > revision || row < 0 || col < 0 || row >= sheet.rowCount || col >= sheet.columns.length) return Promise.resolve()
    const area = bounds(row, col), id = keyFor(area.row, area.col)
    if (blocks.has(id)) return Promise.resolve()
    if (!tasks.has(id) && tasks.size < 64) {
      const promise = new Promise(resolve => waiting.push({ id, area, resolve }))
      tasks.set(id, promise)
      queueMicrotask(fill)
    }
    return tasks.get(id) ?? Promise.resolve()
  }
  function get(row, col) {
    const area = bounds(row, col), id = keyFor(area.row, area.col)
    const block = blocks.get(id)
    if (!block) return undefined
    return block.get(keyFor(row, col)) ?? ''
  }
  function confirm(edits, latest) {
    if (latest <= revision) return
    for (const cell of edits) {
      const area = bounds(cell.row, cell.col)
      const block = blocks.get(keyFor(area.row, area.col))
      if (block) {
        const key = keyFor(cell.row, cell.col)
        if (cell.input === '') block.delete(key)
        else block.set(key, cell.input)
      }
    }
    revision = latest
    onData()
  }
  function close() {
    stopped = true
    controller.abort()
    for (const entry of waiting) entry.resolve()
    waiting.length = 0
  }
  function invalidate() { blocks.clear(); onData() }
  return { load, get, confirm, close, invalidate }
}
