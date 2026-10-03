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
    render(<ChapterStage chapter={luxemburg} chapters={chapters} onOpenSettings={noop} />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Luxemburg')
    expect(leaving()).toBeNull()
    expect(document.querySelector('.card-in-down, .card-in-up')).toBeNull()
  })

  it('slides up to a later chapter and down to an earlier one, keeping the old card until its animation ends', () => {
    const { rerender } = render(
      <ChapterStage chapter={luxemburg} chapters={chapters} onOpenSettings={noop} />,
    )
    rerender(<ChapterStage chapter={frangfurd} chapters={chapters} onOpenSettings={noop} />)
    // A chapter further down: the content moves up, so the new card comes
    // in from below and the old one leaves upward, inert.
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd')
    const out = leaving()!
    expect(out).toHaveClass('card-out-up')
    expect(out).toHaveTextContent('Play Luxemburg')
    expect(document.querySelector('.card-in-up')).toHaveTextContent('Play Frangfurd')
    endAnimation(out)
    expect(leaving()).toBeNull()

    rerender(<ChapterStage chapter={lichdenstein} chapters={chapters} onOpenSettings={noop} />)
    expect(leaving()).toHaveClass('card-out-down')
    expect(document.querySelector('.card-in-down')).toHaveTextContent('Join Lichdenstein')
  })

  it('replaces a card still leaving when the selection moves again', () => {
    const { rerender } = render(
      <ChapterStage chapter={luxemburg} chapters={chapters} onOpenSettings={noop} />,
    )
    rerender(<ChapterStage chapter={lichdenstein} chapters={chapters} onOpenSettings={noop} />)
    rerender(<ChapterStage chapter={frangfurd} chapters={chapters} onOpenSettings={noop} />)
    expect(document.querySelectorAll('[inert]').length).toBe(1)
    expect(leaving()).toHaveTextContent('Join Lichdenstein')
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd')
  })
})
