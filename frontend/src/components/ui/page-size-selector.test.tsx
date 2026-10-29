import { fireEvent, render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { PageSizeSelector } from "./page-size-selector";

beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

describe("PageSizeSelector", () => {
  it("offers bounded positive choices and filters invalid options", () => {
    const onChange = vi.fn();
    render(<PageSizeSelector value={25} onChange={onChange} options={[10, 0, -2, 10.5, 20]} />);
    fireEvent.click(screen.getByRole("combobox"));
    expect(screen.getByRole("option", { name: "10" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "20" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "All" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "0" })).not.toBeInTheDocument();
  });

  it("only exposes All when explicitly enabled", () => {
    const onChange = vi.fn();
    render(<PageSizeSelector value={25} onChange={onChange} allowAll />);
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "All" }));
    expect(onChange).toHaveBeenCalledWith(0);
  });
});
