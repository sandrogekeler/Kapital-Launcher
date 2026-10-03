import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Drift } from './Drift'

/** jsdom has no AnimationEvent, so React listens for the prefixed name there. */
const endAnimation = (el: Element) =>
  fireEvent(el, new Event('webkitAnimationEnd', { bubbles: true }))

describe('Drift', () => {
  afterEach(cleanup)

  it('shows the first content still', () => {
    render(<Drift id="a">first</Drift>)
    expect(screen.getByText('first')).not.toHaveClass('drift-in')
    expect(document.querySelector('[inert]')).toBeNull()
  })

  it('drifts the old content out and the new in, and drops the old when it has gone', () => {
    const { rerender } = render(<Drift id="a">first</Drift>)
    rerender(<Drift id="b">second</Drift>)
    expect(screen.getByText('second')).toHaveClass('drift-in')
    const leaving = document.querySelector('[inert]')!
    expect(leaving).toHaveClass('drift-out')
    expect(leaving).toHaveTextContent('first')
    endAnimation(leaving)
    expect(document.querySelector('[inert]')).toBeNull()
    expect(screen.getByText('second')).toBeInTheDocument()
  })

  it('keeps the same content in place when only the children change', () => {
    const { rerender } = render(<Drift id="a">first</Drift>)
    rerender(<Drift id="a">first, edited</Drift>)
    expect(screen.getByText('first, edited')).toBeInTheDocument()
    expect(document.querySelector('[inert]')).toBeNull()
  })

  it('changes without a drift in a hidden window', () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    try {
      const { rerender } = render(<Drift id="a">first</Drift>)
      rerender(<Drift id="b">second</Drift>)
      expect(document.querySelector('[inert]')).toBeNull()
      expect(screen.getByText('second')).not.toHaveClass('drift-in')
    } finally {
      visibility.mockRestore()
    }
  })
})
