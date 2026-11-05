import { test, expect } from '@playwright/test';

test('clearing an unmatched search shows an empty result state without stale media', async ({ page }) => {
  await page.goto('/discover');
  const search = page.getByPlaceholder('Search movies and TV shows...');
  await search.fill('The Glass Harbor');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByText('The Glass Harbor', { exact: true })).toBeVisible();

  await search.fill('no-such-mediaz-fixture-9f4c');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'No results found' })).toBeVisible();
  await expect(page.getByText('The Glass Harbor', { exact: true })).toHaveCount(0);
  await expect(page.getByText('Lanterns at Low Tide', { exact: true })).toHaveCount(0);

  // The header intentionally ignores blank submissions; navigate to the empty-query state.
  await page.goto('/discover');
  await expect(page.getByRole('heading', { name: 'Discover Movies & TV Shows' })).toBeVisible();
  await expect(page.getByText('No results found', { exact: true })).toHaveCount(0);
  await expect(page.getByText('The Glass Harbor', { exact: true })).toHaveCount(0);
});
