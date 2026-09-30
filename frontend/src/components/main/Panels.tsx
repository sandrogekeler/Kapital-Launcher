import type { Chapter, WikiPage } from '../../types'
import { factValue, sizeValue } from '../../lib/manifest'
import { Fact } from '../ui/Fact'

interface Props {
  chapter: Chapter
  /** The chapter's size on disk, from the instance report; undefined until it is installed. */
  sizeBytes: number | undefined
  /** The wiki page picked for this chapter (#58); undefined shows the manifest's teaser. */
  wikiPage: WikiPage | undefined
  onOpenWiki: () => void
}

function PanelTitle({ children }: { children: string }) {
  return <h2 className="text-fg-faint m-0 text-xs font-medium">{children}</h2>
}

/**
 * The three columns under the actions: the pack's four facts, its changelog,
 * and a line from the wiki. The facts are what a player compares packs by
 * (#57): the loader, the game version, the mod count and the size on disk.
 * Memory is edited in the chapter's own settings (#36), and the server is on
 * the action bar's state line with its ping.
 */
export function Panels({ chapter, sizeBytes, wikiPage, onOpenWiki }: Props) {
  const { pack } = chapter
  const wiki = wikiPage ?? chapter.wiki
  const facts: [string, string][] = [
    ['Loader', pack.type === 'client-visuals' ? 'Client visuals' : factValue(pack.loader)],
    ['Minecraft', factValue(pack.minecraft)],
    ['Mods', factValue(pack.mods)],
    ['Size', sizeValue(sizeBytes)],
  ]

  return (
    <section className="grid shrink-0 grid-cols-3">
      <div className="border-line flex flex-col gap-3 border-r py-6 pr-7 pl-14">
        <PanelTitle>Pack</PanelTitle>
        <div className="flex flex-col gap-3.5">
          {facts.map(([label, value]) => (
            <Fact key={label} label={label} value={value} />
          ))}
        </div>
      </div>
      <div className="border-line flex flex-col gap-3 border-r px-7 py-6">
        <PanelTitle>Changelog</PanelTitle>
        <div className="flex flex-col gap-3">
          {chapter.changelog.length === 0 ? (
            <span className="text-fg-faint text-sm">No entries yet.</span>
          ) : (
            chapter.changelog.map((entry) => (
              <div
                key={entry.version}
                className="grid grid-cols-[64px_minmax(0,1fr)] items-baseline gap-3 text-sm"
              >
                <span className="text-accent text-2xs font-mono">{entry.version}</span>
                <span className="text-fg-soft">{entry.summary}</span>
              </div>
            ))
          )}
        </div>
      </div>
      <div className="flex flex-col gap-3 py-6 pr-14 pl-7">
        <PanelTitle>From the wiki</PanelTitle>
        <p className="font-display m-0 text-lg leading-snug">{wiki.title}</p>
        <span className="text-fg-muted line-clamp-4 text-sm leading-normal">{wiki.line}</span>
        <button
          type="button"
          onClick={onOpenWiki}
          className="text-accent w-fit cursor-pointer text-left text-sm hover:underline"
        >
          Read the history →
        </button>
      </div>
    </section>
  )
}
