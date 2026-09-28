/**
 * Helpers for the two things a Wails rejection can mean.
 *
 * The generated bindings dereference `window.go` directly, so with no Go
 * backend every call throws synchronously. That is the `frontend-dev` preset
 * in .claude/launch.json: a browser-only Vite server, for looking at the UI.
 * A store cannot tell that apart from a real backend failure without asking,
 * so it asks here.
 */

/** Wails rejects with plain strings as often as with Errors. */
export const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e))

/**
 * Whether a Wails backend is attached at all. Reads `window.go`'s presence and
 * never calls through it, so "IPC via generated bindings only" still holds.
 */
export function hasWailsBridge(): boolean {
  return typeof window !== 'undefined' && 'go' in window
}

/**
 * Run a bound read, falling back to `fallback` however it fails. Awaiting
 * inside turns the bindings' synchronous no-bridge throw into a rejection, so
 * one handler covers it and a real backend failure alike: both mean "no
 * value, show the unavailable state".
 */
export async function readOr<T, F>(call: () => Promise<T>, fallback: F): Promise<T | F> {
  try {
    return await call()
  } catch {
    // aislop-ignore-next-line ai-slop/hidden-fallback -- the sanctioned no-bridge read path, see .claude/rules/ipc.md
    return fallback
  }
}
