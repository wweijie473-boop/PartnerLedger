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

test('the default dev server opens the complete dealer and vendor demo', async ({ page }) => {
  await page.goto('/experience/dealer')
  const trigger = page.getByRole('button', { name: '全部功能' })
  const directory = page.getByRole('navigation', { name: '全部工作台功能' })
  await trigger.click()
  for (const name of ['经营看板', '客户档案', '续费跟进', '推广获客二维码', '厂商直收订单', '代客下单', '资金总览', '服务工单']) {
    await expect(directory.getByRole('button', { name, exact: true })).toBeVisible()
  }
  await directory.getByRole('button', { name: '经营看板', exact: true }).click()
  await page.getByRole('button', { name: '开启演示' }).click()
  await expect(page.getByRole('heading', { name: '经营看板' })).toBeVisible()
  await expect(page.getByText('未关闭工单', { exact: true })).toBeVisible()

  await page.goto('/experience/vendor')
  await trigger.click()
  for (const name of ['直收订单与收款', '代理商佣金结算', '财务办理', '渠道工单']) {
    await expect(directory.getByRole('button', { name, exact: true })).toBeVisible()
  }
  await directory.getByRole('button', { name: '渠道工单', exact: true }).click()
  await expect(page.getByRole('heading', { name: '渠道工单', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '开启演示' })).toHaveCount(0)
})

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

test('the live manufacturer saves allowances and resolves tickets through Go API paths', async ({ page }) => {
  const organization = { id: '81', name: '示例厂商', kind: 'MANUFACTURER' }
  const peer = { id: '82', name: '示例代理商', limit_cents: '0', pending_cents: '0', paid_cents: '0' }
  const ticket = { id: '83', reseller_id: peer.id, subject: '补充授权说明', category: '授权与交付', detail: '请确认。',
    created_at: '2026-11-20T02:00:00Z', status: 'open', revision: 1, replies: [] }
  const actions = []
  const ops = '/api/retail/v1/operations'
  await page.addInitScript(() => {
    sessionStorage.setItem('partnerledger-api-session', JSON.stringify({ token: 'test-session', expiresAt: Date.now() + 60_000 }))
  })
  await page.route('**/api/**', async route => {
    const req = route.request()
    const endpoint = new URL(req.url()).pathname
    const method = req.method()
    if (method !== 'GET') {
      const body = req.postDataJSON()
      actions.push({ method, endpoint, body })
      if (endpoint === ops + '/allowances/82') peer.limit_cents = String(body.limit_cents)
      if (endpoint === ops + '/tickets/83/resolve') {
        ticket.status = 'resolved'
        ticket.revision += 1
      }
    }
    let data = { items: [], next_cursor: null }
    if (endpoint === '/api/user/profile') data = { id: '84', username: 'vendor_demo', organization }
    if (endpoint === '/api/org/current') data = organization
    if (endpoint === ops + '/partners') data.items = [peer]
    if (endpoint === ops + '/tickets') data.items = [ticket]
    await route.fulfill({ json: { code: 200, message: 'success', data } })
  })

  await page.goto('/login/vendor?view=vendor-finance')
  await page.getByRole('button', { name: '服务费额度', exact: true }).click()
  await page.getByRole('button', { name: '设置额度', exact: true }).click()
  await page.getByLabel('金额（元）').fill('123.45')
  await page.getByRole('button', { name: '保存额度' }).click()
  await expect(page.getByRole('row').filter({ hasText: peer.name })).toContainText('¥123.45')
  await page.goto('/login/vendor?view=vendor-tickets')
  const summary = page.locator('summary').filter({ hasText: ticket.subject })
  await expect(summary).toContainText(peer.name)
  await summary.click()
  await page.getByRole('button', { name: '填写处理结果' }).click()
  await page.getByLabel('处理结果').fill('授权已核对。')
  await page.getByRole('button', { name: '提交处理结果' }).click()
  await expect(summary).toContainText('待代理商确认')
  expect(actions).toEqual([
    { method: 'PUT', endpoint: ops + '/allowances/82', body: { limit_cents: 12345 } },
    { method: 'POST', endpoint: ops + '/tickets/83/resolve', body: { reply: '授权已核对。', revision: 1 } },
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
