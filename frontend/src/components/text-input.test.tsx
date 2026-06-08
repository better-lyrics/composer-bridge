import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TextInput } from "@/components/text-input";

describe("TextInput", () => {
  it("emits onChange with the new value", () => {
    const onChange = vi.fn();
    render(<TextInput value="" onChange={onChange} ariaLabel="port" />);
    fireEvent.change(screen.getByLabelText(/port/i), { target: { value: "8080" } });
    expect(onChange).toHaveBeenCalledWith("8080");
  });
});
