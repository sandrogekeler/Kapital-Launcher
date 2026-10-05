import { useState } from 'react'
import { useReducedMotion } from '../../lib/useReducedMotion'

interface Props {
  /** The six faces in the game's order: front, right, back, left, top, bottom. */
  faces: readonly string[]
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
export function PanoramaCube({ faces }: Props) {
  const reduced = useReducedMotion()
  const shown = reduced ? faces.slice(0, 1) : faces
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
        <div className="panorama-cube">
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
