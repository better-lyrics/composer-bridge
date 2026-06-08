import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SettingRow, DISABLED_FOOTNOTE } from "@/components/setting-row";

describe("SettingRow", () => {
  it("renders label, description, and child control", () => {
    render(
      <SettingRow label="Listen port" description="Bridge HTTP port">
        <input data-testid="control" />
      </SettingRow>,
    );
    expect(screen.getByText("Listen port")).toBeInTheDocument();
    expect(screen.getByText(/Bridge HTTP port/i)).toBeInTheDocument();
    expect(screen.getByTestId("control")).toBeInTheDocument();
  });

  it("shows the wiring-lands-later footnote when disabled", () => {
    render(
      <SettingRow label="Library size" disabled>
        <span>unknown</span>
      </SettingRow>,
    );
    expect(screen.getByText(DISABLED_FOOTNOTE)).toBeInTheDocument();
  });
});
