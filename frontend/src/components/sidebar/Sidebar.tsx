import { ChapterNav } from './ChapterNav'
import { EngineCard } from './EngineCard'

export function Sidebar() {
  return (
    <aside className="bg-sunken border-line flex w-(--layout-sidebar) shrink-0 flex-col gap-7 border-r px-4 pt-5.5 pb-4">
      <ChapterNav />
      <div className="grow" />
      <EngineCard />
    </aside>
  )
}
