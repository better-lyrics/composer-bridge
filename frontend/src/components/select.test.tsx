import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Select } from "@/components/select";

describe("Select", () => {
  it("emits onChange with the new value", () => {
    const onChange = vi.fn();
    render(
      <Select
        value="a"
        onChange={onChange}
        ariaLabel="thing"
        options={[
          { value: "a", label: "Alpha" },
          { value: "b", label: "Beta" },
        ]}
      />,
    );
    fireEvent.change(screen.getByLabelText(/thing/i), { target: { value: "b" } });
    expect(onChange).toHaveBeenCalledWith("b");
  });
});
