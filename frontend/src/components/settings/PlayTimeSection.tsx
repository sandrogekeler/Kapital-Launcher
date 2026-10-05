import { useEffect, useLayoutEffect, useRef } from 'react'
import { Clock } from '../../lib/icons'
import { formatDuration, lastPlayedLabel, playShare, totalSeconds } from '../../lib/playTime'
import type { Chapter, PlayTime } from '../../types'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import { SettingsCard, SettingsSection } from '../ui/SettingsLayout'

/**
 * Play time on the account page (issue 192): what Prism has counted for each installed chapter
 * and the total across them. Prism's own numbers, read from each instance by Go when the page
 * opens and when the window regains focus (the game may have run in between), never on a timer.
 */
export function PlayTimeSection() {
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const playTimes = useEngineStore((s) => s.playTimes)
  const loadPlayTime = useEngineStore((s) => s.loadPlayTime)

  useEffect(() => {
    void loadPlayTime()
    const onFocus = () => void loadPlayTime()
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [loadPlayTime])

  const installed = chapters.flatMap((c) => {
    const t = playTimes[c.id]
    return t ? [t] : []
  })
  const total = totalSeconds(installed)
  const now = Date.now()
  return (
    <SettingsSection
      title="Play time"
      icon={Clock}
      aside={installed.length > 0 ? formatDuration(total) : undefined}
    >
      <SettingsCard>
        {chapters.map((c) => (
          <PlayTimeRow key={c.id} chapter={c} time={playTimes[c.id]} total={total} now={now} />
        ))}
      </SettingsCard>
    </SettingsSection>
  )
}

interface RowProps {
  chapter: Chapter
  /** Undefined when the chapter is not installed. */
  time: PlayTime | undefined
  total: number
  now: number
}

/**
 * One chapter: its colour as a dot (the row sets its own `data-chapter`, so `bg-accent` is the
 * chapter's, as the sidebar rows do), when it was last played, the time in the data face and a
 * thin bar for its share of the total.
 */
function PlayTimeRow({ chapter, time, total, now }: RowProps) {
  return (
    <div data-chapter={chapter.id} className="flex flex-col gap-2.5">
      <div className="flex items-center gap-3">
        <span aria-hidden className="bg-accent rounded-pill size-2.5 shrink-0" />
        <div className="flex min-w-0 grow flex-col gap-0.5">
          <span className="text-fg truncate text-sm font-medium">{chapter.name}</span>
          <span className="text-fg-faint text-xs">
            {time ? lastPlayedLabel(time.lastLaunchMs, now) : 'Not installed'}
          </span>
        </div>
        {time && (
          <span className="text-fg shrink-0 font-mono text-sm">
            {formatDuration(time.totalSeconds)}
          </span>
        )}
      </div>
      <ShareBar share={time ? playShare(time.totalSeconds, total) : 0} />
    </div>
  )
}

/**
 * The bar under a row: a thin pill in the sunken ground, filled with the row's accent to its
 * share. The width is a custom property set on the element, as RangeField sets `--range-fill`,
 * and never through `style`.
 */
function ShareBar({ share }: { share: number }) {
  const bar = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    bar.current?.style.setProperty('--play-share', `${share}%`)
  }, [share])
  return (
    <div aria-hidden className="bg-sunken rounded-pill h-1 overflow-hidden">
      <div
        ref={bar}
        className="bg-accent rounded-pill duration-slow ease-standard h-full w-(--play-share) transition-[width] motion-reduce:transition-none"
      />
    </div>
  )
}
