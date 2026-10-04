import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { Toasts } from './Toasts'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import type { GameState } from '../../types'

vi.mock('../../../wailsjs/go/main/App')

const failed: GameState = {
  chapterId: 'frangfurd',
  phase: 'failed',
  reason: 'launch',
  since: '2026-10-01T10:00:04Z',
  startedAt: '2026-10-01T10:00:00Z',
}

const notices = () => screen.getByRole('status')

describe('Toasts', () => {
  beforeEach(() => {
    useChapterStore.setState({ manifest: BUNDLED_MANIFEST })
    useEngineStore.setState({ error: null })
    useGameStore.setState({ states: {}, stopErrors: {} })
  })
  afterEach(cleanup)

  it('is an empty live region with nothing to say', () => {
    render(<Toasts onOpenReport={() => undefined} />)
    expect(notices()).toBeEmptyDOMElement()
    expect(notices()).toHaveAttribute('aria-live', 'polite')
  })

  it("shows the engine's refusal and lets it be dismissed, until a new one", () => {
    render(<Toasts onOpenReport={() => undefined} />)
    act(() => useEngineStore.setState({ error: 'launch frangfurd: access is denied' }))
    expect(within(notices()).getByText('launch frangfurd: access is denied')).toHaveClass(
      'select-text',
    )
    fireEvent.click(within(notices()).getByRole('button', { name: 'Dismiss' }))
    expect(notices()).toBeEmptyDOMElement()

    // The same error is still dismissed; a different one is news.
    act(() => useEngineStore.setState({ error: null }))
    act(() => useEngineStore.setState({ error: 'launch frangfurd: access is denied' }))
    expect(notices()).toBeEmptyDOMElement()
    act(() => useEngineStore.setState({ error: 'install frangfurd: no room' }))
    expect(within(notices()).getByText('install frangfurd: no room')).toBeInTheDocument()
  })

  it('goes away when the store clears its error', () => {
    useEngineStore.setState({ error: 'refused' })
    render(<Toasts onOpenReport={() => undefined} />)
    expect(within(notices()).getByText('refused')).toBeInTheDocument()
    act(() => useEngineStore.setState({ error: null }))
    expect(notices()).toBeEmptyDOMElement()
  })

  it("names the failed run in the game line's words and opens its report from Details", () => {
    const onOpenReport = vi.fn()
    render(<Toasts onOpenReport={onOpenReport} />)
    act(() => useGameStore.getState().receive(failed))
    expect(within(notices()).getByText('○ The game did not start')).toHaveClass('text-danger')
    expect(within(notices()).getByText('Prism stopped before the game')).toBeInTheDocument()
    expect(notices().querySelector('svg.lucide-triangle-alert')).not.toBeNull()
    // The title is a label, set in the UI face, not in the data face.
    expect(within(notices()).getByText('○ The game did not start')).not.toHaveClass('font-mono')
    fireEvent.click(within(notices()).getByRole('button', { name: 'Details' }))
    expect(onOpenReport).toHaveBeenCalledExactlyOnceWith('frangfurd')
  })

  it('drops the notice when the chapter is played again, and shows the next end', () => {
    render(<Toasts onOpenReport={() => undefined} />)
    act(() => useGameStore.getState().receive(failed))
    fireEvent.click(within(notices()).getByRole('button', { name: 'Dismiss' }))
    expect(notices()).toBeEmptyDOMElement()
    act(() =>
      useGameStore
        .getState()
        .receive({ ...failed, phase: 'starting', since: '2026-10-01T10:05:00Z' }),
    )
    expect(notices()).toBeEmptyDOMElement()
    act(() => useGameStore.getState().receive({ ...failed, since: '2026-10-01T10:05:03Z' }))
    expect(within(notices()).getByText('○ The game did not start')).toBeInTheDocument()
  })

  // A developer preview (issue 124) is a synthetic game:state through the same path
  // as a run's: the notice is the real one, and a real event replaces it.
  it('shows the notice of a previewed failure and a real event of the chapter replaces it', () => {
    render(<Toasts onOpenReport={() => undefined} />)
    act(() => useGameStore.getState().receive({ ...failed, reason: 'packsync' }))
    expect(within(notices()).getByText('○ The game did not start')).toBeInTheDocument()
    expect(within(notices()).getByText('The pack could not be synced')).toBeInTheDocument()

    act(() =>
      useGameStore
        .getState()
        .receive({ ...failed, phase: 'crashed', reason: undefined, exitCode: 1, since: 'later' }),
    )
    expect(within(notices()).getByText('○ The game stopped')).toBeInTheDocument()
    expect(
      within(notices()).getByText('It closed before it finished, exit code 1'),
    ).toBeInTheDocument()

    act(() =>
      useGameStore
        .getState()
        .receive({ ...failed, phase: 'mods', reason: undefined, since: 'next' }),
    )
    expect(notices()).toBeEmptyDOMElement()
    expect(useGameStore.getState().states.frangfurd?.phase).toBe('mods')
  })

  it('confirms a stop the player asked for in the muted tone, without Details', () => {
    render(<Toasts onOpenReport={() => undefined} />)
    act(() => useGameStore.getState().receive({ ...failed, reason: 'stopped' }))
    expect(within(notices()).getByText('○ The game did not start')).toHaveClass('text-fg-muted')
    expect(within(notices()).queryByRole('button', { name: 'Details' })).toBeNull()
    // A stop the player asked for is not a warning: a neutral mark, not the triangle.
    expect(notices().querySelector('svg.lucide-info')).not.toBeNull()
    expect(notices().querySelector('svg.lucide-triangle-alert')).toBeNull()
  })

  it('shows a refused stop', () => {
    render(<Toasts onOpenReport={() => undefined} />)
    act(() => useGameStore.setState({ stopErrors: { frangfurd: 'Frangfurd has no game to stop' } }))
    expect(within(notices()).getByText('○ The game could not be stopped')).toBeInTheDocument()
    expect(within(notices()).getByText('Frangfurd has no game to stop')).toBeInTheDocument()
  })

  it('stays out of the layout: fixed in the corner, letting clicks through between notices', () => {
    render(<Toasts onOpenReport={() => undefined} />)
    expect(notices().className).toContain('fixed')
    expect(notices().className).toContain('pointer-events-none')
    act(() => useEngineStore.setState({ error: 'refused' }))
    expect(notices().firstElementChild?.className).toContain('pointer-events-auto')
  })
})
