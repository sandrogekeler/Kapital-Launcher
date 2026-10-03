import type { ReactNode } from 'react'

/**
 * A titled section of a page: the title in the display face, the body a
 * column at the standard gap. Pages stack sections ten steps apart; what
 * sits inside a section is five.
 */
export function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-5">
      <h2 className="font-display text-fg m-0 text-lg font-semibold">{title}</h2>
      {children}
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
