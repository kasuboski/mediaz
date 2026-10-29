import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { usePagination } from "./use-pagination";

afterEach(() => localStorage.clear());

describe("usePagination", () => {
  it("defaults to 50 and moves within page bounds", () => {
    const { result } = renderHook(() => usePagination());
    expect(result.current.pageSize).toBe(50);
    act(() => result.current.goToNextPage(3));
    expect(result.current.page).toBe(2);
    act(() => result.current.goToPreviousPage());
    expect(result.current.page).toBe(1);
    act(() => result.current.goToPreviousPage());
    expect(result.current.page).toBe(1);
    act(() => result.current.goToNextPage(0));
    expect(result.current.page).toBe(1);
  });

  it("ignores invalid stored values and tolerates unavailable storage", () => {
    localStorage.setItem("test-page", "0");
    localStorage.setItem("test-pageSize", "NaN");
    const { result } = renderHook(() => usePagination({ storageKey: "test", defaultPageSize: 25 }));
    expect(result.current.page).toBe(1);
    expect(result.current.pageSize).toBe(25);
    const getItem = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked"); });
    const blocked = renderHook(() => usePagination({ storageKey: "blocked" }));
    act(() => blocked.result.current.setPage(2));
    expect(blocked.result.current.page).toBe(2);
    getItem.mockRestore();
  });

  it("rejects stale oversized stored values and enforces the page-size maximum", () => {
    localStorage.setItem("jobs-page", "999999999999999999999");
    localStorage.setItem("jobs-pageSize", "101");
    const { result } = renderHook(() => usePagination({ storageKey: "jobs", defaultPageSize: 25 }));
    expect(result.current).toMatchObject({ page: 1, pageSize: 25 });
    act(() => result.current.setPageSize(101));
    expect(result.current.pageSize).toBe(25);
  });

  it("restores valid numeric values and persists changes, resetting page on size change", () => {
    localStorage.setItem("library-page", "3");
    localStorage.setItem("library-pageSize", "25");
    const { result } = renderHook(() => usePagination({ storageKey: "library" }));
    expect(result.current).toMatchObject({ page: 3, pageSize: 25 });

    act(() => result.current.setPageSize(100));
    expect(result.current).toMatchObject({ page: 1, pageSize: 100 });
    expect(localStorage.getItem("library-page")).toBe("1");
    expect(localStorage.getItem("library-pageSize")).toBe("100");
    act(() => result.current.setPage(4));
    expect(localStorage.getItem("library-page")).toBe("4");
  });
});
