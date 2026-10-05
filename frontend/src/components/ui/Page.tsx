import type { ReactNode } from 'react'
import { ArrowLeft } from '../../lib/icons'
import { useEscapeClose } from '../../lib/useEscapeClose'
import { CARD } from './card'
import { IconButton } from './IconButton'
import { Reveal } from './Reveal'
import { Scrollable } from './Scrollable'
import { SettingsLayout } from './SettingsLayout'

interface HeaderProps {
  title: string
  onBack: () => void
  /** Controls at the right end of the header, such as Open folder. */
  actions?: ReactNode
  /**
   * A small line above the title, in the accent, saying whose page it is: the chapter's name
   * over its settings, "Launcher" over the launcher's own (issue 188).
   */
  eyebrow?: string
}

/** The header every page shares: Back, the title in the display face, and an optional action. */
export function PageHeader({ title, onBack, actions, eyebrow }: HeaderProps) {
  return (
    <div className="border-line flex shrink-0 items-center gap-3 border-b px-7 py-5">
      <IconButton icon={ArrowLeft} title="Back" onClick={onBack} />
      <div className="flex min-w-0 flex-col gap-0.5">
        {eyebrow && (
          <span className="text-accent text-2xs tracking-eyebrow truncate font-medium uppercase">
            {eyebrow}
          </span>
        )}
        <h1 className="font-display m-0 text-2xl leading-tight font-semibold">{title}</h1>
      </div>
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
  /**
   * The body is as tall as the card leaves and does not scroll: for a page that
   * is one view filling the space, which scrolls inside itself (the logs).
   */
  fill?: boolean
  /**
   * A settings page (issue 188): the body is SettingsLayout, its sections in one column with the nav
   * of them beside it, and no other measure.
   */
  settings?: boolean
  children: ReactNode
}

/**
 * A page of the launcher (settings, a chapter's settings, a run report, the
 * logs): the chapter card's own frame, so moving between a chapter and a page
 * changes what is in the card and not the card. The inset and the slide are
 * the page layer's (components/main/PageLayer). The header stays at the top
 * while the body scrolls beneath it, with the app's own scrollbar, and the
 * body reveals once `ready`. Escape closes the page, as Back does.
 */
export function Page({
  label,
  title,
  onBack,
  actions,
  ready = true,
  fill,
  settings,
  eyebrow,
  children,
}: PageProps) {
  useEscapeClose(onBack)
  return (
    <section aria-label={label} className={CARD}>
      <PageHeader title={title} onBack={onBack} actions={actions} eyebrow={eyebrow} />
      {settings ? (
        <Scrollable>
          <div className="px-7 pt-7 pb-14">
            <Reveal ready={ready}>
              <SettingsLayout>{children}</SettingsLayout>
            </Reveal>
          </div>
        </Scrollable>
      ) : fill ? (
        <div className="flex min-h-0 grow flex-col p-7">
          <Reveal ready={ready} className="flex min-h-0 grow flex-col gap-3">
            {children}
          </Reveal>
        </div>
      ) : (
        <Scrollable>
          <div className="max-w-200 p-7">
            <Reveal ready={ready} className="flex flex-col gap-10">
              {children}
            </Reveal>
          </div>
        </Scrollable>
      )}
    </section>
  )
}
