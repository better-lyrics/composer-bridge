import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OriginListInput } from "@/components/origin-list-input";

afterEach(cleanup);

describe("OriginListInput", () => {
  it("renders each origin as a chip", () => {
    render(
      <OriginListInput
        origins={["https://composer.boidu.dev", "http://localhost:5173"]}
        onChange={vi.fn()}
      />,
    );
    expect(screen.getByText("https://composer.boidu.dev")).toBeInTheDocument();
    expect(screen.getByText("http://localhost:5173")).toBeInTheDocument();
  });

  it("shows the empty-state message when origins is empty", () => {
    render(<OriginListInput origins={[]} onChange={vi.fn()} />);
    expect(screen.getByText(/No origins\./i)).toBeInTheDocument();
  });

  it("clicking the remove button drops that origin", () => {
    const onChange = vi.fn();
    render(
      <OriginListInput
        origins={["https://composer.boidu.dev", "http://localhost:5173"]}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByLabelText("Remove https://composer.boidu.dev"));
    expect(onChange).toHaveBeenCalledWith(["http://localhost:5173"]);
  });

  it("clicking Add appends a valid origin", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={["http://localhost:5173"]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://composer.boidu.dev" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).toHaveBeenCalledWith([
      "http://localhost:5173",
      "https://composer.boidu.dev",
    ]);
  });

  it("pressing Enter in the input submits the form and adds the origin", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    const input = screen.getByLabelText("New origin");
    fireEvent.change(input, { target: { value: "https://composer.boidu.dev" } });
    fireEvent.submit(input);
    expect(onChange).toHaveBeenCalledWith(["https://composer.boidu.dev"]);
  });

  it("rejects an invalid origin and surfaces an error", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "not-a-url" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText(/not a valid origin/i)).toBeInTheDocument();
  });

  it("rejects an origin with a non-http(s) protocol", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "ftp://composer.boidu.dev" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText(/not a valid origin/i)).toBeInTheDocument();
  });

  it("rejects an origin with a path beyond the root", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://composer.boidu.dev/some/path" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText(/not a valid origin/i)).toBeInTheDocument();
  });

  it("strips a trailing slash before adding", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://composer.boidu.dev/" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).toHaveBeenCalledWith(["https://composer.boidu.dev"]);
  });

  it("silently drops a duplicate origin without calling onChange", () => {
    const onChange = vi.fn();
    render(
      <OriginListInput origins={["https://composer.boidu.dev"]} onChange={onChange} />,
    );
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://composer.boidu.dev" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("splits a comma-separated paste into multiple origins", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://a.example.com, https://b.example.com" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).toHaveBeenCalledWith([
      "https://a.example.com",
      "https://b.example.com",
    ]);
  });

  it("rejects the whole paste if any entry is invalid", () => {
    const onChange = vi.fn();
    render(<OriginListInput origins={[]} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://a.example.com, not-a-url" },
    });
    fireEvent.click(screen.getByText("Add"));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText(/not a valid origin/i)).toBeInTheDocument();
  });

  it("disables the Add button when the input is empty", () => {
    render(<OriginListInput origins={[]} onChange={vi.fn()} />);
    expect(screen.getByText("Add").closest("button")).toBeDisabled();
  });

  it("disables the Add button for a whitespace-only input", () => {
    render(<OriginListInput origins={[]} onChange={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("New origin"), { target: { value: "   " } });
    expect(screen.getByText("Add").closest("button")).toBeDisabled();
  });

  it("clears the input after a successful add", () => {
    render(<OriginListInput origins={[]} onChange={vi.fn()} />);
    const input = screen.getByLabelText("New origin") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "https://composer.boidu.dev" } });
    fireEvent.click(screen.getByText("Add"));
    expect(input.value).toBe("");
  });

  it("clears the error message when the user starts editing the input", () => {
    render(<OriginListInput origins={[]} onChange={vi.fn()} />);
    fireEvent.change(screen.getByLabelText("New origin"), { target: { value: "bad" } });
    fireEvent.click(screen.getByText("Add"));
    expect(screen.getByText(/not a valid origin/i)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("New origin"), {
      target: { value: "https://composer.boidu.dev" },
    });
    expect(screen.queryByText(/not a valid origin/i)).not.toBeInTheDocument();
  });
});
