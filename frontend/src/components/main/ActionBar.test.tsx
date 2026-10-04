import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import type { ComponentProps } from 'react'
import { ActionBar } from './ActionBar'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { Chapter, GamePhase, GameState, PrismInstallProgress, ServerStatus } from '../../types'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'frangfurd')!
const noop = () => undefined
/** A chapter with no server, whose right side is the pack line. */
const serverless: Chapter = {
  ...BUNDLED_MANIFEST.chapters.find((c) => c.id === 'luxemburg')!,
  server: null,
}

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
      serverChoice={undefined}
      instancePack={undefined}
      packState={undefined}
      game={undefined}
      launching={false}
      installing={false}
      installedNow={false}
      checking={false}
      release={null}
      install={null}
      onPlay={noop}
      onStop={noop}
      onInstall={noop}
      onGetPrism={noop}
      onOpenPrismSite={noop}
      onOpenReleasePage={noop}
      onCheckServer={noop}
      {...over}
    />,
  )
}

const play = () => screen.getByRole('button', { name: new RegExp(chapter.name) })
const stop = () => screen.getByRole('button', { name: /^(Stop|Confirm)$/ })

/** The two rows beside Play (or Stop) are the element right after the button, before the spacer. */
const beside = () => screen.getAllByRole('button')[0]!.nextElementSibling as HTMLElement

