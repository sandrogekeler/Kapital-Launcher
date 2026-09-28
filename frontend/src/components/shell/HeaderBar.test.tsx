import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import * as Runtime from '../../../wailsjs/runtime/runtime'
import { HeaderBar } from './HeaderBar'

vi.mock('../../../wailsjs/runtime/runtime')

describe('HeaderBar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(Runtime.WindowIsMaximised).mockResolvedValue(false)
    // A bridge is present: the window calls go through.
    Object.assign(window, { go: {} })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('draws the brand, the gear and our own window buttons on Windows', async () => {
    render(<HeaderBar platform="windows" />)
    expect(screen.getByAltText('Kapital Launcher')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Settings' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'Minimise' }))
    expect(Runtime.WindowMinimise).toHaveBeenCalledOnce()
    fireEvent.click(await screen.findByRole('button', { name: 'Maximise' }))
    expect(Runtime.WindowToggleMaximise).toHaveBeenCalledOnce()
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(Runtime.Quit).toHaveBeenCalledOnce()
  })

  it('shows Restore once the window reports itself maximised', async () => {
    render(<HeaderBar platform="windows" />)
    await screen.findByRole('button', { name: 'Maximise' })
    vi.mocked(Runtime.WindowIsMaximised).mockResolvedValue(true)
    await act(async () => {
      window.dispatchEvent(new Event('resize'))
    })
    expect(screen.getByRole('button', { name: 'Restore' })).toBeInTheDocument()
  })

  it('maximises on a double-click of the bar, not of its buttons', () => {
    const { container } = render(<HeaderBar platform="windows" />)
    fireEvent.doubleClick(screen.getByRole('button', { name: 'Minimise' }))
    expect(Runtime.WindowToggleMaximise).not.toHaveBeenCalled()
    fireEvent.doubleClick(container.querySelector('header')!)
    expect(Runtime.WindowToggleMaximise).toHaveBeenCalledOnce()
  })

  it('leaves the window buttons to macOS', () => {
    render(<HeaderBar platform="darwin" onOpenSettings={() => undefined} />)
    expect(screen.queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Settings' })).toBeEnabled()
  })

  it('does nothing without a bridge', () => {
    Reflect.deleteProperty(window, 'go')
    render(<HeaderBar platform="windows" />)
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(Runtime.Quit).not.toHaveBeenCalled()
  })
})
