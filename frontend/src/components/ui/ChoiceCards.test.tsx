import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ChoiceCards } from './ChoiceCards'
import type { Choice } from './ChoiceCards'

const choices: Choice<string>[] = [
  { value: 'a', title: 'Alpha', detail: 'alpha.example', note: '', slot: 'Current' },
  { value: 'b', title: 'Beta', detail: '', note: 'Not yet.', slot: undefined, disabled: true },
]
const noop = () => undefined

describe('ChoiceCards', () => {
  afterEach(cleanup)

  it('is a radio group of cards, the chosen one checked and named by its title', () => {
    render(<ChoiceCards label="Pack" value="a" choices={choices} onChange={noop} />)
    expect(screen.getByRole('radiogroup', { name: 'Pack' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Alpha' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'Alpha' })).toHaveAccessibleDescription(
      'alpha.example',
    )
    expect(screen.getByRole('radio', { name: 'Beta' })).not.toBeChecked()
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveAccessibleDescription('Not yet.')
  })

  it('gives the chosen card the nav highlight look, and every card the same lines', () => {
    const open = choices.map((c) => ({ ...c, disabled: false }))
    const { rerender } = render(
      <ChoiceCards label="Pack" value="a" choices={open} onChange={noop} />,
    )
    const lines = (name: string) =>
      screen.getByRole('radio', { name }).querySelectorAll('span > span').length
    expect(screen.getByRole('radio', { name: 'Alpha' })).toHaveClass(
      'bg-raised',
      'border-accent-edge',
    )
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveClass('border-line')
    expect(lines('Alpha')).toBe(lines('Beta'))

    rerender(<ChoiceCards label="Pack" value="b" choices={open} onChange={noop} />)
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveClass(
      'bg-raised',
      'border-accent-edge',
    )
    expect(screen.getByRole('radio', { name: 'Alpha' })).toHaveClass('border-line')
    expect(screen.getByRole('radio', { name: 'Alpha' }).className).not.toContain(
      'border-accent-edge',
    )
    expect(lines('Alpha')).toBe(lines('Beta'))
  })

  it('keeps an empty line height with a non-breaking space', () => {
    render(<ChoiceCards label="Pack" value="a" choices={choices} onChange={noop} />)
    expect(screen.getByRole('radio', { name: 'Alpha' }).textContent).toContain(' ')
  })

  it('does not pick a disabled card', () => {
    const onChange = vi.fn()
    render(<ChoiceCards label="Pack" value="a" choices={choices} onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Beta' }))
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByRole('radio', { name: 'Beta' })).toBeDisabled()
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveClass(
      'disabled:opacity-50',
      'disabled:cursor-default',
    )
  })

  it('picks an enabled card on a click', () => {
    const onChange = vi.fn()
    const open = choices.map((c) => ({ ...c, disabled: false }))
    render(<ChoiceCards label="Pack" value="a" choices={open} onChange={onChange} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Beta' }))
    expect(onChange).toHaveBeenCalledExactlyOnceWith('b')
  })

  it('moves the focus with the arrow keys, wrapping, without picking', () => {
    const onChange = vi.fn()
    const open = choices.map((c) => ({ ...c, disabled: false }))
    const three: Choice<string>[] = [...open, { value: 'c', title: 'Gamma', note: '' }]
    render(<ChoiceCards label="Pack" value="a" choices={three} onChange={onChange} />)
    screen.getByRole('radio', { name: 'Alpha' }).focus()
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Alpha' }), { key: 'ArrowDown' })
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveFocus()
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Beta' }), { key: 'ArrowUp' })
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Alpha' }), { key: 'ArrowUp' })
    expect(screen.getByRole('radio', { name: 'Gamma' })).toHaveFocus()
    expect(onChange).not.toHaveBeenCalled()
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Gamma' }), { key: 'x' })
    expect(screen.getByRole('radio', { name: 'Gamma' })).toHaveFocus()
  })

  it('skips a disabled card on the way', () => {
    const three: Choice<string>[] = [
      { value: 'a', title: 'Alpha' },
      { value: 'b', title: 'Beta', disabled: true },
      { value: 'c', title: 'Gamma' },
    ]
    render(<ChoiceCards label="Pack" value="a" choices={three} onChange={noop} />)
    screen.getByRole('radio', { name: 'Alpha' }).focus()
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Alpha' }), { key: 'ArrowRight' })
    expect(screen.getByRole('radio', { name: 'Gamma' })).toHaveFocus()
  })

  it('has one tab stop: the chosen card, or the first that can be picked when none is chosen', () => {
    const { rerender } = render(
      <ChoiceCards label="Pack" value="a" choices={choices} onChange={noop} />,
    )
    expect(screen.getByRole('radio', { name: 'Alpha' })).toHaveAttribute('tabindex', '0')
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveAttribute('tabindex', '-1')
    rerender(
      <ChoiceCards
        label="Pack"
        value={null}
        choices={choices.map((c) => ({ ...c, disabled: c.value === 'a' }))}
        onChange={noop}
      />,
    )
    expect(screen.getByRole('radio', { name: 'Beta' })).toHaveAttribute('tabindex', '0')
  })

  it('draws no slot when no card has one', () => {
    render(
      <ChoiceCards
        label="Java"
        value="x"
        choices={[{ value: 'x', title: 'Plain' }]}
        onChange={noop}
      />,
    )
    expect(screen.getByRole('radio').querySelectorAll('.w-24')).toHaveLength(0)
  })

  it('keeps no empty note line when no card has a note to show', () => {
    render(
      <ChoiceCards
        label="Pack source"
        value="a"
        choices={[
          { value: 'a', title: 'Published pack', detail: 'example.org', note: '' },
          { value: 'b', title: 'Dev pack', detail: 'localhost:8080', note: '' },
        ]}
        onChange={() => undefined}
      />,
    )
    for (const radio of screen.getAllByRole('radio')) {
      expect(radio.querySelector('[id$="-note"]')).toBeNull()
    }
  })
})
