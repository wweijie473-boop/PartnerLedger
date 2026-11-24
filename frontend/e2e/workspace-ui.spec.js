import { expect, test } from '@playwright/test'

const rolePages = {
  customer: { title: '企业客户工作台', group: '采购项目', view: 'ledger', page: '我的采购项目' },
  dealer: { title: '代理商工作台', group: '客户经营', view: 'visits', page: '记拜访', neighbor: '商机拾遗' },
  vendor: { title: '软件厂商工作台', group: '产品管理', view: 'catalog', page: '软件目录' },
}

for (const width of [1440, 390]) {
  test(`all three workspaces keep the current area and full directory accessible at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 850 })
    for (const [role, target] of Object.entries(rolePages)) {
      await page.goto('/experience/' + role)
      await expect(page.getByRole('heading', { name: target.title, exact: true })).toBeVisible()
      await expect(page.getByRole('region', { name: '项目待办' })).toBeInViewport()
      await expect(page.getByText('本地演示空间', { exact: true })).toBeVisible()
      const trigger = page.getByRole('button', { name: '全部功能' })
      const directory = page.getByRole('navigation', { name: '全部工作台功能' })
      await trigger.press('Enter')
      await expect(directory).toBeVisible()
      await expect(directory.getByRole('button', { name: target.page, exact: true })).toBeVisible()
      await page.keyboard.press('Escape')
      await expect(trigger).toBeFocused()
      await expect(directory).toBeHidden()
      await trigger.click()
      await directory.getByRole('button', { name: target.page, exact: true }).click()
      await expect(page).toHaveURL(new RegExp('[?&]view=' + target.view + '(?:&|$)'))
      await expect(page.getByRole('navigation', { name: '工作台栏目' }).getByRole('button', { name: target.group, exact: true })).toBeVisible()
      if (target.neighbor) {
        const group = page.getByRole('navigation', { name: '当前栏目功能' })
        if (width === 390) {
          await group.getByRole('combobox', { name: '栏目功能' }).selectOption({ label: target.neighbor })
          await expect(page).toHaveURL(/view=opportunities$/)
        } else {
          await expect(group.getByRole('button', { name: target.neighbor })).toBeVisible()
        }
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.body.clientWidth)).toBe(0)
    }
    await page.goto('/experience/retail?role=vendor&view=vendor-tickets')
    await expect(page).toHaveURL(/\/experience\/vendor\?view=vendor-tickets$/)
    await expect(page.getByRole('heading', { name: '渠道工单', exact: true })).toBeVisible()
  })
}

test('mobile transactions and tickets show state, preserve fields, and complete vendor actions', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 850 })
  const actions = []
  const order = {
    id: '41', product_name: '示例财务软件', product_version: '2026', quantity: 2, customer: '示例客户', reseller: '示例代理商',
    source: 'promotion', total_cents: 300000, commission: { cents: 60000, rate_bps: 2000 }, status: 'settlement',
    paid_at: '2026-11-20T02:00:00Z',
  }
  const operations = {
    topups: [{ id: '52', status: 'pending', amount_cents: 250000, reference: '测试预存凭证', created_at: '2026-11-20T02:00:00Z' }],
    orders: [order], invoices: [], claims: [], payouts: [],
    tickets: [{
      id: '61', subject: '授权交付确认', category: '授权与交付', order_id: '41', detail: '请确认授权交付方式。',
      created_at: '2026-11-20T02:00:00Z', status: 'open', replies: [],
    }],
  }
  await page.addInitScript(() => {
    localStorage.setItem('partnerledger-retail-demo-session-v1', JSON.stringify({ dealer: 'test-dealer', vendor: 'test-vendor' }))
  })
  await page.route('**/api/retail-demo/**', async route => {
    const request = route.request()
    const endpoint = new URL(request.url()).pathname
    if (request.method() === 'POST') {
      actions.push({ endpoint, body: request.postDataJSON() })
      if (endpoint.endsWith('/settle')) order.status = 'settled'
      if (endpoint.endsWith('/resolve')) operations.tickets[0].status = 'resolved'
    }
    const data = endpoint.endsWith('/orders') ? {
      items: [order], next_cursor: null, summary: { settlement_cents: 60000, settled_cents: 0, reminded: 0 },
    } : endpoint.endsWith('/operations') ? operations : {}
    await route.fulfill({ json: { code: 200, message: 'success', data } })
  })
  await page.goto('/experience/vendor?view=manufacturer-settlements')
  const orderList = page.getByRole('region', { name: '佣金明细' })
  await expect(orderList.locator('.ui-badge')).toHaveText('待厂商结算')
  await orderList.getByRole('button', { name: '登记结算' }).click()
  await orderList.getByLabel('转账凭证编号').fill('示例结算号')
  await orderList.getByRole('button', { name: '确认登记' }).click()
  await expect(orderList.locator('.ui-badge')).toHaveText('已登记结算')
  await page.goto('/experience/vendor?view=vendor-finance')
  await expect(page.getByRole('cell', { name: '测试预存凭证', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '确认入账' })).toBeVisible()
  await page.goto('/experience/vendor?view=vendor-tickets')
  const ticket = page.locator('summary').filter({ hasText: '授权交付确认' })
  await expect(ticket).toContainText('待厂商处理')
  await ticket.click()
  await page.getByRole('button', { name: '填写处理结果' }).click()
  await page.getByLabel('处理结果').fill('已核对，按订单交付。')
  await page.getByRole('button', { name: '提交处理结果' }).click()
  await expect(ticket).toContainText('待代理商确认')
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.body.clientWidth)).toBe(0)
  expect(actions).toEqual([
    { endpoint: '/api/retail-demo/orders/41/settle', body: { reference: '示例结算号' } },
    { endpoint: '/api/retail-demo/operations/tickets/61/resolve', body: { reply: '已核对，按订单交付。' } },
  ])
})

test('a customer checkout deep link resolves inside its section without becoming a general menu entry', async ({ page }) => {
  const code = 'a'.repeat(32)
  const organization = { id: '71', name: '示例客户', kind: 'CUSTOMER' }
  await page.addInitScript(() => {
    sessionStorage.setItem('partnerledger-api-session', JSON.stringify({ token: 'demo-session', expiresAt: Date.now() + 60_000 }))
  })
  await page.route('**/api/**', async route => {
    const endpoint = new URL(route.request().url()).pathname
    const data = endpoint === '/api/user/profile' ? { id: '72', username: 'customer_demo', organization }
      : endpoint === '/api/org/current' ? organization
        : {
            items: [{ product_id: '51', manufacturer: '示例厂商', name: '示例软件', version: '2026', unit_cents: 12000 }],
            next_cursor: null,
          }
    await route.fulfill({ json: { code: 200, message: 'success', data } })
  })
  await page.goto('/login/customer?view=checkout&product=51&code=' + code)
  await expect(page.getByRole('heading', { name: '确认厂商订单' })).toBeVisible()
  await expect(page.getByLabel('订购产品')).toHaveValue('51')
  await page.getByRole('button', { name: '全部功能' }).click()
  const directory = page.getByRole('navigation', { name: '全部工作台功能' })
  await expect(directory.getByRole('button', { name: '确认厂商订单' })).toHaveCount(0)
  await directory.getByRole('button', { name: '厂商订购订单' }).click()
  await expect(page).toHaveURL(/view=retail-orders$/)
})
