import type { LucideIcon } from '../../lib/icons'

/**
 * Rendered stroke weight in screen pixels, held constant across sizes by
 * lucide's `absoluteStrokeWidth`. The reference draws its glyphs at 1.6 on a
 * 24-unit grid at 18px, which is this value.
 */
export const ICON_STROKE_PX = 1.2

const SIZE = {
  sm: { cls: 'size-4', px: 16 },
  md: { cls: 'size-[18px]', px: 18 },
} as const

export type IconSize = keyof typeof SIZE

interface IconProps {
  /** A component from `lib/icons.ts`. Never import lucide-react directly. */
  icon: LucideIcon
  size?: IconSize
  className?: string
  /** Set only when the icon alone carries meaning; inside a labelled control it stays decorative. */
  label?: string
}

/** The single render path for every icon. lucide strokes currentColor, so a text-* token themes it. */
export function Icon({ icon: Glyph, size = 'md', className, label }: IconProps) {
  return (
    <Glyph
      size={SIZE[size].px}
      absoluteStrokeWidth
      strokeWidth={ICON_STROKE_PX}
      className={`${SIZE[size].cls} shrink-0${className ? ` ${className}` : ''}`}
      aria-hidden={label === undefined ? true : undefined}
      aria-label={label}
      role={label === undefined ? undefined : 'img'}
    />
  )
}
