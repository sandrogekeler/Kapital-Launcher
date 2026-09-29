import { useEngineStore } from '../../stores/useEngineStore'
import { installLine } from '../../lib/prismInstall'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { IconButton } from '../ui/IconButton'
import { RefreshCw } from '../../lib/icons'

/**
 * The account and engine card at the foot of the sidebar. The account line
 * shows the profile *name* from settings and nothing more: the Microsoft
 * account itself lives inside Prism and this app never reads it.
 */
export function EngineCard() {
  const engine = useEngineStore((s) => s.engine)
  const refresh = useEngineStore((s) => s.refresh)
  const release = useEngineStore((s) => s.release)
  const install = useEngineStore((s) => s.install)
  const installPrism = useEngineStore((s) => s.installPrism)
  const profile = useSettingsStore((s) => s.settings.profileName)

  const name = profile || 'Default profile'
  const initials = name
    .split(/[\s_-]+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
    .toUpperCase()

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
    <div className="bg-raised border-line flex flex-col gap-2.5 rounded-lg border p-3">
      <div className="flex items-center gap-2.5">
        <div className="bg-hover flex size-8 items-center justify-center rounded-sm font-mono text-xs">
          {initials || 'KK'}
        </div>
        <div className="flex min-w-0 grow flex-col gap-0.5">
          <span className="truncate text-sm font-semibold">{name}</span>
          <span className="text-fg-faint text-2xs">Microsoft account · via Prism</span>
        </div>
        <IconButton
          icon={RefreshCw}
          title="Re-detect Prism"
          size="sm"
          onClick={() => void refresh()}
        />
      </div>
      <div className="bg-line h-px" />
      <div className="text-fg-faint text-2xs flex justify-between gap-2 font-mono whitespace-nowrap">
        <span>engine</span>
        <span className={engine !== null && !engine.found ? 'text-danger' : 'text-fg-muted'}>
          {engineLine}
        </span>
      </div>
      {offer && (
        <button
          type="button"
          onClick={() => void installPrism()}
          className="text-accent text-2xs cursor-pointer self-start font-mono underline-offset-2 hover:underline"
        >
          Update Prism to {offer.version}
        </button>
      )}
      {updateFailed && (
        <span className="text-danger text-2xs select-text">Update failed: {updateFailed}</span>
      )}
    </div>
  )
}
