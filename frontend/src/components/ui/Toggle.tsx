import { useId } from 'react'
import { ErrorLine, Hint } from './Notes'

interface Props {
  label: string
  checked: boolean
  /** A line under the label: what it does. */
  hint?: string
  /** The backend's rejection, shown under the label in place of the hint. */
  error?: string | null
  /** Called with the new state on a click; the caller saves it. */
  onChange: (checked: boolean) => void
}

/**
 * One labelled on/off setting that saves the moment it is clicked, the switch
 * counterpart of TextField: the label and its line on the left, the switch on
 * the right of the same row. The switch shows what the store holds, so a
 * rejected save puts it back with the error underneath. The label is the
 * control's name, so it stays in the foreground colour, and clicking it
 * toggles the switch.
 */
export function Toggle({ label, checked, hint, error, onChange }: Props) {
  const id = useId()
  const lineId = `${id}-line`
  const track = checked ? 'bg-accent border-accent' : 'bg-sunken border-line-strong'
  const thumb = checked ? 'bg-canvas translate-x-5' : 'bg-fg-muted translate-x-0'
  return (
    <div className="flex items-start justify-between gap-6">
      <div className="flex min-w-0 flex-col gap-1.5">
        <label htmlFor={id} className="text-fg w-fit cursor-pointer text-sm">
          {label}
        </label>
        {error ? (
          <ErrorLine id={lineId}>{error}</ErrorLine>
        ) : (
          hint && <Hint id={lineId}>{hint}</Hint>
        )}
      </div>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        aria-describedby={error || hint ? lineId : undefined}
        onClick={() => onChange(!checked)}
        className={`${track} duration-fast ease-standard rounded-pill inline-flex h-6 w-11 shrink-0 cursor-pointer items-center border p-0.5 transition-colors motion-reduce:transition-none`}
      >
        <span
          aria-hidden
          className={`${thumb} duration-fast ease-standard rounded-pill size-4.5 transition-[transform,background-color] motion-reduce:transition-none`}
        />
      </button>
    </div>
  )
}
