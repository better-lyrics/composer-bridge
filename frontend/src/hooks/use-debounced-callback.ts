import { useEffect, useRef, useCallback } from "react";

// useDebouncedCallback returns a stable function that defers `fn` until `delayMs`
// has elapsed since the last call. Re-running the returned function resets the
// timer. The latest `fn` is always invoked (closure refresh via ref) so callers
// don't have to memoise it.
export function useDebouncedCallback<TArgs extends unknown[]>(
  fn: (...args: TArgs) => void,
  delayMs: number,
): (...args: TArgs) => void {
  const fnRef = useRef(fn);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingArgsRef = useRef<TArgs | null>(null);

  useEffect(() => {
    fnRef.current = fn;
  }, [fn]);

  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
        if (pendingArgsRef.current) {
          fnRef.current(...pendingArgsRef.current);
          pendingArgsRef.current = null;
        }
      }
    };
  }, []);

  return useCallback(
    (...args: TArgs) => {
      if (timerRef.current) clearTimeout(timerRef.current);
      pendingArgsRef.current = args;
      timerRef.current = setTimeout(() => {
        timerRef.current = null;
        pendingArgsRef.current = null;
        fnRef.current(...args);
      }, delayMs);
    },
    [delayMs],
  );
}
