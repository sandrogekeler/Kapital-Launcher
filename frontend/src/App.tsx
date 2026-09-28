import { useEffect } from 'react'
import { Sidebar } from './components/sidebar/Sidebar'
import { Hero } from './components/main/Hero'
import { ActionBar } from './components/main/ActionBar'
import { Panels } from './components/main/Panels'
import { selectChapter, useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { useSettingsStore } from './stores/useSettingsStore'
import { OpenChapterWiki, OpenExternal } from '../wailsjs/go/main/App'
import { errMsg } from './lib/ipc'

const PRISM_SITE = 'https://prismlauncher.org'

export default function App() {
  const chapter = useChapterStore(selectChapter)
  const selectedId = useChapterStore((s) => s.selectedId)
  const loadChapters = useChapterStore((s) => s.load)
  const select = useChapterStore((s) => s.select)

  const engine = useEngineStore((s) => s.engine)
  const launching = useEngineStore((s) => s.launching)
  const launchError = useEngineStore((s) => s.error)
  const loadEngine = useEngineStore((s) => s.load)
  const launch = useEngineStore((s) => s.launch)

  const theme = useSettingsStore((s) => s.settings.theme)
  const lastChapter = useSettingsStore((s) => s.settings.lastChapter)
  const settingsLoaded = useSettingsStore((s) => s.loaded)
  const loadSettings = useSettingsStore((s) => s.load)

  // One read per store on mount. These are reads of state Go holds, not
  // events, so an effect is the right tool.
  useEffect(() => {
    void loadChapters()
    void loadEngine()
    void loadSettings()
  }, [loadChapters, loadEngine, loadSettings])

  // Reopen the chapter that was open last time, once settings have arrived.
  useEffect(() => {
    if (settingsLoaded && lastChapter) select(lastChapter)
  }, [settingsLoaded, lastChapter, select])

  // The accent and the theme are attributes on the root: tokens.css maps
  // data-chapter to --accent and data-theme to the palette.
  useEffect(() => {
    document.documentElement.dataset.chapter = selectedId
  }, [selectedId])
  useEffect(() => {
    if (theme === 'system') delete document.documentElement.dataset.theme
    else document.documentElement.dataset.theme = theme
  }, [theme])

  if (!chapter) return null

  const openWiki = () =>
    OpenChapterWiki(chapter.id).catch((e) => console.warn('open wiki', errMsg(e)))
  const openPrismSite = () =>
    OpenExternal(PRISM_SITE).catch((e) => console.warn('open prism site', errMsg(e)))

  return (
    <div className="bg-canvas flex h-full">
      <ChapterSelectionSync />
      <Sidebar />
      <main className="flex min-w-0 grow flex-col">
        <Hero chapter={chapter} onOpenWiki={openWiki} />
        <ActionBar
          chapter={chapter}
          engine={engine}
          launching={launching === chapter.id}
          error={launchError}
          onPlay={() => void launch(chapter.id)}
          onInstallPrism={openPrismSite}
        />
        <Panels chapter={chapter} onOpenWiki={openWiki} />
      </main>
      <footer className="text-fg-faint text-2xs pointer-events-none fixed inset-x-0 bottom-2 text-center">
        NOT AN OFFICIAL MINECRAFT PRODUCT. NOT APPROVED BY OR ASSOCIATED WITH MOJANG OR MICROSOFT.
      </footer>
    </div>
  )
}

/**
 * Persists a chapter selection made in the sidebar, so the app reopens on it.
 * Kept out of the nav so ChapterNav stays a pure view over the chapter store;
 * the settings write is an app-level concern. Waits for settings to load, or
 * the bundled default would overwrite the saved choice before it was read.
 */
function ChapterSelectionSync() {
  const selectedId = useChapterStore((s) => s.selectedId)
  const loaded = useSettingsStore((s) => s.loaded)
  const last = useSettingsStore((s) => s.settings.lastChapter)
  const update = useSettingsStore((s) => s.update)
  useEffect(() => {
    if (loaded && selectedId && selectedId !== last) {
      update({ lastChapter: selectedId }).catch((e) => console.warn('save chapter', errMsg(e)))
    }
  }, [loaded, selectedId, last, update])
  return null
}
