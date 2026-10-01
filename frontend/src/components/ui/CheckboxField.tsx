import { useId } from 'react'

interface Props {
  label: string
  checked: boolean
  /** A line under the box: what it does. */
  hint?: string
  /** The backend's rejection, shown under the box in place of the hint. */
  error?: string | null
  /** Called with the new state on a click; the caller saves it. */
  onChange: (checked: boolean) => void
}

/**
 * One labelled on/off setting that saves the moment it is clicked, the
 * checkbox counterpart of TextField. The box shows what the store holds, so a
 * rejected save puts it back with the error underneath.
 */
export function CheckboxField({ label, checked, hint, error, onChange }: Props) {
  const id = useId()
  const line = error ?? hint
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-fg flex w-fit cursor-pointer items-center gap-3 text-sm">
        <input
          id={id}
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          aria-describedby={line ? `${id}-line` : undefined}
          className="accent-accent size-4 cursor-pointer"
        />
        {label}
      </label>
      {line && (
        <span
          id={`${id}-line`}
          className={`text-xs leading-normal select-text ${error ? 'text-danger' : 'text-fg-faint'}`}
        >
          {line}
        </span>
      )}
    </div>
  )
}
