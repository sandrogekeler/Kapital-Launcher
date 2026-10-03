import { useState } from 'react'
import type { ReactNode } from 'react'

interface Props {
  /** What the content is; a new id drifts the old content out and the new in. */
  id: string
  children: ReactNode
  /**
   * Classes for the container, which the leaving layer is placed over: it must
   * be positioned, and is `relative` unless the caller positions it.
   */
  className?: string
  /** Classes for each layer, the shown one and the one leaving. */
  layer?: string
}

interface Layer {
  id: string
  children: ReactNode
}

/**
 * Content that changes by drifting (issue 142): the old content fades and
 * blurs out to the left while the new fades and blurs in from the right, both
 * at once, over the drift duration. The leaving layer stays mounted over the
 * new one, inert, until its animation ends; the keyframes and the
 * reduced-motion fade live in style.css, the distance, blur and duration are
 * tokens. The first render and a change in a hidden window show the content
 * still. The leaving layer keeps the content it had when its id was current.
 */
export function Drift({ id, children, className = 'relative', layer = '' }: Props) {
  const [shown, setShown] = useState<Layer>({ id, children })
  const [leaving, setLeaving] = useState<Layer | null>(null)
  if (shown.id !== id) {
    const hidden = typeof document !== 'undefined' && document.visibilityState === 'hidden'
    setLeaving(hidden ? null : shown)
    setShown({ id, children })
  }

  return (
    <div className={className}>
      <div key={id} className={`${layer} ${leaving ? 'drift-in' : ''}`}>
        {children}
      </div>
      {leaving && (
        <div
          key={`out-${leaving.id}`}
          aria-hidden
          inert
          onAnimationEnd={() => setLeaving(null)}
          className={`${layer} drift-out pointer-events-none absolute inset-0`}
        >
          {leaving.children}
        </div>
      )}
    </div>
  )
}
