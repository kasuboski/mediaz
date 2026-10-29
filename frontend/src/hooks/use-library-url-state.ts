import { useCallback, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { normalizeLibraryUrl, type LibraryPageSize } from '@/lib/library-url-state';

export function useLibraryUrlState(filters: readonly string[]) {
  const [params, setParams] = useSearchParams();
  const state = useMemo(() => normalizeLibraryUrl(params, filters), [params, filters]);

  useEffect(() => {
    const canonical = new URLSearchParams(params);
    if (state.page === 1) canonical.delete('page'); else canonical.set('page', String(state.page));
    if (state.pageSize === 50) canonical.delete('pageSize'); else canonical.set('pageSize', String(state.pageSize));
    if (state.state === 'all') canonical.delete('state'); else canonical.set('state', state.state);
    if (canonical.toString() !== params.toString()) setParams(canonical, { replace: true });
  }, [params, setParams, state]);

  const update = useCallback((next: Partial<typeof state>, replace = false) => {
    const values = { ...state, ...next };
    const updated = new URLSearchParams(params);
    if (values.page === 1) updated.delete('page'); else updated.set('page', String(values.page));
    if (values.pageSize === 50) updated.delete('pageSize'); else updated.set('pageSize', String(values.pageSize));
    if (values.state === 'all') updated.delete('state'); else updated.set('state', values.state);
    setParams(updated, { replace });
  }, [params, setParams, state]);

  const setPage = useCallback((page: number, replace = false) => update({ page }, replace), [update]);
  const setState = useCallback((value: string) => update({ state: value, page: 1 }), [update]);
  const setPageSize = useCallback((pageSize: LibraryPageSize) => update({ pageSize, page: 1 }), [update]);
  return { ...state, setPage, setState, setPageSize };
}
