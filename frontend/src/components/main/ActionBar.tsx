import type { Chapter, EngineInfo } from '../../types'
import { playLabel } from '../../lib/manifest'
import { Play, TriangleAlert } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'

interface Props {
  chapter: Chapter
  engine: EngineInfo | null
  launching: boolean
  error: string | null
  onPlay: () => void
  onInstallPrism: () => void
}

/**
 * Play, and the state line beside it. What the line says is only what the app
 * knows: whether Prism is there. Pack sync and server status arrive with
 * milestones 4 to 6 and take the same slot.
 */
export function ActionBar({ chapter, engine, launching, error, onPlay, onInstallPrism }: Props) {
  const missing = engine !== null && !engine.found
  const state = launching
    ? '◐ Launching'
    : missing
      ? '○ Prism not found'
      : engine === null
        ? '○ Checking engine'
        : '● Ready'
  const meta = missing
    ? 'Install Prism Launcher and sign in there'
    : chapter.server
      ? `${chapter.server.address} · status pending`
      : chapter.pack.version
        ? `Pack ${chapter.pack.version}`
        : 'Pack version pending'

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
        <div className="flex flex-col items-end gap-0.5">
          <span className={`font-mono text-xs ${missing ? 'text-danger' : 'text-accent'}`}>
            {state}
          </span>
          <span className="text-fg-faint text-xs">{meta}</span>
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
