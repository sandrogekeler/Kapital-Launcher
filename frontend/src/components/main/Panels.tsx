import type { Chapter } from '../../types'
import { factValue } from '../../lib/manifest'
import { Fact } from '../ui/Fact'

interface Props {
  chapter: Chapter
  onOpenWiki: () => void
}

function PanelTitle({ children }: { children: string }) {
  return <h2 className="text-fg-faint m-0 text-xs font-medium">{children}</h2>
}

/** The three columns under the actions: the pack's facts, its changelog, and a line from the wiki. */
export function Panels({ chapter, onOpenWiki }: Props) {
  const { pack } = chapter
  const facts: [string, string][] = chapter.server
    ? [
        ['Type', 'Client visuals'],
        ['Minecraft', factValue(pack.minecraft)],
        ['Mods', factValue(pack.mods)],
        ['Server', chapter.server.address],
      ]
    : [
        ['Loader', factValue(pack.loader)],
        ['Minecraft', factValue(pack.minecraft)],
        ['Mods', factValue(pack.mods)],
        ['Memory', pack.memoryGb == null ? factValue(null) : `${pack.memoryGb} GB`],
      ]

  return (
    <section className="grid min-h-0 grow grid-cols-3">
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
        <p className="font-display m-0 text-lg leading-snug">{chapter.wiki.title}</p>
        <span className="text-fg-muted text-sm leading-normal">{chapter.wiki.line}</span>
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
