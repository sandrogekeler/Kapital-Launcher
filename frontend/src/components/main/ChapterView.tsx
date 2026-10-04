import type { Chapter } from '../../types'
import { selectSlide, useChapterStore } from '../../stores/useChapterStore'
import {
  selectInstalled,
  selectInstancePack,
  selectInstanceSize,
  selectPackState,
  useEngineStore,
} from '../../stores/useEngineStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { selectStatus, useServerStore } from '../../stores/useServerStore'
import { selectGame, useGameStore } from '../../stores/useGameStore'
import { OpenChapterWiki, OpenExternal, OpenWikiPage } from '../../../wailsjs/go/main/App'
import { errMsg } from '../../lib/ipc'
import { packVersion } from '../../lib/packState'
import { Hero } from './Hero'
import { ActionBar } from './ActionBar'
import { Panels } from './Panels'

const PRISM_SITE = 'https://prismlauncher.org'

/**
 * One chapter's view: the hero, the action bar and the panels, reading the
 * chapter's own slice of each store. It takes the chapter rather than the
 * selection so the card that is leaving can keep showing the chapter it
 * showed while the new one slides in (#59).
 */
interface Props {
  chapter: Chapter
  onOpenSettings: (chapterId: string) => void
  onOpenLogs: (chapterId: string) => void
}

export function ChapterView({ chapter, onOpenSettings, onOpenLogs }: Props) {
  const slide = useChapterStore(selectSlide(chapter.id))
  const wikiPick = slide?.page
  // The slideshow off shows the bundled art; the post still follows the slide.
  const staticArt = useSettingsStore((s) => s.settings.staticArt ?? false)

  const engine = useEngineStore((s) => s.engine)
  const launching = useEngineStore((s) => s.launching)
  const release = useEngineStore((s) => s.release)
  const install = useEngineStore((s) => s.install)
  const installPrism = useEngineStore((s) => s.installPrism)
  const installed = useEngineStore(selectInstalled(chapter.id))
  const instancePack = useEngineStore(selectInstancePack(chapter.id))
  const instanceSize = useEngineStore(selectInstanceSize(chapter.id))
  const packState = useEngineStore(selectPackState(chapter.id))
  const launch = useEngineStore((s) => s.launch)
  const installing = useEngineStore((s) => s.installing)
  const installedNow = useEngineStore((s) => s.installedNow)
  const installChapter = useEngineStore((s) => s.installChapter)

  const status = useServerStore(selectStatus(chapter.id))
  const game = useGameStore(selectGame(chapter.id))
  const stopGame = useGameStore((s) => s.stop)
  const checking = useServerStore((s) => s.checking)
  const checkServer = useServerStore((s) => s.check)

  const devPack = useSettingsStore((s) => s.settings.packOverrides?.[chapter.id])
  const serverChoice = useSettingsStore((s) => s.settings.serverChoices?.[chapter.id])

  // The page the panel shows is the one that opens: the pick, or the
  // manifest's teaser when there is none.
  const openWiki = () =>
    (wikiPick ? OpenWikiPage(wikiPick.url) : OpenChapterWiki(chapter.id)).catch((e) =>
      console.warn('open wiki', errMsg(e)),
    )
  const openPrismSite = () =>
    OpenExternal(PRISM_SITE).catch((e) => console.warn('open prism site', errMsg(e)))
  const openReleasePage = () =>
    OpenExternal(release?.page || PRISM_SITE).catch((e) =>
      console.warn('open prism release', errMsg(e)),
    )

  return (
    <>
      <Hero
        chapter={chapter}
        art={staticArt ? undefined : slide?.art}
        onOpenWiki={openWiki}
        onOpenSettings={() => onOpenSettings(chapter.id)}
        onOpenLogs={() => onOpenLogs(chapter.id)}
        version={packVersion(packState, chapter.pack.version)}
      />
      <ActionBar
        chapter={chapter}
        engine={engine}
        status={status}
        installed={installed}
        devPack={devPack}
        serverChoice={serverChoice}
        instancePack={instancePack}
        packState={packState}
        game={game}
        launching={launching === chapter.id}
        installing={installing === chapter.id}
        installedNow={installedNow === chapter.id}
        checking={checking === chapter.id}
        onPlay={() => void launch(chapter.id)}
        // The store has recorded a refusal for the corner's notice.
        onStop={() => void stopGame(chapter.id).catch((e) => console.warn('stop game', errMsg(e)))}
        onInstall={() => void installChapter(chapter.id)}
        release={release}
        install={install}
        onGetPrism={() => void installPrism()}
        onOpenPrismSite={openPrismSite}
        onOpenReleasePage={openReleasePage}
        onCheckServer={() => void checkServer(chapter.id)}
      />
      <Panels
        chapter={chapter}
        installed={installed}
        sizeBytes={instanceSize}
        packState={packState}
        wikiPage={wikiPick}
        onOpenWiki={openWiki}
      />
    </>
  )
}
