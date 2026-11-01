import { useCallback, useEffect, useRef } from 'react';

/**
 * Returns a callback with a stable identity that always invokes the latest
 * function. Used to keep memoized rows/cells from re-rendering when callers
 * pass inline event handlers.
 */
export function useStableCallback<Args extends unknown[], Result>(fn: ((...args: Args) => Result) | undefined): (...args: Args) => Result | undefined {
  const ref = useRef(fn);
  useEffect(() => {
    ref.current = fn;
  });
  return useCallback((...args: Args) => ref.current?.(...args), []);
}
