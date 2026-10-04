import { useEffect, useState } from 'react'
import type { Chapter } from '../../types'
import { useMapStore } from '../../stores/useMapStore'
import { errMsg } from '../../lib/ipc'
import { ExternalLink } from '../../lib/icons'
import { OpenExternal } from '../../../wailsjs/go/main/App'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { ErrorLine } from '../ui/Notes'
import { Page } from '../ui/Page'

interface Props {
  chapter: Chapter
  onClose: () => void
}

/**
 * A chapter's map (issue 161), in the card the chapter's other pages take. The
 * map is the chapter's BlueMap, a web page on the server's own tunnel, framed
 * here so a player does not leave the launcher for it. An iframe cannot tell a
 * server that is down from one that is slow, so the page asks Go first
 * (CheckChapterMap) and stays unrevealed until it answers; only a map that
 * answered is framed. One that did not, and a chapter with no map yet, show
 * "Map could not be reached" with a Try again.
 *
 * The frame is sandboxed to scripts and its own storage, which BlueMap needs,
 * and no more: no top navigation, no popups, no form submission, and no
 * referrer. It is a different origin from the app, so it cannot reach the
 * bridge, and the app's policy lets the page frame the manifest's map origins
 * and nothing else (SECURITY_CHECKLIST S5.4). Open in browser, in the header,
 * hands the same address to the system browser through the usual checks.
 */
export function MapPanel({ chapter, onClose }: Props) {
  const status = useMapStore((s) => s.status)
  const check = useMapStore((s) => s.check)
  const clear = useMapStore((s) => s.clear)
  const [openError, setOpenError] = useState<string | null>(null)
  const url = chapter.map ?? ''

  useEffect(() => {
    void check(chapter.id, url)
    return clear
  }, [chapter.id, url, check, clear])

  const openInBrowser = () => {
    setOpenError(null)
    OpenExternal(url).catch((e) => setOpenError(errMsg(e)))
  }

  return (
    <Page
      fill
      ready={status !== null}
      label={`${chapter.name} map`}
      title={`${chapter.name} map`}
      onBack={onClose}
      actions={
        url && (
          <Button onClick={openInBrowser}>
            <Icon icon={ExternalLink} size="sm" />
            <span>Open in browser</span>
          </Button>
        )
      }
    >
      {openError && <ErrorLine>{openError}</ErrorLine>}
      {status === null ? (
        // A second check, from Try again: the page is already revealed.
        <p role="status" className="text-fg-muted m-0 text-sm">
          Checking the map.
        </p>
      ) : status.reachable && status.url ? (
        <iframe
          title={`${chapter.name} map`}
          src={status.url}
          sandbox="allow-scripts allow-same-origin"
          referrerPolicy="no-referrer"
          className="border-line-strong bg-sunken min-h-0 w-full grow rounded-md border"
        />
      ) : (
        <div className="flex min-h-0 grow flex-col items-center justify-center gap-4 text-center">
          <div className="flex flex-col gap-1.5">
            <h2 className="font-display m-0 text-xl font-semibold">Map could not be reached</h2>
            <p className="text-fg-muted m-0 max-w-prose text-sm leading-normal">
              {url ? 'The address did not answer.' : `There is no map for ${chapter.name} yet.`}
            </p>
          </div>
          <Button onClick={() => void check(chapter.id, url)}>Try again</Button>
        </div>
      )}
    </Page>
  )
}
