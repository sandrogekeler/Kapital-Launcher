import type { Chapter, EngineInfo, ServerStatus } from '../../types'
import { playLabel } from '../../lib/manifest'
import { serverLine } from '../../lib/serverLine'
import { Play, RefreshCw, TriangleAlert } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'

interface Props {
  chapter: Chapter
  engine: EngineInfo | null
  status: ServerStatus | undefined
  launching: boolean
  checking: boolean
  error: string | null
  onPlay: () => void
  onInstallPrism: () => void
  onCheckServer: () => void
}

/**
 * Play, and the state line beside it. The line says what the app knows:
 * whether Prism is there, and for a chapter with a server, whether it is up.
 * Pack sync arrives with milestone 4 and takes the same slot.
 */
export function ActionBar({
  chapter,
  engine,
  status,
  launching,
  checking,
  error,
  onPlay,
  onInstallPrism,
  onCheckServer,
}: Props) {
  const missing = engine !== null && !engine.found
  let state: string
  let meta: string
  let tone = 'text-accent'
  if (launching) {
    ;[state, meta] = ['◐ Launching', 'Handing over to Prism']
  } else if (missing) {
    ;[state, meta, tone] = [
      '○ Prism not found',
      'Install Prism Launcher and sign in there',
      'text-danger',
    ]
  } else if (engine === null) {
    ;[state, meta] = ['○ Checking engine', '']
  } else if (chapter.server) {
    ;[state, meta] = serverLine(status, chapter.server.address)
    if (status?.checked && !status.online) tone = 'text-fg-muted'
  } else {
    ;[state, meta] = [
      '● Ready',
      chapter.pack.version ? `Pack ${chapter.pack.version}` : 'Pack version pending',
    ]
  }

  return (
    <section className="border-line flex flex-col gap-3 border-b px-14 py-5">
      <div className="flex items-center gap-3">
        <Button variant="play" onClick={onPlay} disabled={launching || missing}>
          <Icon icon={Play} size="sm" className="fill-current" />
          <span>{playLabel(chapter)}</span>
        </Button>
        {missing ? (
          <Button onClick={onInstallPrism}>Get Prism Launcher</Button>
        ) : (
          <Button onClick={() => undefined} disabled title="Pack sync arrives with milestone 4">
            Update pack
          </Button>
        )}
        <div className="grow" />
        <div className="flex items-center gap-2">
          <div className="flex flex-col items-end gap-0.5">
            <span className={`font-mono text-xs ${tone}`}>{state}</span>
            <span className="text-fg-faint text-xs">{meta}</span>
          </div>
          {chapter.server && (
            <IconButton
              icon={RefreshCw}
              title="Check the server now"
              size="sm"
              onClick={onCheckServer}
              disabled={checking}
            />
          )}
        </div>
      </div>
      {error && (
        <div className="text-danger flex items-center gap-2 text-sm select-text">
          <Icon icon={TriangleAlert} size="sm" />
          <span>{error}</span>
        </div>
      )}
    </section>
  )
}
