import { useEffect, useId, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'

interface Props {
  label: string
  /** The committed value. The field keeps its own draft while it is being typed. */
  value: string
  /** What an empty field shows: the value that applies when nothing is set. */
  placeholder?: string
  /** A line under the field: where the value comes from, or what it does. */
  hint?: string
  /** The backend's rejection, shown under the field in place of the hint. */
  error?: string | null
  /** Paths and addresses read better in the data face. */
  mono?: boolean
  /** Called with the trimmed draft when it differs from `value`, on Enter or blur. */
  onCommit: (value: string) => void
  /** A control at the end of the row, such as Browse. */
  trailing?: ReactNode
}

/**
 * One labelled text field that saves when it is committed: Enter, or leaving
 * the field (#5). Escape while a draft is unsaved puts the committed value
 * back and stops there; Escape on a clean field is left to the panel, which
 * closes on it.
 *
 * The draft follows `value` whenever it changes from outside, which is how a
 * rejected save reverts the field: the store puts the old value back, and the
 * field shows it again with the error underneath.
 */
export function TextField({
  label,
  value,
  placeholder,
  hint,
  error,
  mono,
  onCommit,
  trailing,
}: Props) {
  const id = useId()
  const [draft, setDraft] = useState(value)
  useEffect(() => setDraft(value), [value])
  const dirty = draft.trim() !== value

  const commit = () => {
    if (dirty) onCommit(draft.trim())
  }
  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      commit()
    } else if (e.key === 'Escape' && dirty) {
      e.stopPropagation()
      setDraft(value)
    }
  }

  const line = error ?? hint
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-fg-muted text-sm">
        {label}
      </label>
      <div className="flex items-center gap-2">
        <input
          id={id}
          type="text"
          value={draft}
          placeholder={placeholder}
          spellCheck={false}
          autoComplete="off"
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={onKeyDown}
          aria-invalid={error ? true : undefined}
          aria-describedby={line ? `${id}-line` : undefined}
          className={`bg-sunken text-fg placeholder:text-fg-faint h-11 min-w-0 grow rounded-md border px-3 text-sm select-text ${
            error ? 'border-danger' : 'border-line-strong'
          } ${mono ? 'font-mono' : ''}`}
        />
        {trailing}
      </div>
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
