import { useLayoutEffect, useRef, useState } from 'react'
import { recallTurn, rememberTurn, startFace } from '../../lib/panorama'
import { useReducedMotion } from '../../lib/useReducedMotion'

interface Props {
  /** The six faces in the game's order: front, right, back, left, top, bottom. */
  faces: readonly string[]
  /** Where the turn starts, in degrees (lib/panorama): something worth seeing first. */
  start?: number
}

/**
 * A chapter's title-screen panorama (issue 195): the six faces as a cube the
 * camera is inside, turning slowly with the camera tilted a little down, as the
 * game's menu shows it. The geometry and the motion are `.panorama*` in
 * style.css, built from tokens. It fades in once every face has loaded and is
 * transparent until then, so the picture under it stays in view meanwhile.
 * With reduced motion it is one face, still, and needs only that one.
 * Loaded lazily by the hero when the panorama is the chosen picture, so a
 * player who does not choose it never downloads the code.
 */
export function PanoramaCube({ faces, start = 0 }: Props) {
  const reduced = useReducedMotion()
  const still = startFace(start)
  const shown = reduced ? faces.slice(still, still + 1) : faces
  // The start angle is the chapter's, so it reaches the CSS as a custom property set on the
  // element, as RangeField sets its fill, and never through style.
  const cube = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    cube.current?.style.setProperty('--panorama-start', `${start}deg`)
  }, [start])
  // The turn pauses when the chapter is left and goes on from there when it comes back: the
  // CSS animation's time is kept by the first face's address and set again on the next mount.
  const id = faces[0] ?? ''
  useLayoutEffect(() => {
    const turn = cube.current?.getAnimations?.()[0]
    if (!turn) return
    const at = recallTurn(id)
    if (at !== undefined) turn.currentTime = at
    return () => {
      const now = turn.currentTime
      if (typeof now === 'number') rememberTurn(id, now)
    }
  }, [id])
  const [loaded, setLoaded] = useState(0)
  const ready = loaded >= shown.length
  const fade = `transition-opacity duration-slow ease-standard ${ready ? 'opacity-100' : 'opacity-0'}`

  if (reduced) {
    return (
      <img
        src={shown[0]}
        alt=""
        draggable={false}
        onLoad={() => setLoaded((n) => n + 1)}
        className={`absolute inset-0 size-full object-cover object-[center_40%] ${fade}`}
      />
    )
  }
  return (
    <div className={`panorama ${fade}`} aria-hidden>
      <div className="panorama-scene">
        <div ref={cube} className="panorama-cube">
          {shown.map((src, face) => (
            <img
              key={face}
              src={src}
              alt=""
              draggable={false}
              data-face={face}
              onLoad={() => setLoaded((n) => n + 1)}
              className="panorama-face"
            />
          ))}
        </div>
      </div>
    </div>
  )
}
