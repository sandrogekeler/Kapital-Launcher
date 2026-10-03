import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import type { ComponentProps } from 'react'
import { ActionBar } from './ActionBar'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { GamePhase, GameState, PrismInstallProgress, ServerStatus } from '../../types'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'frangfurd')!
const noop = () => undefined

const game = (phase: GamePhase, exitCode?: number): GameState => ({
  chapterId: chapter.id,
  phase,
  since: '2026-10-01T10:00:00Z',
  startedAt: '2026-10-01T09:59:00Z',
  ...(exitCode === undefined ? {} : { exitCode }),
})

const online: ServerStatus = {
  chapterId: chapter.id,
  checked: true,
  online: true,
  players: 3,
  max: 20,
  version: '1.20.1',
  motd: '',
  latencyMs: 42,
  checkedAt: '2026-10-01T10:00:00Z',
}

const installProgress: PrismInstallProgress = {
  phase: 'downloading',
  received: 40,
  total: 100,
  error: '',
}

function bar(over: Partial<ComponentProps<typeof ActionBar>> = {}) {
  return render(
    <ActionBar
      chapter={chapter}
      engine={{ found: true, executable: 'prism', version: '11', root: '', source: 'managed' }}
      status={undefined}
      installed={true}
      devPack={undefined}
      instancePack={undefined}
      packState={undefined}
      game={undefined}
      launching={false}
      installing={false}
      installedNow={false}
      checking={false}
      error={null}
      release={null}
      install={null}
      onPlay={noop}
      onStop={noop}
      onInstall={noop}
      onGetPrism={noop}
      onOpenPrismSite={noop}
      onOpenReleasePage={noop}
      onCheckServer={noop}
      onOpenReport={noop}
      {...over}
    />,
  )
}

const play = () => screen.getByRole('button', { name: new RegExp(chapter.name) })
const stop = () => screen.getByRole('button', { name: /^Stop/ })

/** The two rows beside Play (or Stop) are the element right after the button, before the spacer. */
const beside = () => screen.getAllByRole('button')[0]!.nextElementSibling as HTMLElement

