import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { panoramaStart, startFace, usePanoramaReady } from './panorama'

describe('the panorama start', () => {
  it('starts each chapter where the author chose, and others on face 0', () => {
    expect(panoramaStart('luxemburg')).toBe(50)
    expect(panoramaStart('frangfurd')).toBe(140)
    expect(panoramaStart('lichdenstein')).toBe(-30)
    expect(panoramaStart('atlantis')).toBe(0)
  })

  it('shows the nearest face when it stands still', () => {
    expect(startFace(0)).toBe(0)
    expect(startFace(50)).toBe(1)
    expect(startFace(140)).toBe(2)
    expect(startFace(-30)).toBe(0)
    expect(startFace(-90)).toBe(3)
    expect(startFace(370)).toBe(0)
  })
})

describe('usePanoramaReady', () => {
  const real = HTMLImageElement.prototype.decode
  afterEach(() => {
    HTMLImageElement.prototype.decode = real
  })

  it('is ready only once every face is decoded, and for those faces only', async () => {
    const pending: (() => void)[] = []
    HTMLImageElement.prototype.decode = vi.fn(
      () => new Promise<void>((resolve) => pending.push(resolve)),
    )
    const faces = ['/panorama/x/panorama_0.png?v=a', '/panorama/x/panorama_1.png?v=a']
    const { result, rerender } = renderHook(({ f }) => usePanoramaReady(f), {
      initialProps: { f: faces as readonly string[] | undefined },
    })
    expect(result.current).toBe(false)
    await act(async () => pending[0]?.())
    expect(result.current).toBe(false)
    await act(async () => pending[1]?.())
    expect(result.current).toBe(true)

    // No panorama is never ready, and faces decoded once are ready again at once.
    rerender({ f: undefined })
    expect(result.current).toBe(false)
    rerender({ f: faces })
    await act(async () => undefined)
    expect(result.current).toBe(true)
  })
})
