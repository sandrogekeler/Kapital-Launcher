import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { ComponentProps } from 'react'
import type { GamePhase, GameState } from '../../types'
import { SplashCard } from './SplashCard'

const chapter = { id: 'luxemburg', name: 'Luxemburg', packVersion: '4.2' }
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

const actions = () => ({ onLeave: vi.fn(), onOpenFolder: vi.fn(), onCopyLog: vi.fn() })

function card(state: GameState, props: Partial<ComponentProps<typeof SplashCard>> = {}) {
  return <SplashCard chapter={chapter} state={state} {...actions()} {...props} />
}

describe('SplashCard', () => {
  afterEach(cleanup)

  it('scopes itself to the chapter and shows its title art, name and pack version', () => {
    render(card(at('starting')))
    const section = screen.getByRole('region', { name: `Loading ${chapter.name}` })
    expect(section).toHaveAttribute('data-chapter', chapter.id)
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName(chapter.name)
    expect(screen.getByText('Luxemburg · Pack 4.2')).toBeInTheDocument()
  })

  it.each<[string | undefined]>([[undefined], [''], ['[PLACEHOLDER]']])(
    'shows no pack version for %j',
    (packVersion) => {
      render(card(at('starting'), { chapter: { ...chapter, packVersion } }))
      expect(screen.queryByText(/Pack /)).not.toBeInTheDocument()
    },
  )

  it.each<[GamePhase, string, string]>([
    ['starting', '◐ Starting', `Prism is getting ${chapter.name} ready`],
    ['mods', '◐ Loading mods', 'The game has started'],
    ['window', '◐ Loading mods', 'The game window is up'],
    ['resources', '◐ Loading resources', 'Almost there'],
  ])('says the %s stage', (phase, line, detail) => {
    render(card(at(phase)))
    expect(screen.getByText(line)).toBeInTheDocument()
    expect(screen.getByText(detail)).toBeInTheDocument()
  })

  it('still says something for a phase with no line of its own', () => {
    render(card(at('closed')))
    expect(screen.getByText('◐ Starting')).toBeInTheDocument()
  })

  it('is indeterminate without an estimate', () => {
    render(card(at('starting')))
    expect(bar()).toHaveAttribute('data-bar', 'indeterminate')
    expect(bar()).toHaveClass('splash-shimmer')
  })

  it("eases from the phase's own place to the next one over the gap, through custom properties", () => {
    const { rerender } = render(card(at('starting', { estimate })))
    expect(bar()).toHaveAttribute('data-bar', 'determinate')
    expect(bar()).toHaveAttribute('data-from', '0')
    expect(bar()).toHaveAttribute('data-to', '0.5')
    expect(bar()).toHaveAttribute('data-ms', '20000')
    expect(bar()?.style.getPropertyValue('--splash-fill')).toBe('0.5')
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('20000ms')

    rerender(card(at('mods', { estimate })))
    expect(bar()).toHaveAttribute('data-from', '0.5')
    expect(bar()?.style.getPropertyValue('--splash-fill')).toBe('0.75')
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('10000ms')

    rerender(card(at('resources', { estimate })))
    expect(bar()?.style.getPropertyValue('--splash-fill')).toBe('1')
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('0ms')
  })

  it('does not restart the ease when the same phase is pushed again', () => {
    const { rerender } = render(card(at('mods', { estimate })))
    bar()?.style.setProperty('--splash-ms', 'sentinel')
    rerender(card(at('mods', { estimate: { ...estimate } })))
    expect(bar()?.style.getPropertyValue('--splash-ms')).toBe('sentinel')
  })

  it('asks to leave from Back to launcher', () => {
    const a = actions()
    render(card(at('mods'), a))
    fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
    expect(a.onLeave).toHaveBeenCalledOnce()
  })

  it('shows what a failed action says', () => {
    render(card(at('mods'), { error: 'the window could not be restored' }))
    expect(screen.getByRole('alert')).toHaveTextContent('the window could not be restored')
  })

  it('shows no alert when nothing failed', () => {
    render(card(at('mods')))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  describe('error state', () => {
    it.each<[GamePhase, string]>([
      ['crashed', '○ The game stopped'],
      ['failed', '○ The game did not start'],
    ])('on %s keeps the card with the line, no bar and three buttons', (phase, line) => {
      render(card(at(phase, { estimate })))
      expect(screen.getByText(line)).toHaveClass('text-danger')
      expect(bar()).toBeNull()
      expect(screen.getByRole('button', { name: 'Open folder' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      expect(screen.getAllByRole('button', { name: 'Back to launcher' })).toHaveLength(1)
    })

    it.each<[GameState['reason'], string]>([
      ['packsync', "The pack could not be synced. Prism's window has the details"],
      ['launch', 'Prism stopped before the game. Its window has the details'],
    ])('says why a start failed (%s) and keeps the three buttons', (reason, detail) => {
      render(card(at('failed', { reason, estimate })))
      expect(screen.getByText('○ The game did not start')).toHaveClass('text-danger')
      expect(screen.getByText(detail)).toBeInTheDocument()
      expect(bar()).toBeNull()
      expect(screen.getByRole('button', { name: 'Open folder' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      expect(screen.getAllByRole('button', { name: 'Back to launcher' })).toHaveLength(1)
    })

    it('asks for the three actions', () => {
      const a = actions()
      render(card(at('crashed'), a))
      fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
      fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
      expect(a.onOpenFolder).toHaveBeenCalledOnce()
      expect(a.onCopyLog).toHaveBeenCalledOnce()
      expect(a.onLeave).toHaveBeenCalledOnce()
    })

    it('says the exit code', () => {
      render(card(at('crashed', { exitCode: 1 })))
      expect(screen.getByText(/exit code 1/)).toBeInTheDocument()
    })

    it('says how much of the log was copied', () => {
      render(card(at('crashed'), { copyLog: { lines: 12 } }))
      expect(screen.getByText(/Copied 12 lines to the clipboard/)).toHaveClass('text-fg-muted')
    })

    it('says why the log was not copied', () => {
      render(card(at('failed'), { copyLog: { error: 'empty log' } }))
      expect(screen.getByText('empty log')).toHaveClass('text-danger')
    })

    it('says nothing about the log before one was copied', () => {
      render(card(at('failed')))
      expect(screen.getByRole('status')).toBeEmptyDOMElement()
    })
  })
})
