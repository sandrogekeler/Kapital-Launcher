import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { Reveal } from './Reveal'
import { STYLE_CSS as css } from '../../test-css'

describe('Reveal', () => {
  afterEach(cleanup)

  it('shows nothing of its content until it is ready, and reserves no room for it', () => {
    const { container } = render(
      <Reveal ready={false}>
        <p>The form</p>
      </Reveal>,
    )
    expect(screen.queryByText('The form')).toBeNull()
    expect(container.querySelector('.reveal')).toBeNull()
    // The only thing there is the faint line, one element.
    expect(container.children).toHaveLength(1)
  })

  it('says the read is on its way only after the grace, which the line waits out itself', () => {
    render(
      <Reveal ready={false}>
        <p>The form</p>
      </Reveal>,
    )
    const line = screen.getByText('Reading.')
    expect(line).toHaveClass('reveal-wait', 'text-fg-faint', 'text-sm')
    // The wait is the line's own animation delay, the grace token, with the
    // line held clear until then.
    expect(css).toMatch(
      /@utility reveal-wait \{\s*animation: card-fade-in var\(--duration-fast\) var\(--ease-standard\) var\(--duration-grace\) both;/,
    )
  })

  it('mounts the content in the reveal when it becomes ready, and drops the line', () => {
    const { rerender, container } = render(
      <Reveal ready={false}>
        <p>The form</p>
      </Reveal>,
    )
    rerender(
      <Reveal ready>
        <p>The form</p>
      </Reveal>,
    )
    expect(screen.queryByText('Reading.')).toBeNull()
    const content = screen.getByText('The form')
    expect(content.parentElement).toHaveClass('reveal')
    expect(container.querySelectorAll('.reveal')).toHaveLength(1)
  })

  it('plays at once when it is ready on the first render, and without being asked', () => {
    const { container } = render(
      <Reveal>
        <p>The form</p>
      </Reveal>,
    )
    expect(screen.queryByText('Reading.')).toBeNull()
    expect(container.querySelector('.reveal')).toHaveTextContent('The form')
  })

  it('plays once: a later wait leaves the revealed content, and the same element, alone', () => {
    const { rerender, container } = render(
      <Reveal ready={false}>
        <p>The form</p>
      </Reveal>,
    )
    rerender(
      <Reveal ready>
        <p>The form</p>
      </Reveal>,
    )
    const first = container.querySelector('.reveal')
    rerender(
      <Reveal ready={false}>
        <p>The form again</p>
      </Reveal>,
    )
    expect(container.querySelector('.reveal')).toBe(first)
    expect(screen.getByText('The form again')).toBeInTheDocument()
    expect(screen.queryByText('Reading.')).toBeNull()
  })

  it('takes its layout classes for the content it wraps', () => {
    const { container } = render(
      <Reveal className="flex flex-col gap-10">
        <p>The form</p>
      </Reveal>,
    )
    expect(container.querySelector('.reveal')).toHaveClass('flex', 'flex-col', 'gap-10')
  })

  it('is built from tokens and, with reduced motion, is a plain quick fade', () => {
    expect(css).toMatch(
      /@utility reveal \{\s*animation: reveal-in var\(--duration-reveal\) var\(--ease-standard\) both;/,
    )
    const rise = /@keyframes reveal-in \{\s*from \{([^}]*)\}/.exec(css)![1]
    expect(rise).toContain('translateY(var(--layout-reveal-rise))')
    expect(rise).toContain('blur(var(--effect-reveal-blur))')
    expect(css).toMatch(
      /@media \(prefers-reduced-motion: reduce\) \{\s*\.reveal \{\s*animation: card-fade-in var\(--duration-fast\) ease-in-out both;\s*\}\s*\}/,
    )
  })
})
