import type { Ref } from 'react'
import type { Chapter } from '../../types'
import { chapterArt, chapterIcon } from '../../lib/art'

interface Props {
  chapter: Chapter
  current: boolean
  onSelect: (id: string) => void
  /** The nav measures the row to place the gliding highlight behind it (#59). */
  ref?: Ref<HTMLButtonElement>
}

/**
 * One row of the chapter nav: the pack's icon (#77), number and name in the
 * chapter's own accent, era underneath, and a dot that lights when the
 * chapter is open. The selected row's border is the nav's highlight, drawn
 * behind the rows; the row itself fades its own art in behind its text,
 * blurred, tinted toward its accent and washed, while it is the one open
 * (#69, #77), and stops its hover tint.
 *
 * The row is scoped to its chapter with data-chapter, so `text-accent` inside
 * it resolves to that chapter's colour regardless of which chapter the page
 * is showing. That is the whole accent mechanism: one attribute, no per-colour
 * classes.
 */
export function ChapterButton({ chapter, current, onSelect, ref }: Props) {
  const art = chapterArt(chapter.id)
  const icon = chapterIcon(chapter.id) ?? art
  return (
    <button
      ref={ref}
      type="button"
      data-chapter={chapter.id}
      aria-current={current ? 'true' : undefined}
      onClick={() => onSelect(chapter.id)}
      className={`duration-fast ease-standard relative isolate flex w-full cursor-pointer items-center gap-3 overflow-hidden rounded-lg border border-transparent p-2.5 text-left transition-colors ${
        current ? '' : 'hover:bg-raised-2'
      }`}
    >
      {art && (
        <div
          aria-hidden
          className={`duration-slow ease-glide pointer-events-none absolute inset-0 -z-10 transition-opacity ${
            current ? 'opacity-100' : 'opacity-0'
          }`}
        >
          <img
            src={art}
            alt=""
            className="size-full scale-125 object-cover blur-(--effect-tile-blur)"
            draggable={false}
          />
          {/* The accent's hue over the art's light: a tint toward the chapter's colour. */}
          <div className="bg-accent absolute inset-0 opacity-(--effect-tile-tint) mix-blend-color" />
          <div className="scrim-tile absolute inset-0" />
        </div>
      )}
      {icon ? (
        <img src={icon} alt="" className="size-10 shrink-0 object-contain" draggable={false} />
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
