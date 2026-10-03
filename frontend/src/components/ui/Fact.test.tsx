import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { Fact } from './Fact'
import { PLACEHOLDER } from '../../lib/manifest'

describe('Fact', () => {
  afterEach(cleanup)

  it('sets a value in the data face', () => {
    render(<Fact label="Minecraft" value="1.21.1" />)
    expect(screen.getByText('1.21.1')).toHaveClass('font-mono')
  })

  it('sets a word in the UI face when it is plain', () => {
    render(<Fact label="Loader" value="Client visuals" plain />)
    expect(screen.getByText('Client visuals')).not.toHaveClass('font-mono')
  })

  it('says Unknown, faint, in place of the marker', () => {
    render(<Fact label="Mods" value={PLACEHOLDER} />)
    const value = screen.getByText('Unknown')
    expect(value).toHaveClass('text-fg-faint')
    expect(value).not.toHaveClass('font-mono')
    expect(screen.queryByText(PLACEHOLDER)).toBeNull()
  })

  it('says what it is told in place of the marker', () => {
    render(<Fact label="Size" value={PLACEHOLDER} unset="Not installed" />)
    expect(screen.getByText('Not installed')).toHaveClass('text-fg-faint')
  })
})
