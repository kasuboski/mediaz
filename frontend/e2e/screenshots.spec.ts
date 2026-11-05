import { test, expect } from '@playwright/test';

const viewport = { width: 1440, height: 1000 };
const capture = async (page: import('@playwright/test').Page, testInfo: import('@playwright/test').TestInfo, name: string, size = viewport) => {
  await page.setViewportSize(size);
  // Assert rendered colors, not just token strings: catches missing Tailwind
  // mappings, stale CSS, and accidental light fallback surfaces.
  await expect(page.locator('body')).toHaveCSS('background-color', 'rgb(30, 30, 46)');
  await expect(page.locator('body')).toHaveCSS('color', 'rgb(205, 214, 244)');
  // Give visible remote artwork a bounded chance to load before capture. Do not
  // block screenshots when the CDN is unavailable.
  await page.evaluate(async () => {
    const visible = (element: Element) => {
      const rect = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none';
    };
    const images = [...document.images].filter(visible);
    const backgrounds = [...document.querySelectorAll<HTMLElement>('*')]
      .filter(visible)
      .map(element => ({ element, url: getComputedStyle(element).backgroundImage.match(/url\\([\"']?(.*?)[\"']?\\)/)?.[1] }))
      .filter((entry): entry is { element: HTMLElement; url: string } => Boolean(entry.url));
    await Promise.race([
      Promise.all([
        ...images.map(image => image.decode().catch(() => undefined)),
        ...backgrounds.map(({ url }) => new Promise<void>(resolve => {
          const image = new Image();
          image.onload = () => resolve();
          image.onerror = () => resolve();
          image.src = url;
        })),
      ]),
      new Promise(resolve => setTimeout(resolve, 5000)),
    ]);
  });
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: true, animations: 'disabled' });
  await testInfo.attach(`${name}.png`, { path, contentType: 'image/png' });
};

async function ensureFixtureLibrary(page: import('@playwright/test').Page) {
  // These IDs are fixed fake-catalog records. 409 means a prior action in this
  // test already added the item; the suite uses an isolated DB per run.
  const [movieProfilesResponse, seriesProfilesResponse] = await Promise.all([
    page.request.get('/api/v1/quality/profiles?type=movie'),
    page.request.get('/api/v1/quality/profiles?type=series'),
  ]);
  expect(movieProfilesResponse.ok()).toBeTruthy();
  expect(seriesProfilesResponse.ok()).toBeTruthy();
  const movieProfiles = (await movieProfilesResponse.json()).response.items;
  const seriesProfiles = (await seriesProfilesResponse.json()).response.items;
  const movieProfile = movieProfiles[0];
  const seriesProfile = seriesProfiles[0];
  expect(movieProfile, 'default movie quality profile').toBeTruthy();
  expect(seriesProfile, 'default series quality profile').toBeTruthy();

  for (const [endpoint, id, profile] of [
    ['/library/movies', 1001, movieProfile],
    ['/library/tv', 2001, seriesProfile],
  ] as const) {
    const payload = endpoint.endsWith('movies')
      ? { tmdbID: id, qualityProfileID: profile.id }
      : { tmdbID: id, qualityProfileID: profile.id };
    const response = await page.request.post(`/api/v1${endpoint}`, { data: payload });
    if (!response.ok()) {
      const detail = await response.text();
      // Idempotency is achieved by checking the actual library, not masking
      // arbitrary server errors as duplicates.
      const listEndpoint = endpoint.endsWith('movies') ? '/api/v1/library/movies' : '/api/v1/library/tv';
      const list = await page.request.get(listEndpoint);
      expect(list.ok(), detail).toBeTruthy();
      const data = await list.json();
      const items = data.response?.items ?? [];
      expect(items.some((item: { tmdbID: number }) => item.tmdbID === id), detail).toBeTruthy();
    }
  }
}

test('capture representative application pages', async ({ page }, testInfo) => {
  await page.setViewportSize(viewport);
  await page.goto('/discover');
  await page.getByPlaceholder('Search movies and TV shows...').fill('The Glass Harbor');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Search Results' })).toBeVisible();
  await expect(page.getByText('The Glass Harbor', { exact: true })).toBeVisible();
  await page.mouse.move(0, 0); // Assert the default color, not the post-click hover state.
  const searchButton = page.getByRole('button', { name: 'Search', exact: true });
  await expect(searchButton).toHaveCSS('background-image', 'linear-gradient(135deg, rgb(203, 166, 247) 0%, rgb(180, 190, 254) 100%)');
  await expect(searchButton).toHaveCSS('color', 'rgb(30, 30, 46)');
  await capture(page, testInfo, 'discover-search-results');
  await capture(page, testInfo, 'discover-search-results-mobile', { width: 390, height: 844 });

  await ensureFixtureLibrary(page);
  await page.goto('/series');
  await expect(page.getByRole('heading', { name: 'TV Show Library' })).toBeVisible();
  await expect(page.getByText('1 TV shows in your library')).toBeVisible();
  await expect(page.getByText('Lanterns at Low Tide', { exact: true })).toBeVisible();
  await capture(page, testInfo, 'tv-series-library');

  await page.goto('/tv/2001');
  await expect(page.getByRole('heading', { name: 'Lanterns at Low Tide' })).toBeVisible();
  await capture(page, testInfo, 'tv-series-detail');

  await page.goto('/movies');
  await expect(page.getByRole('heading', { name: 'Movie Library' })).toBeVisible();
  await expect(page.getByText('1 movies in your library')).toBeVisible();
  await expect(page.getByText('The Glass Harbor', { exact: true })).toBeVisible();
  await capture(page, testInfo, 'movie-library');

  await page.goto('/activity');
  await expect(page.getByRole('heading', { name: 'Activity', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Active Processes', exact: true })).toBeVisible();
  // No downloads are started for screenshots; an empty activity timeline is
  // an honest reflection of this isolated run's real backend state.
  await expect(page.getByRole('heading', { name: 'No Active Processes' })).toBeVisible();
  await capture(page, testInfo, 'activity');

  await page.goto('/clients');
  await page.getByRole('button', { name: 'Add Download Client', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveCSS('background-color', 'rgb(30, 30, 46)');
  const host = dialog.getByLabel('Host', { exact: true });
  await host.focus();
  await expect(host).toHaveCSS('border-color', 'rgb(166, 173, 200)');
  await expect(host).toHaveCSS('color', 'rgb(205, 214, 244)');
  const focusShadow = await host.evaluate(element => getComputedStyle(element).boxShadow);
  expect(focusShadow, 'focused form input uses the Mauve ring').toContain('203, 166, 247');
  const placeholderColor = await host.evaluate(element => getComputedStyle(element, '::placeholder').color);
  expect(placeholderColor).toBe('rgb(166, 173, 200)');
  await capture(page, testInfo, 'download-client-form');
  await capture(page, testInfo, 'download-client-form-mobile', { width: 390, height: 844 });
});
