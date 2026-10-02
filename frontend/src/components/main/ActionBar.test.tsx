import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
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

/** The two rows beside Play are the element right after the button, before the spacer. */
const beside = () => play().nextElementSibling as HTMLElement

describe('ActionBar game line', () => {
  afterEach(cleanup)

  it('shows the game line and holds Play while the game is active', () => {
    bar({ game: game('resources') })
    expect(screen.getByText('◐ Loading resources')).toBeInTheDocument()
    expect(play()).toBeDisabled()
  })

  it('holds Install while the game is active', () => {
    bar({ game: game('running'), installed: false })
    expect(screen.getByText('● Playing')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /install/i })).toBeDisabled()
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

  it('shows the usual line once the game has closed', () => {
    bar({ game: game('closed') })
    expect(screen.queryByText(/The game/)).toBeNull()
    expect(play()).toBeEnabled()
  })
})
