import { lazy } from 'react'
import type { ComponentProps } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { PageLayer } from './PageLayer'
import { STYLE_CSS as css } from '../../test-css'

/** jsdom has no AnimationEvent, so React listens for the prefixed name there. */
const endAnimation = (el: Element) =>
  fireEvent(el, new Event('webkitAnimationEnd', { bubbles: true }))

function layer(over: Partial<ComponentProps<typeof PageLayer>> = {}) {
  return (
    <PageLayer edge="right" open onExited={() => undefined} {...over}>
      <section aria-label="Things">
        <button>Back</button>
        <span>Body</span>
      </section>
    </PageLayer>
  )
}

/** The element that slides: the layer around the page. */
const sliding = () => screen.getByRole('region', { name: 'Things', hidden: true }).parentElement!
/** The layer's frame, which clips it and is inert once the page is leaving. */
const frame = () => sliding().parentElement!

describe('PageLayer', () => {
  afterEach(cleanup)

  it("slides a chapter's page in from the right, inside a frame the card's corners clip", () => {
    render(layer())
    expect(sliding()).toHaveClass('page-in-right', 'flex', 'h-full', 'flex-col')
    expect(frame()).toHaveClass('absolute', 'inset-0', 'overflow-hidden', 'rounded-lg')
    expect(frame()).not.toHaveAttribute('inert')
  })

  it('slides the settings down from the top over the whole card area', () => {
    render(layer({ edge: 'top' }))
    expect(sliding()).toHaveClass('page-in-top')
    expect(frame()).toHaveClass('absolute', 'inset-0')
    // The stage's own clip bounds it; the card's rounded frame does not.
    expect(frame()).not.toHaveClass('overflow-hidden')
  })

  it('stays mounted while it slides out, inert and hidden, and clicks pass through', () => {
    const onExited = vi.fn()
    const { rerender } = render(layer({ onExited }))
    rerender(layer({ onExited, open: false }))
    expect(sliding()).toHaveClass('page-out-right')
    expect(sliding()).not.toHaveClass('page-in-right')
    expect(frame()).toHaveAttribute('inert')
    expect(frame()).toHaveAttribute('aria-hidden', 'true')
    expect(frame()).toHaveClass('pointer-events-none')
    expect(screen.getByText('Body')).toBeInTheDocument()
    expect(onExited).not.toHaveBeenCalled()
    endAnimation(sliding())
    expect(onExited).toHaveBeenCalledTimes(1)
  })

  it('goes out upward for the settings', () => {
    const { rerender } = render(layer({ edge: 'top' }))
    rerender(layer({ edge: 'top', open: false }))
    expect(sliding()).toHaveClass('page-out-top')
  })

  it("ignores the end of the page's own animations, and any end while it is open", () => {
    const onExited = vi.fn()
    const { rerender } = render(layer({ onExited }))
    endAnimation(sliding())
    expect(onExited).not.toHaveBeenCalled()
    rerender(layer({ onExited, open: false }))
    endAnimation(screen.getByText('Body'))
    expect(onExited).not.toHaveBeenCalled()
  })

  it('comes back if it is opened again on its way out', () => {
    const { rerender } = render(layer())
    rerender(layer({ open: false }))
    rerender(layer())
    expect(sliding()).toHaveClass('page-in-right')
    expect(frame()).not.toHaveAttribute('inert')
    expect(frame()).not.toHaveAttribute('aria-hidden')
  })

  it('is removed at once when the window is hidden, as its animation would wait for it', () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    try {
      const onExited = vi.fn()
      const { rerender } = render(layer({ onExited }))
      expect(onExited).not.toHaveBeenCalled()
      rerender(layer({ onExited, open: false }))
      expect(onExited).toHaveBeenCalledTimes(1)
    } finally {
      visibility.mockRestore()
    }
  })

  it('shows the card frame, empty, while the page is on its way, inside the sliding layer', () => {
    const Pending = lazy(() => new Promise<never>(() => undefined))
    const { container } = render(
      <PageLayer edge="right" open onExited={() => undefined}>
        <Pending />
      </PageLayer>,
    )
    const fallback = container.querySelector('[aria-hidden="true"]')!
    expect(fallback).toHaveClass('bg-raised', 'border-line', 'rounded-lg', 'border')
    expect(fallback.parentElement).toHaveClass('page-in-right')
  })

  describe('focus', () => {
    it('moves to the page when it opens and back to the control that opened it when it closes', () => {
      const opener = document.createElement('button')
      opener.textContent = 'Open'
      document.body.append(opener)
      try {
        opener.focus()
        const { rerender } = render(layer())
        expect(sliding()).toHaveFocus()
        // Back, from inside, is the way a keyboard player closes it.
        screen.getByRole('button', { name: 'Back' }).focus()
        rerender(layer({ open: false }))
        expect(opener).toHaveFocus()
      } finally {
        opener.remove()
      }
    })

    it('leaves focus alone when the player has already moved it elsewhere', () => {
      const opener = document.createElement('button')
      const elsewhere = document.createElement('button')
      document.body.append(opener, elsewhere)
      try {
        opener.focus()
        const { rerender } = render(layer())
        elsewhere.focus()
        rerender(layer({ open: false }))
        expect(elsewhere).toHaveFocus()
      } finally {
        opener.remove()
        elsewhere.remove()
      }
    })

    it('does not reach for an opener that has left the page', () => {
      const opener = document.createElement('button')
      document.body.append(opener)
      opener.focus()
      const { rerender } = render(layer())
      opener.remove()
      expect(() => rerender(layer({ open: false }))).not.toThrow()
    })
  })

  it('is built from tokens and, with reduced motion, only fades, over the same duration', () => {
    for (const name of ['in-right', 'out-right', 'in-top', 'out-top', 'in-bottom', 'out-bottom']) {
      expect(css).toMatch(
        new RegExp(
          `@utility page-${name} \\{\\s*animation: page-${name} var\\(--duration-page\\) var\\(--ease-glide\\) both;`,
        ),
      )
    }
    const enter = /@keyframes page-in-right \{\s*from \{([^}]*)\}/.exec(css)![1]
    expect(enter).toContain('blur(var(--effect-reveal-blur))')
    expect(css).toMatch(
      /\.page-in-right,\s*\.page-in-top,\s*\.page-in-bottom \{\s*animation: card-fade-in var\(--duration-page\) ease-in-out both;/,
    )
    expect(css).toMatch(
      /\.page-out-right,\s*\.page-out-top,\s*\.page-out-bottom \{\s*animation: card-fade-out var\(--duration-page\) ease-in-out both;/,
    )
  })
})
