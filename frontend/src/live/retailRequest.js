export function pendingRetailRequest(key, body) {
  const payload = JSON.stringify(body)
  let pending
  try { pending = JSON.parse(sessionStorage.getItem(key)) } catch {}
  if (pending?.payload === payload) return pending.id

  // getRandomValues also works when the demo is reached over plain HTTP on a phone.
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
  const id = [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-')
  sessionStorage.setItem(key, JSON.stringify({ payload, id }))
  return id
}
