import { useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { Check, ChevronDown } from '../../lib/icons'
import { Icon } from './Icon'
import { Scrollable } from './Scrollable'

/** One entry of a Select: what it is called, and the small print that goes with it. */
export interface SelectOption<T extends string = string> {
  value: T
  /** The entry's name. */
  label: string
  /** A fainter second part after the label, such as a date. */
  note?: string
  /** A small red mark at the end of the entry, such as "Crashed". */
  mark?: string
}

interface Props<T extends string> {
  /** What the control chooses. Never drawn; it is the accessible name, read with the chosen entry. */
  label: string
  /** The chosen entry's value; null when none is chosen yet, which shows `placeholder`. */
  value: T | null
  options: readonly SelectOption<T>[]
  onChange: (value: T) => void
  placeholder?: string
  disabled?: boolean
}

function Entry({ option }: { option: SelectOption }) {
  return (
    <span className="flex items-baseline gap-3 whitespace-nowrap">
      <span className="text-fg text-sm">{option.label}</span>
      {/* The spaces are for a screen reader, which joins the parts; the flex box ignores them. */}
      {option.note && (
        <>
          {' '}
          <span className="text-fg-muted text-sm">{option.note}</span>
        </>
      )}
      {option.mark && (
        <>
          {' '}
          <span className="text-danger ml-auto pl-3 text-xs font-medium">{option.mark}</span>
        </>
      )}
    </span>
  )
}

/**
 * A choice of one entry from a list that opens under the button that shows it:
 * the same 44px box as a field, as wide as its widest entry (every entry is laid
 * in the button, in one cell, and only the chosen one is seen), and a popover
 * that scrolls when it is long. A button with `aria-haspopup="listbox"` and a
 * listbox of options; the arrows, Home and End move through them, Enter or
 * Space picks, Escape closes and gives the focus back to the button, and so
 * does a click outside it. Escape is stopped here, so it never closes the page
 * the control is on.
 */
export function Select<T extends string>({
  label,
  value,
  options,
  onChange,
  placeholder,
  disabled,
}: Props<T>) {
  const id = useId()
  const labelId = `${id}-label`
  const buttonId = `${id}-button`
  const listId = `${id}-list`
  const optionId = (i: number) => `${id}-option-${i}`

  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const root = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const list = useRef<HTMLUListElement>(null)
  // Whether the active entry moved by a key, and so is to be scrolled into view:
  // one the pointer is over is already seen.
  const byKey = useRef(false)

  const chosen = options.findIndex((o) => o.value === value)

  const show = () => {
    if (disabled || options.length === 0) return
    setActive(Math.max(chosen, 0))
    byKey.current = true
    setOpen(true)
  }
  const close = (refocus: boolean) => {
    setOpen(false)
    if (refocus) button.current?.focus()
  }
  const pick = (i: number) => {
    const option = options[i]
    if (option) onChange(option.value)
    close(true)
  }
  const move = (to: number) => {
    byKey.current = true
    setActive(Math.min(Math.max(to, 0), options.length - 1))
  }

  // The focus goes into the list when it opens, so the keys act on it.
  useEffect(() => {
    if (open) list.current?.focus()
  }, [open])

  useEffect(() => {
    if (!open || !byKey.current) return
    byKey.current = false
    const item = list.current?.children[active]
    // jsdom has no scrollIntoView.
    if (item instanceof HTMLElement && typeof item.scrollIntoView === 'function') {
      item.scrollIntoView({ block: 'nearest' })
    }
  }, [open, active])

  // A press outside the control closes it, and leaves the focus where the press put it.
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      if (e.target instanceof Node && !root.current?.contains(e.target)) setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const onButtonKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      show()
    }
  }

  const onListKey = (e: KeyboardEvent<HTMLUListElement>) => {
    switch (e.key) {
      case 'ArrowDown':
        move(active + 1)
        break
      case 'ArrowUp':
        move(active - 1)
        break
      case 'Home':
        move(0)
        break
      case 'End':
        move(options.length - 1)
        break
      case 'Enter':
      case ' ':
        pick(active)
        break
      case 'Escape':
        // The page closes on Escape too; this press is the list's.
        e.stopPropagation()
        close(true)
        return
      case 'Tab':
        // Focus moves on as it would; the list does not stay open behind it.
        close(false)
        return
      default:
        return
    }
    e.preventDefault()
  }

  return (
    <div ref={root} className="relative inline-block max-w-full">
      <span id={labelId} className="sr-only">
        {label}
      </span>
      <button
        ref={button}
        id={buttonId}
        type="button"
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-labelledby={`${labelId} ${buttonId}`}
        onClick={() => (open ? close(false) : show())}
        onKeyDown={onButtonKey}
        className="bg-sunken border-line-strong hover:bg-hover duration-fast ease-standard disabled:hover:bg-sunken flex h-11 max-w-full cursor-pointer items-center gap-3 rounded-md border pr-3 pl-4 text-left transition-colors disabled:cursor-default disabled:opacity-50"
      >
        {/* Every entry in one cell, so the button is as wide as the widest;
            the chosen one is seen and the others only take room. */}
        <span className="grid min-w-0 grow">
          {chosen < 0 && (
            <span className="text-fg-muted col-start-1 row-start-1 text-sm">{placeholder}</span>
          )}
          {options.map((o, i) => (
            <span
              key={o.value}
              aria-hidden={i === chosen ? undefined : true}
              className={`col-start-1 row-start-1 ${i === chosen ? '' : 'invisible'}`}
            >
              <Entry option={o} />
            </span>
          ))}
        </span>
        <Icon icon={ChevronDown} size="sm" className="text-fg-muted" />
      </button>
      {open && (
        <div className="bg-raised-2 border-line-strong absolute top-full left-0 z-10 mt-1 flex max-h-72 w-max min-w-full overflow-hidden rounded-md border">
          <Scrollable>
            <ul
              ref={list}
              id={listId}
              role="listbox"
              tabIndex={-1}
              aria-labelledby={labelId}
              aria-activedescendant={optionId(active)}
              onKeyDown={onListKey}
              className="m-0 flex list-none flex-col p-1 outline-none"
            >
              {options.map((o, i) => (
                <li
                  key={o.value}
                  id={optionId(i)}
                  role="option"
                  aria-selected={i === chosen}
                  onClick={() => pick(i)}
                  onMouseMove={() => setActive(i)}
                  className={`flex cursor-pointer items-center gap-3 rounded-sm px-3 py-2 ${
                    i === active ? 'bg-hover' : ''
                  }`}
                >
                  <span className="min-w-0 grow">
                    <Entry option={o} />
                  </span>
                  <span className="size-(--layout-icon-sm) shrink-0">
                    {i === chosen && <Icon icon={Check} size="sm" className="text-accent" />}
                  </span>
                </li>
              ))}
            </ul>
          </Scrollable>
        </div>
      )}
    </div>
  )
}
