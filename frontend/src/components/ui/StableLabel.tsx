interface Props {
  /** The label showing now; it must be one of `labels`. */
  current: string
  /** Every label this control can carry. */
  labels: readonly string[]
}

/**
 * A label that keeps the width of the widest it can say. The labels share one
 * grid cell and all but the current one are invisible, so the control keeps
 * its box when its words change and whatever sits beside it never moves.
 * Hidden ones are also aria-hidden, so a screen reader reads only the current.
 */
export function StableLabel({ current, labels }: Props) {
  return (
    <span className="inline-grid">
      {labels.map((label) => (
        <span
          key={label}
          aria-hidden={label !== current || undefined}
          className={`col-start-1 row-start-1 ${label === current ? '' : 'invisible'}`}
        >
          {label}
        </span>
      ))}
    </span>
  )
}
