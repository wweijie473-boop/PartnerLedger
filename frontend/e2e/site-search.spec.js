import { expect, test } from '@playwright/test'

test('visitors find a formal workspace shortcut without sending a business search', async ({ page }) => {
  const queries = []
  await page.route('**/api/search/v1/records?*', async route => {
    queries.push(route.request().url())
    await route.fulfill({ status: 401, json: { code: 401, message: 'authentication_required', data: null } })
  })
  await page.goto('/')
  await page.getByRole('searchbox', { name: '搜索站内功能' }).fill('资金总览')
  const panel = page.getByRole('search', { name: '站内搜索' })
  await expect(panel.getByRole('link', { name: /资金总览/ })).toHaveAttribute('href', '/login/dealer?view=funds-overview')
  await panel.getByRole('link', { name: /资金总览/ }).click()
  await expect(page).toHaveURL(/\/login\/dealer\?view=funds-overview$/)
  expect(queries).toEqual([])
})

test('a signed-in manufacturer can open a matching order directly from the header', async ({ page }) => {
  const organization = { id: '81', name: '示例厂商', kind: 'MANUFACTURER' }
  const order = { id: '41', product_name: '财务授权', product_version: '年度版', quantity: 1,
    customer: '示例客户', reseller: '示例代理商', source: 'assisted', total_cents: 10000, status: 'payment' }
  const queries = []
  await page.addInitScript(() => {
    sessionStorage.setItem('partnerledger-api-session', JSON.stringify({ token: 'search-session', expiresAt: Date.now() + 60_000 }))
  })
  await page.route('**/api/**', async route => {
    const request = route.request()
    const url = new URL(request.url())
    let data = {}
    if (url.pathname === '/api/user/profile') data = { id: '82', username: 'vendor_demo', organization }
    if (url.pathname === '/api/org/current') data = organization
    if (url.pathname === '/api/search/v1/records') {
      queries.push([url.searchParams.get('q'), request.headers().authorization])
      data = { items: [{ kind: 'order', label: '直购订单 #41', description: '财务授权 · 年度版',
        href: '/login/vendor?order_id=41&view=manufacturer-receipts' }] }
    }
    if (url.pathname === '/api/retail/v1/orders') {
      expect(url.searchParams.get('order_id')).toBe('41')
      data = { items: [order], next_cursor: null }
    }
    await route.fulfill({ json: { code: 200, message: 'success', data } })
  })
  await page.goto('/')
  await expect(page.getByRole('link', { name: /vendor_demo/ })).toBeVisible()
  const search = page.getByRole('searchbox', { name: '搜索站内功能' })
  await search.fill('４１')
  await search.press('Enter')
  await expect(page).toHaveURL(/\/login\/vendor\?order_id=41&view=manufacturer-receipts$/)
  await expect(page.getByRole('heading', { name: '直收订单与收款' })).toBeVisible()
  await expect(page.getByText('财务授权', { exact: true })).toBeVisible()
  expect(queries).toEqual([['41', 'Bearer search-session']])
})
