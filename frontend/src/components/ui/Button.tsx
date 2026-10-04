import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  onClick: () => void
  /**
   * `play` is the one accent-filled control on the screen, and `stop` takes its place while a game
   * runs, in the danger colour; `ghost` is everything else.
   */
  variant?: 'play' | 'stop' | 'ghost'
  disabled?: boolean
  /** Announces a change of the button's own label, for one that says how it went. */
  live?: boolean
}

// The filled variants lift on hover: brighter, and a soft glow in their own colour (style.css),
// over the reveal's 200 ms rather than the 140 ms of a plain hover, so the glow eases in and
// out instead of snapping.
const FILLED =
  'duration-reveal text-canvas h-13 min-w-(--layout-play-min) justify-center px-7.5 text-md tracking-control enabled:hover:brightness-(--effect-hover-brightness)'
const VARIANT = {
  play: `bg-accent glow-accent ${FILLED}`,
  stop: `bg-danger glow-danger ${FILLED}`,
  ghost: 'duration-fast border-line-strong hover:bg-hover h-11 border px-4.5 text-base',
} as const

export function Button({ children, onClick, variant = 'ghost', disabled, live }: Props) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-live={live ? 'polite' : undefined}
      className={`${VARIANT[variant]} ease-standard inline-flex cursor-pointer items-center gap-2 rounded-md font-semibold transition-[background-color,filter,box-shadow] disabled:cursor-default disabled:opacity-50 motion-reduce:transition-none`}
    >
      {children}
    </button>
  )
}
