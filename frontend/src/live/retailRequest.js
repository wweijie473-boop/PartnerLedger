export const retailApi = '/api/retail/v1'

export function customerCheckoutLink(code, product) {
  return '/login/customer?' + new URLSearchParams({ view: 'checkout', code, product })
}

export function customerOrderLink(orderID) {
  return '/login/customer?' + new URLSearchParams({ view: 'retail-orders', order_id: orderID })
}

export async function readRetailCatalog(read, path) {
  const items = []
  const cursors = new Set()
  let cursor = ''
  do {
    const page = await read(path + (cursor ? (path.includes('?') ? '&' : '?') + 'after=' + encodeURIComponent(cursor) : ''))
    items.push(...page.items)
    cursor = page.next_cursor
    if (cursor && cursors.has(cursor)) throw new Error('列表分页无效，请刷新。')
    cursors.add(cursor)
  } while (cursor)
  return items
}

function readPendingRetailRequest(key) {
  let pending
  try { pending = JSON.parse(sessionStorage.getItem(key)) } catch {}
  return pending
}

export function pendingRetailOrder(key, draft, offer) {
  const previous = readPendingRetailRequest(key)
  const preserveLegacyRetry = previous?.payload === JSON.stringify(draft)
  const body = offer.annual === undefined || preserveLegacyRetry ? draft : { ...draft, expected_annual: offer.annual }
  return { ...body, request_id: pendingRetailRequest(key, body) }
}

export function pendingRetailRequest(key, body) {
  const payload = JSON.stringify(body)
  const pending = readPendingRetailRequest(key)
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
