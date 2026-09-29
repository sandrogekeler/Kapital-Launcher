import type { PrismInstallProgress } from '../types'

const MB = 1024 * 1024

/** Bytes as the size a player reads, one decimal: 20396629 is "19.5 MB". */
export function megabytes(bytes: number): string {
  return `${(bytes / MB).toFixed(1)} MB`
}

/**
 * The state line's two rows while Prism is being installed or updated, from
 * the latest prism:install event. `verb` is "Getting" for a first install and
 * "Updating" for an update.
 */
export function installLine(p: PrismInstallProgress, verb: string): [string, string] {
  switch (p.phase) {
    case 'downloading': {
      const pct = p.total > 0 ? Math.min(100, Math.floor((p.received / p.total) * 100)) : 0
      return [
        `◐ ${verb} Prism · ${pct}%`,
        `Downloading ${(p.received / MB).toFixed(1)} of ${megabytes(p.total)}`,
      ]
    }
    case 'unpacking':
      return [`◐ ${verb} Prism`, 'Unpacking']
    case 'verifying':
      return [`◐ ${verb} Prism`, "Checking Prism's signature"]
    case 'done':
      return [
        '● Prism is ready',
        'The first time you play, Prism asks you to sign in with Microsoft',
      ]
    case 'failed':
      return ['○ Could not get Prism', p.error || 'Something went wrong']
    default:
      return [`◐ ${verb} Prism`, '']
  }
}
