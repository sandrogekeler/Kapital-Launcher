import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { Code, Palette, Triangle } from '../../lib/icons'
import { SettingsCard, SettingsLayout, SettingsSection } from './SettingsLayout'

function Sections({ withAppearance = true }: { withAppearance?: boolean }) {
  return (
    <SettingsLayout>
      <SettingsSection title="Prism Launcher" icon={Triangle}>
        <SettingsCard>
          <span>Prism program</span>
        </SettingsCard>
      </SettingsSection>
      {withAppearance && (
        <SettingsSection title="Appearance" icon={Palette} aside="3 of 4 on">
          <span>Theme</span>
        </SettingsSection>
      )}
      <SettingsSection title="Developer" icon={Code} collapsible>
        <span>Local pack</span>
      </SettingsSection>
    </SettingsLayout>
  )
}

describe('SettingsLayout', () => {
  afterEach(cleanup)

  it('lists the sections that are there, in page order', () => {
    render(<Sections />)
    const nav = screen.getByRole('navigation', { name: 'Sections' })
    const items = Array.from(nav.querySelectorAll('button')).map((b) => b.textContent)
    expect(items).toEqual(['Prism Launcher', 'Appearance', 'Developer'])
    expect(screen.getByText('3 of 4 on')).toBeInTheDocument()
  })

  it('drops a section from the nav when it stops rendering', () => {
    function Toggleable() {
      const [shown, setShown] = useState(true)
      return (
        <>
          <button type="button" onClick={() => setShown(false)}>
            hide
          </button>
          <Sections withAppearance={shown} />
        </>
      )
    }
    render(<Toggleable />)
    fireEvent.click(screen.getByRole('button', { name: 'hide' }))
    expect(screen.queryByRole('button', { name: 'Go to Appearance' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Go to Developer' })).toBeInTheDocument()
  })

  it('scrolls to a section when its nav entry is clicked', () => {
    const real = Element.prototype.scrollIntoView
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    try {
      render(<Sections />)
      fireEvent.click(screen.getByRole('button', { name: 'Go to Appearance' }))
      expect(scroll).toHaveBeenCalledWith(expect.objectContaining({ block: 'start' }))
    } finally {
      Element.prototype.scrollIntoView = real
    }
  })

  it('keeps a collapsible section closed until its heading is clicked', () => {
    render(<Sections />)
    const toggle = screen.getByRole('button', { name: 'Developer' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('Local pack')).toBeNull()
    fireEvent.click(toggle)
    expect(screen.getByText('Local pack')).toBeInTheDocument()
  })
})
