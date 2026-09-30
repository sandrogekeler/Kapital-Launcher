import type { Ref } from 'react'
import type { Chapter } from '../../types'
import { chapterArt } from '../../lib/art'

interface Props {
  chapter: Chapter
  current: boolean
  onSelect: (id: string) => void
  /** The nav measures the row to place the gliding highlight behind it (#59). */
  ref?: Ref<HTMLButtonElement>
}

/**
 * One row of the chapter nav: thumbnail, number and name in the chapter's own
 * accent, era underneath, and a dot that lights when the chapter is open. The
 * selected row's background is the nav's highlight, drawn behind the rows,
 * so the row itself only stops its hover tint while it is the one open.
 *
 * The row is scoped to its chapter with data-chapter, so `text-accent` inside
 * it resolves to that chapter's colour regardless of which chapter the page
 * is showing. That is the whole accent mechanism: one attribute, no per-colour
 * classes.
 */
export function ChapterButton({ chapter, current, onSelect, ref }: Props) {
  const art = chapterArt(chapter.id)
  return (
    <button
      ref={ref}
      type="button"
      data-chapter={chapter.id}
      aria-current={current ? 'true' : undefined}
      onClick={() => onSelect(chapter.id)}
      className={`duration-fast ease-standard relative flex w-full cursor-pointer items-center gap-3 rounded-lg border border-transparent p-2.5 text-left transition-colors ${
        current ? '' : 'hover:bg-raised-2'
      }`}
    >
      {art ? (
        <img
          src={art}
          alt=""
          className="border-line-strong size-10 shrink-0 rounded-sm border object-cover"
          draggable={false}
        />
      ) : (
        <div
          className="art-pending art-pending-thumb border-line-strong size-10 shrink-0 rounded-sm border"
          aria-hidden
        />
      )}
      <div className="flex min-w-0 grow flex-col gap-0.5">
        <div className="flex items-baseline gap-2">
          <span className="text-accent text-2xs font-mono">{chapter.number}</span>
          <span className="font-display truncate text-lg font-semibold">{chapter.name}</span>
        </div>
        <span className="text-fg-faint text-xs">{chapter.era}</span>
      </div>
      <span
        className={`rounded-pill size-1.75 shrink-0 ${current ? 'bg-accent' : 'bg-transparent'}`}
        aria-hidden
      />
    </button>
  )
}
