import type { Chapter } from '../../types'
import { chapterArt } from '../../lib/art'
import { factValue, isPlaceholder, stateLabel } from '../../lib/manifest'
import { BookOpen } from '../../lib/icons'
import { IconButton } from '../ui/IconButton'
import { Pill } from '../ui/Pill'

interface Props {
  chapter: Chapter
  onOpenWiki: () => void
}

/** The chapter's picture, name and blurb. Art is a bundled screenshot or the accent grid. */
export function Hero({ chapter, onOpenWiki }: Props) {
  const art = chapterArt(chapter.id)
  const loader = isPlaceholder(chapter.pack.loader) ? '[Loader]' : chapter.pack.loader
  const mc = isPlaceholder(chapter.pack.minecraft) ? '[MC version]' : chapter.pack.minecraft
  const mods = chapter.pack.mods == null ? '[N] mods' : `${factValue(chapter.pack.mods)} mods`

  return (
    <section className="border-line relative flex h-(--layout-hero) shrink-0 flex-col justify-end overflow-hidden border-b px-14 py-11">
      {art ? (
        <>
          <img
            src={art}
            alt=""
            className="absolute inset-0 size-full object-cover object-[center_40%]"
            draggable={false}
          />
          <div className="hero-scrim absolute inset-0" aria-hidden />
        </>
      ) : (
        <>
          <div className="art-pending absolute inset-0" aria-hidden />
          <div className="text-accent text-2xs absolute top-6 left-14 font-mono tracking-[0.06em]">
            [ {chapter.name.toUpperCase()} SCREENSHOT ]
          </div>
        </>
      )}

      <div className="absolute top-5 right-6 flex gap-1">
        <IconButton icon={BookOpen} title="Read the history on the wiki" onClick={onOpenWiki} />
      </div>

      <div className="relative flex max-w-140 flex-col gap-3.5">
        <div className="text-fg-muted flex items-baseline gap-2.5 text-sm">
          <span className="text-accent font-mono">{chapter.number}</span>
          <span>{chapter.kind}</span>
        </div>
        <h1 className="font-display text-display m-0 font-semibold">{chapter.name}</h1>
        <p className="text-fg-soft m-0 text-lg leading-normal">{chapter.blurb}</p>
        <div className="flex flex-wrap gap-2">
          <Pill>
            {loader} {mc}
          </Pill>
          <Pill>{mods}</Pill>
          <Pill>{stateLabel(chapter.state)}</Pill>
        </div>
      </div>
    </section>
  )
}
