import type { ReactNode } from 'react'

/** A fact stated as a chip: the loader, the mod count, the release state. */
export function Pill({ children }: { children: ReactNode }) {
  return (
    <span className="border-line-strong text-fg-soft rounded-pill bg-sunken/55 inline-flex h-6.5 items-center border px-2.5 text-xs">
      {children}
    </span>
  )
}
