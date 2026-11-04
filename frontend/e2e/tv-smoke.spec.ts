import { test, expect } from '@playwright/test';

test('discover and request a fixture TV series, then verify its library metadata', async ({ page }) => {
  await page.goto('/discover');
  await page.getByPlaceholder('Search movies and TV shows...').fill('Lanterns at Low Tide');
  await page.getByRole('button', { name: 'Search', exact: true }).click();

  await page.getByText('Lanterns at Low Tide', { exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Lanterns at Low Tide' })).toBeVisible();
  await expect(page.getByText('Synthetic ensemble series: Lanterns at Low Tide', { exact: true })).toBeVisible();
  await expect(page.getByText('Ended', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Request Series' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Season 1', { exact: true })).toBeVisible();
  await expect(dialog.getByText('2 episodes', { exact: true }).first()).toBeVisible();
  await dialog.getByRole('combobox').click();
  await page.getByRole('option', { name: /High Definition/ }).last().click();
  await dialog.getByRole('button', { name: 'Request Series', exact: true }).click();
  await expect(dialog).toBeHidden();

  await page.reload();
  await expect(page.getByRole('button', { name: 'Delete Series' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Lanterns at Low Tide' })).toBeVisible();
  await expect(page.getByText('Synthetic ensemble series: Lanterns at Low Tide', { exact: true })).toBeVisible();
  await expect(page.getByText('Netflix', { exact: true })).toBeVisible();
  await expect(page.getByText('Season 1', { exact: true })).toBeVisible();
  await page.getByText('Season 1', { exact: true }).click();
  await expect(page.getByText('1 - Episode 1', { exact: true })).toBeVisible();
  await expect(page.getByText('Synthetic episode 1.', { exact: true })).toBeVisible();
});
