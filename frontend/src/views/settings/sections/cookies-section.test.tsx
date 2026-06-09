import { describe, expect, it, beforeEach, afterEach } from "vitest";
import { fireEvent, render, screen, waitFor, cleanup } from "@testing-library/react";
import { CookiesSection } from "@/views/settings/sections/cookies-section";
import { setupWailsMock, resetWailsMock, type AppBindings } from "@/test/wails-mock";
import type { app, ytdlp } from "../../../../wailsjs/go/models";

const cookiesStatus = (overrides: Partial<app.CookiesStatus> = {}): app.CookiesStatus =>
  ({
    present: false,
    enabled: false,
    path: "/Users/test/.composer-bridge/cookies.txt",
    ...overrides,
  }) as app.CookiesStatus;

const verifyResult = (overrides: Partial<ytdlp.VerifyResult> = {}): ytdlp.VerifyResult =>
  ({
    loaded: false,
    authenticated: false,
    rotated: false,
    detail: "",
    ...overrides,
  }) as ytdlp.VerifyResult;

let bindings: AppBindings;

beforeEach(() => {
  bindings = setupWailsMock();
});

afterEach(() => {
  cleanup();
  resetWailsMock();
});

describe("CookiesSection", () => {
  it("shows the empty status when no cookies are uploaded", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: false }));
    render(<CookiesSection />);
    await waitFor(() =>
      expect(screen.getByText(/No cookies uploaded/i)).toBeInTheDocument(),
    );
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Remove/i })).not.toBeInTheDocument();
  });

  it("shows the active status and an enabled toggle when present + enabled", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: true }));
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Active\./i)).toBeInTheDocument());
    const toggle = screen.getByRole("switch") as HTMLButtonElement;
    expect(toggle.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("button", { name: /Remove/i })).toBeInTheDocument();
  });

  it("shows the paused status and a disabled toggle when present but disabled", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: false }));
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Uploaded but paused/i)).toBeInTheDocument());
    const toggle = screen.getByRole("switch") as HTMLButtonElement;
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });

  it("typing in the textarea and clicking Upload calls UploadCookies with the typed content", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: false }));
    render(<CookiesSection />);
    await waitFor(() =>
      expect(screen.getByText(/No cookies uploaded/i)).toBeInTheDocument(),
    );
    const textarea = screen.getByLabelText(/Cookies file contents/i) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "# Netscape\nfoo bar" } });
    fireEvent.click(screen.getByRole("button", { name: /Upload/i }));
    await waitFor(() =>
      expect(bindings.UploadCookies).toHaveBeenCalledWith("# Netscape\nfoo bar"),
    );
  });

  it("surfaces an upload error inline", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: false }));
    bindings.UploadCookies.mockRejectedValueOnce(new Error("invalid Netscape header"));
    render(<CookiesSection />);
    await waitFor(() =>
      expect(screen.getByText(/No cookies uploaded/i)).toBeInTheDocument(),
    );
    const textarea = screen.getByLabelText(/Cookies file contents/i) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "garbage" } });
    fireEvent.click(screen.getByRole("button", { name: /Upload/i }));
    await waitFor(() =>
      expect(screen.getByText(/invalid Netscape header/i)).toBeInTheDocument(),
    );
  });

  it("toggle off calls SetCookiesEnabled(false)", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: true }));
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Active\./i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole("switch"));
    await waitFor(() => expect(bindings.SetCookiesEnabled).toHaveBeenCalledWith(false));
  });

  it("toggle on calls SetCookiesEnabled(true)", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: false }));
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Uploaded but paused/i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole("switch"));
    await waitFor(() => expect(bindings.SetCookiesEnabled).toHaveBeenCalledWith(true));
  });

  it("Remove button calls RemoveCookies and refreshes state", async () => {
    bindings.CookiesState.mockResolvedValueOnce(cookiesStatus({ present: true, enabled: true }))
      .mockResolvedValueOnce(cookiesStatus({ present: false }));
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Active\./i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /Remove/i }));
    await waitFor(() => expect(bindings.RemoveCookies).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.getByText(/No cookies uploaded/i)).toBeInTheDocument(),
    );
  });

  it("Verify button calls VerifyCookies and renders the returned detail", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: true }));
    bindings.VerifyCookies.mockResolvedValue(
      verifyResult({ loaded: true, authenticated: true, detail: "Signed in as test@example.com" }),
    );
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Active\./i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /Verify now/i }));
    await waitFor(() => expect(bindings.VerifyCookies).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.getByText(/Signed in as test@example.com/i)).toBeInTheDocument(),
    );
  });

  it("Verify with rotated: true shows the rotation hint", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: true }));
    bindings.VerifyCookies.mockResolvedValue(
      verifyResult({ loaded: true, authenticated: true, rotated: true, detail: "Authenticated." }),
    );
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Active\./i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /Verify now/i }));
    await waitFor(() =>
      expect(screen.getByText(/YouTube rotated the cookies/i)).toBeInTheDocument(),
    );
  });

  it("Verify with loaded: false shows the error tone", async () => {
    bindings.CookiesState.mockResolvedValue(cookiesStatus({ present: true, enabled: true }));
    bindings.VerifyCookies.mockResolvedValue(
      verifyResult({ loaded: false, detail: "cookies file unreadable" }),
    );
    render(<CookiesSection />);
    await waitFor(() => expect(screen.getByText(/Active\./i)).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /Verify now/i }));
    await waitFor(() => {
      const detail = screen.getByText(/cookies file unreadable/i);
      expect(detail).toBeInTheDocument();
      expect(detail.className).toMatch(/composer-error-text/);
    });
  });
});
