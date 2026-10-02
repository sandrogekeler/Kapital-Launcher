import { useState } from 'react'
import type { Chapter, PackSource } from '../../types'
import { errMsg } from '../../lib/ipc'
import { packHost, isLocalPack } from '../../lib/packSource'
import { selectInstancePack, useEngineStore } from '../../stores/useEngineStore'
import { isActive, selectGame, useGameStore } from '../../stores/useGameStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { Button } from '../ui/Button'

interface Choice {
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
 * published pack has nothing to switch between.
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
  const choices: Choice[] = [
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
  const busy = playing || running || installing || switching !== null

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

  return (
    <section aria-label="Pack source" className="flex flex-col gap-3">
      <h2 className="text-fg-muted m-0 text-sm font-normal">Pack source</h2>
      <div className="flex flex-col gap-3">
        {choices.map((c) => (
          <div key={c.source} className="flex items-center justify-between gap-4">
            <div className="flex min-w-0 flex-col gap-0.5">
              <span className="text-sm">{c.title}</span>
              {c.host && <span className="text-fg-muted font-mono text-xs">{c.host}</span>}
              {!c.current && c.reason && <span className="text-fg-faint text-xs">{c.reason}</span>}
            </div>
            {c.current ? (
              <span className="text-accent text-sm">● Current</span>
            ) : (
              <Button onClick={() => void onSwitch(c.source)} disabled={busy || c.reason !== null}>
                Switch to {c.title.toLowerCase()}
              </Button>
            )}
          </div>
        ))}
      </div>
      <span className="text-fg-faint text-xs leading-normal">
        The next Play syncs from the chosen pack. Saves and settings stay.
      </span>
      {(playing || running) && (
        <span className="text-warning text-xs">Close the game to switch.</span>
      )}
      {error && <span className="text-danger text-xs select-text">{error}</span>}
    </section>
  )
}
