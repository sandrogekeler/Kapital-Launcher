import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Button } from './Button'
import { STYLE_CSS as css } from '../../test-css'

describe('Button', () => {
  afterEach(cleanup)

  const both = () =>
    render(
      <>
        <Button variant="play" onClick={() => undefined}>
          Play
        </Button>
        <Button variant="stop" onClick={() => undefined}>
          Stop
        </Button>
      </>,
    )

  it('fills Play with the accent and Stop with the danger colour, at the same size', () => {
    both()
    const play = screen.getByRole('button', { name: 'Play' })
    const stop = screen.getByRole('button', { name: 'Stop' })
    expect(play).toHaveClass('bg-accent', 'text-canvas', 'h-13', 'min-w-(--layout-play-min)')
    expect(stop).toHaveClass('bg-danger', 'text-canvas', 'h-13', 'min-w-(--layout-play-min)')
    expect(play).not.toHaveClass('bg-danger')
    expect(stop).not.toHaveClass('bg-accent')
  })

  it('lifts a filled button on hover: brighter, with a glow in its own colour', () => {
    both()
    const play = screen.getByRole('button', { name: 'Play' })
    const stop = screen.getByRole('button', { name: 'Stop' })
    expect(play).toHaveClass('glow-accent', 'enabled:hover:brightness-(--effect-hover-brightness)')
    expect(stop).toHaveClass('glow-danger', 'enabled:hover:brightness-(--effect-hover-brightness)')
    expect(play).not.toHaveClass('glow-danger')
    // The glow is one box-shadow, so the button's own transition carries it,
    // and reduced motion removes the transition and leaves the glow.
    // It eases over the reveal's duration, longer than a plain hover's, in and out.
    expect(play).toHaveClass('duration-reveal', 'transition-[background-color,filter,box-shadow]')
    expect(play).not.toHaveClass('duration-fast')
    expect(play).toHaveClass('motion-reduce:transition-none')
  })

  it("builds the glow from the tokens, in the button's own colour, and never on a disabled button", () => {
    for (const [name, colour] of [
      ['accent', '--accent'],
      ['danger', '--danger'],
    ] as const) {
      const flat = css.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\s+/g, ' ')
      // At rest the same shadow, transparent: only its colour moves, so it
      // fades out as smoothly as it fades in, without shrinking.
      expect(flat).toContain(
        `@utility glow-${name} { box-shadow: 0 0 var(--effect-glow-blur) transparent; &:hover:enabled { box-shadow: 0 0 var(--effect-glow-blur) color-mix(in srgb, var(${colour}) var(--effect-glow-strength), transparent); } }`,
      )
    }
  })

  it('gives the ghost variant no glow', () => {
    render(<Button onClick={() => undefined}>Open folder</Button>)
    const button = screen.getByRole('button', { name: 'Open folder' })
    expect(button.className).not.toMatch(/glow-/)
    expect(button).toHaveClass('hover:bg-hover')
  })

  it('does not click while disabled', () => {
    const onClick = vi.fn()
    render(
      <Button variant="stop" onClick={onClick} disabled>
        Stop
      </Button>,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Stop' }))
    expect(onClick).not.toHaveBeenCalled()
    expect(screen.getByRole('button')).toBeDisabled()
  })
})
