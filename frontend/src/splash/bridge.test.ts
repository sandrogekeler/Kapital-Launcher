import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { installBridge, send, useCardStore } from './bridge'

const state = {
  chapter: { id: 'frangfurd', name: 'Frangfurd' },
  game: {
    chapterId: 'frangfurd',
    phase: 'mods',
    since: '2026-10-01T10:00:00Z',
    startedAt: '2026-10-01T09:59:00Z',
    splash: true,
  },
}

describe('the card bridge', () => {
  let remove = () => {}
  beforeEach(() => {
    useCardStore.setState({ state: null })
    remove = installBridge()
  })
  afterEach(() => {
    remove()
    delete window.chrome
    delete window.webkit
  })

  describe('from Go', () => {
    it('has no state until the first push', () => {
      expect(useCardStore.getState().state).toBeNull()
    })

    it('keeps the newest state pushed through window.kapitalSplash', () => {
      window.kapitalSplash?.update(state)
      expect(useCardStore.getState().state).toEqual(state)
      const next = { ...state, error: 'no folder' }
      window.kapitalSplash?.update(next)
      expect(useCardStore.getState().state?.error).toBe('no folder')
    })

    it.each([
      ['null', null],
      ['a string', 'mods'],
      ['no chapter', { game: state.game }],
      ['no game', { chapter: state.chapter }],
      ['a chapter without a name', { ...state, chapter: { id: 'x' } }],
    ])('drops a push that is %s and keeps the last good state', (_name, bad) => {
      window.kapitalSplash?.update(state)
      window.kapitalSplash?.update(bad)
      expect(useCardStore.getState().state).toEqual(state)
    })

    it('removes window.kapitalSplash when uninstalled', () => {
      remove()
      expect(window.kapitalSplash).toBeUndefined()
    })
  })

  describe('to Go', () => {
    it('posts the action as a JSON string through WebView2', () => {
      const postMessage = vi.fn()
      window.chrome = { webview: { postMessage } }
      send('leave')
      send('openFolder')
      send('copyLog')
      expect(postMessage.mock.calls).toEqual([
        ['{"action":"leave"}'],
        ['{"action":"openFolder"}'],
        ['{"action":"copyLog"}'],
      ])
    })

    it('posts through the WebKit handler named splash when there is no WebView2', () => {
      const postMessage = vi.fn()
      window.webkit = { messageHandlers: { splash: { postMessage } } }
      send('copyLog')
      expect(postMessage).toHaveBeenCalledWith('{"action":"copyLog"}')
    })

    it('prefers WebView2 when both exist', () => {
      const chrome = vi.fn()
      const webkit = vi.fn()
      window.chrome = { webview: { postMessage: chrome } }
      window.webkit = { messageHandlers: { splash: { postMessage: webkit } } }
      send('leave')
      expect(chrome).toHaveBeenCalledOnce()
      expect(webkit).not.toHaveBeenCalled()
    })

    it('does nothing with no bridge, or a WebKit one without the splash handler', () => {
      expect(() => send('leave')).not.toThrow()
      window.webkit = { messageHandlers: {} }
      expect(() => send('leave')).not.toThrow()
    })
  })
})
