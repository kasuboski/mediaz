import { test, expect } from '@playwright/test';

test('discover and request a fixture movie, then verify its library detail', async ({ page }) => {
  await page.goto('/discover');
  await page.getByPlaceholder('Search movies and TV shows...').fill('The Glass Harbor');
  await page.getByRole('button', { name: 'Search', exact: true }).click();

  const movie = page.getByText('The Glass Harbor', { exact: true });
  await expect(movie).toBeVisible();
  await movie.click();
  await expect(page.getByRole('heading', { name: 'The Glass Harbor' })).toBeVisible();
  await expect(page.getByText('2004', { exact: true })).toBeVisible();
  await expect(page.getByText('A synthetic story about The Glass Harbor.', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Request Movie' }).click();
  await page.getByRole('combobox').click();
  await page.getByRole('option', { name: /High Definition/ }).first().click();
  await page.getByRole('button', { name: 'Request Movie', exact: true }).last().click();
  await expect(page.getByRole('dialog')).toBeHidden();
  await page.reload();

  await expect(page.locator('p.font-medium').filter({ hasText: /^Monitoring$/ })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Delete Movie' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'The Glass Harbor' })).toBeVisible();
  await expect(page.getByText('A synthetic story about The Glass Harbor.', { exact: true })).toBeVisible();
});
