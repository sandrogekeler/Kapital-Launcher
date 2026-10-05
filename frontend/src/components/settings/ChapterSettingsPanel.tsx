import { Suspense, lazy, useEffect, useRef, useState } from 'react'
import type { Chapter, ChapterSettings } from '../../types'
import { selectInstalled, useEngineStore } from '../../stores/useEngineStore'
import { isActive, selectGame, useGameStore } from '../../stores/useGameStore'
import { errMsg } from '../../lib/ipc'
import { FolderOpen, MemoryStick } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { ErrorLine } from '../ui/Notes'
import { Page } from '../ui/Page'
import { SettingsSection } from '../ui/SettingsLayout'
import { ChapterServerSection } from './ChapterServerSection'
import { ChapterSettingsForm } from './ChapterSettingsForm'

// The mods section lists the mods folder and has a search; like the pack source it loads when
// the panel asks for it, so its store and its list stay out of the launcher's bundle.
const ModsSection = lazy(() => import('./ModsSection').then((m) => ({ default: m.ModsSection })))

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
 * game is running. Only the tracker's answer (the game store) disables Save;
 * `info.running` also holds a guess from the game log, which is a hint here and
 * is read again when the chapter's run ends and when the window regains focus
 * (issue 126). Save writes both at once; the value shown after
 * is what Go read back from the file. Open folder (#85), in the page header,
 * shows the instance in the file manager; it is there to reach a crash report or a screenshot.
 * A chapter with a server gets the Server section next (issue 163), installed or not: its address
 * and the switch for joining it on Play, both saved in the launcher's settings.
 * Mods (issue 156) come after: the chapter's quick switches and an advanced list of every mod,
 * saved by Go in the launcher's settings and applied by the pre-launch sync. A chapter with a
 * local pack also gets the pack source section, which switches its instance between the
 * published pack and that one.
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
  const playing = useGameStore((s) => isActive(selectGame(chapter.id)(s)?.phase))
  const wasPlaying = useRef(playing)

  useEffect(() => {
    if (installed) void load(chapter.id)
  }, [installed, chapter.id, load])
  // The log's guess goes stale the moment the game closes, and on Windows the
  // minute starts at the close. Read again when the run ends and when the
  // window comes back to the front, never on a timer.
  useEffect(() => {
    const ended = wasPlaying.current && !playing
    wasPlaying.current = playing
    if (ended && installed) void load(chapter.id)
  }, [playing, installed, chapter.id, load])
  useEffect(() => {
    if (!installed) return
    const onFocus = () => void load(chapter.id)
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [installed, chapter.id, load])
  // The draft follows what Go holds: on first read and after every save.
  useEffect(() => {
    if (info) setDraft(info.settings)
  }, [info])

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
        {chapter.name} is not installed yet. Its memory and Java arguments are set on the instance
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
  } else if (info && draft) {
    body = (
      <ChapterSettingsForm
        chapter={chapter}
        info={info}
        draft={draft}
        onDraft={setDraft}
        dirty={dirty}
        saving={saving}
        playing={playing}
        error={error}
        onSave={() => void onSave()}
      />
    )
  }
  // Not installed and unknown are answers at once; only an installed
  // chapter waits for its instance to be read, and the page reveals when it
  // has been.
  const ready = installed !== true || Boolean(info && draft)

  return (
    <Page
      label={`${chapter.name} settings`}
      eyebrow={chapter.name}
      title="Settings"
      onBack={onClose}
      ready={ready}
      settings
      actions={
        <Button onClick={() => void onOpenFolder()} disabled={installed !== true}>
          <Icon icon={FolderOpen} size="sm" />
          <span>Open folder</span>
        </Button>
      }
    >
      {folderError && (
        <div className="pb-6">
          <ErrorLine>{folderError}</ErrorLine>
        </div>
      )}
      {body && (
        <SettingsSection title="Game" icon={MemoryStick}>
          {body}
        </SettingsSection>
      )}
      <ChapterServerSection chapter={chapter} />
      {installed === true && (
        <Suspense fallback={null}>
          <ModsSection chapter={chapter} />
          <PackSourceSection chapter={chapter} />
        </Suspense>
      )}
    </Page>
  )
}
