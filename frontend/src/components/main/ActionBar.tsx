import { useEffect, useState } from 'react'
import type {
  Chapter,
  EngineInfo,
  GamePhase,
  GameState,
  PackState,
  PrismInstallProgress,
  PrismRelease,
  ServerStatus,
} from '../../types'
import { installLabel, isPlaceholderAddress, isPublished, playLabel } from '../../lib/manifest'
import { installLine } from '../../lib/prismInstall'
import { updateAvailable } from '../../lib/packState'
import { packHost, packSourceLine } from '../../lib/packSource'
import { serverLine } from '../../lib/serverLine'
import { gameLine } from '../../lib/gameLine'
import { isActive } from '../../stores/useGameStore'
import { Download, Play, RefreshCw, Square, TriangleAlert } from '../../lib/icons'
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
  /** A local packwiz serve address from settings, which Install uses instead (#41). */
  devPack: string | undefined
  /** The pack URL the chapter's instance syncs from, when the launcher made it. */
  instancePack: string | undefined
  /** Whether the installed pack is its source's current one (#71). */
  packState: PackState | undefined
  /** Where the chapter's game is, from Play to its end (#44). */
  game: GameState | undefined
  launching: boolean
  /** Whether the chapter's instance is being written right now. */
  installing: boolean
  /** Whether the chapter was installed in this session and not played since. */
  installedNow: boolean
  checking: boolean
  error: string | null
  /** What getting Prism would download; null when it cannot be offered. */
  release: PrismRelease | null
  /** The install in progress or just finished, from prism:install. */
  install: PrismInstallProgress | null
  onPlay: () => void
  /** Ends the chapter's run at once. */
  onStop: () => void
  onInstall: () => void
  onGetPrism: () => void
  onOpenPrismSite: () => void
  onOpenReleasePage: () => void
  onCheckServer: () => void
  /** Opens the run report; Details, beside a game that crashed or never started. */
  onOpenReport: () => void
}

const WORKING = ['downloading', 'unpacking', 'verifying']

/** A game that crashed or never started: the phases that have a run report to read. */
const isEndedBadly = (phase: GamePhase | undefined) => phase === 'crashed' || phase === 'failed'

/**
 * The phases in which a stop asks twice: the game has a window or is past it, so a stray click
 * would throw away a world in play. Before that nothing is lost and one click stops.
 */
const CONFIRM_STOP: readonly (GameState['phase'] | undefined)[] = [
  'window',
  'resources',
  'running',
  'stopping',
]

/** How long "Stop the game?" waits for its second click. */
const CONFIRM_MS = 5000

