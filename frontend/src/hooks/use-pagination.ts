import { useState, useEffect } from 'react';

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

export function usePagination(options: UsePaginationOptions = {}): UsePaginationReturn {
  const { defaultPageSize = 10, defaultPage = 1, storageKey } = options;

  const getInitialPage = () => {
    if (!storageKey) return defaultPage;
    const stored = localStorage.getItem(`${storageKey}-page`);
    return stored ? parseInt(stored, 10) : defaultPage;
  };

  const getInitialPageSize = () => {
    if (!storageKey) return defaultPageSize;
    const stored = localStorage.getItem(`${storageKey}-pageSize`);
    return stored ? parseInt(stored, 10) : defaultPageSize;
  };

  const [page, setPageState] = useState(getInitialPage);
  const [pageSize, setPageSizeState] = useState(getInitialPageSize);

  const setPage = (newPage: number) => {
    setPageState(newPage);
    if (storageKey) {
      localStorage.setItem(`${storageKey}-page`, newPage.toString());
    }
  };

  const setPageSize = (newSize: number) => {
    setPageSizeState(newSize);
    setPageState(1);
    if (storageKey) {
      localStorage.setItem(`${storageKey}-pageSize`, newSize.toString());
      localStorage.setItem(`${storageKey}-page`, '1');
    }
  };

  const goToFirstPage = () => setPage(1);
  const goToLastPage = (totalPages: number) => setPage(Math.max(1, totalPages));

  const goToNextPage = (totalPages: number) => {
    setPage((prev) => Math.min(prev + 1, totalPages));
  };

  const goToPreviousPage = () => {
    setPage((prev) => Math.max(1, prev - 1));
  };

  const canGoNext = (totalPages: number) => page < totalPages;
  const canGoPrevious = () => page > 1;

  const reset = () => {
    setPage(defaultPage);
    setPageSize(defaultPageSize);
  };

  return {
    page,
    pageSize,
    setPage,
    setPageSize,
    goToFirstPage,
    goToLastPage,
    goToNextPage,
    goToPreviousPage,
    canGoNext,
    canGoPrevious,
    reset,
  };
}
