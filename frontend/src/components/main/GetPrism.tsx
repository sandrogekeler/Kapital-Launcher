import type { PrismRelease } from '../../types'
import { megabytes } from '../../lib/prismInstall'
import { Download } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'

interface Props {
  release: PrismRelease
  onConfirm: () => void
  onCancel: () => void
  onOpenReleasePage: () => void
  onOpenPrismSite: () => void
}

/**
 * The approval card for getting Prism (ADR-11). It says what is downloaded,
 * how big it is and where from, before anything is fetched; nothing happens
 * until the player confirms.
 */
export function GetPrism({
  release,
  onConfirm,
  onCancel,
  onOpenReleasePage,
  onOpenPrismSite,
}: Props) {
  const link = 'text-accent cursor-pointer underline-offset-2 hover:underline'
  return (
    <div
      role="region"
      aria-label="Get Prism Launcher"
      className="bg-raised border-line flex max-w-160 flex-col gap-3 rounded-lg border p-4 text-sm"
    >
      <p className="m-0 leading-normal">
        Kapital Launcher can download Prism Launcher {release.version} ({megabytes(release.size)})
        from Prism&apos;s releases on GitHub and set it up for you. Prism is free and open source
        (GPL-3.0), and this copy stays separate from any Prism you install yourself.
      </p>
      <p className="text-fg-muted m-0 leading-normal">
        The first time you play, Prism asks you to sign in with Microsoft. Your account stays with
        Prism; Kapital Launcher never sees it.
      </p>
      <div className="flex flex-wrap items-center gap-3">
        <Button onClick={onConfirm}>
          <Icon icon={Download} size="sm" />
          <span>Download and set up</span>
        </Button>
        <Button onClick={onCancel}>Not now</Button>
        <button type="button" className={link} onClick={onOpenReleasePage}>
          What&apos;s in {release.version}
        </button>
        <button type="button" className={link} onClick={onOpenPrismSite}>
          Prism&apos;s website
        </button>
      </div>
    </div>
  )
}
