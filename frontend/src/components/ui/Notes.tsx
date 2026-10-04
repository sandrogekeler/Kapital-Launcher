import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  /** Lets a control point at the line with aria-describedby. */
  id?: string
}

/**
 * The three kinds of small print under a control, each in one style wherever it
 * appears: a hint says what a control does, a warning says why it is held, an
 * error says what the backend refused. They are capped at a readable measure so
 * a long line does not run the width of the page.
 */

export function Hint({ children, id }: Props) {
  return (
    <p id={id} className="text-fg-faint m-0 max-w-prose text-xs leading-normal">
      {children}
    </p>
  )
}

export function WarningLine({ children, id }: Props) {
  return (
    <p id={id} className="text-warning m-0 max-w-prose text-xs leading-normal">
      {children}
    </p>
  )
}

/** An error follows an action, so it is announced when it appears. */
export function ErrorLine({ children, id }: Props) {
  return (
    <p
      id={id}
      role="alert"
      className="text-danger m-0 max-w-prose text-xs leading-normal select-text"
    >
      {children}
    </p>
  )
}
