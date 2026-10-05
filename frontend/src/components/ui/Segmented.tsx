import { useId } from 'react'
import { moveRadioFocus } from '../../lib/radioKeys'
import { ErrorLine, Hint } from './Notes'

interface Option<T extends string> {
  value: T
  label: string
}

interface Props<T extends string> {
  label: string
  value: T
  options: readonly Option<T>[]
  onChange: (value: T) => void
  /** The backend's rejection, shown under the control. */
  error?: string | null
  /** A line under the label, for what the label cannot say. */
  hint?: string
  /**
   * A settings row (issue 188): the label on the left and the pill on the right of the same row, as a
   * Toggle sits. Otherwise the label is above the pill.
   */
  inline?: boolean
}

/**
 * A short row of exclusive options in one pill: the theme, the chapter a
 * preview is for. A radio group, so a screen reader hears the choice and its
 * options; the arrow keys move between them and Enter or Space picks.
 */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
  error,
  hint,
  inline,
}: Props<T>) {
  const id = useId()
  const lineId = `${id}-line`
  const line = error ? (
    <ErrorLine id={lineId}>{error}</ErrorLine>
  ) : (
    hint && <Hint id={lineId}>{hint}</Hint>
  )
  const group = (
    <div
      role="radiogroup"
      aria-labelledby={id}
      aria-describedby={error || hint ? lineId : undefined}
      onKeyDown={moveRadioFocus}
      className={`bg-sunken border-line-strong inline-flex w-fit shrink-0 items-stretch rounded-md border p-0.5 ${
        inline ? 'h-9' : 'h-11'
      }`}
    >
      {options.map((o) => {
        const selected = o.value === value
        return (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={selected}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(o.value)}
            className={`duration-fast ease-standard cursor-pointer rounded-sm px-3.5 text-sm whitespace-nowrap transition-colors ${
              selected ? 'bg-hover text-fg font-medium' : 'text-fg-muted hover:text-fg'
            }`}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )

  if (inline) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
        <div className="flex min-w-0 flex-col gap-1">
          <span id={id} className="text-fg text-sm font-medium">
            {label}
          </span>
          {line}
        </div>
        {group}
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-1.5">
      <span id={id} className="text-fg-muted text-sm">
        {label}
      </span>
      {group}
      {line}
    </div>
  )
}
