import { useLayoutEffect, useRef, useState } from 'react'
import type { Chapter, GameState } from '../../types'
import { chapterArt, chapterTitleArt } from '../../lib/art'
import { gameLine } from '../../lib/gameLine'
import { errMsg } from '../../lib/ipc'
import { isPlaceholder } from '../../lib/manifest'
import { copyLogDone } from '../../lib/settingsView'
import { barPlan } from '../../lib/splashBar'
import { Copy, FolderOpen } from '../../lib/icons'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'

interface Props {
  chapter: Chapter
  /** The chapter's game, with `splash` set: App shows the card only then. */
  state: GameState
}

type Result = { ok: boolean; text: string }

/**
 * The loading card (#43). While a game starts, Go shrinks the launcher's window
 * to a small card and App shows this in place of the whole layout: the
 * chapter's art blurred and tinted as the nav tile does it, its title in the
 * middle, the stage line and a bar along the bottom. The card ends when Go
 * emits the chapter's state with `splash` false, whether the game's own screen
 * took over or the player pressed Back to launcher.
 *
 * A game that stopped or never started (`crashed`, `failed`) keeps the card as
 * its error state: the bar goes, and Open folder, Copy log and Back to
 * launcher stay until the player leaves.
 *
 * The whole card drags the window, the controls opt out.
 */
export function SplashCard({ chapter, state }: Props) {
  const art = chapterArt(chapter.id)
  const title = chapterTitleArt(chapter.id)
  const leave = useGameStore((s) => s.leaveSplash)
  const leaveError = useGameStore((s) => s.error)
  const openFolder = useEngineStore((s) => s.openInstanceFolder)
  const copyLog = useSettingsStore((s) => s.copyLog)
  const [result, setResult] = useState<Result | null>(null)

  const failed = state.phase === 'crashed' || state.phase === 'failed'
  const [stage, detail, tone] = gameLine(state, chapter.name) ?? ['◐ Starting', '', 'text-accent']
  const version = isPlaceholder(chapter.pack.version) ? null : chapter.pack.version
  const caption = [title ? chapter.name : null, version ? `Pack ${version}` : null]
    .filter(Boolean)
    .join(' · ')

  const onOpenFolder = async () => {
    try {
      await openFolder(chapter.id)
      setResult(null)
    } catch (e) {
      setResult({ ok: false, text: errMsg(e) })
    }
  }
  const onCopyLog = async () => {
    try {
      setResult({ ok: true, text: copyLogDone(await copyLog()) })
    } catch (e) {
      setResult({ ok: false, text: errMsg(e) })
    }
  }

  return (
    <section
      data-chapter={chapter.id}
      aria-label={`Loading ${chapter.name}`}
      className="bg-canvas relative isolate flex h-full w-full flex-col overflow-hidden [--wails-draggable:drag]"
    >
      {/* The nav tile's treatment (ChapterButton): art blurred, the accent's
          hue over it, a scrim for the text. */}
      <div aria-hidden className="pointer-events-none absolute inset-0 -z-10">
        {art ? (
          <>
            <img
              src={art}
              alt=""
              className="size-full scale-125 object-cover blur-(--effect-tile-blur)"
              draggable={false}
            />
            <div className="bg-accent absolute inset-0 opacity-(--effect-tile-tint) mix-blend-color" />
          </>
        ) : (
          <div className="art-pending size-full" />
        )}
        <div className="scrim-tile absolute inset-0" />
      </div>

      <div className="flex grow flex-col items-center justify-center gap-3 px-8">
        <h1 className="font-display text-display m-0 font-semibold">
          {title ? (
            <img
              src={title}
              alt={chapter.name}
              className="block h-(--layout-title) w-auto max-w-full"
              draggable={false}
            />
          ) : (
            chapter.name
          )}
        </h1>
        {caption && <p className="text-fg-muted m-0 text-sm">{caption}</p>}
      </div>

      <div className="flex flex-col gap-3">
        <div className="flex items-end justify-between gap-6 px-8">
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className={`font-mono text-xs ${tone}`}>{stage}</span>
            {detail && <span className="text-fg-muted text-xs">{detail}</span>}
          </div>
          {!failed && (
            <button
              type="button"
              onClick={() => void leave()}
              className="text-fg-muted hover:text-fg duration-fast ease-standard cursor-pointer text-xs transition-colors [--wails-draggable:no-drag]"
            >
              Back to launcher
            </button>
          )}
        </div>
        {leaveError && (
          <p role="alert" className="text-danger m-0 px-8 text-xs select-text">
            {leaveError}
          </p>
        )}
        {failed ? (
          <div className="flex flex-col gap-3 px-8 pb-6 [--wails-draggable:no-drag]">
            <div className="flex flex-wrap items-center gap-3">
              <Button onClick={() => void onOpenFolder()}>
                <Icon icon={FolderOpen} size="sm" />
                <span>Open folder</span>
              </Button>
              <Button onClick={() => void onCopyLog()}>
                <Icon icon={Copy} size="sm" />
                <span>Copy log</span>
              </Button>
              <Button onClick={() => void leave()}>
                <span>Back to launcher</span>
              </Button>
            </div>
            <span role="status" className="text-xs select-text">
              {result && (
                <span className={result.ok ? 'text-fg-muted' : 'text-danger'}>{result.text}</span>
              )}
            </span>
          </div>
        ) : (
          <Bar phase={state.phase} estimate={state.estimate} />
        )}
      </div>
    </section>
  )
}

/**
 * The thin bar along the card's bottom edge, in the accent. Determinate when
 * the chapter has history: at each phase change the fill jumps to that phase's
 * place and eases towards the next one over the estimated gap, by CSS alone,
 * so nothing here polls. Without history it is a shimmer that goes still under
 * reduced motion.
 */
function Bar({ phase, estimate }: Pick<GameState, 'phase' | 'estimate'>) {
  const plan = barPlan(phase, estimate)
  const fill = useRef<HTMLDivElement>(null)
  const determinate = plan !== null
  const from = plan?.from ?? 0
  const to = plan?.to ?? 0
  const ms = plan?.ms ?? 0

  // The ease's duration and target are data, set as custom properties on the
  // element: an inline `style` prop is refused (frontend-style.md). Before
  // paint, jump to where the phase starts with no transition, commit that,
  // then set the target with the transition on.
  useLayoutEffect(() => {
    const el = fill.current
    if (!el) return
    el.style.setProperty('--splash-ms', '0ms')
    el.style.setProperty('--splash-fill', String(from))
    void el.offsetWidth
    el.style.setProperty('--splash-ms', `${ms}ms`)
    el.style.setProperty('--splash-fill', String(to))
  }, [determinate, from, to, ms])

  return (
    <div role="progressbar" aria-label="Loading" className="bg-line-strong h-0.5 overflow-hidden">
      {determinate ? (
        <div
          ref={fill}
          data-bar="determinate"
          data-from={from}
          data-to={to}
          data-ms={ms}
          className="bg-accent size-full origin-left scale-x-(--splash-fill) transition-transform duration-(--splash-ms) ease-linear"
        />
      ) : (
        <div data-bar="indeterminate" className="splash-shimmer size-full" />
      )}
    </div>
  )
}
