import { useState } from "react";
import { IconPlus, IconX } from "@tabler/icons-react";
import { Button } from "@/components/button";
import { TextInput } from "@/components/text-input";

// -- Interfaces ---------------------------------------------------------------

interface OriginListInputProps {
  origins: string[];
  onChange: (origins: string[]) => void;
  ariaLabel?: string;
}

// -- Helpers ------------------------------------------------------------------

function isValidOrigin(raw: string): boolean {
  try {
    const u = new URL(raw);
    if (u.protocol !== "http:" && u.protocol !== "https:") return false;
    if (u.pathname && u.pathname !== "/") return false;
    if (u.search || u.hash) return false;
    return true;
  } catch {
    return false;
  }
}

function normalize(raw: string): string {
  return raw.replace(/\/$/, "");
}

// -- Component ----------------------------------------------------------------

const OriginListInput: React.FC<OriginListInputProps> = ({ origins, onChange, ariaLabel }) => {
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);

  const tryAdd = (raw: string) => {
    const candidates = raw
      .split(",")
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
    if (candidates.length === 0) return;

    const additions: string[] = [];
    for (const c of candidates) {
      if (!isValidOrigin(c)) {
        setError(`"${c}" is not a valid origin (must start with http:// or https://)`);
        return;
      }
      const norm = normalize(c);
      if (origins.includes(norm) || additions.includes(norm)) continue;
      additions.push(norm);
    }
    if (additions.length > 0) onChange([...origins, ...additions]);
    setDraft("");
    setError(null);
  };

  const remove = (origin: string) => {
    onChange(origins.filter((o) => o !== origin));
  };

  return (
    <div className="flex w-full flex-col gap-2" aria-label={ariaLabel}>
      {origins.length === 0 ? (
        <p className="text-xs italic text-composer-text-muted">
          No origins. Add one below or Composer won't be able to reach the bridge.
        </p>
      ) : (
        <ul className="flex flex-wrap gap-1.5">
          {origins.map((origin) => (
            <li
              key={origin}
              className="inline-flex items-center gap-1.5 rounded-md border border-composer-border bg-composer-input px-2 py-0.5 font-mono text-xs text-composer-text"
            >
              <span className="select-text">{origin}</span>
              <button
                type="button"
                onClick={() => remove(origin)}
                aria-label={`Remove ${origin}`}
                className="rounded p-0.5 text-composer-text-muted hover:text-composer-text"
              >
                <IconX size={12} />
              </button>
            </li>
          ))}
        </ul>
      )}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          tryAdd(draft);
        }}
        className="flex gap-1.5"
      >
        <TextInput
          mono
          value={draft}
          onChange={(v) => {
            setDraft(v);
            if (error) setError(null);
          }}
          placeholder="https://composer.boidu.dev"
          ariaLabel="New origin"
          className="flex-1"
        />
        <Button
          type="submit"
          size="sm"
          variant="secondary"
          disabled={draft.trim().length === 0}
          hasIcon
        >
          <IconPlus size={12} />
          Add
        </Button>
      </form>
      {error && <p className="text-xs text-composer-error-text select-text">{error}</p>}
    </div>
  );
};

// -- Exports ------------------------------------------------------------------

export { OriginListInput };
