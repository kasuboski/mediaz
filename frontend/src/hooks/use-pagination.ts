import { useState } from 'react';

export interface UsePaginationOptions {
  defaultPageSize?: number;
  defaultPage?: number;
  storageKey?: string;
}

export interface UsePaginationReturn {
  page: number;
  pageSize: number;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;
  goToFirstPage: () => void;
  goToLastPage: (totalPages: number) => void;
  goToNextPage: (totalPages: number) => void;
  goToPreviousPage: () => void;
  canGoNext: (totalPages: number) => boolean;
  canGoPrevious: () => boolean;
  reset: () => void;
}

function readStoredInteger(key: string, fallback: number, minimum: number, maximum = Number.MAX_SAFE_INTEGER): number {
  try {
    const value = window.localStorage.getItem(key);
    if (value === null || !/^\d+$/.test(value)) return fallback;
    const parsed = Number(value);
    return Number.isSafeInteger(parsed) && parsed >= minimum && parsed <= maximum ? parsed : fallback;
  } catch {
    return fallback;
  }
}

function persist(key: string, value: number) {
  try {
    window.localStorage.setItem(key, String(value));
  } catch {
    // Storage can be unavailable (e.g. private browsing); pagination still works in memory.
  }
}

export function usePagination(options: UsePaginationOptions = {}): UsePaginationReturn {
  const { defaultPageSize = 50, defaultPage = 1, storageKey } = options;
  const safeDefaultPage = Number.isSafeInteger(defaultPage) && defaultPage > 0 ? defaultPage : 1;
  const safeDefaultPageSize = Number.isSafeInteger(defaultPageSize) && defaultPageSize > 0 && defaultPageSize <= 100 ? defaultPageSize : 50;
  const [page, setPageState] = useState(() => storageKey ? readStoredInteger(`${storageKey}-page`, safeDefaultPage, 1) : safeDefaultPage);
  const [pageSize, setPageSizeState] = useState(() => storageKey ? readStoredInteger(`${storageKey}-pageSize`, safeDefaultPageSize, 1, 100) : safeDefaultPageSize);

  const setPage = (newPage: number) => {
    const safePage = Number.isSafeInteger(newPage) && newPage > 0 ? newPage : 1;
    setPageState(safePage);
    if (storageKey) persist(`${storageKey}-page`, safePage);
  };
  const setPageSize = (newSize: number) => {
    if (!Number.isSafeInteger(newSize) || newSize <= 0 || newSize > 100) return;
    setPageSizeState(newSize);
    setPage(1);
    if (storageKey) persist(`${storageKey}-pageSize`, newSize);
  };
  const goToFirstPage = () => setPage(1);
  const goToLastPage = (totalPages: number) => setPage(Math.max(1, Number.isFinite(totalPages) ? totalPages : 1));
  const goToNextPage = (totalPages: number) => setPage(Math.min(page + 1, Math.max(1, totalPages)));
  const goToPreviousPage = () => setPage(Math.max(1, page - 1));
  const canGoNext = (totalPages: number) => totalPages > 0 && page < totalPages;
  const canGoPrevious = () => page > 1;
  const reset = () => { setPage(safeDefaultPage); setPageSize(safeDefaultPageSize); };

  return { page, pageSize, setPage, setPageSize, goToFirstPage, goToLastPage, goToNextPage, goToPreviousPage, canGoNext, canGoPrevious, reset };
}
