import { useState } from 'react'
import { errMsg } from '../../lib/ipc'
import { ExternalLink, KeyRound, UserRound } from '../../lib/icons'
import { profileInitials, profileNameProblem } from '../../lib/profileName'
import { useEngineStore } from '../../stores/useEngineStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { ErrorLine } from '../ui/Notes'
import { Page } from '../ui/Page'
import { SettingsCard, SettingsSection } from '../ui/SettingsLayout'
import { TextField } from '../ui/TextField'

interface Props {
  onClose: () => void
}

/**
 * The account page (issue 192), opened from the tile at the foot of the sidebar. Prism owns the
 * sign-in (CLAUDE.md): this page holds the one account value the launcher keeps, the profile name
 * it passes to `--profile`, and a way into Prism's own window, where accounts are added and
 * switched. It reads nothing of Prism's accounts, so it cannot list them or say which is signed
 * in. The name saves on Enter or blur, as on the settings screen, and a Go refusal shows under it
 * with the old value back.
 */
export function AccountPanel({ onClose }: Props) {
  const profile = useSettingsStore((s) => s.settings.profileName)
  const update = useSettingsStore((s) => s.update)
  const loaded = useSettingsStore((s) => s.loaded)
  const openPrism = useEngineStore((s) => s.openPrism)
  const [error, setError] = useState<string | null>(null)
  const [prismError, setPrismError] = useState<string | null>(null)

  const save = async (profileName: string) => {
    try {
      await update({ profileName })
      setError(null)
    } catch (e) {
      setError(errMsg(e))
    }
  }
  const onOpenPrism = async () => {
    setPrismError(null)
    try {
      await openPrism()
    } catch (e) {
      setPrismError(errMsg(e))
    }
  }

  const name = profile.trim()
  return (
    <Page label="Account" title="Account" onBack={onClose} ready={loaded} settings>
      <div className="bg-raised-2 border-line mb-10 flex items-center gap-5 rounded-lg border p-5">
        <span className="bg-accent-wash border-accent-edge text-accent font-display flex size-16 shrink-0 items-center justify-center rounded-md border text-2xl font-semibold">
          {profileInitials(name) || 'KK'}
        </span>
        <div className="flex min-w-0 flex-col gap-1">
          <span className="font-display text-fg truncate text-2xl font-semibold">
            {name || "Prism's default account"}
          </span>
          <span className="text-fg-muted text-xs">Microsoft account · via Prism</span>
        </div>
      </div>

      <SettingsSection title="Profile" icon={UserRound}>
        <SettingsCard>
          <TextField
            label="Profile name"
            value={profile}
            placeholder="Prism's default account"
            hint={profileNameProblem(profile)}
            error={error}
            onCommit={(v) => void save(v)}
          />
        </SettingsCard>
      </SettingsSection>

      <SettingsSection title="Sign-in" icon={KeyRound}>
        <SettingsCard>
          <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
            <span className="text-fg text-sm font-medium">Add or switch accounts</span>
            <Button onClick={() => void onOpenPrism()}>
              <span>Open Prism</span>
              <Icon icon={ExternalLink} size="sm" className="text-fg-faint" />
            </Button>
          </div>
        </SettingsCard>
        {prismError && <ErrorLine>{prismError}</ErrorLine>}
        <p className="text-fg-muted m-0 text-xs leading-normal">
          Microsoft sign-in happens in Prism. Kapital Launcher only passes the profile name on and
          never sees the account.
        </p>
      </SettingsSection>
    </Page>
  )
}
