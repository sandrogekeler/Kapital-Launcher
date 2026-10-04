import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react'
import type { ReactNode } from 'react'
import { ChevronDown } from '../../lib/icons'
import type { LucideIcon } from '../../lib/icons'
import { Icon } from './Icon'

/** A section as the nav lists it. */
interface NavEntry {
  id: string
  title: string
  icon: LucideIcon
  el: HTMLElement
}

type Register = (entry: NavEntry) => () => void

const NavContext = createContext<Register | null>(null)

/** How far below the top of the scrolling page a section's heading counts as reached, in px. */
const REACHED = 80

/** The nearest ancestor that scrolls: the page's own Scrollable. */
function scroller(el: HTMLElement | null): HTMLElement | null {
  for (let p = el?.parentElement ?? null; p; p = p.parentElement) {
    const y = getComputedStyle(p).overflowY
    if (y === 'auto' || y === 'scroll') return p
  }
  return null
}

/**
 * A settings page's body (issue 188): the sections in one column, and beside it a nav of those
 * sections that follows the scroll and jumps on a click. The nav is built from the sections
 * that are actually there, each registering itself as it mounts, so a section that renders
 * nothing (a chapter without a server, the lazy mods list before it loads) is not listed.
 * In a narrow page the nav becomes a bar along the top that stays in view.
 */
export function SettingsLayout({ children }: { children: ReactNode }) {
  const [entries, setEntries] = useState<readonly NavEntry[]>([])
  const register = useCallback<Register>((entry) => {
    setEntries((prev) =>
      [...prev.filter((e) => e.id !== entry.id), entry].sort((a, b) =>
        a.el.compareDocumentPosition(b.el) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1,
      ),
    )
    return () => setEntries((prev) => prev.filter((e) => e !== entry))
  }, [])

  return (
    <NavContext.Provider value={register}>
      <div className="@container">
        <div className="flex flex-col gap-6 @2xl:flex-row @2xl:justify-center @2xl:gap-10">
          <SettingsNav entries={entries} />
          <div className="flex max-w-160 min-w-0 grow flex-col">{children}</div>
        </div>
      </div>
    </NavContext.Provider>
  )
}

function SettingsNav({ entries }: { entries: readonly NavEntry[] }) {
  const nav = useRef<HTMLElement>(null)
  const bar = useRef<HTMLSpanElement>(null)
  const [active, setActive] = useState<string | null>(null)

  // The section in view is the last whose heading has passed the line under the page's top;
  // at the very bottom it is the last, which may never reach that line.
  useEffect(() => {
    const el = scroller(nav.current)
    const first = entries.at(0)
    const last = entries.at(-1)
    if (!el || !first || !last) return
    const measure = () => {
      const line = el.getBoundingClientRect().top + REACHED
      let current = first.id
      for (const e of entries) if (e.el.getBoundingClientRect().top <= line) current = e.id
      const atEnd = el.scrollTop > 0 && el.scrollTop + el.clientHeight >= el.scrollHeight - 2
      setActive(atEnd ? last.id : current)
    }
    measure()
    el.addEventListener('scroll', measure, { passive: true })
    return () => el.removeEventListener('scroll', measure)
  }, [entries])

  // The accent bar slides to the active entry. Its place is measured, so it is set as two
  // custom properties on the element, as RangeField sets its fill.
  useLayoutEffect(() => {
    const item = nav.current?.querySelector<HTMLElement>('[aria-current="true"]')
    if (!item || !bar.current) return
    bar.current.style.setProperty('--nav-y', `${item.offsetTop}px`)
    bar.current.style.setProperty('--nav-h', `${item.offsetHeight}px`)
  }, [active, entries])

  if (entries.length === 0) return null
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  return (
    <nav
      ref={nav}
      aria-label="Sections"
      className="bg-raised border-line sticky top-0 z-10 -mx-7 flex shrink-0 gap-1 overflow-x-auto border-b px-7 py-2.5 @2xl:top-7 @2xl:mx-0 @2xl:w-max @2xl:min-w-36 @2xl:flex-col @2xl:self-start @2xl:overflow-visible @2xl:border-b-0 @2xl:border-l @2xl:bg-transparent @2xl:py-0 @2xl:pr-0 @2xl:pl-3"
    >
      <span
        ref={bar}
        aria-hidden
        className="bg-accent duration-reveal ease-standard rounded-pill absolute top-0 -left-px hidden h-(--nav-h) w-0.5 translate-y-(--nav-y) transition-[translate,height] motion-reduce:transition-none @2xl:block"
      />
      {entries.map((e) => {
        const on = e.id === active
        return (
          <button
            key={e.id}
            type="button"
            aria-current={on}
            aria-label={`Go to ${e.title}`}
            onClick={() =>
              e.el.scrollIntoView({ block: 'start', behavior: reduced ? 'auto' : 'smooth' })
            }
            className={`duration-fast ease-standard flex shrink-0 cursor-pointer items-center gap-2 rounded-sm px-2.5 py-1.5 text-left text-sm whitespace-nowrap transition-colors ${
              on
                ? 'bg-raised-2 text-fg font-medium'
                : 'text-fg-muted hover:text-fg hover:bg-raised-2'
            }`}
          >
            <Icon icon={e.icon} size="sm" className={on ? 'text-accent' : 'text-fg-faint'} />
            {e.title}
          </button>
        )
      })}
    </nav>
  )
}

