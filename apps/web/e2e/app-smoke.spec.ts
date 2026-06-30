import { expect, test } from '@playwright/test'

test('renders the login route', async ({ page }) => {
  await page.goto('/login')

  await expect(page.getByRole('img', { name: '电力行业知识助手' })).toBeVisible()
  await expect(page.getByLabel('用户名')).toBeVisible()
  await expect(page.getByLabel('密码')).toBeVisible()
  await expect(page.getByRole('button', { name: '登 录' })).toBeVisible()
})

test('redirects anonymous users from protected report records to login', async ({ page }) => {
  await page.goto('/reports/records')

  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('img', { name: '电力行业知识助手' })).toBeVisible()
})
