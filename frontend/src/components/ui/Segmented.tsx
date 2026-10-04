import { useId } from 'react'
import { moveRadioFocus } from '../../lib/radioKeys'
import { ErrorLine } from './Notes'

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
}

/**
 * A short row of exclusive options in one pill: the theme, the chapter a
 * preview is for. A radio group, so a screen reader hears the choice and its
 * options; the arrow keys move between them and Enter or Space picks. The box
 * is the same 44px as a field and a button, so a row of them lines up.
 */
export function Segmented<T extends string>({ label, value, options, onChange, error }: Props<T>) {
  const id = useId()
  return (
    <div className="flex flex-col gap-1.5">
      <span id={id} className="text-fg-muted text-sm">
        {label}
      </span>
      <div
        role="radiogroup"
        aria-labelledby={id}
        onKeyDown={moveRadioFocus}
        className="bg-sunken border-line-strong inline-flex h-11 w-fit items-stretch rounded-md border p-0.5"
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
              className={`duration-fast ease-standard cursor-pointer rounded-sm px-4 text-sm transition-colors ${
                selected ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg'
              }`}
            >
              {o.label}
            </button>
          )
        })}
      </div>
      {error && <ErrorLine>{error}</ErrorLine>}
    </div>
  )
}
