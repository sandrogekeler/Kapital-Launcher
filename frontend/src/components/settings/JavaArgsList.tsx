import { useEffect, useId, useRef, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { Ellipsis, Pencil, Plus, Trash2, X } from '../../lib/icons'
import { MAX_JAVA_ARGS, javaArgProblem } from '../../lib/javaArgs'
import { Icon } from '../ui/Icon'

interface Props {
  /** The draft's own arguments, in order. */
  args: readonly string[]
  onChange: (args: string[]) => void
  /** Nothing can be added, edited or removed (the game is active). */
  disabled?: boolean
}

/** What the list is editing: an argument by its place, a new one at the end, or nothing. */
type Editing = number | 'new' | null

/**
 * The player's own Java arguments (issue 191), after the preset's: one row each, a + button
 * that adds one through a field in the list, and on each row a ⋯ menu, shown on hover or focus,
 * with Edit and Remove. A value is checked as it is committed (lib/javaArgs, which mirrors Go's
 * rule), and the list is part of the Game card's draft, so Save and Revert cover it. Escape in
 * the field or the menu closes that, not the page.
 */
export function JavaArgsList({ args, onChange, disabled }: Props) {
  const [editing, setEditing] = useState<Editing>(null)
  const [menuFor, setMenuFor] = useState<number | null>(null)
  const addRef = useRef<HTMLButtonElement>(null)
  const listId = useId()
  // After a commit, a removal or a cancel the field or the row is gone: focus goes to +.
  const [refocus, setRefocus] = useState(false)
  useEffect(() => {
    if (refocus) {
      addRef.current?.focus()
      setRefocus(false)
    }
  }, [refocus])

  const done = () => {
    setEditing(null)
    setRefocus(true)
  }
  const commit = (value: string, at: Editing) => {
    const next = [...args]
    if (at === 'new') next.push(value)
    else if (at !== null) next[at] = value
    onChange(next)
    done()
  }
  const remove = (at: number) => {
    setMenuFor(null)
    onChange(args.filter((_, i) => i !== at))
    setRefocus(true)
  }

  const full = args.length >= MAX_JAVA_ARGS
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <span id={listId} className="text-fg-soft text-xs font-medium">
          Custom arguments
        </span>
        {args.length > 0 && (
          <span className="bg-hover text-fg-muted rounded-pill text-2xs px-2 font-mono">
            {args.length}
          </span>
        )}
      </div>
      {(args.length > 0 || editing === 'new') && (
        <ul aria-labelledby={listId} className="m-0 flex list-none flex-col gap-1.5 p-0">
          {args.map((arg, i) =>
            editing === i ? (
              <ArgEditor
                key={`edit-${i}`}
                initial={arg}
                action="Save"
                check={(v) => javaArgProblem(v, args, i)}
                onCommit={(v) => commit(v, i)}
                onCancel={done}
              />
            ) : (
              <ArgRow
                key={`${i}-${arg}`}
                arg={arg}
                disabled={disabled || editing !== null}
                open={menuFor === i}
                onOpen={(open) => setMenuFor(open ? i : null)}
                onEdit={() => {
                  setMenuFor(null)
                  setEditing(i)
                }}
                onRemove={() => remove(i)}
              />
            ),
          )}
          {editing === 'new' && (
            <ArgEditor
              initial=""
              action="Add"
              check={(v) => javaArgProblem(v, args, null)}
              onCommit={(v) => commit(v, 'new')}
              onCancel={done}
            />
          )}
        </ul>
      )}
      {editing === null && (
        <button
          ref={addRef}
          type="button"
          disabled={disabled || full}
          onClick={() => setEditing('new')}
          className="text-fg-soft hover:text-fg border-line-strong hover:border-accent-edge duration-fast ease-standard flex h-8 w-fit cursor-pointer items-center gap-1.5 rounded-md border border-dashed pr-3 pl-2 text-sm transition-colors disabled:cursor-default disabled:opacity-50"
        >
          <Icon icon={Plus} size="sm" className="text-accent" />
          {full ? `At most ${MAX_JAVA_ARGS} arguments` : 'Add argument'}
        </button>
      )}
    </div>
  )
}

interface RowProps {
  arg: string
  disabled?: boolean
  open: boolean
  onOpen: (open: boolean) => void
  onEdit: () => void
  onRemove: () => void
}

