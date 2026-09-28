import { useEngineStore } from '../../stores/useEngineStore'
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
  const profile = useSettingsStore((s) => s.settings.profileName)

  const name = profile || 'Default profile'
  const initials = name
    .split(/[\s_-]+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
    .toUpperCase()

  const engineLine =
    engine === null
      ? 'not checked'
      : !engine.found
        ? 'not found'
        : `Prism ${engine.version || ''}${engine.version ? ' · ' : ''}linked`

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
    </div>
  )
}