describe('ActionBar game line', () => {
  afterEach(cleanup)

  it.each<GamePhase>(['crashed', 'failed'])(
    'offers Details beside the rows of a game that is %s, and opens the report',
    (phase) => {
      const onOpenReport = vi.fn()
      bar({ game: game(phase), onOpenReport })
      const details = within(beside()).getByRole('button', { name: 'Details' })
      fireEvent.click(details)
      expect(onOpenReport).toHaveBeenCalledOnce()
    },
  )

  it.each<GamePhase>(['starting', 'running', 'stopping', 'closed', 'idle'])(
    'offers no Details for a game that is %s',
    (phase) => {
      bar({ game: game(phase) })
      expect(screen.queryByRole('button', { name: 'Details' })).toBeNull()
    },
  )

  it('offers no Details for a run the player stopped', () => {
    bar({ game: { ...game('crashed'), reason: 'stopped' } })
    expect(screen.getByText('○ The game was stopped')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Details' })).toBeNull()
  })

  it('offers no Details while the rows beside Play are the hand-over or an install', () => {
    const { unmount } = bar({ game: game('failed'), launching: true })
    expect(screen.queryByRole('button', { name: 'Details' })).toBeNull()
    unmount()
    bar({ game: game('failed'), install: installProgress })
    expect(screen.queryByRole('button', { name: 'Details' })).toBeNull()
  })

  it("shows the game line and Stop in Play's place while the game is active", () => {
    bar({ game: game('resources') })
    expect(screen.getByText('◐ Loading resources')).toBeInTheDocument()
    expect(stop()).toBeEnabled()
    expect(screen.queryByRole('button', { name: new RegExp(chapter.name) })).toBeNull()
  })

  it("puts Stop in Install's place while the game is active", () => {
    bar({ game: game('running'), installed: false })
    expect(screen.getByText('● Playing')).toBeInTheDocument()
    expect(stop()).toBeEnabled()
    expect(screen.queryByRole('button', { name: /install/i })).toBeNull()
  })

  it('keeps the hand-over to Prism ahead of the game line', () => {
    bar({ game: game('running'), launching: true })
    expect(screen.getByText('◐ Launching')).toBeInTheDocument()
    expect(screen.queryByText('● Playing')).toBeNull()
  })

  it('lets an install in progress outrank a crash, and a crash outrank the usual line', () => {
    const { unmount } = bar({ game: game('crashed', 1), install: installProgress })
    expect(screen.queryByText('○ The game stopped')).toBeNull()
    unmount()
    bar({ game: game('crashed', 1) })
    expect(screen.getByText('○ The game stopped')).toHaveClass('text-danger')
    expect(screen.getByText('It closed before it finished, exit code 1')).toBeInTheDocument()
    expect(play()).toBeEnabled()
  })

  it('keeps the server line and its refresh on the right while the game is active', () => {
    bar({ game: game('running'), status: online })
    expect(screen.getByText('● Server online')).toBeInTheDocument()
    expect(screen.getByText(/3\/20 players/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check the server now' })).toBeEnabled()
    expect(within(beside()).getByText('● Playing')).toHaveClass('text-accent')
    expect(within(beside()).getByText(`${chapter.name} is running`)).toBeInTheDocument()
    expect(within(beside()).queryByText('● Server online')).toBeNull()
  })

  it('keeps the server line on the right beside a failed game', () => {
    bar({ game: game('failed'), status: online })
    expect(screen.getByText('● Server online')).toBeInTheDocument()
    expect(within(beside()).getByText('○ The game did not start')).toHaveClass('text-danger')
  })

  it('puts the hand-over to Prism beside Play, with the server on the right', () => {
    bar({ launching: true, status: online })
    expect(within(beside()).getByText('◐ Launching')).toBeInTheDocument()
    expect(screen.getByText('● Server online')).toBeInTheDocument()
  })

  it('keeps Stop disabled while the hand-over to Prism has not returned', () => {
    const onStop = vi.fn()
    bar({ launching: true, onStop })
    expect(stop()).toBeDisabled()
    fireEvent.click(stop())
    expect(onStop).not.toHaveBeenCalled()
  })

  it('shows the install on the right and nothing beside Play for a failed game', () => {
    bar({ game: game('failed'), install: installProgress, status: online })
    expect(screen.getByText('◐ Updating Prism · 40%')).toBeInTheDocument()
    expect(screen.queryByText('○ The game did not start')).toBeNull()
    expect(play().nextElementSibling?.className).toContain('grow')
  })

  it('leaves the right side as the ready line without a server or a game', () => {
    const lux = BUNDLED_MANIFEST.chapters.find((c) => !c.server)!
    bar({ chapter: lux })
    expect(screen.getByText('● Ready')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Check the server now' })).toBeNull()
    expect(
      screen.getByRole('button', { name: new RegExp(lux.name) }).nextElementSibling,
    ).toHaveClass('grow')
  })

  it('names the version the pack source serves on the ready line, not the manifest one', () => {
    const lux = BUNDLED_MANIFEST.chapters.find((c) => !c.server)!
    bar({
      chapter: lux,
      packState: {
        chapterId: lux.id,
        installed: true,
        checked: true,
        upToDate: true,
        version: '7.1.0',
      },
    })
    expect(screen.getByText('Pack 7.1.0')).toBeInTheDocument()
  })

  it('shows the usual line once the game has closed', () => {
    bar({ game: game('closed') })
    expect(screen.queryByText(/The game/)).toBeNull()
    expect(play()).toBeEnabled()
  })
})

describe('ActionBar Stop', () => {
  afterEach(() => {
    cleanup()
    vi.useRealTimers()
  })

  it('stops with one click while the start is only getting ready', () => {
    for (const phase of ['starting', 'mods'] as const) {
      const onStop = vi.fn()
      const { unmount } = bar({ game: game(phase), onStop })
      expect(stop()).toHaveTextContent('Stop')
      fireEvent.click(stop())
      expect(onStop).toHaveBeenCalledTimes(1)
      unmount()
    }
  })

  it('asks once more before ending a game that has a window, and the second click stops', () => {
    for (const phase of ['window', 'resources', 'running', 'stopping'] as const) {
      const onStop = vi.fn()
      const { unmount } = bar({ game: game(phase), onStop })
      fireEvent.click(stop())
      expect(onStop).not.toHaveBeenCalled()
      expect(stop()).toHaveTextContent('Stop the game?')
      fireEvent.click(stop())
      expect(onStop).toHaveBeenCalledTimes(1)
      expect(stop()).toHaveTextContent(/^Stop$/)
      unmount()
    }
  })

  it('takes the question back after five seconds', () => {
    vi.useFakeTimers()
    const onStop = vi.fn()
    bar({ game: game('running'), onStop })
    fireEvent.click(stop())
    expect(stop()).toHaveTextContent('Stop the game?')
    act(() => void vi.advanceTimersByTime(4900))
    expect(stop()).toHaveTextContent('Stop the game?')
    act(() => void vi.advanceTimersByTime(200))
    expect(stop()).toHaveTextContent(/^Stop$/)
    fireEvent.click(stop())
    expect(onStop).not.toHaveBeenCalled()
  })

  it('clears its timer with the component', () => {
    vi.useFakeTimers()
    const { unmount } = bar({ game: game('running') })
    fireEvent.click(stop())
    expect(vi.getTimerCount()).toBe(1)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('offers Play again once the run has ended, whatever the reason', () => {
    const { unmount } = bar({ game: { ...game('failed'), reason: 'stopped' } })
    expect(play()).toBeEnabled()
    expect(within(beside()).getByText('Stopped from the launcher')).toBeInTheDocument()
    expect(within(beside()).getByText('○ The game did not start')).toHaveClass('text-fg-muted')
    unmount()
    bar({ game: { ...game('crashed', 1), reason: 'stopped' } })
    expect(play()).toBeEnabled()
    expect(within(beside()).getByText('○ The game was stopped')).toBeInTheDocument()
  })

  it("shows the bar's error line for a refused stop", () => {
    bar({ game: game('starting'), error: 'Frangfurd has no game to stop' })
    expect(screen.getByText('Frangfurd has no game to stop')).toBeInTheDocument()
  })
})
