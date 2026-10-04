// The custom properties for what a page does when it opens and what a filled
// button does on hover: a page's content rises and unblurs as it is revealed
// once loaded (ui/Reveal), and a Play or Stop button glows in its own colour.
// Read by style.css; a part of gen-tokens.mjs's effects block, kept here so
// that file stays within the repo's size ratchet.

/** Pushes the lines of the four values, for the `:root` block of effects. */
export function emitRevealValues(src, push) {
  const { revealBlur, glow } = src.effect
  push(`  --layout-reveal-rise: ${src.layout.revealRise}${src.layout.unit};`)
  push(`  --effect-reveal-blur: ${revealBlur}px;`)
  push(`  --effect-glow-blur: ${glow.blur}px;`)
  push(`  --effect-glow-strength: ${glow.strength}%;`)
}
