import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { SplashApp } from './SplashApp'
import { installBridge, useCardStore } from './bridge'

const push = (extra: Record<string, unknown> = {}) =>
  act(() =>
    window.kapitalSplash?.update({
      chapter: { id: 'frangfurd', name: 'Frangfurd', packVersion: '1.0.0' },
      game: {
        chapterId: 'frangfurd',
        phase: 'mods',
        since: '2026-10-01T10:00:00Z',
        startedAt: '2026-10-01T09:59:00Z',
        splash: true,
      },
      ...extra,
    }),
  )

describe('the card page', () => {
  const postMessage = vi.fn()
  let remove = () => {}
  beforeEach(() => {
    postMessage.mockReset()
    useCardStore.setState({ state: null })
    window.chrome = { webview: { postMessage } }
    remove = installBridge()
  })
  afterEach(() => {
    cleanup()
    remove()
    delete window.chrome
    delete document.documentElement.dataset.theme
  })

  it('renders nothing until the first state', () => {
    const { container } = render(<SplashApp />)
    expect(container).toBeEmptyDOMElement()
    push()
    expect(screen.getByRole('region', { name: 'Loading Frangfurd' })).toBeInTheDocument()
    expect(screen.getByText('◐ Loading mods')).toBeInTheDocument()
  })

  it('follows each state pushed', () => {
    render(<SplashApp />)
    push()
    const crashed = {
      chapterId: 'frangfurd',
      phase: 'crashed',
      since: '2026-10-01T10:01:00Z',
      startedAt: '2026-10-01T09:59:00Z',
      splash: true,
    }
    push({ game: crashed })
    expect(screen.getByText('○ The game stopped')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
    // The outcome of a copy arrives as the next state, and the button says it.
    push({ game: crashed, copyLog: { lines: 3 } })
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('shows the run report Go pushed with the state, and none before it', () => {
    render(<SplashApp />)
    const game = {
      chapterId: 'frangfurd',
      phase: 'crashed' as const,
      since: '2026-10-01T09:59:31Z',
      startedAt: '2026-10-01T09:59:00Z',
      splash: true,
    }
    push({ game })
    expect(screen.queryByLabelText("The end of the game's log")).toBeNull()
    push({
      game,
      report: {
        game,
        phases: [
          { phase: 'starting', ms: 0 },
          { phase: 'mods', ms: 4200 },
        ],
        logTail: '[Render thread/ERROR]: Reported exception thrown!\n',
        logLines: 1,
        logTruncated: false,
        crashReport: 'crash-1.txt',
        consoleAvailable: false,
      },
    })
    expect(screen.getByLabelText('Timeline')).toHaveTextContent('mods 4.2 s, crashed 31 s')
    expect(screen.getByLabelText("The end of the game's log")).toHaveTextContent(
      'Reported exception thrown!',
    )
    expect(screen.getByText('crash-1.txt')).toBeInTheDocument()
  })

  it("posts the page's three actions to Go", () => {
    render(<SplashApp />)
    push({
      game: {
        chapterId: 'frangfurd',
        phase: 'failed',
        since: '2026-10-01T10:01:00Z',
        startedAt: '2026-10-01T09:59:00Z',
        splash: true,
      },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    fireEvent.click(screen.getByRole('button', { name: 'Back to launcher' }))
    expect(postMessage.mock.calls.map((c) => c[0])).toEqual([
      '{"action":"openFolder"}',
      '{"action":"copyLog"}',
      '{"action":"leave"}',
    ])
  })

  it('takes the theme from the state, and follows the system without one', () => {
    render(<SplashApp />)
    push({ theme: 'light' })
    expect(document.documentElement.dataset.theme).toBe('light')
    push()
    expect(document.documentElement.dataset.theme).toBeUndefined()
  })
})
