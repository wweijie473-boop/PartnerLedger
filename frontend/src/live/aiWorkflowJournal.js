import { shallowReactive } from 'vue'

function makeRequestId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
  return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-')
}

function validSavedJob(saved) {
  return Boolean(
    saved &&
    typeof saved.skillId === 'string' &&
    Array.isArray(saved.steps) &&
    saved.steps.length > 0 &&
    Number.isInteger(saved.currentIndex) &&
    saved.currentIndex >= 0 &&
    saved.currentIndex < saved.steps.length &&
    saved.steps.every(step => step && typeof step.kind === 'string' && typeof step.requestId === 'string'),
  )
}

export function createWorkflowJournal(key, handlers, storage) {
  const state = shallowReactive({ job: null, busy: false, error: '' })
  const persist = Boolean(key)

  if (persist) {
    try {
      storage ??= globalThis.window?.sessionStorage
      const raw = storage?.getItem(key)
      if (raw) {
        const saved = JSON.parse(raw)
        if (validSavedJob(saved)) state.job = saved
      }
    } catch {
      state.error = '上次自动执行进度无法读取，请先核对服务端业务记录。'
    }
  }

  function saveJob(job) {
    state.job = job
    if (!persist || !storage) return
    if (job) storage.setItem(key, JSON.stringify(job))
    else storage.removeItem(key)
  }

  async function executeCurrent(onProgress) {
    if (state.busy || !state.job) return null
    state.busy = true
    state.error = ''
    const job = state.job
    try {
      while (job.currentIndex < job.steps.length) {
        const index = job.currentIndex
        const step = job.steps[index]
        const handler = handlers[step.kind]
        if (typeof handler !== 'function') {
          throw Object.assign(new Error(`未知执行步骤：${step.kind}`), { status: 400 })
        }
        onProgress?.({ index, total: job.steps.length, label: step.label, status: 'running', completed: [...job.completed] })
        const output = await handler({
          input: step.input,
          context: job.context,
          requestId: step.requestId,
        })
        job.context[step.id || `step_${index}`] = output
        job.completed.push({ id: step.id, label: step.label })
        job.currentIndex = index + 1
        if (job.currentIndex < job.steps.length) saveJob(job)
        onProgress?.({ index, total: job.steps.length, label: step.label, status: 'done', output, completed: [...job.completed] })
      }
      const summary = { skillId: job.skillId, meta: job.meta, context: job.context, completed: [...job.completed] }
      saveJob(null)
      return summary
    } catch (error) {
      error.completedSteps = [...(job.completed || [])]
      if (persist && !(error.status >= 400 && error.status < 500 && error.status !== 401)) {
        saveJob(job)
        state.error = `步骤「${job.steps[job.currentIndex]?.label || '当前操作'}」结果待确认，请用原请求继续执行。`
      } else {
        saveJob(null)
        state.error = error.message || '执行未完成，请核对最新记录后重试。'
      }
      throw error
    } finally {
      state.busy = false
    }
  }

  return {
    state,
    makeRequestId,
    retry(onProgress) {
      return executeCurrent(onProgress)
    },
    discard() {
      if (state.busy) return
      saveJob(null)
      state.error = ''
    },
    async run(skillId, steps, meta = {}, onProgress) {
      if (state.job || state.busy) throw new Error('请先确认上一组未完成工作流的结果。')
      const normalized = steps.map((step, index) => ({
        id: step.id || `step_${index}`,
        label: step.label,
        kind: step.kind,
        input: step.input ? JSON.parse(JSON.stringify(step.input)) : {},
        requestId: step.requestId || makeRequestId(),
      }))
      const job = {
        id: makeRequestId(),
        skillId,
        meta: JSON.parse(JSON.stringify(meta)),
        currentIndex: 0,
        completed: [],
        context: {},
        steps: normalized,
      }
      saveJob(job)
      return executeCurrent(onProgress)
    },
  }
}
