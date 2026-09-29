import { useEffect, useRef, useState } from 'react'
import { HeaderBar } from './components/shell/HeaderBar'
import { Sidebar } from './components/sidebar/Sidebar'
import { Hero } from './components/main/Hero'
import { ActionBar } from './components/main/ActionBar'
import { Panels } from './components/main/Panels'
import { selectChapter, useChapterStore } from './stores/useChapterStore'
import { selectInstalled, useEngineStore } from './stores/useEngineStore'
import { useSettingsStore } from './stores/useSettingsStore'
import { selectStatus, useServerStore } from './stores/useServerStore'
import { OpenChapterWiki, OpenExternal } from '../wailsjs/go/main/App'
import { Environment } from '../wailsjs/runtime/runtime'
import { errMsg, readOr } from './lib/ipc'

const PRISM_SITE = 'https://prismlauncher.org'

export default function App() {
  const chapter = useChapterStore(selectChapter)
  const selectedId = useChapterStore((s) => s.selectedId)
  const loadChapters = useChapterStore((s) => s.load)

  const engine = useEngineStore((s) => s.engine)
  const launching = useEngineStore((s) => s.launching)
  const launchError = useEngineStore((s) => s.error)
  const loadEngine = useEngineStore((s) => s.load)
  const loadInstances = useEngineStore((s) => s.loadInstances)
  const installed = useEngineStore(selectInstalled(selectedId))
  const launch = useEngineStore((s) => s.launch)

  const status = useServerStore(selectStatus(selectedId))
  const checking = useServerStore((s) => s.checking)
  const listenServers = useServerStore((s) => s.listen)
  const checkServer = useServerStore((s) => s.check)

  const theme = useSettingsStore((s) => s.settings.theme)
  const loadSettings = useSettingsStore((s) => s.load)

  // Which OS draws the window, for the header's controls. Without a bridge
  // (the browser-only preview) it shows the Windows bar, the primary target.
  const [platform, setPlatform] = useState('windows')
  useEffect(() => {
    void readOr(Environment, null).then((env) => env && setPlatform(env.platform))
  }, [])

  // One read per store on mount. These are reads of state Go holds, not
  // events, so an effect is the right tool.
  useEffect(() => {
    void loadChapters()
    void loadEngine()
    void loadSettings()
  }, [loadChapters, loadEngine, loadSettings])

  // An instance appears when the user imports one in Prism, which happens in
  // another window. Coming back is the moment to look again; a focus event,
  // not a timer, so nothing polls.
  useEffect(() => {
    const onFocus = () => void loadInstances()
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [loadInstances])

  // Server status arrives as events from Go's ticker (.claude/rules/ipc.md);
  // this is the one listener, for the app's lifetime.
  useEffect(() => listenServers(), [listenServers])

  // Opening a chapter with a server asks for a fresh ping rather than waiting
  // for the next tick, so the line is current when it is looked at.
  useEffect(() => {
    if (chapter?.server) void checkServer(chapter.id)
  }, [chapter?.id, chapter?.server, checkServer])

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
    <div className="bg-canvas flex h-full flex-col">
      <ChapterSelectionSync />
      <HeaderBar platform={platform} />
      <div className="flex min-h-0 grow">
        <Sidebar />
        <main className="flex min-w-0 grow flex-col">
          <Hero chapter={chapter} onOpenWiki={openWiki} />
          <ActionBar
            chapter={chapter}
            engine={engine}
            status={status}
            installed={installed}
            launching={launching === chapter.id}
            checking={checking === chapter.id}
            error={launchError}
            onPlay={() => void launch(chapter.id)}
            onInstallPrism={openPrismSite}
            onCheckServer={() => void checkServer(chapter.id)}
          />
          <Panels chapter={chapter} status={status} onOpenWiki={openWiki} />
        </main>
      </div>
      <footer className="text-fg-faint text-2xs pointer-events-none fixed inset-x-0 bottom-2 text-center">
        NOT AN OFFICIAL MINECRAFT PRODUCT. NOT APPROVED BY OR ASSOCIATED WITH MOJANG OR MICROSOFT.
      </footer>
    </div>
  )
}

/**
 * Keeps the open chapter and the saved `lastChapter` in step, in both
 * directions, so the app reopens where it was left. Kept out of the nav so
 * ChapterNav stays a pure view over the chapter store; the settings write is
 * an app-level concern.
 *
 * One component owns both directions on purpose. Split across two effects
 * (restore in App, save here) they each saw the other's stale value and
 * swapped forever, and every save spawned Prism (#6).
 */
function ChapterSelectionSync() {
  const selectedId = useChapterStore((s) => s.selectedId)
  const select = useChapterStore((s) => s.select)
  const loaded = useSettingsStore((s) => s.loaded)
  const last = useSettingsStore((s) => s.settings.lastChapter)
  const update = useSettingsStore((s) => s.update)
  // Whether the saved chapter has been applied yet. A ref, not state: flipping
  // it must not cause a render of its own.
  const restored = useRef(false)

  useEffect(() => {
    // Until settings arrive, `last` is the default and says nothing.
    if (!loaded) return
    // First pass: reopen the saved chapter and save nothing. If it no longer
    // exists, select is a no-op and the next pass saves the default once.
    if (!restored.current) {
      restored.current = true
      if (last) select(last)
      return
    }
    if (selectedId && selectedId !== last) {
      update({ lastChapter: selectedId }).catch((e) => console.warn('save chapter', errMsg(e)))
    }
  }, [loaded, selectedId, last, select, update])
  return null
}
