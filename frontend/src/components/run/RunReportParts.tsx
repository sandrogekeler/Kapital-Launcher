import { useLayoutEffect, useRef } from 'react'
import type { ReactNode } from 'react'
import type { RunReport } from '../../types'
import { timeline } from '../../lib/runReport'
import { Hint } from '../ui/Notes'
import { Scrollable } from '../ui/Scrollable'

interface Props {
  report: RunReport
  /**
   * The log block's height, as the classes of a whole number of its 16px lines
   * (`h-24` is six), so the surface that shows it keeps its size.
   */
  logHeight: string
  /**
   * Whether the log is explained: a line under it saying how much of it this
   * is, and a sentence in its place when the start wrote none. The card
   * (#97) keeps to the state line, the timing and its buttons, so it has none.
   */
  note?: boolean
}

/**
 * What the launcher knows of a run, as the card and the launcher's own panel
 * both show it (ADR-2, sixth amendment): the timeline in one row, the crash
 * report's name when there is one, and the end of the game's log, already
 * redacted by Go. Presentational: the report is read elsewhere and kept nowhere.
 */
export function RunReportParts({ report, logHeight, note = true }: Props) {
  const line = timeline(report)
  return (
    <div className="flex min-w-0 flex-col gap-2">
      {line && (
        <p className="text-fg-muted m-0 font-mono text-xs select-text" aria-label="Timeline">
          {line}
        </p>
      )}
      {report.crashReport && (
        <p className="text-fg-muted m-0 text-xs">
          Crash report <span className="text-fg font-mono select-text">{report.crashReport}</span>
        </p>
      )}
      <LogTail report={report} height={logHeight} note={note} />
    </div>
  )
}

/** The log's tail in a block that opens on its last line, where the failure is. */
function LogTail({ report, height, note }: { report: RunReport; height: string; note: boolean }) {
  const end = useRef<HTMLSpanElement>(null)
  // The scrolling element is the sentinel's parent: Scrollable does not hand
  // its ref out, and scrollIntoView would move the ancestors as well.
  useLayoutEffect(() => {
    const block = end.current?.parentElement
    if (block) block.scrollTop = block.scrollHeight
  }, [report.logTail])

  if (!report.logTail) {
    if (!note) return null
    return <Hint>The game wrote no log for this run, so the start stopped before the game.</Hint>
  }
  const lines = `${report.logLines} ${report.logLines === 1 ? 'line' : 'lines'}`
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <LogBlock height={height} label="The end of the game's log">
        {report.logTail}
        <span ref={end} />
      </LogBlock>
      {note && (
        <Hint>
          {report.logTruncated ? `The last ${lines}` : `All ${lines}`} of the game's log, with your
          name, folders and server addresses masked
        </Hint>
      )}
    </div>
  )
}

interface BlockProps {
  /**
   * The block's height, as the classes of a whole number of its 16px lines, or
   * `grow` to take the height its column leaves.
   */
  height: string
  label: string
  children: ReactNode
}

/**
 * The log block every view of a log shares: a mono `pre` on the sunken surface
 * that scrolls inside a box of a whole number of lines. Its children are the
 * text, and a sentinel span in them is how a view finds the scrolling element:
 * Scrollable does not hand its ref out.
 */
export function LogBlock({ height, label, children }: BlockProps) {
  return (
    // box-content: the height is the lines', and the border is on top of it.
    <div
      className={`bg-sunken border-line box-content flex min-h-0 overflow-hidden rounded-md border ${height}`}
    >
      <Scrollable
        as="pre"
        aria-label={label}
        className="text-fg-soft text-2xs m-0 px-2 font-mono leading-4 break-all whitespace-pre-wrap select-text"
      >
        {children}
      </Scrollable>
    </div>
  )
}
