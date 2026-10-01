import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { DISCLAIMER } from '../../lib/disclaimer'
import { CREDITS, PRISM_SOURCE } from '../../lib/licences'
import { AboutSection } from './AboutSection'

vi.mock('../../../wailsjs/go/main/App')

describe('AboutSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(App.GetAppVersion).mockResolvedValue('0.4.2')
    vi.mocked(App.OpenExternal).mockResolvedValue()
  })
  afterEach(cleanup)

  it('shows the version and the disclaimer', async () => {
    render(<AboutSection />)
    expect(await screen.findByText('0.4.2')).toBeInTheDocument()
    expect(screen.getByText(DISCLAIMER)).toBeInTheDocument()
  })

  it('says the version is unknown when Go cannot answer', async () => {
    vi.mocked(App.GetAppVersion).mockRejectedValue('no backend')
    render(<AboutSection />)
    expect(await screen.findByText('unknown')).toBeInTheDocument()
  })

  it('credits each font and the icon set with its licence', () => {
    render(<AboutSection />)
    for (const name of ['Fraunces', 'Inter', 'JetBrains Mono', 'Lucide']) {
      expect(screen.getByText(name)).toBeInTheDocument()
    }
    expect(screen.getAllByText(/SIL Open Font License 1\.1/)).toHaveLength(3)
    expect(screen.getByText(/ISC License/)).toBeInTheDocument()
  })

  it('credits Prism as a separate program and links to its source', () => {
    render(<AboutSection />)
    expect(screen.getByText(/GPL-3\.0 and is not part of Kapital Launcher/)).toBeInTheDocument()
    expect(screen.getByText(/unmodified official release/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Prism Launcher source' }))
    expect(App.OpenExternal).toHaveBeenCalledWith(PRISM_SOURCE)
  })

  it('opens a licence text in place, and closes it again', async () => {
    render(<AboutSection />)
    const toggle = screen.getByRole('button', { name: 'Show licence, Inter' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText(/SIL OPEN FONT LICENSE Version 1\.1/)).not.toBeInTheDocument()

    fireEvent.click(toggle)
    expect(await screen.findByText(/SIL OPEN FONT LICENSE Version 1\.1/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hide licence, Inter' })).toHaveAttribute(
      'aria-expanded',
      'true',
    )

    fireEvent.click(screen.getByRole('button', { name: 'Hide licence, Inter' }))
    expect(screen.queryByText(/SIL OPEN FONT LICENSE Version 1\.1/)).not.toBeInTheDocument()
  })

  it('has a licence text for every credit', async () => {
    for (const credit of CREDITS) {
      expect((await credit.loadText()).length).toBeGreaterThan(500)
    }
    expect(await CREDITS.find((c) => c.id === 'lucide')?.loadText()).toMatch(/ISC License/)
  })

  it('shows why a licence text could not load', async () => {
    const lucide = CREDITS.find((c) => c.id === 'lucide')!
    const real = lucide.loadText
    lucide.loadText = () => Promise.reject(new Error('chunk failed'))
    try {
      render(<AboutSection />)
      fireEvent.click(screen.getByRole('button', { name: 'Show licence, Lucide' }))
      expect(await screen.findByText('chunk failed')).toBeInTheDocument()
    } finally {
      lucide.loadText = real
    }
  })
})
