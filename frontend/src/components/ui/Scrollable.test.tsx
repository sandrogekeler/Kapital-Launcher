import { afterEach, describe, expect, it } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Scrollable } from './Scrollable'

// jsdom lays nothing out, so the element's geometry is set by hand: a 600px
// viewport over 1200px of content, scrolled by writing scrollTop.
function sized(el: HTMLElement, scrollHeight: number, clientHeight: number) {
  Object.defineProperty(el, 'scrollHeight', { value: scrollHeight, configurable: true })
  Object.defineProperty(el, 'clientHeight', { value: clientHeight, configurable: true })
}

describe('Scrollable', () => {
  afterEach(cleanup)

  it('draws no thumb while the content fits', () => {
    render(
      <Scrollable as="main">
        <p>short</p>
      </Scrollable>,
    )
    expect(screen.queryByRole('scrollbar')).not.toBeInTheDocument()
  })

  it('sizes and moves the thumb with the scroll position', () => {
    render(
      <Scrollable as="main" aria-label="Chapter">
        <p>long</p>
      </Scrollable>,
    )
    const main = screen.getByRole('main')
    sized(main, 1200, 600)
    act(() => {
      fireEvent.scroll(main)
    })
    const thumb = screen.getByRole('scrollbar')
    // Half the content is visible, so the thumb is half the track.
    expect(thumb.style.height).toBe('300px')
    expect(thumb.style.top).toBe('0px')

    main.scrollTop = 600
    act(() => {
      fireEvent.scroll(main)
    })
    expect(screen.getByRole('scrollbar').style.top).toBe('300px')
  })

  it('drags the content by the thumb', () => {
    render(
      <Scrollable as="main">
        <p>long</p>
      </Scrollable>,
    )
    const main = screen.getByRole('main')
    sized(main, 1200, 600)
    act(() => {
      fireEvent.scroll(main)
    })
    const thumb = screen.getByRole('scrollbar')
    fireEvent.pointerDown(thumb, { clientY: 100 })
    // 150px of thumb travel over a 300px track is half the 600px of hidden content.
    fireEvent(window, new MouseEvent('pointermove', { clientY: 250 }))
    expect(main.scrollTop).toBe(300)
    fireEvent(window, new MouseEvent('pointerup'))
    fireEvent(window, new MouseEvent('pointermove', { clientY: 400 }))
    expect(main.scrollTop).toBe(300)
  })

  it('keeps the thumb grabbable on a very long page', () => {
    render(
      <Scrollable as="main">
        <p>long</p>
      </Scrollable>,
    )
    const main = screen.getByRole('main')
    sized(main, 60000, 600)
    act(() => {
      fireEvent.scroll(main)
    })
    expect(screen.getByRole('scrollbar').style.height).toBe('32px')
  })
})
