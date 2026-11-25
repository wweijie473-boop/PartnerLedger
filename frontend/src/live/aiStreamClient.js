import { ApiError } from '../api.js'
import { currentSession, saveSession } from '../auth-session.js'

export async function streamWorkspaceChat({
  messages,
  signal,
  onStatus,
  onRoute,
  onDelta,
  onDone,
  fetchImpl = globalThis.fetch,
}) {
  const active = currentSession()
  if (!active?.token || typeof fetchImpl !== 'function') {
    return { unsupported: true }
  }

  const deadline = AbortSignal.timeout(90000)
  const requestSignal = signal ? AbortSignal.any([signal, deadline]) : deadline

  let response
  try {
    response = await fetchImpl('/api/ai/workspace/stream', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${active.token}`,
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      },
      body: JSON.stringify({ messages }),
      signal: requestSignal,
      cache: 'no-store',
      redirect: 'error',
    })
  } catch (error) {
    if (signal?.aborted) throw error
    const message = deadline.aborted ? '请求超时，请确认结果后再重试。' : '无法连接后端，请确认 Go 服务已启动。'
    throw new ApiError(0, '', message)
  }

  if (response.status === 404) {
    return { unsupported: true }
  }
  if (!response.ok) {
    const envelope = await response.json().catch(() => null)
    if (response.status === 401) saveSession(null)
    const fallback = response.status >= 500 && !envelope ? '无法连接后端，请确认 Go 服务已启动。' : undefined
    throw new ApiError(response.status, envelope?.message, fallback)
  }
  if (!response.body) {
    return { unsupported: true }
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''
  let routed = null
  let streamedAny = false

  function handleFrame(rawFrame) {
    const lines = rawFrame.split('\n')
    let eventName = 'message'
    const dataLines = []
    for (const line of lines) {
      if (line.startsWith('event:')) {
        eventName = line.slice(6).trim()
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice(5).trimStart())
      }
    }
    if (!dataLines.length) return
    let payload
    try {
      payload = JSON.parse(dataLines.join('\n'))
    } catch {
      return
    }
    if (eventName === 'status' && payload?.label) {
      onStatus?.(payload.label)
    } else if (eventName === 'route' && payload?.intent) {
      routed = payload
      onRoute?.(payload)
    } else if (eventName === 'delta' && typeof payload?.text === 'string' && payload.text) {
      streamedAny = true
      onDelta?.(payload.text)
    } else if (eventName === 'done') {
      onDone?.({
        links: Array.isArray(payload?.links) ? payload.links : [],
        model: payload?.model || '',
      })
    } else if (eventName === 'error') {
      const code = payload?.code || 'model_response_error'
      const status = code === 'model_unavailable' ? 503 : 502
      throw new ApiError(status, code)
    }
  }

  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
      let boundary = buffer.indexOf('\n\n')
      while (boundary !== -1) {
        const frame = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)
        if (frame.trim()) handleFrame(frame)
        boundary = buffer.indexOf('\n\n')
      }
    }
    buffer += decoder.decode().replace(/\r\n/g, '\n')
    if (buffer.trim()) handleFrame(buffer)
  } catch (error) {
    if (error instanceof ApiError || signal?.aborted) throw error
    const message = deadline.aborted ? '流式响应超时，请稍后重试。' : '网络连接中断，请稍后重试。'
    throw new ApiError(0, '', message)
  } finally {
    reader.releaseLock()
  }

  return { unsupported: false, routed, streamedAny }
}
