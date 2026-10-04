import { useEffect } from 'react'
import { usePageSlot } from './usePageSlot'

/** A page over a chapter: its own settings, its logs, its map, or the report of its latest run. */
export interface ChapterPageSpec {
  kind: 'settings' | 'logs' | 'map' | 'report'
  chapterId: string
}

/**
 * The pages laid over the chapter card, and what opens and closes them.
 *
 * The app's settings (the gear in the header) come down over the card area; a
 * chapter's own pages, its settings (the pen), its logs, its map (issue 161)
 * and its run report (Details, on the corner's notice of a game that ended
 * badly), come in from the right inside the card. Each closes with Back and
 * Escape, and a chapter's pages close with the chapter they belong to: the
 * effects fire on the selection, including the restore at startup, when
 * nothing is open anyway.
 *
 * The notice can be another chapter's, so opening its report selects that
 * chapter too. Every page closes when a chapter is brought up, the chapter's
 * settings and report included: selecting the chapter that is already open
 * changes no selection, so the effects would leave them where they were (a
 * preview started from Settings landed on the chapter's settings). The
 * settings and a chapter's page are never open together: opening one closes
 * the other. Closing keeps a page until it has slid out (`usePageSlot`).
 */
export function usePages(selectedId: string, select: (chapterId: string) => void) {
  const settings = usePageSlot<true>()
  const chapterPage = usePageSlot<ChapterPageSpec>()
  const { hide: hideSettings, show: showSettings } = settings
  const { hide: hideChapterPage, show: showChapterPage } = chapterPage
  const openPage = chapterPage.slot?.page

  useEffect(() => hideSettings(), [selectedId, hideSettings])
  useEffect(() => {
    if (openPage?.chapterId !== selectedId) hideChapterPage()
  }, [openPage, selectedId, hideChapterPage])

  const showChapter = (chapterId: string) => {
    select(chapterId)
    hideSettings()
    hideChapterPage()
  }
  const showPage = (kind: ChapterPageSpec['kind']) => (chapterId: string) =>
    showChapterPage({ kind, chapterId })

  return {
    settings,
    chapterPage,
    covered: !!(settings.slot?.open || chapterPage.slot?.open),
    toggleSettings: () => {
      if (settings.slot?.open) return hideSettings()
      hideChapterPage()
      showSettings(true)
    },
    showChapter,
    openReport: (chapterId: string) => {
      showChapter(chapterId)
      showChapterPage({ kind: 'report', chapterId })
    },
    openChapterSettings: showPage('settings'),
    openLogs: showPage('logs'),
    openMap: showPage('map'),
  }
}
