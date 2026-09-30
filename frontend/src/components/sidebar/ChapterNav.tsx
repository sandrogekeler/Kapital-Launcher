import { useLayoutEffect, useRef, useState } from 'react'
import { useChapterStore } from '../../stores/useChapterStore'
import { ChapterButton } from './ChapterButton'

/**
 * The chapter list with one highlight behind it (#59). The highlight is a
 * single element that glides to the selected row, with the same duration and
 * easing as the chapter card's slide, so the two movements read as one. The
 * rows stay pure: they draw no selected state of their own beyond the dot.
 *
 * Its place is measured from the selected row before paint, so the first
 * render shows it in place with no transition; every later selection moves
 * it. A window resize is a re-measure, not a glide.
 */
export function ChapterNav() {
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const selectedId = useChapterStore((s) => s.selectedId)
  const select = useChapterStore((s) => s.select)

  const rows = useRef(new Map<string, HTMLElement>())
  const [place, setPlace] = useState<{ top: number; height: number } | null>(null)
  const [settled, setSettled] = useState(false)

  useLayoutEffect(() => {
    const measure = () => {
      const row = rows.current.get(selectedId)
      if (!row) return
      setPlace({ top: row.offsetTop, height: row.offsetHeight })
    }
    measure()
    // The first placement is instant; after it, moves glide.
    const settle = requestAnimationFrame(() => setSettled(true))
    window.addEventListener('resize', measure)
    return () => {
      cancelAnimationFrame(settle)
      window.removeEventListener('resize', measure)
    }
  }, [selectedId, chapters])

  return (
    <nav aria-label="Chapters" className="relative flex flex-col gap-1">
      <div className="text-fg-faint px-2.5 pb-1.5 text-xs">Chapters</div>
      {place && (
        <div
          aria-hidden
          // Measured from the selected row: the computed-value exception the
          // style rule allows, as the scroll thumb's is.
          // eslint-disable-next-line no-restricted-syntax
          style={{ transform: `translateY(${place.top}px)`, height: place.height }}
          className={`bg-raised border-line-strong pointer-events-none absolute inset-x-0 top-0 rounded-lg border ease-out motion-reduce:transition-none ${
            settled ? 'duration-slow transition-[transform,height]' : ''
          }`}
        />
      )}
      {chapters.map((c) => (
        <ChapterButton
          key={c.id}
          ref={(el) => {
            if (el) rows.current.set(c.id, el)
            else rows.current.delete(c.id)
          }}
          chapter={c}
          current={c.id === selectedId}
          onSelect={select}
        />
      ))}
    </nav>
  )
}
