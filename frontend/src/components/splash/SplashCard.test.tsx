import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { ComponentProps } from 'react'
import type { GamePhase, GameState, RunReport } from '../../types'
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

const runReport = (over: Partial<RunReport> = {}): RunReport => ({
  game: at('crashed', { since: '2026-10-01T09:59:31Z' }),
  phases: [
    { phase: 'starting', ms: 0 },
    { phase: 'mods', ms: 4200 },
    { phase: 'window', ms: 9800 },
  ],
  logTail: '[Render thread/ERROR]: Reported exception thrown!\n',
  logLines: 1,
  logTruncated: false,
  crashReport: 'crash-2026-10-02_10.00.00-client.txt',
  consoleAvailable: false,
  ...over,
})

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
    ['starting', '◐ Starting', `Getting ${chapter.name} ready`],
    ['mods', '◐ Loading mods', 'The game has started'],
    ['window', '◐ Loading mods', 'The game window is running'],
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
      ['packsync', 'The pack could not be synced'],
      ['launch', 'Prism stopped before the game'],
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

    it('says a copy worked on the button itself, for a second, with no line under it', () => {
      vi.useFakeTimers()
      try {
        const { rerender } = render(card(at('crashed')))
        expect(screen.getByRole('button', { name: 'Copy log' })).toHaveAttribute(
          'aria-live',
          'polite',
        )
        rerender(card(at('crashed'), { copyLog: { lines: 12 } }))
        expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
        expect(screen.queryByRole('status')).toBeNull()
        expect(screen.queryByText(/to the clipboard/)).toBeNull()
        act(() => void vi.advanceTimersByTime(999))
        expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
        act(() => void vi.advanceTimersByTime(1))
        expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      } finally {
        vi.useRealTimers()
      }
    })

    it('says a copy failed on the button, for two seconds, and shows no reason under it', () => {
      vi.useFakeTimers()
      try {
        const { rerender } = render(card(at('failed')))
        rerender(card(at('failed'), { copyLog: { error: 'empty log' } }))
        expect(screen.getByRole('button', { name: 'Copy failed' })).toBeInTheDocument()
        expect(screen.queryByText('empty log')).toBeNull()
        act(() => void vi.advanceTimersByTime(1999))
        expect(screen.getByRole('button', { name: 'Copy failed' })).toBeInTheDocument()
        act(() => void vi.advanceTimersByTime(1))
        expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      } finally {
        vi.useRealTimers()
      }
    })

    it('flashes again for a second copy, and clears its timer when the card goes', () => {
      vi.useFakeTimers()
      try {
        const { rerender, unmount } = render(card(at('crashed')))
        rerender(card(at('crashed'), { copyLog: { lines: 1 } }))
        act(() => void vi.advanceTimersByTime(600))
        rerender(card(at('crashed'), { copyLog: { lines: 1 } }))
        act(() => void vi.advanceTimersByTime(600))
        expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
        unmount()
        expect(vi.getTimerCount()).toBe(0)
      } finally {
        vi.useRealTimers()
      }
    })

    it('says nothing about the log before one was copied', () => {
      render(card(at('failed')))
      expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      expect(screen.queryByRole('status')).toBeNull()
    })
  })

  describe('with a run report', () => {
    it('shows the timeline, the end of the log and the crash report beside the reason', () => {
      render(card(at('crashed', { since: '2026-10-01T09:59:31Z' }), { report: runReport() }))
      expect(screen.getByText('○ The game stopped')).toHaveClass('text-danger')
      expect(screen.getByLabelText('Timeline')).toHaveTextContent(
        'mods 4.2 s, window 9.8 s, crashed 31 s',
      )
      const log = screen.getByLabelText("The end of the game's log")
      expect(log).toHaveTextContent('Reported exception thrown!')
      expect(log).toHaveClass('select-text')
      expect(screen.getByText('crash-2026-10-02_10.00.00-client.txt')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Open folder' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
      expect(screen.getAllByRole('button', { name: 'Back to launcher' })).toHaveLength(1)
    })

    it("offers Prism's console only when the report says it can be shown", () => {
      const { unmount } = render(card(at('failed'), { report: runReport() }))
      expect(screen.queryByRole('button', { name: "Show Prism's console" })).toBeNull()
      unmount()
      const onShowConsole = vi.fn()
      render(card(at('failed'), { report: runReport({ consoleAvailable: true }), onShowConsole }))
      fireEvent.click(screen.getByRole('button', { name: "Show Prism's console" }))
      expect(onShowConsole).toHaveBeenCalledOnce()
    })

    it('shows no log, no sentence about it and no crash report when the start wrote none', () => {
      render(
        card(at('failed'), {
          report: runReport({ logTail: '', logLines: 0, crashReport: '', phases: [] }),
        }),
      )
      expect(screen.queryByText(/wrote no log for this run/)).toBeNull()
      expect(screen.queryByText(/Crash report/)).toBeNull()
      expect(screen.queryByLabelText("The end of the game's log")).toBeNull()
      expect(screen.getByText('○ The game did not start')).toBeInTheDocument()
    })

    it('shows no report while the game is starting, even if one were sent', () => {
      render(card(at('mods'), { report: runReport() }))
      expect(screen.queryByLabelText('Timeline')).toBeNull()
      expect(screen.queryByLabelText("The end of the game's log")).toBeNull()
    })

    it('has the failed card as before without one', () => {
      render(card(at('crashed')))
      expect(screen.queryByLabelText('Timeline')).toBeNull()
      expect(screen.getByRole('button', { name: 'Open folder' })).toBeInTheDocument()
    })
  })
})
