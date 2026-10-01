import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { useGameStore } from '../../stores/useGameStore'
import type { GamePhase, GameState } from '../../types'
import { SplashCard } from './SplashCard'

vi.mock('../../../wailsjs/go/main/App')

const chapter = BUNDLED_MANIFEST.chapters[0]!
const estimate = { mods: 20_000, window: 30_000, resources: 40_000, running: 50_000 }

const at = (phase: GamePhase, extra: Partial<GameState> = {}): GameState => ({
  chapterId: chapter.id,
  phase,
  since: '2026-10-01T10:00:00Z',
  startedAt: '2026-10-01T09:59:00Z',
  splash: true,
  ...extra,
})

const bar = () => document.querySelector<HTMLElement>('[data-bar]')

describe('SplashCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useGameStore.setState({ states: {}, error: null })
    Object.assign(window, { go: {} })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('scopes itself to the chapter and shows its title art, name and pack version', () => {
    render(<SplashCard chapter={chapter} state={at('starting')} />)
    const card = screen.getByRole('region', { name: `Loading ${chapter.name}` })
    expect(card).toHaveAttribute('data-chapter', chapter.id)
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName(chapter.name)
    expect(screen.getByText(/Pack /)).toBeInTheDocument()
  })

  it.each<[GamePhase, string, string]>([
    ['starting', '◐ Starting', `Prism is getting ${chapter.name} ready`],
    ['mods', '◐ Loading mods', 'The game has started'],
    ['window', '◐ Loading mods', 'The game window is up'],
    ['resources', '◐ Loading resources', 'Almost there'],
  ])('says the %s stage', (phase, line, detail) => {
    render(<SplashCard chapter={chapter} state={at(phase)} />)
    expect(screen.getByText(line)).toBeInTheDocument()
    expect(screen.getByText(detail)).toBeInTheDocument()
  })

  it('still says something for a phase with no line of its own', () => {
    render(<SplashCard chapter={chapter} state={at('closed')} />)
    expect(screen.getByText('◐ Starting')).toBeInTheDocument()
  })

  it('is indeterminate without an estimate', () => {
    render(<SplashCard chapter={chapter} state={at('starting')} />)
    expect(bar()).toHaveAttribute('data-bar', 'indeterminate')
    expect(bar()).toHaveClass('splash-shimmer')
  })

  it("eases from the phase's own place to the next one over the gap, through custom properties", () => {
    const { rerender } = render(
      <SplashCard chapter={chapter} state={at('starting', { estimate })} />,
    )
    expect(bar()).toHaveAttribute('data-bar', 'determinate')
    expect(bar()).toHaveAttribute('data-from', '0')
    expect(bar()).toHaveAttribute('data-to', '0.5')
    expect(bar()).toHaveAttribute('data-ms', '20000')
    expect(bar()?.style.getPropertyValue('--splash-fill')).toBe('0.5')
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('20000ms')

    rerender(<SplashCard chapter={chapter} state={at('mods', { estimate })} />)
    expect(bar()).toHaveAttribute('data-from', '0.5')
    expect(bar()?.style.getPropertyValue('--splash-fill')).toBe('0.75')
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('10000ms')

    rerender(<SplashCard chapter={chapter} state={at('resources', { estimate })} />)
    expect(bar()?.style.getPropertyValue('--splash-fill')).toBe('1')
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('0ms')
  })

  it('does not restart the ease when the same phase is emitted again', () => {
    const { rerender } = render(<SplashCard chapter={chapter} state={at('mods', { estimate })} />)
    const fill = bar()
    fill?.style.setProperty('--splash-ms', 'sentinel')
    rerender(<SplashCard chapter={chapter} state={at('mods', { estimate: { ...estimate } })} />)
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('sentinel')
  })

  it('leaves through the store from Back to launcher', () => {
    vi.mocked(App.LeaveSplash).mockResolvedValue()
    render(<SplashCard chapter={chapter} state={at('mods')} />)
    fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
    expect(App.LeaveSplash).toHaveBeenCalledOnce()
  })

  it('does nothing visible when there is no bridge', () => {
    Reflect.deleteProperty(window, 'go')
    render(<SplashCard chapter={chapter} state={at('mods')} />)
    fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
    expect(App.LeaveSplash).not.toHaveBeenCalled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows a refused leave on the card', async () => {
    vi.mocked(App.LeaveSplash).mockRejectedValue('the window could not be restored')
    render(<SplashCard chapter={chapter} state={at('mods')} />)
    fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('the window could not be restored')
  })

  describe('error state', () => {
    it.each<[GamePhase, string]>([
      ['crashed', '○ The game stopped'],
      ['failed', '○ The game did not start'],
    ])('on %s keeps the card with the line, no bar and three buttons', (phase, line) => {
      render(<SplashCard chapter={chapter} state={at(phase, { estimate })} />)
      expect(screen.getByText(line)).toHaveClass('text-danger')
      expect(bar()).toBeNull()
      expect(screen.getByRole('button', { name: 'Open folder' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      expect(screen.getAllByRole('button', { name: 'Back to launcher' })).toHaveLength(1)
    })

    it('opens the instance folder', async () => {
      vi.mocked(App.OpenInstanceFolder).mockResolvedValue()
      render(<SplashCard chapter={chapter} state={at('crashed')} />)
      fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
      expect(App.OpenInstanceFolder).toHaveBeenCalledWith(chapter.id)
      await Promise.resolve()
    })

    it('says why the folder did not open', async () => {
      vi.mocked(App.OpenInstanceFolder).mockRejectedValue('no instance folder')
      render(<SplashCard chapter={chapter} state={at('failed')} />)
      fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
      expect(await screen.findByText('no instance folder')).toHaveClass('text-danger')
    })

    it('copies the log and says how much went, or why not', async () => {
      vi.mocked(App.CopyRedactedLog).mockResolvedValueOnce(12).mockRejectedValueOnce('empty log')
      render(<SplashCard chapter={chapter} state={at('crashed', { exitCode: 1 })} />)
      expect(screen.getByText(/exit code 1/)).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
      expect(await screen.findByText(/Copied 12 lines to the clipboard/)).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
      expect(await screen.findByText('empty log')).toBeInTheDocument()
    })

    it('leaves through the store', () => {
      vi.mocked(App.LeaveSplash).mockResolvedValue()
      render(<SplashCard chapter={chapter} state={at('crashed')} />)
      fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
      expect(App.LeaveSplash).toHaveBeenCalledOnce()
    })
  })
})
