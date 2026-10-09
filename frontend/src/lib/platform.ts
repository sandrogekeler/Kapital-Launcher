/**
 * The OS drawing the window, as Wails' `Environment().platform` names it, guessed
 * from the WebView's user agent before that call answers (#225). Without the
 * guess the header drew the Windows buttons on a Mac for a moment at start.
 * WKWebView reports "Macintosh", WebView2 "Windows". Without a bridge (the
 * browser-only preview) it is the Windows bar, the primary target, whatever
 * browser the preview runs in.
 */
export function guessPlatform(userAgent: string, bridged: boolean): string {
  if (!bridged) return 'windows'
  if (/Macintosh|Mac OS X/.test(userAgent)) return 'darwin'
  if (/Windows/.test(userAgent)) return 'windows'
  return 'linux'
}
