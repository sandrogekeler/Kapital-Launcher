import { useState } from 'react'
import type { Chapter } from '../../types'
import { ChapterView } from './ChapterView'

interface Props {
  chapter: Chapter
  /** The nav's order, which decides which way a switch moves. */
  chapters: readonly Chapter[]
  onOpenSettings: (chapterId: string) => void
}

type Direction = 'down' | 'up'

interface Outgoing {
  chapter: Chapter
  direction: Direction
}

// A column, so the hero can take the height left over (#68).
const CARD = 'bg-raised border-line flex min-h-0 grow flex-col overflow-hidden rounded-lg border'

/**
 * The chapter card and its motion (#59, #67). The open chapter's view sits
 * in a card; when the selection moves to another chapter, the content moves
 * the way it would on a long page: picking a chapter higher in the nav
 * brings the new card down from above while the old one leaves downward,
 * further down the reverse. The leaving card stays mounted over the new one, inert, until its
 * animation ends. The keyframes, the blur and the reduced-motion fade live
 * in style.css; the distance and the blur are tokens.
 *
 * The previous chapter is remembered in state and compared during render,
 * React's pattern for state derived from a prop change, so the first render
 * shows the card still and every later change animates.
 */
export function ChapterStage({ chapter, chapters, onOpenSettings }: Props) {
  const [shown, setShown] = useState(chapter)
  const [outgoing, setOutgoing] = useState<Outgoing | null>(null)
  if (shown.id !== chapter.id) {
    const from = chapters.findIndex((c) => c.id === shown.id)
    const to = chapters.findIndex((c) => c.id === chapter.id)
    setOutgoing({ chapter: shown, direction: to < from ? 'down' : 'up' })
    setShown(chapter)
  }
  const entering = outgoing ? (outgoing.direction === 'down' ? 'card-in-down' : 'card-in-up') : ''
  const leaving = outgoing?.direction === 'down' ? 'card-out-down' : 'card-out-up'

  return (
    <div className="relative m-5 mb-8 flex min-h-0 grow flex-col">
      <div key={chapter.id} className={`${CARD} ${entering}`}>
        <ChapterView chapter={chapter} onOpenSettings={onOpenSettings} />
      </div>
      {outgoing && (
        <div
          key={`out-${outgoing.chapter.id}`}
          aria-hidden
          inert
          onAnimationEnd={() => setOutgoing(null)}
          className={`${CARD} ${leaving} pointer-events-none absolute inset-0`}
        >
          <ChapterView chapter={outgoing.chapter} onOpenSettings={onOpenSettings} />
        </div>
      )}
    </div>
  )
}
