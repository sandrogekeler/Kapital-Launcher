import { useEffect, useId, useRef, useState } from 'react'
import type { Chapter, ModToggleState } from '../../types'
import { errMsg } from '../../lib/ipc'
import { ChevronRight, Puzzle } from '../../lib/icons'
import { filterMods, modLabel } from '../../lib/mods'
import { formatBytes } from '../../lib/bytes'
import { isActive, selectGame, useGameStore } from '../../stores/useGameStore'
import { useModStore } from '../../stores/useModStore'
import { Icon } from '../ui/Icon'
import { ErrorLine, Hint, WarningLine } from '../ui/Notes'
import { Scrollable } from '../ui/Scrollable'
import { SettingsCard, SettingsSection } from '../ui/SettingsLayout'
import { Toggle } from '../ui/Toggle'
import { RunningHint } from './RunningHint'

interface Props {
  chapter: Chapter
}

/**
 * A chapter's mods (issue 156, ADR-2's ninth amendment): the few the manifest names as quick
 * switches, and under them a collapsible list of every mod in the instance's mods folder with
 * a switch and a search. A switch shows the mod as it is, on or off. The choice is the
 * launcher's own, kept in its settings and applied by the sync Prism runs before the game, so
 * a disabled mod is not downloaded again; Go also renames the files at once, so the folder
 * matches before the next Play does anything. Only the tracker's answer (`playing`) disables
 * a switch, with a write in flight; `running` is the log's guess, shown as a hint, and Go
 * refuses the write if it was right (issue 126). The view is read again when the game ends
 * and when the window regains focus.
 */
export function ModsSection({ chapter }: Props) {
  const view = useModStore((s) => s.byChapter[chapter.id])
  const unavailable = useModStore((s) => s.unavailable)
  const readError = useModStore((s) => s.readError[chapter.id])
  const writing = useModStore((s) => s.writing !== null)
  const load = useModStore((s) => s.load)
  const setJars = useModStore((s) => s.setJars)
  const playing = useGameStore((s) => isActive(selectGame(chapter.id)(s)?.phase))
  const wasPlaying = useRef(playing)
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [error, setError] = useState<string | null>(null)
  const panelId = useId()
  const searchId = useId()

  useEffect(() => {
    void load(chapter.id)
  }, [chapter.id, load])
  // The log's guess goes stale the moment the game closes: read again when the run ends and
  // when the window comes back to the front, never on a timer.
  useEffect(() => {
    const ended = wasPlaying.current && !playing
    wasPlaying.current = playing
    if (ended) void load(chapter.id)
  }, [playing, chapter.id, load])
  useEffect(() => {
    const onFocus = () => void load(chapter.id)
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [chapter.id, load])

  if (unavailable) return null
  if (!view) {
    return readError ? (
      <SettingsSection title="Mods" icon={Puzzle}>
        <ErrorLine>{readError}</ErrorLine>
      </SettingsSection>
    ) : null
  }

  const busy = playing || writing
  const set = async (jars: readonly string[], off: boolean) => {
    setError(null)
    try {
      await setJars(chapter.id, jars, off)
    } catch (e) {
      setError(errMsg(e))
    }
  }
  const shown = filterMods(view.mods, query)
  const nothingYet = view.mods.length === 0

  const toggleHint = (t: ModToggleState): string =>
    t.jars.length === 0
      ? 'Not downloaded yet. The first Play fetches it, and it can be switched after that.'
      : t.jars.map(modLabel).join(', ')

  const on = view.toggles.filter((t) => !t.disabled).length
  return (
    <SettingsSection
      title="Mods"
      icon={Puzzle}
      aside={view.toggles.length > 0 ? `${on} of ${view.toggles.length} on` : undefined}
    >
      <SettingsCard>
        {view.toggles.map((t) => (
          <Toggle
            key={t.jarPrefix}
            label={t.name}
            checked={!t.disabled}
            hint={toggleHint(t)}
            disabled={busy || t.jars.length === 0}
            onChange={(on) => void set(t.jars, !on)}
          />
        ))}
        <button
          type="button"
          aria-expanded={open}
          aria-controls={panelId}
          onClick={() => setOpen((o) => !o)}
          className="text-fg-soft hover:text-fg hover:bg-hover duration-fast ease-standard flex w-full cursor-pointer items-center gap-2 text-left text-sm transition-colors first:rounded-t-lg last:rounded-b-lg"
        >
          <Icon
            icon={ChevronRight}
            size="sm"
            className={`duration-fast ease-standard transition-transform motion-reduce:transition-none ${open ? 'rotate-90' : ''}`}
          />
          <span>Advanced mod control</span>
        </button>
      </SettingsCard>
      <Hint>Changes apply on the next Play. A disabled mod is not downloaded again.</Hint>
      {playing && <WarningLine>Close the game to change mods.</WarningLine>}
      {!playing && view.running && <RunningHint chapterName={chapter.name} />}
      {error && <ErrorLine>{error}</ErrorLine>}

      <div className="flex flex-col gap-3">
        {open && (
          <div id={panelId} className="flex flex-col gap-3">
            {nothingYet ? (
              <Hint>The pack's mods appear here after the first Play.</Hint>
            ) : (
              <>
                <div className="flex flex-col gap-1.5">
                  <label htmlFor={searchId} className="text-fg-muted text-sm">
                    Search mods
                  </label>
                  <input
                    id={searchId}
                    type="search"
                    value={query}
                    placeholder="Name"
                    spellCheck={false}
                    autoComplete="off"
                    onChange={(e) => setQuery(e.target.value)}
                    className="bg-sunken text-fg placeholder:text-fg-faint border-line-strong h-11 min-w-0 rounded-md border px-3 text-sm select-text"
                  />
                </div>
                <div className="bg-sunken border-line box-content flex max-h-96 min-h-0 overflow-hidden rounded-md border">
                  <Scrollable
                    as="ul"
                    aria-label="Mods"
                    className="m-0 flex list-none flex-col gap-3 p-3"
                  >
                    {shown.map((m) => (
                      <li key={m.name}>
                        <Toggle
                          label={modLabel(m.name)}
                          checked={!m.disabled}
                          hint={formatBytes(m.size)}
                          disabled={busy}
                          onChange={(on) => void set([m.name], !on)}
                        />
                      </li>
                    ))}
                    {shown.length === 0 && (
                      <li>
                        <Hint>No mod has that name.</Hint>
                      </li>
                    )}
                  </Scrollable>
                </div>
                <Hint>
                  {shown.length === view.mods.length
                    ? `${view.mods.length} mods, ${view.mods.filter((m) => m.disabled).length} switched off.`
                    : `${shown.length} of ${view.mods.length} mods.`}
                </Hint>
              </>
            )}
          </div>
        )}
      </div>
    </SettingsSection>
  )
}