interface SectionProps {
  title: string
  icon: LucideIcon
  /** A short fact at the right of the heading: a count, a version. */
  aside?: ReactNode
  /** Starts closed behind its heading, for a section most players never need. */
  collapsible?: boolean
  children: ReactNode
}

/**
 * One section of a settings page: an icon tile and the title, then its body. A full-width rule
 * and a wide gap separate it from the section before. Listed in the page's nav while it is
 * mounted.
 */
export function SettingsSection({
  title,
  icon,
  aside,
  collapsible = false,
  children,
}: SectionProps) {
  const register = useContext(NavContext)
  const ref = useRef<HTMLElement>(null)
  const id = useId()
  const bodyId = `${id}-body`
  const [open, setOpen] = useState(!collapsible)

  useLayoutEffect(() => {
    if (!register || !ref.current) return
    return register({ id, title, icon, el: ref.current })
  }, [register, id, title, icon])

  const heading = (
    <>
      <span className="bg-accent-wash border-accent-edge text-accent flex size-9 shrink-0 items-center justify-center rounded-md border">
        <Icon icon={icon} />
      </span>
      <span className="font-display text-fg text-xl font-semibold">{title}</span>
    </>
  )
  return (
    <section
      ref={ref}
      aria-label={title}
      className="border-line-strong flex scroll-mt-16 flex-col gap-4 py-10 first:pt-0 @2xl:scroll-mt-7 [&+&]:border-t"
    >
      <div className="flex items-center gap-3.5">
        {collapsible ? (
          <h2 className="m-0 grow">
            <button
              type="button"
              aria-expanded={open}
              aria-controls={bodyId}
              onClick={() => setOpen((o) => !o)}
              className="group flex w-full cursor-pointer items-center gap-3.5 text-left"
            >
              {heading}
              <Icon
                icon={ChevronDown}
                size="sm"
                className={`text-fg-muted group-hover:text-fg duration-fast ease-standard transition-transform motion-reduce:transition-none ${
                  open ? 'rotate-180' : ''
                }`}
              />
            </button>
          </h2>
        ) : (
          <h2 className="m-0 flex grow items-center gap-3.5">{heading}</h2>
        )}
        {aside && <span className="text-fg-muted shrink-0 text-xs">{aside}</span>}
      </div>
      <div id={bodyId} className={open ? 'flex flex-col gap-3' : 'hidden'}>
        {open && children}
      </div>
    </section>
  )
}

/**
 * The bordered card a section's settings sit in, one row each with a rule between. Each child is
 * a row and gets the row's padding.
 */
export function SettingsCard({ children, label }: { children: ReactNode; label?: string }) {
  return (
    <div
      role={label ? 'group' : undefined}
      aria-label={label}
      className="bg-raised-2 border-line divide-line flex flex-col divide-y rounded-lg border *:px-4 *:py-3.5"
    >
      {children}
    </div>
  )
}
