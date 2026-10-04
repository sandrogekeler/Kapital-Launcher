import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
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
    vi.useRealTimers()
    Reflect.deleteProperty(window, 'go')
  })

  it('says the copy worked on the button for a second, and then goes back', async () => {
    vi.mocked(App.CopyRedactedLog).mockResolvedValueOnce(214)
    render(<SupportSection />)
    expect(screen.getByText('Log for a bug report')).toBeInTheDocument()
    vi.useFakeTimers()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    })
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
    expect(screen.queryByText(/to the clipboard/)).toBeNull()
    act(() => void vi.advanceTimersByTime(1000))
    expect(screen.getByRole('button', { name: 'Copy log' })).toBeInTheDocument()
  })

  it('says a failure on the button for two seconds and lets the player try again', async () => {
    vi.mocked(App.CopyRedactedLog)
      .mockRejectedValueOnce('the log has nothing in it yet')
      .mockResolvedValueOnce(3)
    render(<SupportSection />)
    vi.useFakeTimers()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    })
    expect(screen.getByRole('button', { name: 'Copy failed' })).toBeInTheDocument()
    expect(screen.queryByText('the log has nothing in it yet')).toBeNull()
    act(() => void vi.advanceTimersByTime(1999))
    expect(screen.getByRole('button', { name: 'Copy failed' })).toBeInTheDocument()
    act(() => void vi.advanceTimersByTime(1))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    })
    expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('fails on the button without a bridge instead of throwing', async () => {
    Reflect.deleteProperty(window, 'go')
    render(<SupportSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy log' }))
    expect(await screen.findByRole('button', { name: 'Copy failed' })).toBeInTheDocument()
    expect(App.CopyRedactedLog).not.toHaveBeenCalled()
  })
})
