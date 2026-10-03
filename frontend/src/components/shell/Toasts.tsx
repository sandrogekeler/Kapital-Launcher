import { useState } from 'react'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import { errorToasts, type Toast } from '../../lib/toasts'
import { TriangleAlert, X } from '../../lib/icons'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'

interface Props {
  /** Opens the chapter's run report; Details on a run that crashed or never started. */
  onOpenReport: (chapterId: string) => void
}

const TONE = {
  danger: 'text-danger',
  muted: 'text-fg-muted',
} as const

/**
 * The launcher's notices, stacked in the bottom-right corner over the chapter
 * card, so an error never moves what is on the screen: a launch, install or
 * stop the backend refused, and a run that crashed or never started, with
 * Details to its report. The notices are derived from the stores
 * (`lib/toasts.ts`), so one leaves when its cause does, and the cross puts
 * one away until its source has something new to say.
 */
export function Toasts({ onOpenReport }: Props) {
  const engineError = useEngineStore((s) => s.error)
  const stopErrors = useGameStore((s) => s.stopErrors)
  const games = useGameStore((s) => s.states)
  const chapters = useChapterStore((s) => s.manifest.chapters)
  // The stamp each notice was dismissed at; a notice with a newer stamp shows again.
  const [dismissed, setDismissed] = useState<Record<string, string>>({})

  const chapterName = (id: string) => chapters.find((c) => c.id === id)?.name ?? id
  const toasts = errorToasts({ engineError, stopErrors, games, chapterName }).filter(
    (t) => dismissed[t.id] !== t.stamp,
  )
  const dismiss = (t: Toast) => setDismissed((d) => ({ ...d, [t.id]: t.stamp }))

  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed right-5 bottom-8 z-10 flex w-80 flex-col gap-2"
    >
      {toasts.map((t) => (
        <div
          key={t.id}
          data-toast={t.id}
          className="bg-raised-2 border-line-strong pointer-events-auto flex items-start gap-3 rounded-md border py-3 pr-2 pl-4"
        >
          <Icon icon={TriangleAlert} size="sm" className={`mt-0.5 ${TONE[t.tone]}`} />
          <div className="flex min-w-0 grow flex-col gap-0.5">
            <span className={`font-mono text-xs ${TONE[t.tone]}`}>{t.title}</span>
            <span className="text-fg-muted text-xs break-words select-text">{t.detail}</span>
            {t.reportFor && (
              <button
                type="button"
                onClick={() => onOpenReport(t.reportFor!)}
                className="text-fg hover:text-accent duration-fast ease-standard mt-1 cursor-pointer self-start text-xs font-semibold transition-colors"
              >
                Details
              </button>
            )}
          </div>
          <IconButton icon={X} title="Dismiss" size="sm" onClick={() => dismiss(t)} />
        </div>
      ))}
    </div>
  )
}
