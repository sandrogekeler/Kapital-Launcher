import type { ReactNode } from 'react'
import { ArrowLeft } from '../../lib/icons'
import { useEscapeClose } from '../../lib/useEscapeClose'
import { CARD } from './card'
import { IconButton } from './IconButton'
import { Reveal } from './Reveal'
import { Scrollable } from './Scrollable'

export { CARD }

interface HeaderProps {
  title: string
  onBack: () => void
  /** Controls at the right end of the header, such as Open folder. */
  actions?: ReactNode
}

/** The header every page shares: Back, the title in the display face, and an optional action. */
export function PageHeader({ title, onBack, actions }: HeaderProps) {
  return (
    <div className="border-line flex shrink-0 items-center gap-3 border-b px-14 py-5">
      <IconButton icon={ArrowLeft} title="Back" onClick={onBack} />
      <h1 className="font-display m-0 text-2xl font-semibold">{title}</h1>
      {actions && <div className="ml-auto flex items-center gap-3">{actions}</div>}
    </div>
  )
}

interface PageProps extends HeaderProps {
  /** The section's accessible name. */
  label: string
  /**
   * Whether the page's data has loaded. The body reveals when it has, and
   * shows a faint line if the read takes a moment. A page with nothing to
   * wait for leaves it out.
   */
  ready?: boolean
  children: ReactNode
}

/**
 * A page of the launcher (settings, a chapter's settings, a run report, the
 * logs): the chapter card's own frame and inset, so moving between a chapter
 * and a page changes what is in the card and not the card. The header stays
 * at the top while the body scrolls beneath it, with the app's own scrollbar,
 * and the body reveals once `ready`. Escape closes the page, as Back does.
 */
export function Page({ label, title, onBack, actions, ready = true, children }: PageProps) {
  useEscapeClose(onBack)
  return (
    <div className="m-5 flex min-h-0 grow flex-col">
      <section aria-label={label} className={CARD}>
        <PageHeader title={title} onBack={onBack} actions={actions} />
        <Scrollable>
          <div className="max-w-200 px-14 pt-8 pb-16">
            <Reveal ready={ready} className="flex flex-col gap-10">
              {children}
            </Reveal>
          </div>
        </Scrollable>
      </section>
    </div>
  )
}
