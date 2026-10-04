import { useState } from 'react'
import type { ReactNode } from 'react'
import type { Chapter } from '../../types'
import { CARD } from '../ui/card'
import { ChapterView } from './ChapterView'

interface Props {
  chapter: Chapter
  /** The nav's order, which decides which way a switch moves. */
  chapters: readonly Chapter[]
  onOpenSettings: (chapterId: string) => void
  onOpenLogs: (chapterId: string) => void
  onOpenMap: (chapterId: string) => void
  /** Whether a page covers the chapter, which leaves the view beneath it inert. */
  covered?: boolean
  /** The pages laid over the card, each a `PageLayer`. */
  children?: ReactNode
}

type Direction = 'down' | 'up'

interface Outgoing {
  chapter: Chapter
  direction: Direction
}

/**
 * The chapter card and its motion (#59, #67). The open chapter's view sits
 * in a card; when the selection moves to another chapter, the content moves
 * the way it would on a long page: picking a chapter higher in the nav
 * brings the new card down from above while the old one leaves downward,
 * further down the reverse. The leaving card stays mounted over the new one, inert, until its
 * animation ends. The keyframes, the blur and the reduced-motion fade live
 * in style.css; the distance and the blur are tokens.
 *
 * The stage clips the slide at the card's gap (`.card-stage`), so a moving
 * card never makes the main area scroll (issue 145). A page (settings, a
 * chapter's settings, its logs, a run report) is laid over the card as a layer
 * in the same stage; the card does not move for it, and while it is covered
 * the card is inert and hidden from the accessibility tree.
 *
 * The previous chapter is remembered in state and compared during render,
 * React's pattern for state derived from a prop change, so the first render
 * shows the card still and every later change animates.
 */
export function ChapterStage({
  chapter,
  chapters,
  onOpenSettings,
  onOpenLogs,
  onOpenMap,
  covered = false,
  children,
}: Props) {
  const [shown, setShown] = useState(chapter)
  const [outgoing, setOutgoing] = useState<Outgoing | null>(null)
  if (shown.id !== chapter.id) {
    const from = chapters.findIndex((c) => c.id === shown.id)
    const to = chapters.findIndex((c) => c.id === chapter.id)
    // A switch in a hidden window (the launcher minimised while a game runs)
    // is not seen, and its slide would wait there until the window came
    // back: the card is simply replaced (issue 145).
    const hidden = typeof document !== 'undefined' && document.visibilityState === 'hidden'
    setOutgoing(hidden ? null : { chapter: shown, direction: to < from ? 'down' : 'up' })
    setShown(chapter)
  }
  const entering = outgoing ? (outgoing.direction === 'down' ? 'card-in-down' : 'card-in-up') : ''
  const leaving = outgoing?.direction === 'down' ? 'card-out-down' : 'card-out-up'

  return (
    <div className="card-stage relative m-5 flex min-h-0 grow flex-col">
      <div
        key={chapter.id}
        inert={covered}
        aria-hidden={covered || undefined}
        className={`${CARD} ${entering}`}
      >
        <ChapterView
          chapter={chapter}
          onOpenSettings={onOpenSettings}
          onOpenLogs={onOpenLogs}
          onOpenMap={onOpenMap}
        />
      </div>
      {outgoing && (
        <div
          key={`out-${outgoing.chapter.id}`}
          aria-hidden
          inert
          onAnimationEnd={() => setOutgoing(null)}
          className={`${CARD} ${leaving} pointer-events-none absolute inset-0`}
        >
          <ChapterView
            chapter={outgoing.chapter}
            onOpenSettings={onOpenSettings}
            onOpenLogs={onOpenLogs}
            onOpenMap={onOpenMap}
          />
        </div>
      )}
      {children}
    </div>
  )
}
