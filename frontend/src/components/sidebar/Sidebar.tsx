import { ChapterNav } from './ChapterNav'
import { EngineCard } from './EngineCard'

interface Props {
  /** Opens or closes the account page (issue 192). */
  onOpenAccount?: () => void
  /** Whether the account page is the one open, for the tile's pressed look. */
  accountOpen?: boolean
}

export function Sidebar({ onOpenAccount, accountOpen = false }: Props) {
  return (
    <aside className="bg-sunken border-line flex w-(--layout-sidebar) shrink-0 flex-col gap-7 border-r px-4 pt-5.5 pb-4">
      <ChapterNav />
      <div className="grow" />
      <EngineCard onOpenAccount={onOpenAccount} accountOpen={accountOpen} />
    </aside>
  )
}
