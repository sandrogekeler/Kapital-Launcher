import { useId, useState } from 'react'
import type { ReactNode } from 'react'
import { ChevronDown } from '../../lib/icons'
import { Icon } from './Icon'

interface SectionProps {
  title: string
  children: ReactNode
  /**
   * Starts closed behind its title, which opens and closes it: for a section
   * most players never need, such as Developer. Its body is not rendered
   * while closed, so what it loads on demand waits until it is opened.
   */
  collapsible?: boolean
}

const TITLE = 'font-display text-fg m-0 text-lg font-semibold'

/**
 * A titled section of a page: the title in the display face, the body a
 * column at the standard gap. Pages stack sections ten steps apart; what
 * sits inside a section is five.
 */
export function Section({ title, children, collapsible = false }: SectionProps) {
  const [open, setOpen] = useState(!collapsible)
  const bodyId = useId()
  if (!collapsible) {
    return (
      <div className="flex flex-col gap-5">
        <h2 className={TITLE}>{title}</h2>
        {children}
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-5">
      <h2 className={TITLE}>
        <button
          type="button"
          aria-expanded={open}
          aria-controls={bodyId}
          onClick={() => setOpen((o) => !o)}
          className="hover:text-accent duration-fast ease-standard flex cursor-pointer items-center gap-2 transition-colors"
        >
          {title}
          <Icon
            icon={ChevronDown}
            size="sm"
            className={`text-fg-muted duration-fast ease-standard transition-transform motion-reduce:transition-none ${
              open ? 'rotate-180' : ''
            }`}
          />
        </button>
      </h2>
      <div id={bodyId} className={open ? 'flex flex-col gap-5' : 'hidden'}>
        {open && children}
      </div>
    </div>
  )
}

/** The heading of a block inside a section or a page, one style wherever it is. */
export function SubHeading({ id, children }: { id?: string; children: ReactNode }) {
  return (
    <h3 id={id} className="text-fg m-0 text-sm font-semibold">
      {children}
    </h3>
  )
}
