import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
  onClick: () => void
  /** `play` is the one accent-filled control on the screen; `ghost` is everything else. */
  variant?: 'play' | 'ghost'
  disabled?: boolean
}

const VARIANT = {
  play: 'bg-accent text-canvas h-13 px-7.5 text-md tracking-control hover:brightness-(--effect-hover-brightness)',
  ghost: 'border-line-strong hover:bg-hover h-11 border px-4.5 text-base',
} as const

export function Button({ children, onClick, variant = 'ghost', disabled }: Props) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`${VARIANT[variant]} duration-fast ease-standard inline-flex cursor-pointer items-center gap-2 rounded-md font-semibold transition-[background-color,filter] disabled:cursor-default disabled:opacity-50`}
    >
      {children}
    </button>
  )
}
