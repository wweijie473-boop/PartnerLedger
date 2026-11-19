import { keyFor, MAX_CHANGES } from './cells.js'

export function createSaveQueue({ revision, save, recover, onChange, onSaved, delay = 450 }) {
  let version = revision, observed = revision
  let batch = recover?.batch ?? null
  let groups = recover?.groups ?? []
  let timer, running, stopped = false, attempts = 0
  let error = null
  if (batch || groups.length) version = recover.revision

  function dirty() { return Boolean(batch) || groups.length > 0 }
  function conflict() {
    return Object.assign(new Error('此表格已有更新，本次编辑保存在当前标签页。'), { status: 409 })
  }
  if (!batch && dirty() && version !== observed) error = conflict()

  function state() {
    const overlay = new Map()
    for (const edits of [...(batch ? [batch.cells] : []), ...groups]) {
      for (const entry of edits) overlay.set(keyFor(entry.row, entry.col), entry.input)
    }
    return {
      dirty: dirty(), running: Boolean(running), error, revision: version, observed, overlay,
      recovery: dirty() ? { revision: version, batch, groups } : null,
    }
  }
  function notify() { if (!stopped) onChange(state()) }
  function schedule(ms = delay) {
    clearTimeout(timer)
    if (!stopped) timer = setTimeout(() => { void flush() }, ms)
  }
  function add(cells) {
    if (!cells.length) return
    const size = (batch?.cells.length ?? 0) + groups.reduce((sum, g) => sum + g.length, 0)
    if (size + cells.length > 1000 || cells.length > MAX_CHANGES) throw new Error('待保存的改动较多，请先保存。')
    groups = [...groups, cells]
    notify()
    if (!error) schedule()
  }
  function takeGroup() {
    const merged = new Map()
    let count = 0
    while (count < groups.length && merged.size + groups[count].length <= MAX_CHANGES) {
      for (const cell of groups[count++]) merged.set(keyFor(cell.row, cell.col), cell)
    }
    groups = groups.slice(count)
    return [...merged.values()]
  }
  async function flush() {
    clearTimeout(timer)
    if (running) return running
    if (error || stopped || !dirty()) return !dirty()
    running = (async () => {
      while (!stopped && dirty() && !error) {
        batch ??= { requestId: crypto.randomUUID(), revision: version, cells: takeGroup() }
        notify()
        try {
          const result = await save(batch)
          const saved = batch
          version = result.revision
          batch = null
          attempts = 0
          if (!stopped) onSaved(saved.cells, version)
          if (observed > version && groups.length) error = conflict()
        } catch (failure) {
          error = failure
          if ((failure.status === 0 || failure.status >= 500) && attempts < 3) {
            attempts++
            clearTimeout(timer)
            timer = setTimeout(() => {
              error = null
              void flush()
            }, 800 * 2 ** (attempts - 1))
          }
        }
      }
      return !dirty()
    })()
    notify()
    try { return await running } finally { running = null; notify() }
  }
  function observe(latest) {
    observed = Math.max(observed, latest)
    if (!dirty()) version = observed
    else if (!batch && observed > version) error = conflict()
    notify()
  }
  function reconcile(latest, keep) {
    if (running) return false
    clearTimeout(timer)
    groups = keep ? [...(batch ? [batch.cells] : []), ...groups] : []
    batch = null
    error = null
    version = observed = latest
    notify()
    if (dirty()) schedule(0)
    return true
  }
  function retry() {
    if (error?.status === 409) return false
    error = null
    attempts = 0
    return flush()
  }
  function stop() { stopped = true; clearTimeout(timer) }

  if (dirty() && !error) schedule(0)
  return { add, flush, retry, observe, reconcile, state, stop }
}
