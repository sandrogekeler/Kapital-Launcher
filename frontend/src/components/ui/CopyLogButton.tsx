import { useEffect, useState } from 'react'
import { Check, Copy, X } from '../../lib/icons'
import { Button } from './Button'
import { Icon } from './Icon'

/** How long the button says Copied, and how long Copy failed, before it goes back. */
export const COPIED_MS = 1000
export const FAILED_MS = 2000

/** The outcome of one copy: an error when it failed, nothing to say when it worked. */
export interface CopyResult {
  error?: string
}

interface Props {
  onClick: () => void
  /**
   * The outcome of the last copy. Each attempt is a new object, which is what
   * starts the button's flash, so a result that is merely pushed again does
   * not; null or undefined before any attempt.
   */
  result?: CopyResult | null
  disabled?: boolean
}

const LABELS = ['Copy log', 'Copied', 'Copy failed'] as const

/**
 * The Copy log button, which says how it went by changing itself and not by
 * adding a line under it: Copied with a check for a second, Copy failed with a
 * cross for two, then back to the Copy icon and label. The three labels share
 * one box, so the buttons after it never move. `aria-live` on the button is
 * what announces the change.
 */
export function CopyLogButton({ onClick, result, disabled }: Props) {
  // The result being flashed, as a fresh object per attempt so a second copy
  // inside the window starts its timer again.
  const [flash, setFlash] = useState<{ failed: boolean } | null>(null)
  const [seen, setSeen] = useState(result)
  if (result !== seen) {
    setSeen(result)
    setFlash(result ? { failed: Boolean(result.error) } : null)
  }
  useEffect(() => {
    if (!flash) return
    const timer = setTimeout(() => setFlash(null), flash.failed ? FAILED_MS : COPIED_MS)
    return () => clearTimeout(timer)
  }, [flash])

  const current = flash ? (flash.failed ? 'Copy failed' : 'Copied') : 'Copy log'
  return (
    <Button onClick={onClick} disabled={disabled} live>
      <Icon icon={flash ? (flash.failed ? X : Check) : Copy} size="sm" />
      <span className="inline-grid">
        {LABELS.map((label) => (
          <span
            key={label}
            aria-hidden={label !== current || undefined}
            className={`col-start-1 row-start-1 ${label === current ? '' : 'invisible'}`}
          >
            {label}
          </span>
        ))}
      </span>
    </Button>
  )
}
