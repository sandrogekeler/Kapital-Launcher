import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useLoadedImage } from './useLoadedImage'

/** An Image whose decode the test settles by hand, per src. */
const pending = new Map<string, { resolve: () => void; reject: () => void }>()

class FakeImage {
  src = ''
  decode() {
    return new Promise<void>((resolve, reject) => {
      pending.set(this.src, { resolve, reject })
    })
  }
}

const settle = async (src: string, ok = true) => {
  await act(async () => {
    const p = pending.get(src)
    if (ok) p?.resolve()
    else p?.reject()
    await Promise.resolve()
  })
}

describe('useLoadedImage', () => {
  beforeEach(() => {
    pending.clear()
    vi.stubGlobal('Image', FakeImage)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('shows the fallback until the picture has decoded, then the picture', async () => {
    const { result } = renderHook(() => useLoadedImage('/wiki-art/a-1.webp', '/own.webp'))
    expect(result.current).toBe('/own.webp')
    await settle('/wiki-art/a-1.webp')
    expect(result.current).toBe('/wiki-art/a-1.webp')
  })

  it('keeps the last picture while the next one loads, so a change starts only when it is ready', async () => {
    const { result, rerender } = renderHook(({ src }) => useLoadedImage(src, '/own.webp'), {
      initialProps: { src: '/wiki-art/b-1.webp' },
    })
    await settle('/wiki-art/b-1.webp')
    rerender({ src: '/wiki-art/b-2.webp' })
    expect(result.current).toBe('/wiki-art/b-1.webp')
    await settle('/wiki-art/b-2.webp')
    expect(result.current).toBe('/wiki-art/b-2.webp')
  })

  it('keeps the last picture when the next one fails to load', async () => {
    const { result, rerender } = renderHook(({ src }) => useLoadedImage(src, '/own.webp'), {
      initialProps: { src: '/wiki-art/c-1.webp' },
    })
    await settle('/wiki-art/c-1.webp')
    rerender({ src: '/wiki-art/c-broken.webp' })
    await settle('/wiki-art/c-broken.webp', false)
    expect(result.current).toBe('/wiki-art/c-1.webp')
  })

  it('draws a picture decoded before at once', async () => {
    const first = renderHook(() => useLoadedImage('/wiki-art/d-1.webp', '/own.webp'))
    await settle('/wiki-art/d-1.webp')
    first.unmount()
    const { result } = renderHook(() => useLoadedImage('/wiki-art/d-1.webp', '/own.webp'))
    expect(result.current).toBe('/wiki-art/d-1.webp')
  })
})
