import { describe, expect, it } from 'vitest'
import { guessPlatform } from './platform'

const wkwebview =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)'
const webview2 =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0'
const webkitgtk = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko)'

describe('guessPlatform', () => {
  it('reads the OS from the WebView in the app', () => {
    expect(guessPlatform(wkwebview, true)).toBe('darwin')
    expect(guessPlatform(webview2, true)).toBe('windows')
    expect(guessPlatform(webkitgtk, true)).toBe('linux')
  })

  it('is the Windows bar in the browser-only preview, on any OS', () => {
    expect(guessPlatform(wkwebview, false)).toBe('windows')
  })
})
