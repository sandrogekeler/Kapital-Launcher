import { LogFrontendError } from '../../wailsjs/go/main/App'
import { errMsg, hasWailsBridge } from './ipc'

/**
 * Sends an error caught in the view to Go's log. A packaged build has no
 * console anyone reads, so this is how a crash reaches the file a player would
 * share. The kinds are the three places an error is caught; Go refuses any
 * other and caps how many it writes per run.
 */
export type ErrorKind = 'render' | 'error' | 'rejection'

/** What a thrown value says: Error.message, a string as is, anything else as text. */
function describe(thrown: unknown): { message: string; stack: string } {
  if (thrown instanceof Error) return { message: thrown.message, stack: thrown.stack ?? '' }
  if (typeof thrown === 'string') return { message: thrown, stack: '' }
  try {
    return { message: JSON.stringify(thrown) ?? String(thrown), stack: '' }
  } catch {
    return { message: String(thrown), stack: '' }
  }
}

/**
 * Report `thrown` as `kind`. The console always hears it; Go hears it when a
 * backend is attached. Never throws and never rejects: a reporter that fails
 * inside an error handler would only bury the first error.
 */
export function reportError(kind: ErrorKind, thrown: unknown, componentStack?: string): void {
  try {
    console.error(`${kind} error`, thrown, componentStack ?? '')
    if (!hasWailsBridge()) return
    const { message, stack } = describe(thrown)
    const full = componentStack ? `${stack}\n\nComponent stack:${componentStack}` : stack
    // The binding can also throw synchronously, which the outer try covers.
    LogFrontendError(kind, message, full).catch((e) =>
      console.warn('report frontend error', errMsg(e)),
    )
  } catch (e) {
    console.warn('report frontend error', errMsg(e))
  }
}

/**
 * Report errors that React never sees: a thrown event handler or timer
 * (`error`) and a promise nobody caught (`rejection`). Returns the cleanup,
 * for a test or a hot reload.
 */
export function installErrorReporting(target: Window = window): () => void {
  const onError = (e: ErrorEvent) => reportError('error', e.error ?? e.message)
  const onRejection = (e: PromiseRejectionEvent) => reportError('rejection', e.reason)
  target.addEventListener('error', onError)
  target.addEventListener('unhandledrejection', onRejection)
  return () => {
    target.removeEventListener('error', onError)
    target.removeEventListener('unhandledrejection', onRejection)
  }
}
