import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PaginationControls } from "./pagination-controls";

describe("PaginationControls", () => {
  it("supports keyboard-operable page buttons and disables boundaries", () => {
    const onPageChange = vi.fn();
    render(<PaginationControls currentPage={1} totalPages={4} onPageChange={onPageChange} />);
    expect(screen.getByRole("button", { name: "Go to previous page" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Go to next page" }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });
  it("renders nothing for zero pages", () => {
    const { container } = render(<PaginationControls currentPage={1} totalPages={0} onPageChange={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });
});
