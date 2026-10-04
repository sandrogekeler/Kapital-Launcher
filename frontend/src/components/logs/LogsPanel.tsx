import { useEffect, useState } from 'react'
import type { Chapter } from '../../types'
import { selectInstalled, useEngineStore } from '../../stores/useEngineStore'
import { selectOpenKey, useLogStore } from '../../stores/useLogStore'
import { errMsg } from '../../lib/ipc'
import { FolderOpen } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { ErrorLine } from '../ui/Notes'
import { Page } from '../ui/Page'
import { LogList } from './LogList'
import { LogViewer } from './LogViewer'

interface Props {
  chapter: Chapter
  onClose: () => void
}

/**
 * A chapter's logs (issue 155), in the column the chapter's settings take: its
 * game logs and crash reports newest first, read from the instance's own
 * folders so a run started from Prism is in it too, and the one chosen, masked
 * by Go. Read only: Open folder, in the header, is how a file is reached on
 * disk. The list is read when the page opens, and the page's data is dropped
 * when it closes.
 */
export function LogsPanel({ chapter, onClose }: Props) {
  const installed = useEngineStore(selectInstalled(chapter.id))
  const openFolder = useEngineStore((s) => s.openInstanceFolder)
  const logs = useLogStore((s) => s.logs)
  const unavailable = useLogStore((s) => s.unavailable)
  const listError = useLogStore((s) => s.listError)
  const openKey = useLogStore(selectOpenKey)
  const load = useLogStore((s) => s.load)
  const open = useLogStore((s) => s.open)
  const clear = useLogStore((s) => s.clear)
  const [folderError, setFolderError] = useState<string | null>(null)

  useEffect(() => {
    if (installed) void load(chapter.id)
    return clear
  }, [installed, chapter.id, load, clear])

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
        {chapter.name} is not installed yet, so it has no logs.
      </p>
    )
  } else if (installed === undefined) {
    body = (
      <p className="text-fg-muted m-0 text-sm leading-normal">
        The instance could not be looked for. Set the Prism data folder in Settings.
      </p>
    )
  } else if (listError) {
    body = <ErrorLine>{listError}</ErrorLine>
  } else if (unavailable) {
    body = <p className="text-fg-muted m-0 text-sm">The logs can only be read in the app window.</p>
  } else if (!logs) {
    body = <p className="text-fg-muted m-0 text-sm">Reading the logs.</p>
  } else if (logs.length === 0) {
    body = (
      <p className="text-fg-muted m-0 text-sm">No logs yet. They appear after the first Play.</p>
    )
  } else {
    body = (
      <>
        <LogList logs={logs} openKey={openKey} onOpen={(log) => void open(chapter.id, log)} />
        <LogViewer chapter={chapter} />
      </>
    )
  }

  return (
    <Page
      label={`${chapter.name} logs`}
      title={`${chapter.name} logs`}
      onBack={onClose}
      actions={
        <Button onClick={() => void onOpenFolder()} disabled={installed !== true}>
          <Icon icon={FolderOpen} size="sm" />
          <span>Open folder</span>
        </Button>
      }
    >
      {folderError && <ErrorLine>{folderError}</ErrorLine>}
      {/* One list and its viewer, not sections: they sit closer than a page's blocks do. */}
      <div className="flex flex-col gap-6">{body}</div>
    </Page>
  )
}
