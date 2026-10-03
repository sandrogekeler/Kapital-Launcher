import type { Chapter } from '../../types'
import { chapterArt, chapterTitleArt } from '../../lib/art'
import { factValue, isPlaceholder, stateLabel } from '../../lib/manifest'
import { BookOpen, Pencil } from '../../lib/icons'
import { Drift } from '../ui/Drift'
import { IconButton } from '../ui/IconButton'
import { Pill } from '../ui/Pill'

interface Props {
  chapter: Chapter
  /** The slide's picture (issue 142); undefined shows the bundled art. */
  art?: string
  onOpenWiki: () => void
  /** Opens the chapter's own settings: memory and JVM preset (#36). */
  onOpenSettings: () => void
}

/**
 * The chapter's picture, name and blurb. Art is the slide's picture from the
 * wiki, which drifts to the next one in place (issue 142), else the bundled
 * screenshot, else the accent grid. The hero takes whatever height the card has left above the
 * action bar and the panels, up to the layout token, so the card fits the
 * window without scrolling (#68).
 */
export function Hero({ chapter, art: slideArt, onOpenWiki, onOpenSettings }: Props) {
  const art = slideArt ?? chapterArt(chapter.id)
  const title = chapterTitleArt(chapter.id)
  const loader = isPlaceholder(chapter.pack.loader) ? '[Loader]' : chapter.pack.loader
  const mc = isPlaceholder(chapter.pack.minecraft) ? '[MC version]' : chapter.pack.minecraft
  const mods = chapter.pack.mods == null ? '[N] mods' : `${factValue(chapter.pack.mods)} mods`

  return (
    <section className="border-line relative flex max-h-(--layout-hero) min-h-0 grow flex-col justify-end overflow-hidden border-b px-14 py-11">
      {art ? (
        <>
          <Drift id={art} className="absolute inset-0" layer="absolute inset-0">
            <img
              src={art}
              alt=""
              className="size-full object-cover object-[center_40%]"
              draggable={false}
            />
          </Drift>
          <div className="scrim-hero absolute inset-0" aria-hidden />
        </>
      ) : (
        <>
          <div className="art-pending absolute inset-0" aria-hidden />
          <div className="text-accent text-2xs tracking-label absolute top-6 left-14 font-mono">
            [ {chapter.name.toUpperCase()} SCREENSHOT ]
          </div>
        </>
      )}

      {/* A wash behind the tools, so they read on bright art too (#77). */}
      <div className="bg-sunken/55 absolute top-5 right-6 flex gap-1 rounded-md backdrop-blur-sm">
        <IconButton icon={Pencil} title={`${chapter.name} settings`} onClick={onOpenSettings} />
        <IconButton icon={BookOpen} title="Read the history on the wiki" onClick={onOpenWiki} />
      </div>

      <div className="relative flex max-w-140 flex-col gap-3.5">
        <div className="text-fg-muted flex items-baseline gap-2.5 text-sm">
          <span className="text-accent font-mono">{chapter.number}</span>
          <span>{chapter.kind}</span>
        </div>
        {/* The heading is the artwork when there is one, at one height for
            every chapter (#77); its alt keeps the name for readers. */}
        <h1 className="font-display text-display m-0 font-semibold">
          {title ? (
            <img
              src={title}
              alt={chapter.name}
              className="block h-(--layout-title) w-auto max-w-full"
              draggable={false}
            />
          ) : (
            chapter.name
          )}
        </h1>
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
