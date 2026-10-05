import { useEngineStore } from '../../stores/useEngineStore'
import { installLine } from '../../lib/prismInstall'
import { profileInitials } from '../../lib/profileName'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { IconButton } from '../ui/IconButton'
import { ErrorLine } from '../ui/Notes'
import { TextLink } from '../ui/TextLink'
import { ChevronRight, RefreshCw } from '../../lib/icons'
import { Icon } from '../ui/Icon'

interface Props {
  /** Opens or closes the account page (issue 192); without it the account half is plain text. */
  onOpenAccount?: () => void
  /** The account page is open: the card takes the accent, as a chosen chapter does. */
  accountOpen?: boolean
}

/**
 * The account and engine card at the foot of the sidebar. The account line
 * shows the profile *name* from settings and nothing more: the Microsoft
 * account itself lives inside Prism and this app never reads it. The account
 * half is a button that opens the account page (issue 192); Re-detect stays a
 * button of its own beside it.
 */
export function EngineCard({ onOpenAccount, accountOpen = false }: Props) {
  const engine = useEngineStore((s) => s.engine)
  const refresh = useEngineStore((s) => s.refresh)
  const release = useEngineStore((s) => s.release)
  const install = useEngineStore((s) => s.install)
  const installPrism = useEngineStore((s) => s.installPrism)
  const profile = useSettingsStore((s) => s.settings.profileName)
  const offline = useSettingsStore((s) => s.settings.offline)

  const name = profile || 'Default profile'
  const initials = profileInitials(name)
  const accountLine = offline ? 'Offline' : 'Microsoft account · via Prism'

  // An update runs while Prism is found; the first install is the action bar's.
  const updateLine =
    engine?.found && install && ['downloading', 'unpacking', 'verifying'].includes(install.phase)
      ? installLine(install, 'Updating')[0].replace(/^◐ /, '')
      : null
  const updateFailed = engine?.found && install?.phase === 'failed' ? install.error : null
  const how = engine?.source === 'managed' ? 'managed' : 'linked'
  const engineLine =
    engine === null
      ? 'not checked'
      : !engine.found
        ? 'not found'
        : (updateLine ?? `Prism ${engine.version || ''}${engine.version ? ' · ' : ''}${how}`)
  // GetPrismRelease only offers an update for the managed Prism, in use.
  const offer = engine?.found && release?.updateAvailable && !updateLine ? release : null

  return (
    <div
      className={`duration-fast ease-standard flex flex-col gap-2.5 rounded-lg border p-3 transition-colors ${
        accountOpen ? 'bg-accent-wash border-accent-edge' : 'bg-raised border-line'
      }`}
    >
      <div className="flex items-center gap-1.5">
        <button
          type="button"
          aria-label={`Account, ${name}`}
          aria-pressed={accountOpen}
          disabled={!onOpenAccount}
          onClick={() => onOpenAccount?.()}
          className="hover:bg-hover duration-fast ease-standard -m-1.5 flex min-w-0 grow cursor-pointer items-center gap-2.5 rounded-md p-1.5 text-left transition-colors disabled:cursor-default disabled:hover:bg-transparent"
        >
          <span className="bg-hover flex size-8 shrink-0 items-center justify-center rounded-sm font-mono text-xs">
            {initials || 'KK'}
          </span>
          <span className="flex min-w-0 grow flex-col gap-0.5">
            <span className="truncate text-sm font-semibold">{name}</span>
            <span className="text-fg-faint text-2xs truncate" title={accountLine}>
              {accountLine}
            </span>
          </span>
          {onOpenAccount && <Icon icon={ChevronRight} size="sm" className="text-fg-faint" />}
        </button>
        <IconButton
          icon={RefreshCw}
          title="Re-detect Prism"
          size="sm"
          onClick={() => void refresh()}
        />
      </div>
      <div className="bg-line h-px" />
      <div className="text-fg-faint text-2xs flex justify-between gap-2 whitespace-nowrap">
        <span>engine</span>
        <span
          className={`min-w-0 truncate font-mono ${
            engine !== null && !engine.found ? 'text-danger' : 'text-fg-muted'
          }`}
          title={engineLine}
        >
          {engineLine}
        </span>
      </div>
      {offer && (
        <TextLink size="xs" className="self-start" onClick={() => void installPrism()}>
          Update Prism to {offer.version}
        </TextLink>
      )}
      {updateFailed && <ErrorLine>Update failed: {updateFailed}</ErrorLine>}
    </div>
  )
}
