import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Toggle } from "@/components/toggle";

describe("Toggle", () => {
  it("toggles checked state on click", () => {
    const onChange = vi.fn();
    render(<Toggle checked={false} onChange={onChange} ariaLabel="enable thing" />);
    fireEvent.click(screen.getByRole("switch", { name: /enable thing/i }));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("does not call onChange when disabled", () => {
    const onChange = vi.fn();
    render(<Toggle checked={false} onChange={onChange} disabled ariaLabel="disabled thing" />);
    fireEvent.click(screen.getByRole("switch", { name: /disabled thing/i }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
