import { Suspense, lazy, useEffect, useState } from 'react'
import type { Chapter, ChapterSettings } from '../../types'
import { selectInstalled, useEngineStore } from '../../stores/useEngineStore'
import { errMsg } from '../../lib/ipc'
import {
  MEMORY_STEP_MB,
  MIN_MEMORY_MB,
  memoryLabel,
  presetLabel,
  sliderMaxMb,
} from '../../lib/chapterSettings'
import { ArrowLeft, FolderOpen } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'

// Only a chapter with a local pack has anything to switch, so the section loads
// when the panel asks for it and stays out of the launcher's bundle budget
// (scripts/check-bundle-size.mjs), as the run report does.
const PackSourceSection = lazy(() =>
  import('./PackSourceSection').then((m) => ({ default: m.PackSourceSection })),
)

interface Props {
  chapter: Chapter
  onClose: () => void
}

/**
 * A chapter's own settings (#36): how much memory its game may take and
 * which JVM preset it runs with. Both live in the instance's instance.cfg,
 * so the panel needs the chapter installed, and Go refuses a save while the
 * game looks to be running. Save writes both at once; the value shown after
 * is what Go read back from the file. Open folder (#85) shows the instance in
 * the file manager; it is there to reach a crash report or a screenshot.
 * A chapter with a local pack also gets the pack source section, which switches
 * its instance between the published pack and that one.
 */
export function ChapterSettingsPanel({ chapter, onClose }: Props) {
  const installed = useEngineStore(selectInstalled(chapter.id))
  const info = useEngineStore((s) => s.chapterSettings[chapter.id])
  const load = useEngineStore((s) => s.loadChapterSettings)
  const save = useEngineStore((s) => s.saveChapterSettings)
  const openFolder = useEngineStore((s) => s.openInstanceFolder)
  const [draft, setDraft] = useState<ChapterSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [folderError, setFolderError] = useState<string | null>(null)

  useEffect(() => {
    if (installed) void load(chapter.id)
  }, [installed, chapter.id, load])
  // The draft follows what Go holds: on first read and after every save.
  useEffect(() => {
    if (info) setDraft(info.settings)
  }, [info])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const dirty =
    info !== undefined &&
    draft !== null &&
    (draft.maxMemoryMb !== info.settings.maxMemoryMb || draft.jvm !== info.settings.jvm)

  const onSave = async () => {
    if (!draft) return
    setSaving(true)
    setError(null)
    try {
      await save(chapter.id, draft)
    } catch (e) {
      setError(errMsg(e))
    } finally {
      setSaving(false)
    }
  }

  const onOpenFolder = async () => {
    setFolderError(null)
    try {
      await openFolder(chapter.id)
    } catch (e) {
      setFolderError(errMsg(e))
    }
  }

  let body
  if (installed === false) {
    body = (
      <p className="text-fg-muted m-0 text-sm leading-normal">
        {chapter.name} is not installed yet. Its memory and JVM preset are set on the instance
        Install writes, so there is nothing to change until then.
      </p>
    )
  } else if (installed === undefined) {
    // No instance report at all: the Prism root is unknown, or there is no
    // bridge (the browser-only preview).
    body = (
      <p className="text-fg-muted m-0 text-sm leading-normal">
        The instance could not be looked for. Set the Prism data folder in Settings.
      </p>
    )
  } else if (!info || !draft) {
    body = <p className="text-fg-muted m-0 text-sm">Reading the instance.</p>
  } else {
    const max = sliderMaxMb(info.machineMemoryMb)
    body = (
      <>
        <div className="flex flex-col gap-3">
          <div className="flex items-baseline justify-between">
            <label htmlFor="chapter-memory" className="text-fg-muted text-sm">
              Memory
            </label>
            <span className="font-mono text-sm">{memoryLabel(draft.maxMemoryMb)}</span>
          </div>
          <input
            id="chapter-memory"
            type="range"
            min={MIN_MEMORY_MB}
            max={max}
            step={MEMORY_STEP_MB}
            value={Math.min(max, draft.maxMemoryMb)}
            onChange={(e) => setDraft({ ...draft, maxMemoryMb: Number(e.target.value) })}
            className="w-full accent-(--accent)"
          />
          <span className="text-fg-faint text-xs leading-normal">
            The most the game may take, of{' '}
            {info.machineMemoryMb > 0 ? memoryLabel(info.machineMemoryMb) : 'an unknown amount'} on
            this machine. Prism would pick {memoryLabel(info.prismDefaultMb)}
            {info.packMemoryMb > 0 && `; the pack recommends ${memoryLabel(info.packMemoryMb)}`}.
          </span>
        </div>

        <div className="flex flex-col gap-3">
          <span className="text-fg-muted text-sm">Java arguments</span>
          <div role="radiogroup" aria-label="Java arguments" className="flex flex-col gap-2">
            {['', ...info.presets].map((name) => (
              <label key={name} className="flex cursor-pointer items-center gap-2.5 text-sm">
                <input
                  type="radio"
                  name="chapter-jvm"
                  value={name}
                  checked={draft.jvm === name}
                  onChange={() => setDraft({ ...draft, jvm: name })}
                  className="accent-(--accent)"
                />
                {presetLabel(name)}
              </label>
            ))}
          </div>
          <span className="text-fg-faint text-xs leading-normal">
            A preset is a fixed set of arguments the launcher knows; nothing typed here reaches the
            game.
          </span>
        </div>

        <div className="flex items-center gap-3">
          <Button onClick={() => void onSave()} disabled={!dirty || saving || info.running}>
            {saving ? 'Saving' : 'Save'}
          </Button>
          {info.running && (
            <span className="text-warning text-xs">
              {chapter.name} looks to be running. Close the game to change these.
            </span>
          )}
          {error && <span className="text-danger text-xs select-text">{error}</span>}
        </div>
      </>
    )
  }

  return (
    <section aria-label={`${chapter.name} settings`} className="flex flex-col">
      <div className="border-line flex items-center gap-3 border-b px-14 py-5">
        <IconButton icon={ArrowLeft} title="Back" onClick={onClose} />
        <h1 className="font-display m-0 text-2xl font-semibold">{chapter.name} settings</h1>
      </div>
      <div className="flex max-w-200 flex-col gap-10 px-14 pt-8 pb-16">
        <div className="flex items-center gap-3">
          <Button onClick={() => void onOpenFolder()} disabled={installed !== true}>
            <Icon icon={FolderOpen} size="sm" />
            <span>Open folder</span>
          </Button>
          {folderError && <span className="text-danger text-xs select-text">{folderError}</span>}
        </div>
        {body}
        {installed === true && (
          <Suspense fallback={null}>
            <PackSourceSection chapter={chapter} />
          </Suspense>
        )}
      </div>
    </section>
  )
}
