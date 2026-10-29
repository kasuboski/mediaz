import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it } from 'vitest';
import { queryKeys } from './queries';

describe('library query keys', () => {
  it('keeps state permutations and duplicates on the same cache key', () => {
    expect(queryKeys.movies.library({ page: 2, pageSize: 25, states: ['downloaded', 'discovered', 'downloaded'] }))
      .toEqual(queryKeys.movies.library({ page: 2, pageSize: 25, states: ['discovered', 'downloaded'] }));
  });

  it('keeps every failure page under a shared invalidation prefix', async () => {
    const client = new QueryClient();
    const keys = [queryKeys.activity.failures(24, { page: 1 }), queryKeys.activity.failures(48, { page: 2, pageSize: 10 })];
    keys.forEach((key) => client.setQueryData<unknown>(key, {}));
    await client.invalidateQueries({ queryKey: queryKeys.activity.failures() });
    expect(keys.every((key) => client.getQueryState(key)?.isInvalidated)).toBe(true);
    client.clear();
  });

  it('invalidates every cached library page with the two-part prefix', async () => {
    const client = new QueryClient();
    const keys = [
      queryKeys.movies.library({ page: 1, pageSize: 25 }),
      queryKeys.movies.library({ page: 2, pageSize: 25, states: ['downloaded'] }),
      queryKeys.tv.library({ page: 1, pageSize: 50 }),
    ];
    keys.forEach((key) => client.setQueryData<unknown[]>(key, []));

    await client.invalidateQueries({ queryKey: queryKeys.movies.library() });

    expect(client.getQueryState(keys[0])?.isInvalidated).toBe(true);
    expect(client.getQueryState(keys[1])?.isInvalidated).toBe(true);
    expect(client.getQueryState(keys[2])?.isInvalidated).toBe(false);
    client.clear();
  });
});
