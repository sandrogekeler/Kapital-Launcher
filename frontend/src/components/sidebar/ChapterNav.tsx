import { useChapterStore } from '../../stores/useChapterStore'
import { ChapterButton } from './ChapterButton'

export function ChapterNav() {
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const selectedId = useChapterStore((s) => s.selectedId)
  const select = useChapterStore((s) => s.select)
  return (
    <nav aria-label="Chapters" className="flex flex-col gap-1">
      <div className="text-fg-faint px-2.5 pb-1.5 text-xs">Chapters</div>
      {chapters.map((c) => (
        <ChapterButton key={c.id} chapter={c} current={c.id === selectedId} onSelect={select} />
      ))}
    </nav>
  )
}
