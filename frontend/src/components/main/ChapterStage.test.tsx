import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ChapterStage } from './ChapterStage'
import { BUNDLED_MANIFEST } from '../../lib/manifest'

vi.mock('../../../wailsjs/go/main/App')

const chapters = BUNDLED_MANIFEST.chapters
const noop = () => undefined
const [luxemburg, lichdenstein, frangfurd] = [chapters[0]!, chapters[1]!, chapters[2]!]

/** The card that is leaving, or null once its animation has ended. */
const leaving = () => document.querySelector<HTMLElement>('[inert]')
/** jsdom has no AnimationEvent, so React listens for the prefixed name there. */
const endAnimation = (el: HTMLElement) =>
  fireEvent(el, new Event('webkitAnimationEnd', { bubbles: true }))

describe('ChapterStage', () => {
  afterEach(cleanup)

  it('shows the first chapter still, with no card leaving', () => {
    render(
      <ChapterStage
        chapter={luxemburg}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Luxemburg')
    expect(leaving()).toBeNull()
    expect(document.querySelector('.card-in-down, .card-in-up')).toBeNull()
  })

  it('slides up to a later chapter and down to an earlier one, keeping the old card until its animation ends', () => {
    const { rerender } = render(
      <ChapterStage
        chapter={luxemburg}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    rerender(
      <ChapterStage
        chapter={frangfurd}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    // A chapter further down: the content moves up, so the new card comes
    // in from below and the old one leaves upward, inert.
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd')
    const out = leaving()!
    expect(out).toHaveClass('card-out-up')
    expect(out).toHaveTextContent('Play Luxemburg')
    expect(document.querySelector('.card-in-up')).toHaveTextContent('Play Frangfurd')
    endAnimation(out)
    expect(leaving()).toBeNull()

    rerender(
      <ChapterStage
        chapter={lichdenstein}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    expect(leaving()).toHaveClass('card-out-down')
    expect(document.querySelector('.card-in-down')).toHaveTextContent('Join Lichdenstein')
  })

  it('replaces the card without a slide when the window is hidden', () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    try {
      const { rerender } = render(
        <ChapterStage
          chapter={luxemburg}
          chapters={chapters}
          onOpenSettings={noop}
          onOpenLogs={noop}
          onOpenMap={noop}
        />,
      )
      rerender(
        <ChapterStage
          chapter={frangfurd}
          chapters={chapters}
          onOpenSettings={noop}
          onOpenLogs={noop}
          onOpenMap={noop}
        />,
      )
      expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd')
      expect(leaving()).toBeNull()
      expect(document.querySelector('.card-in-down, .card-in-up')).toBeNull()
    } finally {
      visibility.mockRestore()
    }
  })

  it('leaves the card inert and hidden while a page covers it, and the page in the same stage', () => {
    const { rerender, container } = render(
      <ChapterStage
        chapter={luxemburg}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      >
        <section aria-label="A page">Page</section>
      </ChapterStage>,
    )
    const card = () => container.querySelector<HTMLElement>('.card-stage > div')!
    expect(card()).not.toHaveAttribute('inert')
    expect(card()).not.toHaveAttribute('aria-hidden')
    rerender(
      <ChapterStage
        chapter={luxemburg}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
        covered
      >
        <section aria-label="A page">Page</section>
      </ChapterStage>,
    )
    expect(card()).toHaveAttribute('inert')
    expect(card()).toHaveAttribute('aria-hidden', 'true')
    // The card is what is covered, never moved, and the page is a sibling in the stage.
    expect(card()).toHaveTextContent('Play Luxemburg')
    expect(card().className).not.toMatch(/card-(in|out)/)
    expect(screen.getByRole('region', { name: 'A page' }).parentElement).toBe(card().parentElement)
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull()
    rerender(
      <ChapterStage
        chapter={luxemburg}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    expect(card()).not.toHaveAttribute('inert')
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Luxemburg')
  })

  it('replaces a card still leaving when the selection moves again', () => {
    const { rerender } = render(
      <ChapterStage
        chapter={luxemburg}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    rerender(
      <ChapterStage
        chapter={lichdenstein}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    rerender(
      <ChapterStage
        chapter={frangfurd}
        chapters={chapters}
        onOpenSettings={noop}
        onOpenLogs={noop}
        onOpenMap={noop}
      />,
    )
    expect(document.querySelectorAll('[inert]').length).toBe(1)
    expect(leaving()).toHaveTextContent('Join Lichdenstein')
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd')
  })
})
