import { useEffect, useRef, useState } from 'react'
import { errMsg } from '../../lib/ipc'
import { Check, Copy, ExternalLink, KeyRound, UserRound } from '../../lib/icons'
import { profileInitials, profileNameProblem } from '../../lib/profileName'
import { useEngineStore } from '../../stores/useEngineStore'
import { usePlayerStore } from '../../stores/usePlayerStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { OpenExternal } from '../../../wailsjs/go/main/App'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'
import { ErrorLine } from '../ui/Notes'
import { Page } from '../ui/Page'
import { SettingsCard, SettingsSection } from '../ui/SettingsLayout'
import { TextField } from '../ui/TextField'
import { Toggle } from '../ui/Toggle'
import { PlayTimeSection } from './PlayTimeSection'
import { TextLink } from '../ui/TextLink'

/** Mojang's own page for changing a skin; the launcher never touches the account. */
const EDIT_SKIN_URL = 'https://www.minecraft.net/msaprofile/mygames/editskin'
/** How long the copy button shows it worked. */
const COPIED_MS = 2000

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
 *
 * The saved name is also looked up on Mojang's public profile endpoints (issue 193, ADR-13), when
 * the page opens and after a save and never per keystroke: the card's tile becomes the skin's
 * face and gains the UUID, and the field says whether Mojang knows the name. Offline, or for a
 * name Mojang does not know, the tile keeps the initials and an unreachable Mojang says nothing.
 *
 * Play offline (Prism's `--offline`) swaps the profile for a player name
 * of the launcher's own, and the play time section shows what Prism has counted per chapter.
 */
export function AccountPanel({ onClose }: Props) {
  const profile = useSettingsStore((s) => s.settings.profileName)
  const offline = useSettingsStore((s) => s.settings.offline)
  const offlineName = useSettingsStore((s) => s.settings.offlineName)
  const update = useSettingsStore((s) => s.update)
  const loaded = useSettingsStore((s) => s.loaded)
  const openPrism = useEngineStore((s) => s.openPrism)
  const [error, setError] = useState<string | null>(null)
  const [prismError, setPrismError] = useState<string | null>(null)
  const player = usePlayerStore((s) => s.profile)
  const checkPlayer = usePlayerStore((s) => s.check)
  const copyUuid = usePlayerStore((s) => s.copyUuid)
  const clearPlayer = usePlayerStore((s) => s.clear)
  const [cardError, setCardError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const copiedTimer = useRef<number | undefined>(undefined)
  // The face that failed to load, so a broken picture falls back to the initials.
  const [brokenFace, setBrokenFace] = useState<string | null>(null)

  // The saved name changes only on a commit (the field keeps its draft to itself), so this
  // asks Mojang when the page opens and after each save.
  useEffect(() => {
    if (loaded) void checkPlayer(profile.trim())
  }, [loaded, profile, checkPlayer])
  useEffect(
    () => () => {
      window.clearTimeout(copiedTimer.current)
      clearPlayer()
    },
    [clearPlayer],
  )
  const [offlineError, setOfflineError] = useState<string | null>(null)
  // The switch is on but no name exists to save it with yet: Go refuses an empty name, so the
  // field shows first and the pair saves when a name is committed.
  const [offlinePending, setOfflinePending] = useState(false)

  const save = async (profileName: string) => {
    try {
      await update({ profileName })
      setError(null)
    } catch (e) {
      setError(errMsg(e))
    }
  }
  const saveOffline = async (patch: { offline?: boolean; offlineName?: string }) => {
    try {
      await update(patch)
      setOfflineError(null)
      setOfflinePending(false)
    } catch (e) {
      setOfflineError(errMsg(e))
    }
  }
  const onOfflineSwitch = (on: boolean) => {
    if (!on) {
      setOfflinePending(false)
      void saveOffline({ offline: false })
      return
    }
    // The name to start from: the one kept from last time, else the profile name if it is one
    // Minecraft allows.
    const start = offlineName.trim() || (profileNameProblem(profile) ? '' : profile.trim())
    if (start === '') {
      setOfflinePending(true)
      return
    }
    void saveOffline({ offline: true, offlineName: start })
  }
  const onOpenPrism = async () => {
    setPrismError(null)
    try {
      await openPrism()
    } catch (e) {
      setPrismError(errMsg(e))
    }
  }

  const onCopy = async () => {
    setCardError(null)
    try {
      await copyUuid()
      setCopied(true)
      window.clearTimeout(copiedTimer.current)
      copiedTimer.current = window.setTimeout(() => setCopied(false), COPIED_MS)
    } catch (e) {
      setCardError(errMsg(e))
    }
  }
  const onChangeSkin = () => {
    setCardError(null)
    OpenExternal(EDIT_SKIN_URL).catch((e) => setCardError(errMsg(e)))
  }

  const name = profile.trim()
  const found = player?.status === 'found' ? player : null
  const face = found?.faceSrc && found.faceSrc !== brokenFace ? found.faceSrc : null
  const offlineShown = offline || offlinePending
  return (
    <Page
      label="Account"
      eyebrow="Launcher"
      title="Account"
      onBack={onClose}
      ready={loaded}
      settings
    >
      <div className="bg-raised-2 border-line flex items-center gap-5 rounded-lg border p-5">
        {face ? (
          <img
            src={face}
            alt={`${found?.name}'s skin`}
            onError={() => setBrokenFace(face)}
            className="pixelated border-accent-edge size-16 shrink-0 rounded-md border"
          />
        ) : (
          <span className="bg-accent-wash border-accent-edge text-accent font-display flex size-16 shrink-0 items-center justify-center rounded-md border text-2xl font-semibold">
            {profileInitials(name) || 'KK'}
          </span>
        )}
        <div className="flex min-w-0 flex-col gap-1">
          <span className="font-display text-fg truncate text-2xl font-semibold">
            {name || "Prism's default account"}
          </span>
          <span className="text-fg-muted text-xs">
            {offline ? 'Offline' : 'Microsoft account · via Prism'}
          </span>
          {found && (
            <div className="flex items-center gap-1">
              <span className="text-fg-muted truncate font-mono text-xs select-text">
                {found.uuid}
              </span>
              <IconButton
                icon={copied ? Check : Copy}
                title={copied ? 'UUID copied' : 'Copy UUID'}
                size="sm"
                onClick={() => void onCopy()}
              />
            </div>
          )}
          <TextLink size="xs" className="w-fit" onClick={onChangeSkin}>
            Change skin
          </TextLink>
        </div>
      </div>
      {cardError && <ErrorLine>{cardError}</ErrorLine>}

      <SettingsSection title="Profile" icon={UserRound}>
        <SettingsCard>
          <div className="flex flex-col gap-2">
            <TextField
              label="Profile name"
              value={profile}
              placeholder="Prism's default account"
              hint={profileNameProblem(profile)}
              error={error}
              onCommit={(v) => void save(v)}
            />
            {player?.status === 'found' && (
              <p
                role="status"
                className="text-accent m-0 flex items-center gap-1.5 text-xs leading-normal"
              >
                <Icon icon={Check} size="sm" />
                <span>Found on Mojang</span>
              </p>
            )}
            {player?.status === 'not_found' && (
              <ErrorLine>No Minecraft profile has this name</ErrorLine>
            )}
          </div>
          <Toggle
            label="Play offline"
            checked={offlineShown}
            hint={offlineShown ? 'Online servers refuse an offline player.' : undefined}
            error={offlineError}
            onChange={onOfflineSwitch}
          />
          {offlineShown && (
            <TextField
              label="Offline name"
              value={offlineName}
              placeholder={name || 'Your Minecraft name'}
              hint={profileNameProblem(offlineName)}
              onCommit={(v) => void saveOffline({ offlineName: v, offline: true })}
            />
          )}
        </SettingsCard>
      </SettingsSection>

      <PlayTimeSection />

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
