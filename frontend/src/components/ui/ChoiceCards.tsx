import { useId } from 'react'
import type { ReactNode } from 'react'
import { moveRadioFocus } from '../../lib/radioKeys'

export interface Choice<T extends string> {
  value: T
  title: string
  /** A line in the data face under the title: an address, a host. */
  detail?: string
  /** A faint line under that: why the card cannot be taken. */
  note?: string | null
  /** The fixed-width slot at the right: the state of the card, or its action. */
  slot?: ReactNode
  disabled?: boolean
}

interface Props<T extends string> {
  /** The group's accessible name. */
  label: string
  /** The chosen card's value; null when none is. */
  value: T | null
  choices: readonly Choice<T>[]
  onChange: (value: T) => void
  /** Two cards a row, for short choices side by side (the Java presets, issue 191); one otherwise. */
  columns?: 1 | 2
}

const BLANK = '\u00a0'

/**
 * Stacked cards for a choice with something to say about each option: the Java
 * preset, the pack a chapter syncs from. The chosen card takes the nav
 * highlight's look (a raised ground and the accent's edge). Every card draws
 * the same lines whether it is chosen or not, and a line that only some cards
 * have is kept, empty, on the others, so nothing changes size when the choice
 * moves. The slot is as wide for every card, so its text never moves anything.
 *
 * A radio group: the arrow keys move between the cards, Enter or Space picks.
 * A disabled card cannot be focused or picked.
 */
export function ChoiceCards<T extends string>({
  label,
  value,
  choices,
  onChange,
  columns = 1,
}: Props<T>) {
  const id = useId()
  const hasDetail = choices.some((c) => c.detail !== undefined)
  // A note line is kept on every card only when some card has one to show, so a card whose
  // notes are all empty stays one line and centred (issue 188).
  const hasNote = choices.some((c) => !!c.note)
  const hasSlot = choices.some((c) => c.slot !== undefined)
  // The one card Tab stops at: the chosen one, or the first that can be picked
  // when nothing is chosen.
  const stop =
    choices.find((c) => c.value === value && !c.disabled) ?? choices.find((c) => !c.disabled)
  return (
    <div
      role="radiogroup"
      aria-label={label}
      onKeyDown={moveRadioFocus}
      className={columns === 2 ? 'grid grid-cols-1 gap-2 @lg:grid-cols-2' : 'flex flex-col gap-2'}
    >
      {choices.map((c, i) => {
        const chosen = c.value === value
        const key = `${id}-${i}`
        return (
          <button
            key={c.value}
            type="button"
            role="radio"
            aria-checked={chosen}
            aria-labelledby={`${key}-title`}
            aria-describedby={
              [c.detail && `${key}-detail`, c.note && `${key}-note`].filter(Boolean).join(' ') ||
              undefined
            }
            disabled={c.disabled}
            tabIndex={c === stop ? 0 : -1}
            onClick={() => onChange(c.value)}
            className={`${
              chosen ? 'bg-raised border-accent-edge' : 'border-line hover:bg-hover'
            } duration-fast ease-standard flex w-full cursor-pointer items-center justify-between gap-4 rounded-lg border px-4 py-3 text-left transition-colors disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent motion-reduce:transition-none`}
          >
            <span className="flex min-w-0 flex-col gap-0.5">
              <span id={`${key}-title`} className="text-fg text-sm">
                {c.title}
              </span>
              {hasDetail && (
                <span
                  id={`${key}-detail`}
                  title={c.detail}
                  className="text-fg-muted truncate font-mono text-xs"
                >
                  {c.detail || BLANK}
                </span>
              )}
              {hasNote && (
                <span id={`${key}-note`} className="text-fg-faint truncate text-xs">
                  {c.note || BLANK}
                </span>
              )}
            </span>
            {hasSlot && <span className="w-24 shrink-0 text-right text-sm">{c.slot}</span>}
          </button>
        )
      })}
    </div>
  )
}
