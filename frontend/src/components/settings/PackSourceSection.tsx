import { useState } from 'react'
import type { Chapter, PackSource } from '../../types'
import { errMsg } from '../../lib/ipc'
import { packHost, isLocalPack } from '../../lib/packSource'
import { selectInstancePack, useEngineStore } from '../../stores/useEngineStore'
import { isActive, selectGame, useGameStore } from '../../stores/useGameStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { ChoiceCards } from '../ui/ChoiceCards'
import type { Choice } from '../ui/ChoiceCards'
import { ErrorLine, Hint, WarningLine } from '../ui/Notes'
import { SubHeading } from '../ui/Section'
import { RunningHint } from './RunningHint'

interface Source {
  source: PackSource
  title: string
  /** Host and port of the pack this choice syncs from, when there is one. */
  host: string | undefined
  current: boolean
  /** Why the choice cannot be taken, when it cannot. */
  reason: string | null
}

interface Props {
  chapter: Chapter
}

/**
 * Which pack an installed chapter syncs from (ADR-2, seventh amendment): its
 * published one or the developer's local `packwiz serve` from settings. Go
 * rewrites the pack address in the instance's own pre-launch command and the
 * next Play syncs from it. Shown only for a chapter that has a local pack, in
 * settings or already in its instance, since a chapter that never left its
 * published pack has nothing to switch between. Only the tracker's answer
 * (`playing`) disables a card; `running` is the log's guess, shown as a hint,
 * and Go refuses the write if it was right (issue 126).
 */
export function PackSourceSection({ chapter }: Props) {
  const installed = useEngineStore((s) => s.instances?.present[chapter.id])
  const instancePack = useEngineStore(selectInstancePack(chapter.id))
  const installing = useEngineStore((s) => s.installing === chapter.id)
  const running = useEngineStore((s) => s.chapterSettings[chapter.id]?.running ?? false)
  const setPackSource = useEngineStore((s) => s.setPackSource)
  const playing = useGameStore((s) => isActive(selectGame(chapter.id)(s)?.phase))
  const override = useSettingsStore((s) => s.settings.packOverrides?.[chapter.id]?.trim())
  const [switching, setSwitching] = useState<PackSource | null>(null)
  const [error, setError] = useState<string | null>(null)

  const local = instancePack !== undefined && isLocalPack(instancePack)
  if (installed !== true || (!override && !local)) return null

  const published = chapter.pack.packwiz ?? undefined
  // A command the launcher did not write has no pack address Go could read,
  // and Go will not rewrite it.
  const unreadable = instancePack === undefined ? 'Not made by this launcher.' : null
  const choices: Source[] = [
    {
      source: 'published',
      title: 'Published pack',
      host: published && packHost(published),
      current: published !== undefined && instancePack === published,
      reason: published ? unreadable : 'No published pack yet.',
    },
    {
      source: 'dev',
      title: 'Dev pack',
      host: packHost(override ?? instancePack ?? ''),
      current: local && (!override || instancePack === override),
      reason: override ? unreadable : 'No local pack in settings.',
    },
  ]
  const busy = playing || installing || switching !== null

  const onSwitch = async (source: PackSource) => {
    setSwitching(source)
    setError(null)
    try {
      await setPackSource(chapter.id, source)
    } catch (e) {
      setError(errMsg(e))
    } finally {
      setSwitching(null)
    }
  }

  const current = choices.find((c) => c.current)?.source ?? null
  const cards: Choice<PackSource>[] = choices.map((c) => {
    const takeable = !c.current && c.reason === null
    return {
      value: c.source,
      title: c.title,
      detail: c.host ?? '',
      note: c.current ? '' : (c.reason ?? ''),
      disabled: !c.current && (busy || c.reason !== null),
      slot: c.current ? (
        <span className="text-accent">● Current</span>
      ) : switching === c.source ? (
        <span className="text-fg-muted">Switching</span>
      ) : (
        takeable && <span className="text-accent">Switch</span>
      ),
    }
  })

  return (
    <section aria-label="Pack source" className="flex flex-col gap-3">
      <SubHeading>Pack source</SubHeading>
      <ChoiceCards
        label="Pack source"
        value={current}
        choices={cards}
        onChange={(source) => source !== current && void onSwitch(source)}
      />
      <Hint>The next Play syncs from the chosen pack. Saves and settings stay.</Hint>
      {playing && <WarningLine>Close the game to switch.</WarningLine>}
      {!playing && running && <RunningHint chapterName={chapter.name} />}
      {error && <ErrorLine>{error}</ErrorLine>}
    </section>
  )
}