describe('ActionBar game line', () => {
  afterEach(cleanup)

  it.each<GamePhase>(['crashed', 'failed', 'closed', 'idle'])(
    'shows nothing beside Play for a game that is %s: a run that ended is the corner notice',
    (phase) => {
      bar({ game: game(phase) })
      expect(screen.queryByText('○ The game did not start')).toBeNull()
      expect(screen.queryByText('○ The game stopped')).toBeNull()
      expect(screen.queryByRole('button', { name: 'Details' })).toBeNull()
      expect(play().nextElementSibling?.className).toContain('grow')
    },
  )

  it('shows nothing beside Play for a run the player stopped', () => {
    bar({ game: { ...game('crashed'), reason: 'stopped' } })
    expect(screen.queryByText('○ The game was stopped')).toBeNull()
    expect(play().nextElementSibling?.className).toContain('grow')
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

  it('shows the usual line and Play again after a crash', () => {
    bar({ game: game('crashed', 1), status: online })
    expect(screen.queryByText('○ The game stopped')).toBeNull()
    expect(screen.getByText('● Server online')).toBeInTheDocument()
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

  it('keeps the server line on the right after a failed game', () => {
    bar({ game: game('failed'), status: online })
    expect(screen.getByText('● Server online')).toBeInTheDocument()
    expect(screen.queryByText('○ The game did not start')).toBeNull()
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

  it('leaves the right side as the pack line without a server or a game', () => {
    const lux = serverless
    bar({ chapter: lux })
    expect(screen.getByText('● Updated')).toBeInTheDocument()
    expect(screen.queryByText('● Ready')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Check the server now' })).toBeNull()
    expect(
      screen.getByRole('button', { name: new RegExp(lux.name) }).nextElementSibling,
    ).toHaveClass('grow')
  })

  it.each([
    ['in step', true, '7.1.0'],
    ['behind its source', false, '7.1.1'],
  ])(
    'reads Play and names the source version when the installed pack is %s',
    (_, upToDate, version) => {
      const lux = serverless
      bar({
        chapter: lux,
        packState: { chapterId: lux.id, installed: true, checked: true, upToDate, version },
      })
      expect(screen.getByRole('button', { name: `Play ${lux.name}` })).toBeEnabled()
      expect(screen.queryByRole('button', { name: /update/i })).toBeNull()
      expect(screen.getByText('● Updated')).toBeInTheDocument()
      expect(screen.getByText(`Version ${version}`)).toBeInTheDocument()
      expect(screen.queryByText('● Update available')).toBeNull()
    },
  )

  it('says the version is pending when none is known', () => {
    const lux = serverless
    bar({ chapter: { ...lux, pack: { ...lux.pack, version: null } } })
    expect(screen.getByText('● Updated')).toBeInTheDocument()
    expect(screen.getByText('Version pending')).toBeInTheDocument()
  })

  it.each([true, false])(
    'keeps the server line for a chapter with a server whether the pack is in step (%s) or behind',
    (upToDate) => {
      bar({
        status: online,
        packState: {
          chapterId: chapter.id,
          installed: true,
          checked: true,
          upToDate,
          version: '1.0.1',
        },
      })
      expect(play()).toBeEnabled()
      expect(screen.getByText('● Server online')).toBeInTheDocument()
      expect(screen.queryByText('● Updated')).toBeNull()
      expect(screen.queryByText('● Update available')).toBeNull()
    },
  )

  it('shows the server refresh only while the server line is the one shown', () => {
    bar({ status: online })
    expect(screen.getByRole('button', { name: 'Check the server now' })).toBeInTheDocument()
    cleanup()
    // Not installed, a dev pack, and an install in progress are not about the server.
    bar({ installed: false })
    expect(screen.getByText('○ Not installed')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Check the server now' })).toBeNull()
    cleanup()
    bar({ installedNow: true })
    expect(screen.getByText('● Installed')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Check the server now' })).toBeNull()
  })

  it('keeps the refresh button place when another line is shown, so the text does not step', () => {
    bar({ installed: false })
    const line = screen.getByText('○ Not installed').parentElement!
    expect(line.nextElementSibling).toHaveAttribute('aria-hidden', 'true')
    expect(line.nextElementSibling).toHaveClass('size-8')
  })

  it('keeps two lines in the state block while the engine is being checked', () => {
    bar({ engine: null })
    const state = screen.getByText('○ Checking engine')
    expect(state.nextElementSibling?.textContent).toBe('\u00a0')
  })

  it('shows players and ping under the server state, and never an address', () => {
    bar({ status: online })
    expect(screen.getByText('3/20 players · 42 ms')).toBeInTheDocument()
    const text = document.body.textContent ?? ''
    for (const { address } of chapter.server!.addresses) expect(text).not.toContain(address)
    // Whichever address the player picked.
    cleanup()
    bar({ status: online, serverChoice: 'Germany' })
    for (const { address } of chapter.server!.addresses) {
      expect(document.body.textContent).not.toContain(address)
    }
  })

  it('keeps both rows while the first ping is out, and says offline with the time', () => {
    bar({ status: undefined })
    const checking = screen.getByText('○ Checking server')
    expect(checking.nextElementSibling?.textContent).toBe(' ')
    cleanup()
    bar({ status: { ...online, online: false } })
    expect(screen.getByText('○ Server offline').nextElementSibling?.textContent).toMatch(/^as of /)
    // Down is red; an address nobody has settled yet stays quiet.
    expect(screen.getByText('○ Server offline')).toHaveClass('text-danger')
  })

  it('reads a placeholder address as pending only when that is the address in use', () => {
    const lichdenstein = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'lichdenstein')!
    const unsettled: Chapter = {
      ...lichdenstein,
      server: {
        ...lichdenstein.server!,
        addresses: [{ label: 'Main', address: 'placeholder.invalid' }],
      },
    }
    bar({ chapter: unsettled, status: online })
    expect(screen.getByText('○ No server yet')).toHaveClass('text-fg-muted')
    expect(screen.getByText('Address pending')).toBeInTheDocument()
  })

  it('sets a status label in the UI face and its detail in the data it names', () => {
    bar({ status: online })
    expect(screen.getByText('● Server online')).toHaveClass('font-medium')
    expect(screen.getByText('● Server online')).not.toHaveClass('font-mono')
  })

  it('gives the play button one width whatever it says', () => {
    bar()
    expect(play()).toHaveClass('min-w-(--layout-play-min)', 'justify-center')
    cleanup()
    bar({ game: game('running') })
    expect(stop()).toHaveClass('min-w-(--layout-play-min)')
  })

  it('keeps the width of the longer label on the Prism button, Try again or Get Prism Launcher', () => {
    bar({ engine: { found: false, executable: '', version: '', root: '', source: '' } })
    const button = screen.getByRole('button', { name: 'Get Prism Launcher' })
    expect(button).toHaveTextContent('Get Prism LauncherTry again')
    expect(screen.getByText('Try again')).toHaveClass('invisible')
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
      expect(stop()).toHaveTextContent('Confirm')
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
    expect(stop()).toHaveTextContent('Confirm')
    act(() => void vi.advanceTimersByTime(4900))
    expect(stop()).toHaveTextContent('Confirm')
    act(() => void vi.advanceTimersByTime(200))
    expect(stop()).toHaveTextContent(/^Stop$/)
    fireEvent.click(stop())
    expect(onStop).not.toHaveBeenCalled()
  })

  it('is red, the danger colour, from the start to the question and back', () => {
    bar({ game: game('running') })
    expect(stop()).toHaveClass('bg-danger', 'glow-danger', 'min-w-(--layout-play-min)')
    expect(stop()).not.toHaveClass('bg-accent')
    fireEvent.click(stop())
    expect(stop()).toHaveTextContent('Confirm')
    expect(stop()).toHaveClass('bg-danger', 'glow-danger')
  })

  it('is red while the hand-over to Prism is still out, and waits', () => {
    bar({ launching: true })
    expect(stop()).toHaveClass('bg-danger')
    expect(stop()).toBeDisabled()
  })

  it('keeps Play in the accent', () => {
    bar()
    expect(play()).toHaveClass('bg-accent', 'glow-accent')
    expect(play()).not.toHaveClass('bg-danger')
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
    unmount()
    bar({ game: { ...game('crashed', 1), reason: 'stopped' } })
    expect(play()).toBeEnabled()
  })
})
