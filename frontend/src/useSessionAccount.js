import { onMounted, onUnmounted, ref } from 'vue'
import { requestApi } from './api.js'
import { currentSession, saveSession } from './auth-session.js'
import { organizationRole } from './live/entrances.js'

export function useSessionAccount() {
  const signedIn = ref(Boolean(currentSession()))
  const account = ref(null)
  let request
  let expiryTimer

  async function refresh() {
    request?.abort()
    clearTimeout(expiryTimer)
    const active = currentSession()
    signedIn.value = Boolean(active)
    account.value = null
    if (!active) return

    expiryTimer = setTimeout(refresh, active.expiresAt - Date.now())
    const controller = new AbortController()
    request = controller
    try {
      const profile = await requestApi('/api/user/profile', { token: active.token, signal: controller.signal })
      if (!controller.signal.aborted && currentSession() === active && organizationRole(profile, profile?.organization)) {
        account.value = profile
      }
    } catch (error) {
      if (!controller.signal.aborted && error.status === 401 && currentSession() === active) {
        saveSession(null)
        signedIn.value = false
        clearTimeout(expiryTimer)
      }
    }
  }

  function restore(event) {
    if (event.persisted) refresh()
  }

  onMounted(() => {
    refresh()
    window.addEventListener('pageshow', restore)
  })
  onUnmounted(() => {
    request?.abort()
    clearTimeout(expiryTimer)
    window.removeEventListener('pageshow', restore)
  })

  return { account, signedIn }
}
