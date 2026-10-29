import { describe, expect, it } from 'vitest';
import { normalizeLibraryUrl, parsePageSize, parsePositiveInteger } from './library-url-state';

describe('library URL state', () => {
  it('validates positive integer pages without accepting fractions, signs, or overflow', () => {
    expect(parsePositiveInteger('2', 1)).toBe(2);
    for (const value of ['1.5', '-2', '0', '1e2', '9007199254740992']) expect(parsePositiveInteger(value, 1)).toBe(1);
  });
  it('restricts page sizes and normalizes unsupported tabs', () => {
    expect(parsePageSize('100')).toBe(100);
    expect(parsePageSize('10')).toBe(50);
    expect(normalizeLibraryUrl(new URLSearchParams('page=3&state=bogus'), ['all', 'missing'])).toEqual({ page: 3, pageSize: 50, state: 'all' });
  });
});
