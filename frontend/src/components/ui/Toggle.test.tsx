import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { Toggle } from './Toggle'

describe('Toggle', () => {
  afterEach(cleanup)

  it('is a switch named by its label, in the state it is given', () => {
    render(<Toggle label="Loading splash" checked onChange={() => undefined} />)
    expect(screen.getByRole('switch', { name: 'Loading splash' })).toBeChecked()
    cleanup()
    render(<Toggle label="Loading splash" checked={false} onChange={() => undefined} />)
    expect(screen.getByRole('switch', { name: 'Loading splash' })).not.toBeChecked()
  })

  it('reports the opposite state on a click of the switch or of its label', () => {
    const onChange = vi.fn()
    render(<Toggle label="Loading splash" checked={false} onChange={onChange} />)
    fireEvent.click(screen.getByRole('switch'))
    expect(onChange).toHaveBeenLastCalledWith(true)
    fireEvent.click(screen.getByText('Loading splash'))
    expect(onChange).toHaveBeenCalledTimes(2)
  })

  it('shows the state the caller holds, not its own: a refused save leaves it as it was', () => {
    const { rerender } = render(<Toggle label="Splash" checked onChange={() => undefined} />)
    fireEvent.click(screen.getByRole('switch'))
    expect(screen.getByRole('switch')).toBeChecked()
    rerender(<Toggle label="Splash" checked={false} onChange={() => undefined} />)
    expect(screen.getByRole('switch')).not.toBeChecked()
  })

  it('cannot be used while disabled, and its label no longer toggles it', () => {
    const onChange = vi.fn()
    render(<Toggle label="Mod" checked disabled onChange={onChange} />)
    const sw = screen.getByRole('switch', { name: 'Mod' })
    expect(sw).toBeDisabled()
    expect(sw).toBeChecked()
    fireEvent.click(sw)
    fireEvent.click(screen.getByText('Mod'))
    expect(onChange).not.toHaveBeenCalled()
  })

  it('describes the switch with its hint, and with the error in the hint place', () => {
    const { rerender } = render(
      <Toggle label="Splash" checked hint="What it does." onChange={() => undefined} />,
    )
    expect(screen.getByRole('switch')).toHaveAccessibleDescription('What it does.')
    rerender(
      <Toggle
        label="Splash"
        checked
        hint="What it does."
        error="Refused."
        onChange={() => undefined}
      />,
    )
    expect(screen.getByRole('switch')).toHaveAccessibleDescription('Refused.')
    expect(screen.getByRole('alert')).toHaveTextContent('Refused.')
    expect(screen.queryByText('What it does.')).toBeNull()
  })
})
