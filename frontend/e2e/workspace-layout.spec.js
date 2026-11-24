import { expect, test } from '@playwright/test'

test.use({ launchOptions: { ignoreDefaultArgs: ['--hide-scrollbars'] } })

const height = element => element.evaluate(field => field.getBoundingClientRect().height)

test('multiline fields start at their configured height, grow with content, and shrink again', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 850 })
  await page.goto('/experience/dealer?view=visits')
  const notes = page.getByRole('textbox', { name: '拜访记录' })
  await expect(notes).toHaveCSS('resize', 'none')
  const initial = await height(notes)
  await notes.fill('核对采购模块')
  expect(await height(notes)).toBe(initial)
  const lines = Array.from({ length: 17 }, (_, index) => '客户还需核对第 ' + (index + 1) + ' 项软件部署与授权要求')
  await notes.fill(lines.join('\n'))
  await expect.poll(() => height(notes)).toBeGreaterThan(initial * 2)
  expect(await notes.evaluate(field => field.scrollHeight - field.clientHeight)).toBeLessThanOrEqual(1)
  await page.getByRole('button', { name: '核对两类需求' }).click()
  const additional = page.getByRole('textbox', { name: '附加需求' })
  await expect(additional).toHaveValue(lines.join('；'))
  await expect.poll(() => additional.evaluate(field => field.scrollHeight - field.clientHeight)).toBeLessThanOrEqual(1)
  const wrapped = await height(additional)
  await page.setViewportSize({ width: 1440, height: 850 })
  await expect.poll(() => height(additional)).toBeLessThan(wrapped)
  await page.setViewportSize({ width: 390, height: 850 })
  await expect.poll(() => height(additional)).toBe(wrapped)
  await notes.clear()
  await expect.poll(() => height(notes)).toBe(initial)
})

test('the scrollbar gutter and blue all-functions button stay stable across pages and dialogs', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 850 })
  await page.goto('/experience/dealer')
  await expect(page.getByRole('heading', { name: '代理商工作台' })).toBeVisible()
  const header = page.locator('.pl-bar')
  const readWidth = () => header.evaluate(element => ({
    left: element.getBoundingClientRect().left, width: element.getBoundingClientRect().width,
  }))
  const initial = await readWidth()
  await expect(page.locator('html')).toHaveCSS('scrollbar-gutter', 'stable')
  expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(850)
  expect(await page.evaluate(() => document.body.getBoundingClientRect().width)).toBeLessThan(1440)
  const all = page.getByRole('button', { name: '全部功能' })
  await expect(all).toHaveCSS('background-color', 'rgb(0, 103, 184)')
  await expect(all).toHaveCSS('color', 'rgb(255, 255, 255)')
  await all.click()
  await expect(all).toHaveCSS('background-color', 'rgb(0, 90, 158)')
  await page.getByRole('navigation', { name: '全部工作台功能' }).getByRole('button', { name: '记拜访' }).click()
  await page.getByRole('textbox', { name: '拜访记录' }).fill('核对系统权限。\n'.repeat(75))
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollHeight)).toBeGreaterThan(850)
  expect(await readWidth()).toEqual(initial)
  await page.getByRole('button', { name: '切换身份', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  expect(await readWidth()).toEqual(initial)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toBeHidden()
  await page.getByRole('navigation', { name: '工作台栏目' }).getByRole('button', { name: '工作总览' }).click()
  await expect(page.getByRole('heading', { name: '代理商工作台' })).toBeVisible()
  expect(await readWidth()).toEqual(initial)
})

test('site search fits its full placeholder before and after focus', async ({ page }) => {
  const search = page.getByRole('searchbox', { name: '搜索站内功能' })
  for (const path of ['/', '/experience/dealer']) {
    for (const width of [1440, 1021, 941, 761, 320]) {
      await page.setViewportSize({ width, height: 850 })
      await page.goto(path)
      await expect(search).toHaveCSS('font-size', '12px')
      const { available, required } = await search.evaluate(field => {
        const style = getComputedStyle(field)
        const context = document.createElement('canvas').getContext('2d')
        context.font = `${style.fontSize} ${style.fontFamily}`
        return {
          available: field.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
          required: context.measureText(field.placeholder).width,
        }
      })
      expect(available, `${path} at ${width}px`).toBeGreaterThan(required)
      const before = await search.boundingBox()
      await search.focus()
      expect(await search.boundingBox()).toEqual(before)
      await search.blur()
    }
  }
})

test('the full directory packs groups within each column and fits a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 850 })
  await page.goto('/experience/dealer')
  const trigger = page.getByRole('button', { name: '全部功能' })
  const directory = page.getByRole('navigation', { name: '全部工作台功能' })
  await trigger.click()
  const columns = () => directory.locator('.workspace-menu-group').evaluateAll(groups => {
    const byColumn = new Map()
    for (const group of groups) {
      const { left, top, bottom } = group.getBoundingClientRect()
      const column = byColumn.get(left) ?? []
      column.push({ top, bottom })
      byColumn.set(left, column)
    }
    return [...byColumn.values()]
  })
  const desktop = await columns()
  expect(desktop).toHaveLength(2)
  for (const column of desktop) {
    for (let index = 1; index < column.length; index++) {
      expect(column[index].top - column[index - 1].bottom).toBeLessThanOrEqual(1)
    }
  }
  await page.setViewportSize({ width: 320, height: 630 })
  expect(await columns()).toHaveLength(1)
  expect(await directory.evaluate(panel => panel.scrollWidth - panel.clientWidth)).toBe(0)
  const lastEntry = directory.getByRole('button').last()
  await lastEntry.scrollIntoViewIfNeeded()
  await expect(lastEntry).toBeInViewport()
})
