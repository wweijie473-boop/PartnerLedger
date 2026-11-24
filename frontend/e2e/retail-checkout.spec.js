import { expect, test } from '@playwright/test'

for (const outcome of ['committed', 'replay', 'conflict']) {
  test(`checkout keeps the original request until an order is recovered (${outcome})`, async ({ page }) => {
    const code = 'a'.repeat(32)
    const organization = { id: '71', name: '示例客户', kind: 'CUSTOMER' }
    const order = { id: '81', product_name: '示例软件', product_version: 'v1', quantity: 1, manufacturer: '示例厂商',
      source: 'promotion', total_cents: 12000, status: 'payment' }
    const requests = []
    const lookups = []
    await page.addInitScript(useFallback => {
      sessionStorage.setItem('partnerledger-api-session', JSON.stringify({ token: 'test-session', expiresAt: Date.now() + 60_000 }))
      if (useFallback) Object.defineProperty(navigator, 'locks', { value: undefined })
    }, outcome === 'replay')
    await page.route('**/api/**', async route => {
      const request = route.request()
      const endpoint = new URL(request.url()).pathname
      let data = { items: [], next_cursor: null }
      if (endpoint === '/api/user/profile') data = { id: '72', username: 'customer_demo', organization }
      if (endpoint === '/api/org/current') data = organization
      if (endpoint === '/api/retail/v1/store/' + code && !requests.length) {
        data.items = [{ product_id: '51', manufacturer: order.manufacturer, name: order.product_name, version: order.product_version,
          unit_cents: 12000, annual: true }]
      }
      if (endpoint.startsWith('/api/retail/v1/orders/requests/')) {
        lookups.push(endpoint.split('/').at(-1))
        if (outcome === 'replay') return route.fulfill({ status: 404, json: { code: 404, message: 'not_found', data: null } })
        data = { id: order.id }
      }
      if (endpoint === '/api/retail/v1/orders') {
        if (request.method() === 'POST') {
          requests.push(request.postDataJSON())
          if (requests.length === 1) {
            if (outcome === 'conflict') return route.fulfill({ status: 409, json: { code: 409, message: 'retail_conflict', data: null } })
            return route.abort()
          }
          data = { id: order.id }
        } else {
          data.items = [order]
        }
      }
      await route.fulfill({ json: { code: 200, message: 'success', data } })
    })
    await page.goto('/login/customer?view=checkout&product=51&code=' + code)
    await page.getByRole('button', { name: '确认并下单' }).click()
    const recovery = page.getByRole('button', { name: '核对并重试原请求' })
    if (outcome !== 'conflict') {
      await expect(recovery).toBeEnabled()
      await page.reload()
      await expect(page.getByRole('button', { name: '确认并下单' })).toHaveCount(0)
      await expect(page.getByText(/原订单商品编号 51/)).toBeVisible()
      await recovery.click()
    }
    await expect(page).toHaveURL(/view=retail-orders&order_id=81$/)
    await expect(page.getByText(/订单 81/)).toBeVisible()
    await expect(recovery).toHaveCount(0)
    expect(lookups).toEqual([requests[0].request_id])
    expect(requests).toHaveLength(outcome === 'replay' ? 2 : 1)
    expect(requests[0]).toMatchObject({ product_id: '51', code, quantity: 1, expected_unit_cents: 12000, expected_annual: true })
    if (outcome === 'replay') expect(requests[1]).toEqual(requests[0])
  })
}