/** One argument, with its ⋯ menu: shown while the row is hovered or holds the focus. */
function ArgRow({ arg, disabled, open, onOpen, onEdit, onRemove }: RowProps) {
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const menuId = useId()

  // The menu takes the focus when it opens, and closes on a press anywhere outside it. The
  // callback is read through a ref, so a new one from the parent does not take the focus back.
  const onOpenRef = useRef(onOpen)
  onOpenRef.current = onOpen
  useEffect(() => {
    if (!open) return
    menu.current?.querySelector<HTMLButtonElement>('[role="menuitem"]')?.focus()
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node
      if (!menu.current?.contains(t) && !trigger.current?.contains(t)) onOpenRef.current(false)
    }
    document.addEventListener('pointerdown', onDown)
    return () => document.removeEventListener('pointerdown', onDown)
  }, [open])

  const onMenuKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const items = Array.from(
      menu.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? [],
    )
    const at = items.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const step = e.key === 'ArrowDown' ? 1 : items.length - 1
      items[(at + step) % items.length]?.focus()
    } else if (e.key === 'Escape') {
      // The menu closes, not the page.
      e.stopPropagation()
      onOpen(false)
      trigger.current?.focus()
    } else if (e.key === 'Tab') {
      onOpen(false)
    }
  }

  return (
    <li className="group bg-sunken border-line hover:border-line-strong duration-fast ease-standard relative flex min-h-9 items-center gap-2 rounded-md border py-1 pr-1 pl-3 transition-colors">
      <code className="text-fg min-w-0 grow font-mono text-xs break-all select-text">{arg}</code>
      <button
        ref={trigger}
        type="button"
        aria-label={`Options for ${arg}`}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={disabled}
        onClick={() => onOpen(!open)}
        className="text-fg-muted hover:text-fg hover:bg-hover aria-expanded:bg-hover aria-expanded:text-fg duration-fast ease-standard flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-sm opacity-0 transition-[opacity,background-color] group-focus-within:opacity-100 group-hover:opacity-100 disabled:hidden aria-expanded:opacity-100"
      >
        <Icon icon={Ellipsis} size="sm" />
      </button>
      {open && (
        <div
          ref={menu}
          id={menuId}
          role="menu"
          aria-label={`Options for ${arg}`}
          onKeyDown={onMenuKey}
          className="bg-raised-2 border-line-strong absolute top-full right-1 z-20 mt-1 flex min-w-36 flex-col rounded-md border p-1"
        >
          <button
            type="button"
            role="menuitem"
            onClick={onEdit}
            className="text-fg hover:bg-hover focus-visible:bg-hover flex cursor-pointer items-center gap-2.5 rounded-sm px-2.5 py-1.5 text-left text-sm outline-none"
          >
            <Icon icon={Pencil} size="sm" className="text-fg-muted" />
            Edit
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={onRemove}
            className="text-danger hover:bg-hover focus-visible:bg-hover flex cursor-pointer items-center gap-2.5 rounded-sm px-2.5 py-1.5 text-left text-sm outline-none"
          >
            <Icon icon={Trash2} size="sm" />
            Remove
          </button>
        </div>
      )}
    </li>
  )
}

interface EditorProps {
  initial: string
  /** The commit button's word: Add for a new argument, Save for an edit. */
  action: 'Add' | 'Save'
  check: (value: string) => string | null
  onCommit: (value: string) => void
  onCancel: () => void
}

/** The field that adds or edits one argument, in the row's place. Enter commits, Escape cancels. */
function ArgEditor({ initial, action, check, onCommit, onCancel }: EditorProps) {
  const [value, setValue] = useState(initial)
  const [problem, setProblem] = useState<string | null>(null)
  const input = useRef<HTMLInputElement>(null)
  const lineId = useId()
  useEffect(() => {
    input.current?.focus()
    input.current?.select()
  }, [])

  const commit = () => {
    const v = value.trim()
    if (v === '' || v === initial) return onCancel()
    const why = check(v)
    if (why) {
      setProblem(why)
      input.current?.focus()
      return
    }
    onCommit(v)
  }
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Escape') {
      e.stopPropagation()
      onCancel()
    }
  }

  return (
    <li className="bg-sunken border-accent-edge flex flex-wrap items-center gap-1 rounded-md border p-1">
      <input
        ref={input}
        type="text"
        value={value}
        aria-label="Java argument"
        aria-invalid={problem ? true : undefined}
        aria-describedby={problem ? lineId : undefined}
        placeholder="-XX:+UseStringDeduplication"
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => {
          setValue(e.target.value)
          setProblem(null)
        }}
        onKeyDown={onKey}
        className="text-fg placeholder:text-fg-faint h-7 min-w-0 grow bg-transparent px-2 font-mono text-xs outline-none select-text"
      />
      <button
        type="button"
        onClick={commit}
        className="bg-hover text-fg h-7 cursor-pointer rounded-sm px-3 text-sm font-medium hover:brightness-(--effect-hover-brightness)"
      >
        {action}
      </button>
      <button
        type="button"
        aria-label="Cancel"
        onClick={onCancel}
        className="text-fg-muted hover:text-fg hover:bg-hover flex size-7 cursor-pointer items-center justify-center rounded-sm"
      >
        <Icon icon={X} size="sm" />
      </button>
      {problem && (
        <p id={lineId} role="alert" className="text-danger m-0 basis-full px-2 pb-1 text-xs">
          {problem}
        </p>
      )}
    </li>
  )
}
