import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { Chapter, RunLog } from '../../types'
import { useGameStore, isActive, selectGame } from '../../stores/useGameStore'
import { useLogStore } from '../../stores/useLogStore'
import { errMsg } from '../../lib/ipc'
import { ArrowDown } from '../../lib/icons'
import { joinLines } from '../../lib/runLogs'
import { Button } from '../ui/Button'
import { CopyLogButton } from '../ui/CopyLogButton'
import type { CopyResult } from '../ui/CopyLogButton'
import { Icon } from '../ui/Icon'
import { Hint, ErrorLine } from '../ui/Notes'
import { LogBlock } from '../run/RunReportParts'
import { LogSelect } from './LogSelect'

// The scrolling element is the sentinel's parent: Scrollable does not hand its
// ref out.
const scrollerOf = (sentinel: HTMLElement | null) => sentinel?.parentElement ?? null

/** How far from the bottom, in px, still counts as being at it: a fractional scroll position. */
const AT_BOTTOM = 8

/**
 * What the dropdown chose, shown: the row of the dropdown, Copy and, for a past
 * run, Load earlier, and under it the text filling the card's height. A file
 * opens on its last line, where a crash is, and text loaded in front of it
 * keeps what was being read where it was. The live log follows the bottom while
 * the player is at it; scrolled up, it stays put and offers Jump to latest.
 * Copy puts on the clipboard what is shown, no more.
 */
export function LogViewer({ chapter, logs }: { chapter: Chapter; logs: readonly RunLog[] }) {
  const live = useLogStore((s) => s.live !== null)
  const liveLines = useLogStore((s) => s.liveLines)
  const opened = useLogStore((s) => s.opened)
  const reading = useLogStore((s) => s.reading)
  const readError = useLogStore((s) => s.readError)
  const loadEarlier = useLogStore((s) => s.loadEarlier)
  const copy = useLogStore((s) => s.copy)
  const gameActive = useGameStore((s) => isActive(selectGame(chapter.id)(s)?.phase))
  const [copied, setCopied] = useState<CopyResult | null>(null)
  // Whether the live log is scrolled away from its end.
  const [away, setAway] = useState(false)

  const liveText = useMemo(() => joinLines(liveLines), [liveLines])
  const ready = live ? !reading : opened !== null
  const text = live ? liveText : (opened?.text ?? '')
  const name = live ? 'The live log' : (opened?.name ?? '')

  const end = useRef<HTMLSpanElement>(null)
  // Whether the end of the live log is followed: the view was at the bottom when
  // the last line came in.
  const stick = useRef(true)
  // What the block held when it last changed, to tell text put in front of it
  // from a new file.
  const last = useRef<{ key: string; offset: number; height: number } | null>(null)

  useEffect(() => {
    const block = scrollerOf(end.current)
    if (!block) return
    const onScroll = () => {
      const atEnd = block.scrollHeight - block.scrollTop - block.clientHeight <= AT_BOTTOM
      stick.current = atEnd
      setAway(!atEnd)
    }
    block.addEventListener('scroll', onScroll, { passive: true })
    return () => block.removeEventListener('scroll', onScroll)
  }, [ready])

  // Before paint, so a view is never seen jumping. A file opens on its last line;
  // text put in front of it leaves the line that was first at the top; the live
  // log goes to the end while it is followed.
  useLayoutEffect(() => {
    const block = scrollerOf(end.current)
    if (!block || !ready) {
      last.current = null
      return
    }
    const key = live ? 'live' : `${opened?.kind}:${opened?.name}`
    const prev = last.current
    const sameView = prev !== null && prev.key === key
    if (live) {
      if (!sameView) stick.current = true
      if (stick.current) block.scrollTop = block.scrollHeight
    } else {
      const prepended = sameView && opened !== null && opened.offset < prev.offset
      block.scrollTop = prepended ? block.scrollHeight - prev.height : block.scrollHeight
    }
    last.current = { key, offset: opened?.offset ?? 0, height: block.scrollHeight }
  }, [ready, live, liveText, opened])

  const onCopy = async () => {
    try {
      await copy()
      setCopied({})
    } catch (e) {
      setCopied({ error: errMsg(e) })
    }
  }

  const jump = () => {
    const block = scrollerOf(end.current)
    if (!block) return
    block.scrollTop = block.scrollHeight
    stick.current = true
    setAway(false)
  }

  return (
    <div className="flex min-h-0 grow flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <LogSelect chapterId={chapter.id} logs={logs} />
        <CopyLogButton
          onClick={() => void onCopy()}
          result={copied}
          disabled={!ready || text === '' || reading}
        />
        {!live && opened?.truncated && (
          <Button onClick={() => void loadEarlier(chapter.id)} disabled={reading}>
            <span>Load earlier</span>
          </Button>
        )}
      </div>
      {live && <LiveStatus active={gameActive} />}
      {ready ? (
        <div className="relative flex min-h-0 grow flex-col">
          <LogBlock height="grow" label={`${name}, masked`}>
            {text || (live ? 'Nothing has been written yet.' : 'This file is empty.')}
            <span ref={end} />
          </LogBlock>
          {live && away && (
            <button
              type="button"
              onClick={jump}
              className="bg-raised-2 border-line-strong hover:bg-hover duration-fast ease-standard absolute right-5 bottom-3 inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-md border px-3 text-xs font-semibold transition-colors"
            >
              <Icon icon={ArrowDown} size="sm" />
              <span>Jump to latest</span>
            </button>
          )}
        </div>
      ) : (
        !readError && <p className="text-fg-muted m-0 text-sm">Reading the file.</p>
      )}
      {readError && <ErrorLine>{readError}</ErrorLine>}
      {ready && (
        <Hint>Your name, folders and server addresses are masked, here and in what you copy.</Hint>
      )}
    </div>
  )
}

/**
 * Whether what the live log shows is still being written. While the chapter's
 * game is starting or running it says Live, with a dot that pulses unless the
 * player asked for less motion; otherwise it says what the player is reading is
 * the end of a run that is over.
 */
function LiveStatus({ active }: { active: boolean }) {
  if (!active) return <Hint>The game is not running. This is the end of its last run.</Hint>
  return (
    <p role="status" className="text-accent m-0 flex items-center gap-2 text-xs font-semibold">
      <span
        aria-hidden="true"
        className="bg-accent rounded-pill size-1.5 animate-pulse motion-reduce:animate-none"
      />
      <span>Live</span>
    </p>
  )
}
