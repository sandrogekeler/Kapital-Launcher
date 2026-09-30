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

const CARD = 'bg-raised border-line overflow-hidden rounded-lg border'

/**
 * The chapter card and its motion (#59). The open chapter's view sits in a
 * card; when the selection moves to another chapter, the card moves the way
 * the selection moved in the nav: picking a chapter further down slides the
 * old card down and out and brings the new one in from above, further up the
 * reverse. The leaving card stays mounted over the new one, inert, until its
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
    setOutgoing({ chapter: shown, direction: to > from ? 'down' : 'up' })
    setShown(chapter)
  }
  const entering = outgoing ? (outgoing.direction === 'down' ? 'card-in-down' : 'card-in-up') : ''
  const leaving = outgoing?.direction === 'down' ? 'card-out-down' : 'card-out-up'

  return (
    <div className="relative m-5 mb-8">
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
