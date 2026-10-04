import type { Chapter, ChangelogEntry, PackChangelog, PackState, WikiPage } from '../../types'
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
  /** What the pack source publishes as its changelog (issue 164); undefined until read. */
  changelog: PackChangelog | undefined
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
 * entries, each clamped, so it can never be the tallest column. The entries
 * are the pack source's own changelog (issue 164), their first line each and
 * the rest in the tooltip, and the manifest's when the source has none.
 *
 * The columns are equal, and each has the same 28 px on every side: the first
 * column's text starts as far from the card's edge as the others' from their
 * divider, and the space under the row matches its sides (the author, issue
 * 162, to see how it looks; the Play button and the hero keep their gutter).
 */
export function Panels({
  chapter,
  installed,
  sizeBytes,
  packState,
  changelog,
  wikiPage,
  onOpenWiki,
}: Props) {
  const { pack } = chapter
  const wiki = wikiPage ?? chapter.wiki
  const unset = unsetLabel(installed)
  // The pack source's entries when it has some, else the manifest's, which have no details.
  const entries: (ChangelogEntry & { details?: string })[] = (
    changelog?.entries.length ? changelog.entries : chapter.changelog
  ).slice(0, CHANGELOG_SHOWN)
  const facts: [string, string, string?][] = [
    ['Version', versionValue(packState), unset],
    ['Loader', pack.type === 'client-visuals' ? 'Client visuals' : factValue(pack.loader)],
    ['Minecraft', factValue(pack.minecraft)],
    ['Mods', factValue(pack.mods)],
    ['Size', sizeValue(sizeBytes), unset],
  ]

  return (
    <section className="grid shrink-0 grid-cols-3">
      <div className="border-line flex flex-col gap-3 border-r p-7">
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
      <div className="border-line flex flex-col gap-3 border-r p-7">
        <PanelTitle>Changelog</PanelTitle>
        <div className="flex flex-col gap-3">
          {entries.length === 0 ? (
            <span className="text-fg-faint text-sm">No entries yet.</span>
          ) : (
            entries.map((entry) => (
              <div
                key={entry.version}
                className="grid grid-cols-[64px_minmax(0,1fr)] items-baseline gap-3 text-sm"
              >
                <span className="text-accent text-2xs truncate font-mono">{entry.version}</span>
                <span className="text-fg-soft line-clamp-2" title={entry.details ?? entry.summary}>
                  {entry.summary}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
      <div className="flex flex-col gap-3 p-7">
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
