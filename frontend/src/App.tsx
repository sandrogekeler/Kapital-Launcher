import { Suspense, lazy, useCallback, useEffect, useRef, useState } from 'react'
import { HeaderBar } from './components/shell/HeaderBar'
import { Sidebar } from './components/sidebar/Sidebar'
import { ChapterStage } from './components/main/ChapterStage'
import { ChapterSettingsPanel } from './components/settings/ChapterSettingsPanel'
import { SettingsPanel } from './components/settings/SettingsPanel'
import { Scrollable } from './components/ui/Scrollable'
import { selectChapter, useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { useSettingsStore } from './stores/useSettingsStore'
import { useServerStore } from './stores/useServerStore'
import { useGameStore } from './stores/useGameStore'
import { Environment } from '../wailsjs/runtime/runtime'
import { DISCLAIMER } from './lib/disclaimer'
import { errMsg, readOr } from './lib/ipc'

// The run report is opened rarely, from Details beside a game that went wrong,
// so it loads when asked for and stays out of the launcher's first paint and
// its bundle budget (scripts/check-bundle-size.mjs).
const RunReportPanel = lazy(() =>
  import('./components/main/RunReportPanel').then((m) => ({ default: m.RunReportPanel })),
)

export default function App() {
  const chapter = useChapterStore(selectChapter)
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const selectedId = useChapterStore((s) => s.selectedId)
  const loadChapters = useChapterStore((s) => s.load)
  const loadWikiPages = useChapterStore((s) => s.loadWikiPages)

  const engine = useEngineStore((s) => s.engine)
  const loadEngine = useEngineStore((s) => s.load)
  const loadInstances = useEngineStore((s) => s.loadInstances)
  const loadRelease = useEngineStore((s) => s.loadRelease)
  const listenInstall = useEngineStore((s) => s.listenInstall)

  const listenServers = useServerStore((s) => s.listen)
  const checkServer = useServerStore((s) => s.check)

  const listenGame = useGameStore((s) => s.listen)
  const loadGames = useGameStore((s) => s.load)

  const theme = useSettingsStore((s) => s.settings.theme)
  const loadSettings = useSettingsStore((s) => s.load)

  // Which OS draws the window, for the header's controls. Without a bridge
  // (the browser-only preview) it shows the Windows bar, the primary target.
  const [platform, setPlatform] = useState('windows')
  useEffect(() => {
    void readOr(Environment, null).then((env) => env && setPlatform(env.platform))
  }, [])

  // The settings screen takes the main column while open (#5). The gear
  // toggles it, Back and Escape close it, and so does picking a chapter: the
  // effect fires on the selection, including the restore at startup, when the
  // panel is closed anyway.
  const [settingsOpen, setSettingsOpen] = useState(false)
  const closeSettings = useCallback(() => setSettingsOpen(false), [])
  useEffect(() => setSettingsOpen(false), [selectedId])
  // A chapter's own settings (#36) take the column the same way, opened from
  // the pen in its hero, and close with the chapter they belong to.
  const [chapterSettingsFor, setChapterSettingsFor] = useState<string | null>(null)
  const closeChapterSettings = useCallback(() => setChapterSettingsFor(null), [])
  useEffect(() => setChapterSettingsFor(null), [selectedId])
  // A run's report (Details, beside a game that ended badly) takes it likewise.
  const [reportFor, setReportFor] = useState<string | null>(null)
  const closeReport = useCallback(() => setReportFor(null), [])
  useEffect(() => setReportFor(null), [selectedId])

  // One read per store on mount. These are reads of state Go holds, not
  // events, so an effect is the right tool.
  useEffect(() => {
    void loadChapters()
    void loadEngine()
    void loadSettings()
    void loadWikiPages()
  }, [loadChapters, loadEngine, loadSettings, loadWikiPages])

  // An instance appears when the user imports one in Prism, which happens in
  // another window. Coming back is the moment to look again; a focus event,
  // not a timer, so nothing polls.
  useEffect(() => {
    const onFocus = () => void loadInstances()
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [loadInstances])

  // Prism install progress arrives as events too; one listener for the app.
  useEffect(() => listenInstall(), [listenInstall])

  // Prism's latest release matters in two cases: there is no Prism, so the
  // launcher can offer to get it; or the launcher's own copy is the one in
  // use, so it can offer an update (ADR-11). Checked when the engine is
  // known, never on a timer.
  const engineFound = engine?.found
  const engineSource = engine?.source
  useEffect(() => {
    if (engineFound === false || engineSource === 'managed') void loadRelease()
  }, [engineFound, engineSource, loadRelease])

  // Server status arrives as events from Go's ticker (.claude/rules/ipc.md);
  // this is the one listener, for the app's lifetime.
  useEffect(() => listenServers(), [listenServers])

  // A game's phases arrive as events too (#44). The read covers a window that
  // opens after a game began, and runs after the listener so no phase is lost
  // between them.
  useEffect(() => {
    const off = listenGame()
    void loadGames()
    return off
  }, [listenGame, loadGames])

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

  return (
    <div className="bg-canvas flex h-full flex-col">
      <ChapterSelectionSync />
      <HeaderBar platform={platform} onOpenSettings={() => setSettingsOpen((open) => !open)} />
      <div className="flex min-h-0 grow">
        <Sidebar />
        {/* The chapter card scrolls under the header bar and beside the
            sidebar when the window is shorter than it (#56); the card's
            margin keeps it clear of the fixed disclaimer. */}
        <Scrollable as="main" className="flex flex-col">
          {settingsOpen ? (
            <SettingsPanel onClose={closeSettings} />
          ) : chapterSettingsFor === chapter.id ? (
            <ChapterSettingsPanel chapter={chapter} onClose={closeChapterSettings} />
          ) : reportFor === chapter.id ? (
            <Suspense fallback={null}>
              <RunReportPanel chapter={chapter} onClose={closeReport} />
            </Suspense>
          ) : (
            <ChapterStage
              chapter={chapter}
              chapters={chapters}
              onOpenSettings={setChapterSettingsFor}
              onOpenReport={setReportFor}
            />
          )}
        </Scrollable>
      </div>
      {/* Centred on the main column, beside the sidebar, not on the window. */}
      <footer className="text-fg-faint text-2xs pointer-events-none fixed right-0 bottom-2 left-(--layout-sidebar) text-center">
        {DISCLAIMER}
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
