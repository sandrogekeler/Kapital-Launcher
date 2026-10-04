import { useState } from 'react'
import type { ReactNode } from 'react'

interface Props {
  /** Whether what the content shows has loaded. Without it the content is always ready. */
  ready?: boolean
  /** Layout classes for the revealed content's wrapper. */
  className?: string
  children: ReactNode
}

/**
 * Content that should not pop in. Until `ready` it shows nothing and reserves
 * nothing: for a short grace (a token) the place is blank, and only then does
 * a faint line say the read is on its way, so a slow read is not a blank page
 * and a quick one never flashes it. When the content is ready it is mounted
 * and plays a short entrance once (a small rise out of a slight blur, a fade;
 * a plain quick fade with reduced motion); content that is ready on the first
 * render plays it at once. Once revealed it stays, so a later re-read does
 * not play it again. The keyframes are in style.css.
 */
export function Reveal({ ready = true, className = '', children }: Props) {
  const [revealed, setRevealed] = useState(ready)
  if (ready && !revealed) setRevealed(true)
  if (!ready && !revealed) {
    return <p className="reveal-wait text-fg-faint m-0 text-sm">Reading.</p>
  }
  return <div className={`reveal ${className}`}>{children}</div>
}
