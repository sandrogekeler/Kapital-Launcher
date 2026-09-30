import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ChapterStage } from './ChapterStage'
import { BUNDLED_MANIFEST } from '../../lib/manifest'

vi.mock('../../../wailsjs/go/main/App')

const chapters = BUNDLED_MANIFEST.chapters
const [luxemburg, lichdenstein, frangfurd] = [chapters[0]!, chapters[1]!, chapters[2]!]

/** The card that is leaving, or null once its animation has ended. */
const leaving = () => document.querySelector<HTMLElement>('[inert]')
/** jsdom has no AnimationEvent, so React listens for the prefixed name there. */
const endAnimation = (el: HTMLElement) =>
  fireEvent(el, new Event('webkitAnimationEnd', { bubbles: true }))

describe('ChapterStage', () => {
  afterEach(cleanup)

  it('shows the first chapter still, with no card leaving', () => {
    render(<ChapterStage chapter={luxemburg} chapters={chapters} />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Luxemburg')
    expect(leaving()).toBeNull()
    expect(document.querySelector('.card-in-down, .card-in-up')).toBeNull()
  })

  it('slides down to a later chapter and up to an earlier one, keeping the old card until its animation ends', () => {
    const { rerender } = render(<ChapterStage chapter={luxemburg} chapters={chapters} />)
    rerender(<ChapterStage chapter={frangfurd} chapters={chapters} />)
    // The new card comes in from above, the old one goes down and out, inert.
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd')
    const out = leaving()!
    expect(out).toHaveClass('card-out-down')
    expect(out).toHaveTextContent('Luxemburg')
    expect(document.querySelector('.card-in-down')).toHaveTextContent('Frangfurd')
    endAnimation(out)
    expect(leaving()).toBeNull()

    rerender(<ChapterStage chapter={lichdenstein} chapters={chapters} />)
    expect(leaving()).toHaveClass('card-out-up')
    expect(document.querySelector('.card-in-up')).toHaveTextContent('Lichdenstein')
  })

  it('replaces a card still leaving when the selection moves again', () => {
    const { rerender } = render(<ChapterStage chapter={luxemburg} chapters={chapters} />)
    rerender(<ChapterStage chapter={lichdenstein} chapters={chapters} />)
    rerender(<ChapterStage chapter={frangfurd} chapters={chapters} />)
    expect(document.querySelectorAll('[inert]').length).toBe(1)
    expect(leaving()).toHaveTextContent('Lichdenstein')
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd')
  })
})
