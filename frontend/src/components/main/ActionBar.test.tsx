import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import type { ComponentProps } from 'react'
import { ActionBar } from './ActionBar'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { GamePhase, GameState, PrismInstallProgress } from '../../types'

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

  it('shows the usual line once the game has closed', () => {
    bar({ game: game('closed') })
    expect(screen.queryByText(/The game/)).toBeNull()
    expect(play()).toBeEnabled()
  })
})
