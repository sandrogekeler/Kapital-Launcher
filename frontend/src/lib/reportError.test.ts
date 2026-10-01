import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as Bindings from '../../wailsjs/go/main/App'
import { installErrorReporting, reportError } from './reportError'

vi.mock('../../wailsjs/go/main/App')

// jsdom has no window.go, so a test that wants a backend adds one.
function attachBridge() {
  Object.defineProperty(window, 'go', { value: {}, configurable: true })
}

describe('reportError', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.mocked(Bindings.LogFrontendError).mockResolvedValue()
  })
  afterEach(() => {
    Reflect.deleteProperty(window, 'go')
    vi.restoreAllMocks()
    vi.resetAllMocks()
  })

  it('only logs to the console without a bridge', () => {
    reportError('render', new Error('boom'))
    expect(console.error).toHaveBeenCalled()
    expect(Bindings.LogFrontendError).not.toHaveBeenCalled()
  })

  it('sends an Error with its message and stack', () => {
    attachBridge()
    const err = new Error('boom')
    reportError('error', err)
    expect(Bindings.LogFrontendError).toHaveBeenCalledWith('error', 'boom', err.stack)
  })

  it('appends the component stack to a render error', () => {
    attachBridge()
    const err = new Error('boom')
    reportError('render', err, '\n    at Chapter\n    at App')
    expect(Bindings.LogFrontendError).toHaveBeenCalledWith(
      'render',
      'boom',
      expect.stringContaining(err.stack ?? ''),
    )
    expect(Bindings.LogFrontendError).toHaveBeenCalledWith(
      'render',
      'boom',
      expect.stringContaining('Component stack:\n    at Chapter\n    at App'),
    )
  })

  it('describes a string and an object that are not Errors', () => {
    attachBridge()
    reportError('rejection', 'plain text')
    reportError('rejection', { code: 7 })
    const calls = vi.mocked(Bindings.LogFrontendError).mock.calls
    expect(calls[0]).toEqual(['rejection', 'plain text', ''])
    expect(calls[1]).toEqual(['rejection', '{"code":7}', ''])
  })

  it('describes values JSON cannot', () => {
    attachBridge()
    const cyclic: Record<string, unknown> = {}
    cyclic.self = cyclic
    reportError('rejection', cyclic)
    reportError('rejection', undefined)
    expect(Bindings.LogFrontendError).toHaveBeenNthCalledWith(1, 'rejection', '[object Object]', '')
    expect(Bindings.LogFrontendError).toHaveBeenNthCalledWith(2, 'rejection', 'undefined', '')
  })

  it('does not throw or reject when the backend refuses', async () => {
    attachBridge()
    vi.mocked(Bindings.LogFrontendError).mockRejectedValue('nope')
    expect(() => reportError('render', new Error('boom'))).not.toThrow()
    await Promise.resolve()
    await Promise.resolve()
    expect(console.warn).toHaveBeenCalledWith('report frontend error', 'nope')
  })

  it('does not throw when the binding throws synchronously', () => {
    attachBridge()
    vi.mocked(Bindings.LogFrontendError).mockImplementation(() => {
      throw new TypeError('window.go is undefined')
    })
    expect(() => reportError('render', new Error('boom'))).not.toThrow()
    expect(console.warn).toHaveBeenCalledWith('report frontend error', 'window.go is undefined')
  })
})

describe('installErrorReporting', () => {
  let remove: () => void
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(Bindings.LogFrontendError).mockResolvedValue()
    attachBridge()
    remove = installErrorReporting()
  })
  afterEach(() => {
    remove()
    Reflect.deleteProperty(window, 'go')
    vi.restoreAllMocks()
    vi.resetAllMocks()
  })

  it('reports a window error event as error', () => {
    const err = new Error('thrown in a handler')
    window.dispatchEvent(new ErrorEvent('error', { error: err, message: err.message }))
    expect(Bindings.LogFrontendError).toHaveBeenCalledWith(
      'error',
      'thrown in a handler',
      err.stack,
    )
  })

  it('falls back to the event message when the error object is hidden', () => {
    window.dispatchEvent(new ErrorEvent('error', { message: 'Script error.' }))
    expect(Bindings.LogFrontendError).toHaveBeenCalledWith('error', 'Script error.', '')
  })

  it('reports an unhandled rejection as rejection', () => {
    const reason = new Error('nobody caught this')
    const event = new Event('unhandledrejection')
    Object.defineProperty(event, 'reason', { value: reason })
    window.dispatchEvent(event)
    expect(Bindings.LogFrontendError).toHaveBeenCalledWith(
      'rejection',
      'nobody caught this',
      reason.stack,
    )
  })

  it('stops reporting once removed', () => {
    remove()
    window.dispatchEvent(new ErrorEvent('error', { message: 'late' }))
    expect(Bindings.LogFrontendError).not.toHaveBeenCalled()
  })
})
