// The layout and effect blocks of tokens.css: geometry that is not a Tailwind
// namespace, and values a utility cannot name on its own, read through var().
// A part of gen-tokens.mjs, kept here so that file stays within the repo's size
// ratchet, as emit-reveal.mjs is.
import { emitRevealValues } from './emit-reveal.mjs'

export function emitLayout(src, push) {
  push(`/* Layout geometry is not a Tailwind namespace. Read it with the var()`)
  push(`   shorthand, w-(--layout-sidebar), or from style.css. */`)
  push(`:root {`)
  const { unit } = src.layout
  for (const k of ['sidebar', 'hero', 'titlebar', 'scrollbar', 'slide', 'drift', 'title']) {
    push(`  --layout-${k}: ${src.layout[k]}${unit};`)
  }
  push(`  --layout-play-min: ${src.layout.playMin}${unit};`)
  push(`  --layout-icon-sm: ${src.layout.icon.sm}${src.layout.unit};`)
  push(`  --layout-icon-md: ${src.layout.icon.md}${src.layout.unit};`)
  push(`  --layout-splash-width: ${src.layout.splash.width}${src.layout.unit};`)
  push(`  --layout-splash-height: ${src.layout.splash.height}${src.layout.unit};`)
  push(`  --layout-window-width: ${src.layout.window.width}${src.layout.unit};`)
  push(`  --layout-window-height: ${src.layout.window.height}${src.layout.unit};`)
  push(`}`)
}

export function emitEffects(src, push) {
  const {
    hoverBrightness,
    focusRing,
    scrim,
    accent,
    artPending,
    motionBlur,
    tileBlur,
    tileTint,
    panorama,
  } = src.effect
  push(`/* Effects: values a utility cannot name on its own. Read by the shared`)
  push(`   styles/base.css and by brightness-(--effect-hover-brightness). */`)
  push(`:root {`)
  push(`  --effect-hover-brightness: ${hoverBrightness};`)
  push(`  --focus-ring-width: ${focusRing.width}px;`)
  push(`  --focus-ring-offset: ${focusRing.offset}px;`)
  push(`  --scrim-edge: ${scrim.edge}${scrim.unit};`)
  push(`  --scrim-mid: ${scrim.mid}${scrim.unit};`)
  push(`  --scrim-mid-at: ${scrim.midAt}${scrim.unit};`)
  push(`  --scrim-far: ${scrim.far}${scrim.unit};`)
  push(`  --scrim-wash: ${scrim.wash}${scrim.unit};`)
  push(`  --scrim-tile: ${scrim.tile}${scrim.unit};`)
  push(`  --accent-wash-mix: ${accent.wash}${accent.unit};`)
  push(`  --accent-edge-mix: ${accent.edge}${accent.unit};`)
  push(`  --art-pending-tint: ${artPending.tint}%;`)
  push(`  --art-pending-line: ${artPending.line}%;`)
  push(`  --art-pending-cell: ${artPending.cell}px;`)
  push(`  --art-pending-cell-thumb: ${artPending.cellThumb}px;`)
  push(`  --effect-motion-blur: ${motionBlur}px;`)
  push(`  --effect-tile-blur: ${tileBlur}px;`)
  push(`  --effect-tile-tint: ${tileTint}%;`)
  push(`  --panorama-fov: ${panorama.fov}deg;`)
  push(`  --panorama-tilt: ${panorama.tilt}deg;`)
  push(`  --panorama-bleed: ${panorama.bleed / 100};`)
  emitRevealValues(src, push)
  push(`}`)
}
