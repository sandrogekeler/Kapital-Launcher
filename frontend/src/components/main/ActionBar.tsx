import { useState } from 'react'
import type {
  Chapter,
  EngineInfo,
  PrismInstallProgress,
  PrismRelease,
  ServerStatus,
} from '../../types'
import { isPlaceholderAddress, playLabel } from '../../lib/manifest'
import { installLine } from '../../lib/prismInstall'
import { serverLine } from '../../lib/serverLine'
import { Play, RefreshCw, TriangleAlert } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'
import { GetPrism } from './GetPrism'

interface Props {
  chapter: Chapter
  engine: EngineInfo | null
  status: ServerStatus | undefined
  /** Whether the chapter's Prism instance exists; undefined when unknown. */
  installed: boolean | undefined
  launching: boolean
  checking: boolean
  error: string | null
  /** What getting Prism would download; null when it cannot be offered. */
  release: PrismRelease | null
  /** The install in progress or just finished, from prism:install. */
  install: PrismInstallProgress | null
  onPlay: () => void
  onGetPrism: () => void
  onOpenPrismSite: () => void
  onOpenReleasePage: () => void
  onCheckServer: () => void
}

const WORKING = ['downloading', 'unpacking', 'verifying']

/**
 * Play, and the state line beside it. The line says what the app knows:
 * whether Prism is there, whether the chapter's instance is, and for a chapter
 * with a server, whether it is up. A missing instance only warns: Play stays
 * enabled and Prism says the rest. Installing arrives with milestone 4.
 *
 * Without Prism, the bar offers to get it (ADR-11): the approval card opens
 * below, and once confirmed the state line follows the install step by step.
 * If the release cannot be read, the button falls back to Prism's website.
 */
export function ActionBar({
  chapter,
  engine,
  status,
  installed,
  launching,
  checking,
  error,
  release,
  install,
  onPlay,
  onGetPrism,
  onOpenPrismSite,
  onOpenReleasePage,
  onCheckServer,
}: Props) {
  const [offering, setOffering] = useState(false)
  const missing = engine !== null && !engine.found
  const working = install !== null && WORKING.includes(install.phase)
  let state: string
  let meta: string
  let tone = 'text-accent'
  if (launching) {
    ;[state, meta] = ['◐ Launching', 'Handing over to Prism']
  } else if (
    install &&
    (working || install.phase === 'done' || (missing && install.phase === 'failed'))
  ) {
    ;[state, meta] = installLine(install, engine?.found ? 'Updating' : 'Getting')
    if (install.phase === 'failed') tone = 'text-danger'
  } else if (missing) {
    ;[state, meta, tone] = [
      '○ Prism not found',
      release ? 'Kapital Launcher can get it for you' : 'Install Prism Launcher and sign in there',
      'text-danger',
    ]
  } else if (engine === null) {
    ;[state, meta] = ['○ Checking engine', '']
  } else if (installed === false) {
    ;[state, meta, tone] = [
      '○ Not in Prism yet',
      `No ${chapter.instance.id} instance was found`,
      'text-warning',
    ]
  } else if (chapter.server) {
    ;[state, meta] = serverLine(status, chapter.server.address)
    if (isPlaceholderAddress(chapter.server.address) || (status?.checked && !status.online)) {
      tone = 'text-fg-muted'
    }
  } else {
    ;[state, meta] = [
      '● Ready',
      chapter.pack.version ? `Pack ${chapter.pack.version}` : 'Pack version pending',
    ]
  }

  return (
    <section className="border-line flex flex-col gap-3 border-b px-14 py-5">
      <div className="flex items-center gap-3">
        <Button variant="play" onClick={onPlay} disabled={launching || missing || working}>
          <Icon icon={Play} size="sm" className="fill-current" />
          <span>{playLabel(chapter)}</span>
        </Button>
        {missing ? (
          <Button
            onClick={() => (release ? setOffering(true) : onOpenPrismSite())}
            disabled={working || offering}
          >
            {install?.phase === 'failed' ? 'Try again' : 'Get Prism Launcher'}
          </Button>
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
      {missing && offering && release && !working && (
        <GetPrism
          release={release}
          onConfirm={() => {
            setOffering(false)
            onGetPrism()
          }}
          onCancel={() => setOffering(false)}
          onOpenReleasePage={onOpenReleasePage}
          onOpenPrismSite={onOpenPrismSite}
        />
      )}
      {error && (
        <div className="text-danger flex items-center gap-2 text-sm select-text">
          <Icon icon={TriangleAlert} size="sm" />
          <span>{error}</span>
        </div>
      )}
    </section>
  )
}
