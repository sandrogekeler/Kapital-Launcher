import { useEffect, useState } from 'react'
import type { Chapter, RunReport } from '../../types'
import { useEngineStore } from '../../stores/useEngineStore'
import { selectGame, useGameStore } from '../../stores/useGameStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { errMsg } from '../../lib/ipc'
import { gameLine } from '../../lib/gameLine'
import { FolderOpen } from '../../lib/icons'
import { Button } from '../ui/Button'
import { CopyLogButton } from '../ui/CopyLogButton'
import type { CopyResult } from '../ui/CopyLogButton'
import { Icon } from '../ui/Icon'
import { ErrorLine } from '../ui/Notes'
import { Page } from '../ui/Page'
import { RunReportParts } from '../run/RunReportParts'

interface Props {
  chapter: Chapter
  onClose: () => void
}

/**
 * What the launcher knows of a chapter's latest run (ADR-2, sixth amendment),
 * in the column the chapter's settings take: how it ended, when each phase was
 * reached, the crash report's name and the end of the game's log, redacted by
 * Go. It is the card's content in the launcher's own window, for a player who
 * left the card or whose game crashed after it closed.
 *
 * The report is read when the panel opens and again on the next `game:state` of
 * the chapter while it is open, never on a timer, and is held in this panel
 * alone: it carries a stretch of the game's log, so closing the panel drops it.
 */
export function RunReportPanel({ chapter, onClose }: Props) {
  const game = useGameStore(selectGame(chapter.id))
  const read = useGameStore((s) => s.report)
  const openFolder = useEngineStore((s) => s.openInstanceFolder)
  const copyLog = useSettingsStore((s) => s.copyLog)
  const showConsole = useGameStore((s) => s.showConsole)
  const [report, setReport] = useState<RunReport | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Why the folder or the console would not open, beside the buttons.
  const [actionError, setActionError] = useState<string | null>(null)
  const [copied, setCopied] = useState<CopyResult | null>(null)

  // `game` is a dependency for the re-read: the store files a new object per
  // event, so a run's next phase reads again and nothing else does.
  useEffect(() => {
    let current = true
    read(chapter.id)
      .then((r) => {
        if (!current) return
        setReport(r)
        setError(null)
        setLoaded(true)
      })
      .catch((e) => {
        if (!current) return
        setError(errMsg(e))
        setLoaded(true)
      })
    return () => {
      current = false
    }
  }, [chapter.id, game, read])

  const onOpenFolder = async () => {
    setActionError(null)
    try {
      await openFolder(chapter.id)
    } catch (e) {
      setActionError(errMsg(e))
    }
  }

  const onShowConsole = async () => {
    setActionError(null)
    try {
      if (!(await showConsole(chapter.id))) setActionError("Prism's console is no longer open")
    } catch (e) {
      setActionError(errMsg(e))
    }
  }

  // The button says how it went; each attempt is a new result object.
  const onCopyLog = async () => {
    try {
      await copyLog()
      setCopied({})
    } catch (e) {
      setCopied({ error: errMsg(e) })
    }
  }

  const rows = gameLine(report?.game ?? game, chapter.name)
  let body
  if (error) {
    body = <ErrorLine>{error}</ErrorLine>
  } else if (!loaded) {
    // The page reveals its body once the report has been read.
    body = null
  } else if (!report) {
    // No bridge at all: the browser-only preview has no run to read.
    body = (
      <p className="text-fg-muted m-0 text-sm">
        The run report can only be read in the app window.
      </p>
    )
  } else {
    body = (
      <>
        {rows && (
          <div className="flex flex-col gap-0.5">
            <span className={`font-ui text-sm font-medium ${rows[2]}`}>{rows[0]}</span>
            <span className="text-fg-muted text-xs select-text">{rows[1]}</span>
          </div>
        )}
        <RunReportParts report={report} logHeight="h-72" />
        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={() => void onOpenFolder()}>
            <Icon icon={FolderOpen} size="sm" />
            <span>Open folder</span>
          </Button>
          <CopyLogButton onClick={() => void onCopyLog()} result={copied} />
          {report.consoleAvailable && (
            <Button onClick={() => void onShowConsole()}>
              <span>Show Prism's console</span>
            </Button>
          )}
          {actionError && <ErrorLine>{actionError}</ErrorLine>}
        </div>
      </>
    )
  }

  return (
    <Page
      label={`${chapter.name} run report`}
      title={`${chapter.name} run report`}
      onBack={onClose}
      ready={loaded}
    >
      {/* One report, not sections: its parts sit closer than a page's blocks do. */}
      <div className="flex flex-col gap-6">{body}</div>
    </Page>
  )
}
