import type { ComponentProps } from 'react'

interface Props extends Omit<ComponentProps<'button'>, 'type'> {
  /** `sm` reads with the body; `xs` sits in small print. */
  size?: 'sm' | 'xs'
  /** A heavier weight, for the one link that is the point of its notice. */
  strong?: boolean
}

const SIZE = { sm: 'text-sm', xs: 'text-xs' } as const

/**
 * The one link style: accent text that underlines on hover, a button because
 * it acts (opens a page, shows a licence) rather than navigating. It takes any
 * button attribute, so a disclosure keeps its aria-expanded.
 */
export function TextLink({ size = 'sm', strong, className = '', ...rest }: Props) {
  return (
    <button
      type="button"
      className={`text-accent cursor-pointer underline-offset-2 hover:underline ${SIZE[size]} ${
        strong ? 'font-semibold' : ''
      } ${className}`}
      {...rest}
    />
  )
}
