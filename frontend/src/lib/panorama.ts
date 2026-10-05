import { useEffect, useState } from 'react'

/**
 * Where each chapter's panorama starts turning, in degrees along the turn (the camera turns
 * right, so +90 begins on face 1). Chosen by the author so the first view is something worth
 * seeing; a chapter not named starts on face 0.
 */
export const PANORAMA_START_DEG: Readonly<Record<string, number>> = {
  luxemburg: 50,
  lichdenstein: -30,
  frangfurd: 140,
}

/** The start angle of a chapter's panorama. */
export function panoramaStart(chapterId: string): number {
  return PANORAMA_START_DEG[chapterId] ?? 0
}

/** The face a still panorama (reduced motion) shows: the one the start angle is nearest. */
export function startFace(deg: number): number {
  return (((Math.round(deg / 90) % 4) + 4) % 4) as 0 | 1 | 2 | 3
}

// Where each panorama's turn was when its chapter was left, by its first face's address: the
// cube picks up there when the chapter comes back instead of starting over. In memory only.
const turnAt = new Map<string, number>()

/** Remembers how far a panorama's turn had gone (the animation's current time, in ms). */
export function rememberTurn(id: string, ms: number): void {
  turnAt.set(id, ms)
}

/** How far a panorama's turn had gone when it was left, or undefined for a first showing. */
export function recallTurn(id: string): number | undefined {
  return turnAt.get(id)
}

// Faces decoded once stay decoded for the session: the browser keeps an image's decoded
// pixels while an element holds it, so going back to a chapter shows its cube at once.
const decoded = new Map<string, Promise<HTMLImageElement>>()

/** Decodes a face off the main thread, once per address. */
function decodeFace(src: string): Promise<HTMLImageElement> {
  let p = decoded.get(src)
  if (!p) {
    const img = new Image()
    img.decoding = 'async'
    img.src = src
    p = img.decode().then(() => img)
    // A face that fails is tried again the next time it is asked for.
    p.catch(() => decoded.delete(src))
    decoded.set(src, p)
  }
  return p
}

/**
 * Whether every face is decoded and ready to draw (issue 195 follow-up). A panorama's faces are
 * megabytes of PNG each; mounting the cube before they are decoded made the browser decode them
 * all while the chapter switched, which held the switch for most of a second. The faces are
 * decoded in the background first, and the cube is drawn once they are.
 */
export function usePanoramaReady(faces: readonly string[] | undefined): boolean {
  const key = faces?.join('\n') ?? ''
  const [ready, setReady] = useState<string | null>(null)
  useEffect(() => {
    if (!key) return
    let live = true
    Promise.all(key.split('\n').map(decodeFace)).then(
      () => live && setReady(key),
      () => undefined,
    )
    return () => {
      live = false
    }
  }, [key])
  return key !== '' && ready === key
}
