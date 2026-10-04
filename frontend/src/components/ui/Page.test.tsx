import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Page } from './Page'

describe('Page', () => {
  afterEach(cleanup)

  it('sits in the chapter card frame with its header above a scrolling body', () => {
    render(
      <Page label="Things" title="Things" onBack={() => undefined} actions={<button>Do</button>}>
        <p>Body</p>
      </Page>,
    )
    const page = screen.getByRole('region', { name: 'Things' })
    expect(page).toHaveClass('bg-raised', 'border-line', 'rounded-lg', 'overflow-hidden')
    expect(page.parentElement).toHaveClass('m-5')
    const heading = screen.getByRole('heading', { level: 1, name: 'Things' })
    expect(heading).toHaveClass('font-display')
    // The header is outside the scrolling element, so it stays put.
    const scroller = screen.getByText('Body').closest('.overflow-y-auto')!
    expect(scroller).not.toContainElement(heading)
    expect(page).toContainElement(scroller as HTMLElement)
    expect(screen.getByRole('button', { name: 'Do' })).toBeInTheDocument()
  })

  it('closes on Back and on Escape', () => {
    const onBack = vi.fn()
    render(
      <Page label="Things" title="Things" onBack={onBack}>
        <p>Body</p>
      </Page>,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    act(() => void window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })))
    act(() => void window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' })))
    expect(onBack).toHaveBeenCalledTimes(2)
  })

  it('stops listening for Escape once it is gone', () => {
    const onBack = vi.fn()
    const { unmount } = render(
      <Page label="Things" title="Things" onBack={onBack}>
        <p>Body</p>
      </Page>,
    )
    unmount()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(onBack).not.toHaveBeenCalled()
  })
})
