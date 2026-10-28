export const LIBRARY_PAGE_SIZES = [25, 50, 100] as const;
export type LibraryPageSize = typeof LIBRARY_PAGE_SIZES[number];

export function parsePositiveInteger(value: string | null, fallback: number): number {
  if (value === null || !/^\d+$/.test(value)) return fallback;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback;
}

export function parsePageSize(value: string | null): LibraryPageSize {
  const parsed = Number(value);
  return LIBRARY_PAGE_SIZES.includes(parsed as LibraryPageSize) ? parsed as LibraryPageSize : 50;
}

export function normalizeLibraryUrl(params: URLSearchParams, filters: readonly string[]) {
  const requestedState = params.get('state');
  const state = requestedState && filters.includes(requestedState) ? requestedState : 'all';
  return { page: parsePositiveInteger(params.get('page'), 1), pageSize: parsePageSize(params.get('pageSize')), state };
}
