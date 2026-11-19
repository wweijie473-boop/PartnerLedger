import { shallowReactive } from 'vue'

export function createWriteJournal(key, execute, storage = window.sessionStorage) {
  const state = shallowReactive({ job: null, busy: false, error: '' })
  try {
    const saved = JSON.parse(storage.getItem(key))
    if (saved?.requestId && saved?.kind && saved?.input) state.job = saved
  } catch {
    state.error = '上次提交记录无法读取，请先核对服务端记录。'
  }
  async function retry() {
    if (state.busy || !state.job) return
    state.busy = true
    state.error = ''
    try {
      const result = await execute(state.job)
      storage.removeItem(key)
      state.job = null
      return result
    } catch (error) {
      if (error.status >= 400 && error.status < 500) {
        storage.removeItem(key)
        state.job = null
        state.error = error.message
      } else {
        state.error = '提交结果待确认。请检查最新记录，再用原请求确认结果。'
      }
      throw error
    } finally {
      state.busy = false
    }
  }
  return {
    state, retry,
    async run(kind, input) {
      if (state.job || state.busy) throw new Error('请先确认上一笔提交的结果。')
      const job = { kind, input: JSON.parse(JSON.stringify(input)), requestId: crypto.randomUUID() }
      storage.setItem(key, JSON.stringify(job))
      state.job = job
      return retry()
    },
  }
}
