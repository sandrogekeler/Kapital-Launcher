import type { ServerStatus } from '../types'

/**
 * The state line's two rows for a chapter with a server: what the last ping
 * said, and the address with the detail that goes with that answer.
 */
export function serverLine(status: ServerStatus | undefined, address: string): [string, string] {
  if (!status?.checked) return ['○ Checking server', address]
  if (!status.online) return ['○ Server offline', `${address} · as of ${clock(status.checkedAt)}`]
  const players = `${status.players}/${status.max} players`
  return ['● Server online', `${address} · ${players} · ${status.latencyMs} ms`]
}

/** A local wall-clock time for "as of", or ? when the timestamp cannot be read. */
export function clock(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime())
    ? '?'
    : d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}