/**
 * Play, and the state line beside it. The line says what the app knows:
 * whether Prism is there, whether the chapter's instance is, and for a chapter
 * with a server, whether it is up.
 *
 * A chapter whose instance is missing shows Install in Play's place (#24),
 * disabled until its pack is hosted. Install writes the instance; the pack
 * downloads on the first Play, which the line says until then. An instance
 * that cannot be looked for (unknown) keeps Play, and Prism says the rest.
 *
 * A developer's local pack (#41) makes Install available without a hosted
 * one, and an instance syncing from anything but the manifest's pack says so.
 *
 * When the installed pack is behind its source (#71), Play reads "Update and
 * play": Prism has no command that syncs without launching, and packwiz
 * syncs in the pre-launch step before the game starts, so the launch is the
 * update.
 *
 * While the chapter's game is starting, running or closing (#44) its own line
 * sits beside Play, right after the hand-over to Prism, and Play and Install
 * wait. Play becomes Stop, so a start that stalls or a game that has died
 * can be ended at once: one click while the start is only getting ready, and
 * once the game has a window the first click asks "Stop the game?" for five
 * seconds. Stop waits, disabled, while the hand-over to Prism has not returned. The server's status stays on the right throughout, so it is still
 * there when the player is launching. A game that crashed or never started
 * keeps its line until the next Play, but yields to an install in progress,
 * whose line is on the right. A game that closed normally says nothing.
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
  devPack,
  instancePack,
  packState,
  game,
  launching,
  installing,
  installedNow,
  checking,
  error,
  release,
  install,
  onPlay,
  onStop,
  onInstall,
  onGetPrism,
  onOpenPrismSite,
  onOpenReleasePage,
  onCheckServer,
  onOpenReport,
}: Props) {
  const [offering, setOffering] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const missing = engine !== null && !engine.found
  const working = install !== null && WORKING.includes(install.phase)
  const needsInstall = installed === false
  const behind = updateAvailable(packState)
  const published = isPublished(chapter) || devPack !== undefined
  const source = installed ? packSourceLine(instancePack, chapter.pack.packwiz) : null
  const playing = isActive(game?.phase)
  // The timer is cleared with the component, and by the click that spends it.
  useEffect(() => {
    if (!confirming) return
    const timer = setTimeout(() => setConfirming(false), CONFIRM_MS)
    return () => clearTimeout(timer)
  }, [confirming])
  const needsConfirm = CONFIRM_STOP.includes(game?.phase)
  const clickStop = () => {
    if (needsConfirm && !confirming) {
      setConfirming(true)
      return
    }
    setConfirming(false)
    onStop()
  }
  const gameRows = gameLine(game, chapter.name)
  const installShown =
    install !== null &&
    (working || install.phase === 'done' || (missing && install.phase === 'failed'))
  // The rows beside Play: the hand-over to Prism, then the game. A dead run's
  // rows give way to an install in progress; a live run's do not, as Play and
  // Install are held for it.
  let beside: [string, string, string] | null = null
  if (launching) {
    beside = ['◐ Launching', 'Handing over to Prism', 'text-accent']
  } else if (gameRows && (playing || !(installing || installShown))) {
    beside = gameRows
  }
  // A run that ended badly has a report to read, with its own rows beside Play.
  // One the player stopped did not go wrong, and has nothing to explain.
  const ended =
    beside !== null &&
    beside === gameRows &&
    isEndedBadly(game?.phase) &&
    game?.reason !== 'stopped'
  let state: string
  let meta: string
  let tone = 'text-accent'
  if (installing) {
    ;[state, meta] = ['◐ Installing', `Adding ${chapter.instance.id} to Prism`]
  } else if (install && installShown) {
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
  } else if (needsInstall && !published) {
    ;[state, meta, tone] = ['○ Not published yet', 'Its pack is not hosted yet', 'text-fg-muted']
  } else if (needsInstall) {
    ;[state, meta, tone] = [
      '○ Not installed',
      devPack
        ? `Install from the dev pack at ${packHost(devPack)}`
        : `Install adds ${chapter.instance.id} to Prism`,
      'text-warning',
    ]
  } else if (installedNow) {
    ;[state, meta] = ['● Installed', 'The first Play downloads the pack']
  } else if (behind) {
    ;[state, meta, tone] = [
      '● Update available',
      packState?.version
        ? `Pack ${packState.version}; it syncs before the game starts`
        : 'It syncs before the game starts',
      'text-warning',
    ]
  } else if (source) {
    ;[state, meta, tone] = [...source, 'text-warning']
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
        {playing || launching ? (
          <Button variant="play" onClick={clickStop} disabled={launching}>
            <Icon icon={Square} size="sm" className="fill-current" />
            <span>{needsConfirm && confirming ? 'Stop the game?' : 'Stop'}</span>
          </Button>
        ) : needsInstall ? (
          <Button
            variant="play"
            onClick={onInstall}
            disabled={!published || installing || missing || working}
          >
            <Icon icon={Download} size="sm" />
            <span>{installLabel(chapter)}</span>
          </Button>
        ) : (
          <Button variant="play" onClick={onPlay} disabled={installing || missing || working}>
            {behind ? (
              <Icon icon={RefreshCw} size="sm" />
            ) : (
              <Icon icon={Play} size="sm" className="fill-current" />
            )}
            <span>{behind ? 'Update and play' : playLabel(chapter)}</span>
          </Button>
        )}
        {missing && (
          <Button
            onClick={() => (release ? setOffering(true) : onOpenPrismSite())}
            disabled={working || offering}
          >
            {install?.phase === 'failed' ? 'Try again' : 'Get Prism Launcher'}
          </Button>
        )}
        {beside && (
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex max-w-sm min-w-0 flex-col gap-0.5">
              <span className={`font-mono text-xs ${beside[2]}`}>{beside[0]}</span>
              {/* Truncated to keep the bar one row; the title carries the full detail. */}
              <span className="text-fg-faint truncate text-xs" title={beside[1]}>
                {beside[1]}
              </span>
            </div>
            {ended && <Button onClick={onOpenReport}>Details</Button>}
          </div>
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
