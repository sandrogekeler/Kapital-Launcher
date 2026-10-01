import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'
import { reportError } from '../lib/reportError'

vi.mock('../lib/reportError')

function Broken(): never {
  throw new Error('view exploded')
}

describe('ErrorBoundary', () => {
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    vi.resetAllMocks()
  })

  it('shows the message and reports a render error with its component stack', () => {
    // React logs the caught error itself; keep the test output quiet.
    vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <ErrorBoundary>
        <Broken />
      </ErrorBoundary>,
    )
    expect(screen.getByText('view exploded')).toBeInTheDocument()
    expect(reportError).toHaveBeenCalledTimes(1)
    expect(reportError).toHaveBeenCalledWith(
      'render',
      expect.objectContaining({ message: 'view exploded' }),
      expect.stringContaining('Broken'),
    )
  })

  it('renders its children and reports nothing when they are fine', () => {
    render(
      <ErrorBoundary>
        <p>fine</p>
      </ErrorBoundary>,
    )
    expect(screen.getByText('fine')).toBeInTheDocument()
    expect(reportError).not.toHaveBeenCalled()
  })
})
