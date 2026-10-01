import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { SupportSection } from './SupportSection'

vi.mock('../../../wailsjs/go/main/App')

describe('SupportSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('says how many lines went and that the identifying parts are masked', async () => {
    vi.mocked(App.CopyRedactedLog).mockResolvedValueOnce(214)
    render(<SupportSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByText(/Copied 214 lines to the clipboard/)).toBeInTheDocument()
    expect(screen.getByText(/name, folders and server addresses masked/)).toBeInTheDocument()
  })

  it('says "line" for one', async () => {
    vi.mocked(App.CopyRedactedLog).mockResolvedValueOnce(1)
    render(<SupportSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByText(/Copied 1 line to the clipboard/)).toBeInTheDocument()
  })

  it('shows a failure in the section and lets the player try again', async () => {
    vi.mocked(App.CopyRedactedLog)
      .mockRejectedValueOnce('the log has nothing in it yet')
      .mockResolvedValueOnce(3)
    render(<SupportSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByText('the log has nothing in it yet')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByText(/Copied 3 lines/)).toBeInTheDocument()
    expect(screen.queryByText('the log has nothing in it yet')).not.toBeInTheDocument()
  })

  it('explains itself without a bridge instead of throwing', async () => {
    Reflect.deleteProperty(window, 'go')
    render(<SupportSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByText(/only be copied from the app window/)).toBeInTheDocument()
    expect(App.CopyRedactedLog).not.toHaveBeenCalled()
  })
})
