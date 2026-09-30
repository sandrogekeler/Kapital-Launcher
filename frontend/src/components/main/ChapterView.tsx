import type { Chapter } from '../../types'
import { useChapterStore } from '../../stores/useChapterStore'
import {
  selectInstalled,
  selectInstancePack,
  selectInstanceSize,
  selectPackState,
  useEngineStore,
} from '../../stores/useEngineStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { selectStatus, useServerStore } from '../../stores/useServerStore'
import { OpenChapterWiki, OpenExternal, OpenWikiPage } from '../../../wailsjs/go/main/App'
import { errMsg } from '../../lib/ipc'
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
}

export function ChapterView({ chapter, onOpenSettings }: Props) {
  const wikiPick = useChapterStore((s) => s.wikiPick[chapter.id])

  const engine = useEngineStore((s) => s.engine)
  const launching = useEngineStore((s) => s.launching)
  const launchError = useEngineStore((s) => s.error)
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
  const checking = useServerStore((s) => s.checking)
  const checkServer = useServerStore((s) => s.check)

  const devPack = useSettingsStore((s) => s.settings.packOverrides?.[chapter.id])

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
        onOpenWiki={openWiki}
        onOpenSettings={() => onOpenSettings(chapter.id)}
      />
      <ActionBar
        chapter={chapter}
        engine={engine}
        status={status}
        installed={installed}
        devPack={devPack}
        instancePack={instancePack}
        packState={packState}
        launching={launching === chapter.id}
        installing={installing === chapter.id}
        installedNow={installedNow === chapter.id}
        checking={checking === chapter.id}
        error={launchError}
        onPlay={() => void launch(chapter.id)}
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
        sizeBytes={instanceSize}
        packState={packState}
        wikiPage={wikiPick}
        onOpenWiki={openWiki}
      />
    </>
  )
}
