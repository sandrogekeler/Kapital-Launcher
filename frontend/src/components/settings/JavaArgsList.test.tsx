import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { JavaArgsList } from './JavaArgsList'

/** The list over a state of its own, as the Game card's draft holds it. */
function Harness({ initial = [] as string[], disabled = false, onEscape = () => {} }) {
  const [args, setArgs] = useState(initial)
  return (
    // The page closes on an Escape that reaches the window; this stands in for it.
    <div onKeyDown={(e) => e.key === 'Escape' && onEscape()}>
      <JavaArgsList args={args} onChange={setArgs} disabled={disabled} />
      <output data-testid="args">{args.join('|')}</output>
    </div>
  )
}

const args = () => screen.getByTestId('args').textContent

describe('JavaArgsList', () => {
  afterEach(cleanup)

  it('adds an argument with + and Enter, and puts the focus back on +', () => {
    render(<Harness />)
    fireEvent.click(screen.getByRole('button', { name: 'Add argument' }))
    const field = screen.getByRole('textbox', { name: 'Java argument' })
    expect(field).toHaveFocus()
    fireEvent.change(field, { target: { value: ' -Xss4m ' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    expect(args()).toBe('-Xss4m')
    expect(screen.getByText('-Xss4m', { selector: 'code' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add argument' })).toHaveFocus()
  })

  it('says why an argument cannot be added and keeps the field', () => {
    render(<Harness initial={['-Xss4m']} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add argument' }))
    const field = screen.getByRole('textbox', { name: 'Java argument' })
    fireEvent.change(field, { target: { value: '-Xmx16G' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(screen.getByRole('alert')).toHaveTextContent(/slider/)
    expect(field).toHaveAttribute('aria-invalid', 'true')
    fireEvent.change(field, { target: { value: '-Xss4m' } })
    expect(screen.queryByRole('alert')).toBeNull()
    fireEvent.keyDown(field, { key: 'Enter' })
    expect(screen.getByRole('alert')).toHaveTextContent(/already in the list/)
    expect(args()).toBe('-Xss4m')
  })

  it('cancels on Escape without closing the page', () => {
    const onEscape = vi.fn()
    render(<Harness onEscape={onEscape} />)
    fireEvent.click(screen.getByRole('button', { name: 'Add argument' }))
    fireEvent.keyDown(screen.getByRole('textbox', { name: 'Java argument' }), { key: 'Escape' })
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(onEscape).not.toHaveBeenCalled()
  })

  it('edits and removes an argument from its menu', () => {
    render(<Harness initial={['-Xss4m', '-Dfoo=bar']} />)
    fireEvent.click(screen.getByRole('button', { name: 'Options for -Xss4m' }))
    const edit = screen.getByRole('menuitem', { name: 'Edit' })
    expect(edit).toHaveFocus()
    fireEvent.keyDown(edit, { key: 'ArrowDown' })
    expect(screen.getByRole('menuitem', { name: 'Remove' })).toHaveFocus()
    fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Remove' }), { key: 'ArrowUp' })
    fireEvent.click(screen.getByRole('menuitem', { name: 'Edit' }))

    const field = screen.getByRole('textbox', { name: 'Java argument' })
    expect(field).toHaveValue('-Xss4m')
    fireEvent.change(field, { target: { value: '-Xss8m' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(args()).toBe('-Xss8m|-Dfoo=bar')

    fireEvent.click(screen.getByRole('button', { name: 'Options for -Dfoo=bar' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove' }))
    expect(args()).toBe('-Xss8m')
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('closes the menu on Escape, back on its button, and on a press outside', () => {
    const onEscape = vi.fn()
    render(<Harness initial={['-Xss4m']} onEscape={onEscape} />)
    const trigger = screen.getByRole('button', { name: 'Options for -Xss4m' })
    fireEvent.click(trigger)
    fireEvent.keyDown(screen.getByRole('menu'), { key: 'Escape' })
    expect(screen.queryByRole('menu')).toBeNull()
    expect(trigger).toHaveFocus()
    expect(onEscape).not.toHaveBeenCalled()

    fireEvent.click(trigger)
    fireEvent.pointerDown(document.body)
    expect(screen.queryByRole('menu')).toBeNull()
  })

  it('changes nothing while disabled', () => {
    render(<Harness initial={['-Xss4m']} disabled />)
    expect(screen.getByRole('button', { name: 'Add argument' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Options for -Xss4m' })).toBeDisabled()
  })
})
