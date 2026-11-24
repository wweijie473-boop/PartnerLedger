export const retailApi = '/api/retail/v1'

export function customerCheckoutLink(code, product) {
  return '/login/customer?' + new URLSearchParams({ view: 'checkout', code, product })
}

export function customerOrderLink(orderID) {
  return '/login/customer?' + new URLSearchParams({ view: 'retail-orders', order_id: orderID })
}

export async function readRetailCatalog(read, path, cursorParam = 'after') {
  const items = []
  const cursors = new Set()
  let cursor = ''
  do {
    const page = await read(path + (cursor ? (path.includes('?') ? '&' : '?') + cursorParam + '=' + encodeURIComponent(cursor) : ''))
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

  const id = retailRequestId()
  sessionStorage.setItem(key, JSON.stringify({ payload, id }))
  return id
}

function retailRequestId() {
  // getRandomValues also works when the demo is reached over plain HTTP on a phone.
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
  return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-')
}

export const retailCheckoutStorageKey = key => key + ':pending'

function parseRetailCheckout(record) {
  if (!/^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/.test(record?.id ?? '')) return null
  try {
    const body = JSON.parse(record.payload)
    if (!body || typeof body !== 'object' || Array.isArray(body) || typeof body.product_id !== 'string' ||
        !/^\d+$/.test(body.product_id) || !Number.isSafeInteger(body.quantity) || !Number.isSafeInteger(body.expected_unit_cents)) return null
    return { ...body, request_id: record.id }
  } catch { return null }
}

export function readRetailCheckout(key) {
  let saved
  try { saved = JSON.parse(localStorage.getItem(retailCheckoutStorageKey(key))) } catch {}
  return parseRetailCheckout(saved) ?? parseRetailCheckout(readPendingRetailRequest(key))
}

export function saveRetailCheckout(key, draft, offer) {
  const prior = readRetailCheckout(key)
  if (prior) return prior
  const body = offer.annual === undefined ? draft : { ...draft, expected_annual: offer.annual }
  const id = retailRequestId()
  try {
    localStorage.setItem(retailCheckoutStorageKey(key), JSON.stringify({ id, payload: JSON.stringify(body) }))
  } catch {
    throw new Error('浏览器无法保存待核对订单，请允许此站点保存数据后再下单。')
  }
  return { ...body, request_id: id }
}

export function clearRetailCheckout(key, id) {
  try {
    const stored = JSON.parse(localStorage.getItem(retailCheckoutStorageKey(key)))
    if (stored?.id === id) localStorage.removeItem(retailCheckoutStorageKey(key))
  } catch {}
  try {
    if (readPendingRetailRequest(key)?.id === id) sessionStorage.removeItem(key)
  } catch {}
  return readRetailCheckout(key)
}

export function withRetailCheckoutLock(key, action) {
  if (!navigator.locks) return action()
  return navigator.locks.request(key, { ifAvailable: true }, lock => {
    if (!lock) throw new Error('此订单正在另一个标签页核对。')
    return action()
  })
}
