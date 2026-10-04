import type { Chapter, PackState, WikiPage } from '../../types'
import { factValue, sizeValue, unsetLabel } from '../../lib/manifest'
import { versionValue } from '../../lib/packState'
import { Drift } from '../ui/Drift'
import { Fact } from '../ui/Fact'
import { TextLink } from '../ui/TextLink'

interface Props {
  chapter: Chapter
  /** Whether the chapter's instance exists; undefined when that cannot be looked up. */
  installed: boolean | undefined
  /** The chapter's size on disk, from the instance report; undefined until it is installed. */
  sizeBytes: number | undefined
  /** Whether the installed pack is its source's current one (#71); undefined until checked. */
  packState: PackState | undefined
  /** The slide's wiki page (#58, issue 142); undefined shows the manifest's teaser. */
  wikiPage: WikiPage | undefined
  onOpenWiki: () => void
}

/** How many of the latest changelog entries the column shows, whatever the manifest holds. */
const CHANGELOG_SHOWN = 3

function PanelTitle({ children }: { children: string }) {
  return <h2 className="text-fg-faint m-0 text-xs font-medium">{children}</h2>
}

/**
 * The three columns under the actions: the pack's facts, its changelog, and
 * a line from the wiki. The facts are what a player compares packs by (#57):
 * the installed pack's version (#71), the loader, the game version, the mod
 * count and the size on disk. A chapter that is not installed has no version
 * and no size, and says so.
 * Memory is edited in the chapter's own settings (#36), and the server is on
 * the action bar's state line with its ping.
 *
 * The row keeps one height whatever it holds, because the hero takes what the
 * panels leave: the wiki post's title is one line and its text reserves four,
 * with its link pinned under them, and the changelog shows only its latest
 * entries, each clamped, so it can never be the tallest column.
 *
 * The columns are equal, with the same padding on both sides of each divider,
 * and the row's own inset makes the first column's text start on the card's
 * gutter, the line the Play button and the hero's title stand on.
 */
export function Panels({ chapter, installed, sizeBytes, packState, wikiPage, onOpenWiki }: Props) {
  const { pack } = chapter
  const wiki = wikiPage ?? chapter.wiki
  const unset = unsetLabel(installed)
  const facts: [string, string, string?][] = [
    ['Version', versionValue(packState), unset],
    ['Loader', pack.type === 'client-visuals' ? 'Client visuals' : factValue(pack.loader)],
    ['Minecraft', factValue(pack.minecraft)],
    ['Mods', factValue(pack.mods)],
    ['Size', sizeValue(sizeBytes), unset],
  ]

  return (
    <section className="grid shrink-0 grid-cols-3 px-7">
      <div className="border-line flex flex-col gap-3 border-r px-7 py-6">
        <PanelTitle>Pack</PanelTitle>
        <div className="flex flex-col gap-3.5">
          {facts.map(([label, value, unsetText]) => (
            <Fact
              key={label}
              label={label}
              value={value}
              unset={unsetText}
              plain={value === 'Client visuals'}
            />
          ))}
        </div>
      </div>
      <div className="border-line flex flex-col gap-3 border-r px-7 py-6">
        <PanelTitle>Changelog</PanelTitle>
        <div className="flex flex-col gap-3">
          {chapter.changelog.length === 0 ? (
            <span className="text-fg-faint text-sm">No entries yet.</span>
          ) : (
            chapter.changelog.slice(0, CHANGELOG_SHOWN).map((entry) => (
              <div
                key={entry.version}
                className="grid grid-cols-[64px_minmax(0,1fr)] items-baseline gap-3 text-sm"
              >
                <span className="text-accent text-2xs font-mono">{entry.version}</span>
                <span className="text-fg-soft line-clamp-2" title={entry.summary}>
                  {entry.summary}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
      <div className="flex flex-col gap-3 px-7 py-6">
        <PanelTitle>From the wiki</PanelTitle>
        {/* The post drifts to the next with the hero's picture (issue 142). */}
        <Drift
          id={wikiPage?.url ?? 'teaser'}
          className="relative flex grow flex-col"
          layer="flex grow flex-col gap-3"
        >
          <p className="font-display m-0 line-clamp-1 text-lg leading-snug" title={wiki.title}>
            {wiki.title}
          </p>
          <p className="text-fg-muted m-0 line-clamp-4 min-h-[4lh] text-sm leading-normal">
            {wiki.line}
          </p>
          <TextLink className="mt-auto self-start text-left" onClick={onOpenWiki}>
            Read the history →
          </TextLink>
        </Drift>
      </div>
    </section>
  )
}
