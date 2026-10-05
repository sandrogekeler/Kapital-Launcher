import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import { PanoramaCube } from './PanoramaCube'
import { STYLE_CSS as css } from '../../test-css'

const faces = Array.from({ length: 6 }, (_, n) => `/panorama/frangfurd/panorama_${n}.png?v=k`)

/** Makes `prefers-reduced-motion` answer, as a system setting does. */
function reducedMotion(matches: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation(() => ({
      matches,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  )
}

describe('PanoramaCube', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('draws the six faces, each in its own place of the game order, as one turning cube', () => {
    const { container } = render(<PanoramaCube faces={faces} />)
    const imgs = Array.from(container.querySelectorAll<HTMLImageElement>('.panorama-cube img'))
    expect(imgs.map((i) => i.dataset.face)).toEqual(['0', '1', '2', '3', '4', '5'])
    expect(imgs.map((i) => i.getAttribute('src'))).toEqual(faces)
    for (const img of imgs) {
      expect(img).toHaveClass('panorama-face')
      expect(img).toHaveAttribute('alt', '')
    }
    expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('stays transparent until every face has loaded, then fades in', () => {
    const { container } = render(<PanoramaCube faces={faces} />)
    const root = container.firstElementChild!
    const imgs = Array.from(container.querySelectorAll('img'))
    expect(root).toHaveClass('opacity-0')
    for (const img of imgs.slice(0, 5)) fireEvent.load(img)
    expect(root).toHaveClass('opacity-0')
    fireEvent.load(imgs[5]!)
    expect(root).toHaveClass('opacity-100')
    expect(root).not.toHaveClass('opacity-0')
  })

  it('is one face, still, with reduced motion', () => {
    reducedMotion(true)
    const { container } = render(<PanoramaCube faces={faces} />)
    expect(container.querySelector('.panorama-cube')).toBeNull()
    const imgs = container.querySelectorAll('img')
    expect(imgs).toHaveLength(1)
    expect(imgs[0]).toHaveAttribute('src', faces[0])
    expect(imgs[0]).toHaveClass('opacity-0')
    fireEvent.load(imgs[0]!)
    expect(imgs[0]).toHaveClass('opacity-100')
  })

  // The geometry, which jsdom cannot draw: each face stands where the game's order
  // puts it, seen from inside (checked against the author's real faces, whose
  // neighbouring edges meet in exactly this ring), and the cube turns right.
  it('places the faces front, right, back, left, top, bottom and turns about the vertical axis', () => {
    const face = (n: number) =>
      new RegExp(String.raw`\.panorama-face\[data-face='${n}'\] \{\s*transform: ([^;]*);`).exec(
        css,
      )?.[1]
    expect([0, 1, 2, 3, 4, 5].map(face)).toEqual([
      'translateZ(var(--panorama-back))',
      'rotateY(-90deg) translateZ(var(--panorama-back))',
      'rotateY(180deg) translateZ(var(--panorama-back))',
      'rotateY(90deg) translateZ(var(--panorama-back))',
      'rotateX(-90deg) translateZ(var(--panorama-back))',
      'rotateX(90deg) translateZ(var(--panorama-back))',
    ])
    const turn = /@keyframes panorama-turn \{\s*from \{([^}]*)\}\s*to \{([^}]*)\}/.exec(css)!
    // One whole turn from the chapter's start angle (lib/panorama).
    expect(turn[1]).toContain('rotateY(var(--panorama-start, 0deg))')
    expect(turn[2]).toContain('rotateY(calc(var(--panorama-start, 0deg) + 360deg))')
    // The tilt is outside the turn, down, so the horizon stays level.
    expect(turn[1]).toMatch(/rotateX\(calc\(-1 \* var\(--panorama-tilt\)\)\)\s*rotateY/)
  })

  it('is built from tokens: the turn takes the panorama duration, the view its field of view', () => {
    expect(css).toMatch(/animation: panorama-turn var\(--duration-panorama\) linear infinite/)
    expect(css).toContain('--panorama-reach: calc(50cqw / tan(var(--panorama-fov) / 2))')
    expect(css).toContain('(1 + var(--panorama-bleed))')
  })

  it('goes on from where its turn was when its chapter comes back, instead of starting over', () => {
    const turn = { currentTime: 0 as number | null }
    const real = HTMLElement.prototype.getAnimations
    HTMLElement.prototype.getAnimations = function () {
      return this.classList.contains('panorama-cube') ? [turn as unknown as Animation] : []
    }
    try {
      const faces = Array.from({ length: 6 }, (_, n) => `/panorama/frangfurd/panorama_${n}.png?v=t`)
      const first = render(<PanoramaCube faces={faces} />)
      turn.currentTime = 42_000
      first.unmount()
      turn.currentTime = 0
      render(<PanoramaCube faces={faces} />)
      expect(turn.currentTime).toBe(42_000)
    } finally {
      HTMLElement.prototype.getAnimations = real
    }
  })
})
