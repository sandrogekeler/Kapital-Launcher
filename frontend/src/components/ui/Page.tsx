import type { ReactNode } from 'react'
import { ArrowLeft } from '../../lib/icons'
import { useEscapeClose } from '../../lib/useEscapeClose'
import { IconButton } from './IconButton'
import { Scrollable } from './Scrollable'

/** The raised card every screen sits in; a column, so a page's body can take the height left. */
export const CARD =
  'bg-raised border-line flex min-h-0 grow flex-col overflow-hidden rounded-lg border'

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
  children: ReactNode
}

/**
 * A page of the launcher (settings, a chapter's settings, a run report): the
 * chapter card's own frame and inset, so moving between a chapter and a page
 * changes what is in the card and not the card. The header stays at the top
 * while the body scrolls beneath it, with the app's own scrollbar. Escape
 * closes the page, as Back does.
 */
export function Page({ label, title, onBack, actions, children }: PageProps) {
  useEscapeClose(onBack)
  return (
    <div className="m-5 flex min-h-0 grow flex-col">
      <section aria-label={label} className={CARD}>
        <PageHeader title={title} onBack={onBack} actions={actions} />
        <Scrollable>
          <div className="flex max-w-200 flex-col gap-10 px-14 pt-8 pb-16">{children}</div>
        </Scrollable>
      </section>
    </div>
  )
}
