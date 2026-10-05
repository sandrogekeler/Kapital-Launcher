import { lazy, useEffect, useRef, useState } from 'react'
import { HeaderBar } from './components/shell/HeaderBar'
import { Toasts } from './components/shell/Toasts'
import { Sidebar } from './components/sidebar/Sidebar'
import { ChapterStage } from './components/main/ChapterStage'
import { PageLayer } from './components/main/PageLayer'
import { Scrollable } from './components/ui/Scrollable'
import { selectCanRotate, selectChapter, useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { useSettingsStore } from './stores/useSettingsStore'
import { useServerStore } from './stores/useServerStore'
import { isActive, useGameStore } from './stores/useGameStore'
import { Environment } from '../wailsjs/runtime/runtime'
import { errMsg, readOr } from './lib/ipc'
import { SLIDE_INTERVAL_MS } from './lib/slides'
import { usePages } from './lib/usePages'
import { heroArtOf } from './lib/heroArt'

// The pages that slide over the chapter card are opened on demand, so they
// load when asked for and stay out of the launcher's first paint and its
// bundle budget (scripts/check-bundle-size.mjs): the settings, and a chapter's
// own pages (components/main/ChapterPage), its settings, its logs, its map and
// the run report, opened rarely, from Details beside a game that went wrong.
const SettingsPanel = lazy(() =>
  import('./components/settings/SettingsPanel').then((m) => ({ default: m.SettingsPanel })),
)
const AccountPanel = lazy(() =>
  import('./components/settings/AccountPanel').then((m) => ({ default: m.AccountPanel })),
)
const ChapterPage = lazy(() =>
  import('./components/main/ChapterPage').then((m) => ({ default: m.ChapterPage })),
)

export default function App() {
  const chapter = useChapterStore(selectChapter)
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const selectedId = useChapterStore((s) => s.selectedId)
  const select = useChapterStore((s) => s.select)
  const loadChapters = useChapterStore((s) => s.load)
  const loadWikiPages = useChapterStore((s) => s.loadWikiPages)
  const loadWikiShots = useChapterStore((s) => s.loadWikiShots)
  const advanceSlide = useChapterStore((s) => s.advance)
  const canRotate = useChapterStore(selectCanRotate)
  const loadPanoramas = useChapterStore((s) => s.loadPanoramas)
  const heroArt = useSettingsStore((s) => heroArtOf(s.settings))

  const engine = useEngineStore((s) => s.engine)
  const loadEngine = useEngineStore((s) => s.load)
  const loadInstances = useEngineStore((s) => s.loadInstances)
  const loadRelease = useEngineStore((s) => s.loadRelease)
  const installedNow = useEngineStore((s) => s.installedNow)
  const listenInstall = useEngineStore((s) => s.listenInstall)

  const listenServers = useServerStore((s) => s.listen)
  const checkServer = useServerStore((s) => s.check)

  const listenGame = useGameStore((s) => s.listen)
  const loadGames = useGameStore((s) => s.load)
  const gameActive = useGameStore((s) => Object.values(s.states).some((g) => isActive(g.phase)))

  const theme = useSettingsStore((s) => s.settings.theme)
  const loadSettings = useSettingsStore((s) => s.load)

  // Which OS draws the window, for the header's controls. Without a bridge
  // (the browser-only preview) it shows the Windows bar, the primary target.
  const [platform, setPlatform] = useState('windows')
  useEffect(() => {
    void readOr(Environment, null).then((env) => env && setPlatform(env.platform))
  }, [])

  // The pages over the chapter card: the settings from the gear, a chapter's
  // own from its hero and from the corner's notices (lib/usePages).
  const pages = usePages(selectedId, select)
  const { settings, account, chapterPage } = pages
  const openedChapter =
    chapterPage.slot && chapters.find((c) => c.id === chapterPage.slot?.page.chapterId)

  // One read per store on mount. These are reads of state Go holds, not
  // events, so an effect is the right tool.
  useEffect(() => {
    void loadChapters()
    void loadEngine()
    void loadSettings()
    void loadWikiPages()
    void loadWikiShots()
  }, [loadChapters, loadEngine, loadSettings, loadWikiPages, loadWikiShots])

  // The open chapter's slide moves on every minute (issue 142): a timer for
  // what is on screen, not a poll of anything Go holds. It runs only while the
  // window is in view, starts over on every switch, and not at all unless the
  // slideshow is what the hero shows, or with nothing to move on to.
  const slideshow = heroArt === 'slideshow'
  useEffect(() => {
    if (!slideshow || !canRotate) return
    let timer: number | undefined
    const stop = () => {
      window.clearInterval(timer)
      timer = undefined
    }
    const start = () => {
      stop()
      if (document.visibilityState === 'visible')
        timer = window.setInterval(() => void advanceSlide(), SLIDE_INTERVAL_MS)
    }
    start()
    document.addEventListener('visibilitychange', start)
    return () => {
      stop()
      document.removeEventListener('visibilitychange', start)
    }
  }, [slideshow, canRotate, selectedId, advanceSlide])

  // An instance appears when the user imports one in Prism, which happens in
  // another window. Coming back is the moment to look again; a focus event,
  // not a timer, so nothing polls.
  useEffect(() => {
    const onFocus = () => void loadInstances()
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [loadInstances])

  // The panoramas are the resources packs in the instances, which change in
  // other windows (Prism, the game) and when an install finishes. They are read
  // only while the panorama is the hero's picture: when it is chosen, when the
  // window is focused again, and when an install or a game run has ended (a game
  // running leaves them alone). A focus event or one of those, never a timer.
  const panoramaChosen = heroArt === 'panorama'
  useEffect(() => {
    if (!panoramaChosen || gameActive) return
    void loadPanoramas()
    const onFocus = () => void loadPanoramas()
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [panoramaChosen, gameActive, installedNow, loadPanoramas])

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
      <HeaderBar platform={platform} onOpenSettings={pages.toggleSettings} />
      <div className="flex min-h-0 grow">
        <Sidebar onOpenAccount={pages.toggleAccount} accountOpen={account.slot?.open === true} />
        {/* The chapter card scrolls under the header bar and beside the
            sidebar when the window is shorter than it (issue 56). A page slides
            over the card in the same stage, and scrolls inside its own frame. */}
        <Scrollable as="main" className="flex flex-col">
          <ChapterStage
            chapter={chapter}
            chapters={chapters}
            onOpenSettings={pages.openChapterSettings}
            onOpenLogs={pages.openLogs}
            onOpenMap={pages.openMap}
            covered={pages.covered}
          >
            {chapterPage.slot && openedChapter && (
              <PageLayer
                key={`${chapterPage.slot.page.kind}:${openedChapter.id}`}
                edge="right"
                open={chapterPage.slot.open}
                onExited={chapterPage.done}
              >
                <ChapterPage
                  page={chapterPage.slot.page}
                  chapter={openedChapter}
                  onClose={chapterPage.hide}
                />
              </PageLayer>
            )}
            {settings.slot && (
              <PageLayer edge="top" open={settings.slot.open} onExited={settings.done}>
                <SettingsPanel onClose={settings.hide} onShowChapter={pages.showChapter} />
              </PageLayer>
            )}
            {account.slot && (
              <PageLayer edge="bottom" open={account.slot.open} onExited={account.done}>
                <AccountPanel onClose={account.hide} />
              </PageLayer>
            )}
          </ChapterStage>
        </Scrollable>
      </div>
      <Toasts onOpenReport={pages.openReport} />
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
