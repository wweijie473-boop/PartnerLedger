const storageKey = 'partnerledger-api-session'

function readStoredSession() {
  try {
    const stored = JSON.parse(sessionStorage.getItem(storageKey))
    return typeof stored?.token === 'string' && stored.token && Number.isFinite(stored.expiresAt) ? stored : null
  } catch {
    return undefined
  }
}

let session = readStoredSession() ?? null

export function currentSession() {
  if (session && session.expiresAt <= Date.now()) saveSession(null)
  return session
}

export function saveSession(value) {
  session = value
  try {
    if (value) sessionStorage.setItem(storageKey, JSON.stringify(value))
    else sessionStorage.removeItem(storageKey)
  } catch { /* Keep the current page usable when browser storage is unavailable. */ }
}

export function sessionFromCredentials(credentials) {
  if (typeof credentials?.access_token !== 'string' || !credentials.access_token ||
      credentials.token_type !== 'Bearer' || !Number.isFinite(credentials.expires_in) || credentials.expires_in <= 0) {
    throw new Error('后端未返回可用的登录会话。')
  }
  return { token: credentials.access_token, expiresAt: Date.now() + credentials.expires_in * 1000 }
}

export function reloadOnSessionChange(event) {
  if (!event.persisted) return
  const stored = readStoredSession()
  if (stored === undefined) return
  if (stored?.token !== session?.token || stored?.expiresAt !== session?.expiresAt || stored?.expiresAt <= Date.now()) {
    session = stored
    location.reload()
  }
}

export const accountPath = '/login?view=account'
