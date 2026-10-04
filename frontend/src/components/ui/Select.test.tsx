import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { Select } from './Select'
import type { SelectOption } from './Select'

const options: SelectOption[] = [
  { value: 'a', label: 'Live log' },
  { value: 'b', label: 'Most recent', note: '3 Oct 2026, 15:00' },
  { value: 'c', label: '2 Oct 2026, 20:30', mark: 'Crashed' },
  { value: 'd', label: 'Crash report', note: '3 Oct 2026, 14:59' },
]

const button = () => screen.getByRole('button', { name: /Choose a log/ })
const listbox = () => screen.getByRole('listbox')
const optionNamed = (name: RegExp | string) => screen.getByRole('option', { name })

/** A Select that keeps its own choice, as a page does, with a spy on what it reports. */
function Harness({
  onChange = vi.fn(),
  start = 'b',
}: {
  onChange?: (v: string) => void
  start?: string
}) {
  const [value, setValue] = useState<string | null>(start)
  return (
    <>
      <Select
        label="Choose a log"
        value={value}
        options={options}
        onChange={(v) => {
          setValue(v)
          onChange(v)
        }}
      />
      <button type="button">After</button>
    </>
  )
}

describe('Select', () => {
  afterEach(cleanup)

  it('is a closed button naming the chosen entry, with a listbox popup', () => {
    render(<Harness />)
    expect(button()).toHaveAttribute('aria-haspopup', 'listbox')
    expect(button()).toHaveAttribute('aria-expanded', 'false')
    // The label and the chosen entry are the name; the other entries only give it width.
    expect(button()).toHaveAccessibleName('Choose a log Most recent 3 Oct 2026, 15:00')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(button()).toHaveClass('h-11', 'border-line-strong', 'rounded-md', 'bg-sunken')
  })

  it('is as wide as its widest entry: every entry is in the button, and only the chosen one is seen', () => {
    render(<Harness />)
    const cell = button().querySelector('.grid')!
    expect(cell.children).toHaveLength(options.length)
    const hidden = [...cell.children].filter((c) => c.classList.contains('invisible'))
    expect(hidden).toHaveLength(options.length - 1)
    for (const c of hidden) expect(c).toHaveAttribute('aria-hidden', 'true')
    expect(cell).toHaveTextContent('Crash report')
  })

  it('shows the placeholder while nothing is chosen', () => {
    render(
      <Select
        label="Choose a log"
        value={null}
        options={options}
        onChange={() => undefined}
        placeholder="Pick one"
      />,
    )
    expect(button()).toHaveTextContent('Pick one')
  })

  it('opens on a click into a listbox of options, the chosen one selected and marked', () => {
    render(<Harness />)
    fireEvent.click(button())
    expect(button()).toHaveAttribute('aria-expanded', 'true')
    expect(listbox()).toHaveAccessibleName('Choose a log')
    expect(within(listbox()).getAllByRole('option')).toHaveLength(4)
    expect(optionNamed(/Most recent/)).toHaveAttribute('aria-selected', 'true')
    expect(optionNamed(/Live log/)).toHaveAttribute('aria-selected', 'false')
    // The chosen entry carries a check; the others do not.
    expect(optionNamed(/Most recent/).querySelector('svg')).not.toBeNull()
    expect(optionNamed(/Live log/).querySelector('svg')).toBeNull()
    // The crash mark shows in the entry, in the danger colour.
    expect(within(optionNamed(/Crashed/)).getByText('Crashed')).toHaveClass('text-danger')
    // The focus is in the list, on the chosen entry.
    expect(listbox()).toHaveFocus()
    expect(listbox()).toHaveAttribute('aria-activedescendant', optionNamed(/Most recent/).id)
    expect(optionNamed(/Most recent/)).toHaveClass('bg-hover')
  })

  it('picks an entry on a click, closes, and gives the focus back to the button', () => {
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    fireEvent.click(button())
    fireEvent.click(optionNamed(/Crash report/))
    expect(onChange).toHaveBeenCalledExactlyOnceWith('d')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(button()).toHaveFocus()
    expect(button()).toHaveAccessibleName('Choose a log Crash report 3 Oct 2026, 14:59')
  })

  it('opens on the arrow keys from the button, and moves with them, Home and End', () => {
    render(<Harness />)
    button().focus()
    fireEvent.keyDown(button(), { key: 'ArrowDown' })
    const active = () => listbox().getAttribute('aria-activedescendant')
    expect(active()).toBe(optionNamed(/Most recent/).id)
    fireEvent.keyDown(listbox(), { key: 'ArrowDown' })
    expect(active()).toBe(optionNamed(/Crashed/).id)
    fireEvent.keyDown(listbox(), { key: 'End' })
    expect(active()).toBe(optionNamed(/Crash report/).id)
    // The ends hold: no wrapping past the last or the first.
    fireEvent.keyDown(listbox(), { key: 'ArrowDown' })
    expect(active()).toBe(optionNamed(/Crash report/).id)
    fireEvent.keyDown(listbox(), { key: 'Home' })
    expect(active()).toBe(optionNamed(/Live log/).id)
    fireEvent.keyDown(listbox(), { key: 'ArrowUp' })
    expect(active()).toBe(optionNamed(/Live log/).id)
    // Moving picks nothing.
    expect(button()).toHaveAttribute('aria-expanded', 'true')
  })

  it('opens upward keys too, from the button', () => {
    render(<Harness />)
    fireEvent.keyDown(button(), { key: 'ArrowUp' })
    expect(screen.getByRole('listbox')).toBeInTheDocument()
  })

  it('picks the active entry with Enter and with Space', () => {
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    fireEvent.click(button())
    fireEvent.keyDown(listbox(), { key: 'ArrowDown' })
    fireEvent.keyDown(listbox(), { key: 'Enter' })
    expect(onChange).toHaveBeenLastCalledWith('c')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(button()).toHaveFocus()

    fireEvent.click(button())
    fireEvent.keyDown(listbox(), { key: 'Home' })
    fireEvent.keyDown(listbox(), { key: ' ' })
    expect(onChange).toHaveBeenLastCalledWith('a')
    expect(button()).toHaveAccessibleName(/Live log/)
  })

  it('closes on Escape, returns the focus, and keeps the key from the page', () => {
    const onPage = vi.fn()
    window.addEventListener('keydown', onPage)
    try {
      const onChange = vi.fn()
      render(<Harness onChange={onChange} />)
      fireEvent.click(button())
      fireEvent.keyDown(listbox(), { key: 'Escape' })
      expect(screen.queryByRole('listbox')).toBeNull()
      expect(button()).toHaveFocus()
      expect(onChange).not.toHaveBeenCalled()
      expect(onPage).not.toHaveBeenCalled()
      // With the list closed, Escape is the page's again.
      fireEvent.keyDown(button(), { key: 'Escape' })
      expect(onPage).toHaveBeenCalledTimes(1)
    } finally {
      window.removeEventListener('keydown', onPage)
    }
  })

  it('closes on a press outside, and not on one inside', () => {
    render(<Harness />)
    fireEvent.click(button())
    fireEvent.mouseDown(listbox())
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    fireEvent.mouseDown(screen.getByRole('button', { name: 'After' }))
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(button()).toHaveAttribute('aria-expanded', 'false')
  })

  it('closes on a second click of the button and on Tab, without choosing', () => {
    const onChange = vi.fn()
    render(<Harness onChange={onChange} />)
    fireEvent.click(button())
    fireEvent.click(button())
    expect(screen.queryByRole('listbox')).toBeNull()
    fireEvent.click(button())
    fireEvent.keyDown(listbox(), { key: 'Tab' })
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('follows the pointer with its highlight', () => {
    render(<Harness />)
    fireEvent.click(button())
    fireEvent.mouseMove(optionNamed(/Live log/))
    expect(optionNamed(/Live log/)).toHaveClass('bg-hover')
    expect(optionNamed(/Most recent/)).not.toHaveClass('bg-hover')
  })

  it('scrolls inside a popover of a height limit when it is long', () => {
    render(<Harness />)
    fireEvent.click(button())
    const popover = listbox().closest('.max-h-72')
    expect(popover).not.toBeNull()
    expect(popover).toHaveClass(
      'bg-raised-2',
      'border-line-strong',
      'rounded-md',
      'overflow-hidden',
    )
  })

  it('does not open when it is disabled or has no entries', () => {
    const { rerender } = render(
      <Select
        label="Choose a log"
        value="a"
        options={options}
        onChange={() => undefined}
        disabled
      />,
    )
    fireEvent.click(button())
    expect(screen.queryByRole('listbox')).toBeNull()
    rerender(<Select label="Choose a log" value={null} options={[]} onChange={() => undefined} />)
    fireEvent.click(button())
    expect(screen.queryByRole('listbox')).toBeNull()
  })
})
