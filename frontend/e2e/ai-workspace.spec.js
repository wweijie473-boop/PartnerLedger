import { expect, test } from '@playwright/test'

test('打包弹窗一次审批自动执行建项与询价，草稿阶段厂商不可见，发送后厂商可见', async ({ page, context }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/experience/dealer')
  const navigation = page.getByRole('navigation', { name: '工作台栏目' })
  const tab = navigation.getByRole('button', { name: 'AI 工作台' })
  await expect(tab.locator('svg')).toBeVisible()
  await tab.click()
  await expect(page.getByRole('button', { name: '语音输入' })).toBeVisible()
  await page.getByRole('button', { name: '试试 ERP 询价示例' }).click()

  const dialog = page.getByRole('dialog', { name: '审批采购询价工作流' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('员工人数')).toHaveValue('120')
  await expect(dialog.getByLabel('授权用户数')).toHaveValue('')
  await expect(dialog.getByLabel('所需模块')).toHaveValue('财务、库存')
  await dialog.getByRole('button', { name: '不同意' }).click()
  await expect(page.getByText('已取消本次审批，未执行任何业务写入。')).toBeVisible()
  await expect(page.getByText('审批已执行完成：已自动保存询价草稿，项目仍待询价。')).toHaveCount(0)

  const composer = page.getByRole('textbox', { name: '给渠道业务助手发消息' })
  await composer.fill('制造企业员工 120 人，需要财务和库存模块，希望云部署。帮我准备询价草稿。')
  await composer.press('Enter')
  await expect(dialog).toBeVisible()
  await dialog.getByLabel('授权用户数').fill('25')
  await dialog.getByRole('button', { name: '同意并自动执行' }).click()
  await expect(page.getByText('审批已执行完成：已自动保存询价草稿，项目仍待询价。')).toBeVisible()
  await expect(page.getByText(/授权席位：25/)).toBeVisible()
  await page.getByText('查看厂商资料').click()
  await expect(page.getByText('演示软件目录', { exact: false })).toBeVisible()

  const readProject = page.getByRole('combobox', { name: '项目上下文' })
  const project = await readProject.inputValue()
  expect(project).toBeTruthy()

  const vendor = await context.newPage()
  await vendor.goto('/experience/vendor?project=' + project)
  await expect(vendor.getByText('当前身份无法查看此项目')).toBeVisible()

  await composer.fill('把这份询价发送给厂商')
  await composer.press('Enter')
  const sendDialog = page.getByRole('dialog', { name: '审批发送厂商询价' })
  await expect(sendDialog).toBeVisible()
  await expect(sendDialog.getByText(/授权席位：25/)).toBeVisible()
  await expect(vendor.getByText('当前身份无法查看此项目')).toBeVisible()
  await sendDialog.getByRole('button', { name: '同意并发送询价' }).click()
  await expect(page.getByText('审批已执行完成：询价已提交给软件厂商。')).toBeVisible()
  await expect(vendor.getByRole('definition').filter({ hasText: /授权席位：25/ })).toBeVisible()

  await composer.fill('帮我查一下当前项目进展')
  await composer.press('Enter')
  await expect(page.getByText('这是「海川精工 · ERP 采购」的当前进展。')).toBeVisible()
  await expect(page.getByRole('dialog')).not.toBeVisible()
})

test('多候选产品在弹窗内一次选定并自动执行，对客报价与拜访转商机技能一键跑通', async ({ page }) => {
  await page.goto('/experience/dealer?view=requirement')
  const composer = page.getByRole('textbox', { name: '给渠道业务助手发消息' })

  await composer.fill('请帮我匹配财务和库存的产品，要求云部署')
  await composer.press('Enter')
  const matchDialog = page.getByRole('dialog', { name: '审批产品匹配条件' })
  await expect(matchDialog).toBeVisible()
  await matchDialog.getByRole('button', { name: '同意并匹配产品' }).click()
  await expect(page.getByText('匹配完成，找到 1 款可进一步核对的产品。')).toBeVisible()
  await expect(page.getByText('审批已执行完成：已自动保存询价草稿，项目仍待询价。')).toHaveCount(0)

  await composer.fill('帮我准备询价草稿，要求云部署')
  await composer.press('Enter')
  const inquiryDialog = page.getByRole('dialog', { name: '审批采购询价工作流' })
  await expect(inquiryDialog).toBeVisible()
  await inquiryDialog.locator('label').filter({ hasText: '云序 OA 协作套件' }).click()
  await inquiryDialog.getByRole('button', { name: '同意并自动执行' }).click()
  await expect(page.getByText('询价草稿 · 云序 OA 协作套件')).toBeVisible()

  await composer.fill('给财务共享中心项目按加价 25% 生成并发送对客报价')
  await composer.press('Enter')
  const quoteDialog = page.getByRole('dialog', { name: '审批编制并发送对客报价' })
  await expect(quoteDialog).toBeVisible()
  await quoteDialog.getByRole('button', { name: '同意并自动执行' }).click()
  await expect(page.getByText(/对客报价已自动生成并发送给客户/)).toBeVisible()

  await composer.fill('记一条拜访：今天回访确认了主项目，另外客户还需要把 ERP 对接 MES 排程接口')
  await composer.press('Enter')
  const visitDialog = page.getByRole('dialog', { name: '审批记拜访并沉淀团队商机' })
  await expect(visitDialog).toBeVisible()
  await visitDialog.getByRole('button', { name: '同意并自动执行' }).click()
  await expect(page.getByText('拜访记录与团队附加商机已自动沉淀完成，主项目订单不受影响。')).toBeVisible()
})

test('Codex 声波录音胶囊支持实时波形展示、左键取消与蓝键一键转写直发', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 860 })
  await page.addInitScript(() => {
    sessionStorage.setItem('partnerledger-api-session', JSON.stringify({ token: 'demo-session', expiresAt: Date.now() + 60_000 }))
    navigator.mediaDevices.getUserMedia = async () => {
      const ctx = new AudioContext()
      const osc = ctx.createOscillator()
      const gain = ctx.createGain()
      osc.frequency.value = 260
      gain.gain.value = 0.28
      const dest = ctx.createMediaStreamDestination()
      osc.connect(gain)
      gain.connect(dest)
      osc.start()
      let tick = 0
      setInterval(() => {
        tick++
        gain.gain.value = tick % 3 === 0 ? 0.002 : (0.12 + (tick % 5) * 0.06)
      }, 60)
      return dest.stream
    }
  })

  await page.route('**/api/**', async route => {
    const url = new URL(route.request().url())
    const p = url.pathname
    if (p === '/api/user/profile') {
      return route.fulfill({ json: { code: 200, message: 'success', data: { id: '1', username: 'test_dealer', organization: { id: '2', name: '测试代理商', kind: 'RESELLER' } } } })
    }
    if (p === '/api/org/current') {
      return route.fulfill({ json: { code: 200, message: 'success', data: { id: '2', name: '测试代理商', kind: 'RESELLER' } } })
    }
    if (p === '/api/workspace/badges') {
      return route.fulfill({ json: { code: 200, message: 'success', data: { total: 0, pages: {}, items: [] } } })
    }
    if (p === '/api/ai/transcribe') {
      return route.fulfill({ json: { code: 200, message: 'success', data: { text: '帮我查看一下我的所有客户信息' } } })
    }
    if (p === '/api/ai/workspace/stream') {
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream; charset=utf-8',
        body: 'event: delta\ndata: {"text":"已整理客户：**海川精密机械制造有限公司**"}\n\nevent: done\ndata: {"links":["customers"],"model":"test"}\n\n',
      })
    }
    return route.fulfill({ json: { code: 200, message: 'success', data: { items: [], next_cursor: null } } })
  })

  await page.goto('/login/dealer?view=requirement')
  const micBtn = page.getByRole('button', { name: '语音输入' })
  await expect(micBtn).toBeVisible()
  await micBtn.click()

  await expect(page.getByRole('button', { name: '取消录音' })).toBeVisible()
  await expect(page.getByRole('button', { name: '完成说词并转写' })).toBeVisible()
  await expect(page.getByRole('button', { name: '完成录音并直接发送' })).toBeVisible()
  await expect.poll(() => page.locator('.ai-wave-bar.is-active').count()).toBeGreaterThan(3)

  await page.getByRole('button', { name: '取消录音' }).click()
  await expect(page.getByRole('textbox', { name: '给渠道业务助手发消息' })).toBeVisible()

  await micBtn.click()
  await page.waitForTimeout(350)
  await page.getByRole('button', { name: '完成录音并直接发送' }).click()
  await expect(page.getByText('海川精密机械制造有限公司')).toBeVisible()
})
