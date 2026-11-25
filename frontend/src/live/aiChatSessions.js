const MAX_SESSIONS = 20
const MAX_TURNS_PER_SESSION = 60

function makeSessionId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return 'sess-' + Date.now() + '-' + Math.random().toString(36).slice(2, 8)
}

function sessionTitleFromTurns(turns) {
  const firstUser = turns.find(item => item.role === 'user' && item.text?.trim())
  if (!firstUser) return '新对话'
  const clean = firstUser.text.trim().replace(/\s+/g, ' ')
  return [...clean].length > 18 ? [...clean].slice(0, 18).join('') + '…' : clean
}

export function serializeTurn(turn) {
  const copy = {
    id: turn.id,
    role: turn.role,
    text: String(turn.text ?? ''),
  }
  for (const key of [
    'stepsLog', 'matches', 'draft', 'product', 'project',
    'opportunityLink', 'navAction', 'chatLinks',
    'customersCard', 'operationsCard', 'projectsList',
    'manualSource', 'fallbackIntent',
  ]) {
    if (turn[key] !== undefined) {
      copy[key] = JSON.parse(JSON.stringify(turn[key]))
    }
  }
  if (turn.approval) {
    copy.approval = { title: turn.approval.title, archived: true }
  }
  return copy
}

export function clearDemoAiSessions(storage = globalThis.window?.localStorage) {
  try {
    storage?.removeItem('partnerledger-ai-sessions-v1:demo')
  } catch {}
}

export function createAiSessionStore(scopeKey, storage = globalThis.window?.localStorage) {
  const storageKey = 'partnerledger-ai-sessions-v1:' + scopeKey

  function readStore() {
    if (!storage) return null
    try {
      const raw = storage.getItem(storageKey)
      if (!raw) return null
      const parsed = JSON.parse(raw)
      if (!parsed || !Array.isArray(parsed.sessions) || !parsed.sessions.length) return null
      return parsed
    } catch {
      return null
    }
  }

  function writeStore(data) {
    if (!storage) return
    try {
      storage.setItem(storageKey, JSON.stringify({
        activeId: data.activeId,
        sessions: data.sessions.slice(0, MAX_SESSIONS),
      }))
    } catch {}
  }

  function makeEmptySession(selectedProjectId = '') {
    return {
      id: makeSessionId(),
      title: '新对话',
      updatedAt: new Date().toISOString(),
      selectedProjectId,
      turns: [],
    }
  }

  function loadInitial(initialProject = '') {
    const saved = readStore()
    if (!saved) {
      const fresh = makeEmptySession(initialProject)
      const state = { activeId: fresh.id, sessions: [fresh] }
      writeStore(state)
      return state
    }
    // If opened with an explicit deep-linked project (e.g. "用 AI 整理需求"), start or select a session for it if current has unrelated turns
    if (initialProject) {
      const active = saved.sessions.find(s => s.id === saved.activeId) ?? saved.sessions[0]
      if (active.turns.length > 0 && active.selectedProjectId !== initialProject) {
        const existingForProject = saved.sessions.find(s => s.selectedProjectId === initialProject && s.turns.length === 0)
        if (existingForProject) {
          saved.activeId = existingForProject.id
        } else {
          const fresh = makeEmptySession(initialProject)
          saved.sessions.unshift(fresh)
          saved.activeId = fresh.id
        }
        writeStore(saved)
        return saved
      }
    }
    if (!saved.sessions.some(s => s.id === saved.activeId)) {
      saved.activeId = saved.sessions[0].id
    }
    return saved
  }

  return {
    makeEmptySession,
    loadInitial,
    saveSessions(activeId, sessions) {
      writeStore({ activeId, sessions })
    },
    buildUpdatedSession(session, turns, selectedProjectId) {
      const serialized = turns.slice(-MAX_TURNS_PER_SESSION).map(serializeTurn)
      return {
        ...session,
        title: sessionTitleFromTurns(serialized),
        updatedAt: new Date().toISOString(),
        selectedProjectId: selectedProjectId || '',
        turns: serialized,
      }
    },
  }
}
