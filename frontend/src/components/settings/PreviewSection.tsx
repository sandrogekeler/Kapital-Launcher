import { useEffect, useState } from 'react'
import type { PreviewSituation } from '../../types'
import { useChapterStore } from '../../stores/useChapterStore'
import { usePreviewStore } from '../../stores/usePreviewStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { errMsg } from '../../lib/ipc'
import { Button } from '../ui/Button'

interface Props {
  /** Called once a preview is on, with the chapter picked, so the screen under review is shown. */
  onStarted: (chapterId: string) => void
}

/**
 * Previews (issue 124), in the Developer section: one button per situation a run
 * only shows when something goes wrong, for the picked chapter, and Prism's.
 * Go pushes made-up state through the real paths, so the bar, the notices, the
 * card and the sidebar render their real copy. The list is Go's.
 *
 * A start brings the chapter up on the main screen, closing settings, because
 * the screen under review is not this one. Loaded on demand, like the pack
 * source section, to stay out of the entry bundle.
 */
export function PreviewSection({ onStarted }: Props) {
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const selectedId = useChapterStore((s) => s.selectedId)
  const situations = usePreviewStore((s) => s.situations)
  const load = usePreviewStore((s) => s.load)
  const start = usePreviewStore((s) => s.start)
  const clear = usePreviewStore((s) => s.clear)
  const splashOn = useSettingsStore((s) => s.settings.loadingSplashOn)
  // The chapter the previews are for; the open one until another is picked.
  const [picked, setPicked] = useState<string | null>(null)
  const chapterId = picked ?? selectedId
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void load()
  }, [load])

  const run = async (act: () => Promise<void>, then?: () => void) => {
    setBusy(true)
    setError(null)
    try {
      await act()
      then?.()
    } catch (e) {
      setError(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  const button = (s: PreviewSituation) => (
    <Button
      key={s.id}
      disabled={busy}
      onClick={() =>
        void run(
          () => start(chapterId, s),
          () => onStarted(chapterId),
        )
      }
    >
      {s.label}
    </Button>
  )
  const forChapter = situations.filter((s) => s.scope === 'chapter')
  const forPrism = situations.filter((s) => s.scope === 'prism')

  return (
    <div className="flex flex-col gap-4">
      <h3 className="text-fg-muted m-0 text-sm font-medium">Preview</h3>
      <p className="text-fg-faint m-0 text-xs leading-normal">
        Shows a screen that a run only has when something goes wrong, so it can be looked at. A
        preview is made up: it writes nothing, starts nothing and reaches no network, and a real
        launch ends it.
      </p>
      {splashOn === false && (
        <p className="text-fg-faint m-0 text-xs leading-normal">
          The loading splash is off here, so previews that have a card show the bar and the notice
          only.
        </p>
      )}
      <div className="flex flex-col gap-1.5">
        <span className="text-fg-muted text-sm">Chapter</span>
        <div
          role="group"
          aria-label="Chapter to preview"
          className="bg-sunken border-line-strong inline-flex w-fit rounded-md border p-0.5"
        >
          {chapters.map((c) => (
            <button
              key={c.id}
              type="button"
              aria-pressed={c.id === chapterId}
              onClick={() => setPicked(c.id)}
              className={`duration-fast ease-standard h-9 cursor-pointer rounded-sm px-4 text-sm transition-colors ${
                c.id === chapterId ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg'
              }`}
            >
              {c.name}
            </button>
          ))}
        </div>
      </div>
      <div role="group" aria-label="Chapter situations" className="flex flex-wrap gap-2">
        {forChapter.map(button)}
      </div>
      <span className="text-fg-muted text-sm">Prism</span>
      <div role="group" aria-label="Prism situations" className="flex flex-wrap gap-2">
        {forPrism.map(button)}
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={busy} onClick={() => void run(clear)}>
          Clear previews
        </Button>
        {error && (
          <span role="alert" className="text-danger text-xs select-text">
            {error}
          </span>
        )}
      </div>
    </div>
  )
}
