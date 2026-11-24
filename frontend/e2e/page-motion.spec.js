import { expect, test } from '@playwright/test'

async function observeMotion(page) {
  await page.addInitScript(() => {
    const animate = Element.prototype.animate
    window.recordedMotion = []
    Element.prototype.animate = function (frames, options) {
      if (Array.isArray(frames)) {
        window.recordedMotion.push({
          target: this.getAttribute('class') || '',
          homepage: Boolean(this.closest('.pl-portal')),
          duration: options?.duration,
          moved: frames.some(frame => Object.hasOwn(frame, 'translate') || Object.hasOwn(frame, 'transform')),
        })
      }
      return animate.call(this, frames, options)
    }
  })
}

test('only the homepage rises and workspace navigation fades the content in place', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await observeMotion(page)
  await page.goto('/')
  await expect.poll(() => page.evaluate(() => window.recordedMotion.some(animation => animation.homepage && animation.moved))).toBe(true)

  await page.goto('/login/customer')
  await expect(page.locator('.access-page')).toHaveCSS('animation-name', 'ui-page-fade')
  await expect(page.locator('[data-ui-entrance-pending]')).toHaveCount(0)

  await page.goto('/experience/customer')
  const content = page.locator('.workspace-view')
  await expect(content.getByRole('heading', { name: '企业客户工作台' })).toBeVisible()
  await page.getByRole('navigation', { name: '工作台栏目' }).getByRole('button', { name: '采购项目' }).click()
  await expect(content.getByRole('heading', { name: '我的采购项目' })).toBeVisible()
  await content.getByRole('list', { name: '项目列表' }).getByRole('button').first().click()
  await expect(content.getByRole('region', { name: '当前办理' })).toBeVisible()
  await expect(page.locator('[data-ui-entrance-pending]')).toHaveCount(0)
  const animations = await page.evaluate(() => window.recordedMotion)
  expect(animations.filter(animation => animation.target === 'workspace-view')).toHaveLength(3)
  expect(animations.every(animation => !animation.moved && animation.duration === 140)).toBe(true)
})

test('reduced motion skips both page entrance styles', async ({ page }) => {
  await observeMotion(page)
  await page.goto('/')
  await expect(page.locator('#portal-title')).toBeVisible()
  expect(await page.evaluate(() => window.recordedMotion.some(animation => animation.moved))).toBe(false)
  await page.goto('/login/customer')
  await expect(page.locator('.access-page')).toHaveCSS('animation-name', 'none')
  await page.goto('/experience/vendor')
  await expect(page.locator('.workspace-view')).toBeVisible()
  await page.getByRole('navigation', { name: '工作台栏目' }).getByRole('button', { name: '产品管理' }).click()
  await expect(page.getByRole('heading', { name: '软件目录' })).toBeVisible()
  expect(await page.evaluate(() => window.recordedMotion)).toEqual([])
})
