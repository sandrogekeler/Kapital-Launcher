import type { ServerStatus } from '../types'
import { isPlaceholderAddress } from './manifest'

/**
 * The state line's two rows for a chapter with a server: what the last ping
 * said, and the address with the detail that goes with that answer. An address
 * nobody has settled yet is never pinged for real, so the line says so rather
 * than reporting the placeholder host as offline.
 */
export function serverLine(status: ServerStatus | undefined, address: string): [string, string] {
  if (isPlaceholderAddress(address)) return ['○ No server yet', 'Address pending']
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
