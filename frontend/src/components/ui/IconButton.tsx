import type { LucideIcon } from '../../lib/icons'
import { Icon } from './Icon'

interface Props {
  icon: LucideIcon
  /** Both the tooltip and the accessible name; the icon carries neither. */
  title: string
  onClick: () => void
  /** `md` is the 44px box the hero tools use; `sm` the 32px one in the account card. */
  size?: 'sm' | 'md'
  disabled?: boolean
}

const BOX = {
  sm: 'size-8',
  md: 'size-11',
} as const

/** One square, centred box for every icon control, so a column of them lines up. */
export function IconButton({ icon, title, onClick, size = 'md', disabled }: Props) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={title}
      aria-label={title}
      disabled={disabled}
      className={`${BOX[size]} text-fg-muted hover:bg-hover hover:text-fg duration-fast ease-standard inline-flex shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors disabled:cursor-default disabled:opacity-50 disabled:hover:bg-transparent`}
    >
      <Icon icon={icon} size={size === 'sm' ? 'md' : 'md'} />
    </button>
  )
}
