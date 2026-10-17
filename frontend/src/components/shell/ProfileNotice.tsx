import { useState } from 'react'
import { errMsg } from '../../lib/ipc'
import { Play, UserRound } from '../../lib/icons'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { Button } from '../ui/Button'
import { Dialog } from '../ui/Dialog'
import { Icon } from '../ui/Icon'

interface Props {
  /** Opens the account page, where the profile name is changed. */
  onOpenAccount: () => void
}

/**
 * The popup for a Play that Prism ignored because none of its accounts has the profile name
 * the launcher passed (`reason: 'profile'`). Prism then starts nothing and says nothing, so the
 * popup says it and offers the two ways out: clear the name and play again with Prism's default
 * account, or open the account page to correct it. The launcher still reads none of Prism's
 * accounts (CLAUDE.md): it cannot say which names Prism has.
 *
 * Derived from the game store like the corner's notices, and in place of the corner's notice for
 * such a run: it shows for the newest run that ended this way and stays away once put away,
 * until a later run ends the same way.
 */
export function ProfileNotice({ onOpenAccount }: Props) {
  const games = useGameStore((s) => s.states)
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const profile = useSettingsStore((s) => s.settings.profileName)
  const update = useSettingsStore((s) => s.update)
  const launch = useEngineStore((s) => s.launch)
  // The run each popup was put away for: its chapter and when it failed.
  const [dismissed, setDismissed] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const failed = Object.values(games)
    .filter((g) => g.phase === 'failed' && g.reason === 'profile')
    .sort((a, b) => b.since.localeCompare(a.since))[0]
  const stamp = failed ? `${failed.chapterId}:${failed.since}` : null
  // A name cleared since has nothing left to correct.
  if (!failed || stamp === dismissed || profile.trim() === '') return null

  const chapter = chapters.find((c) => c.id === failed.chapterId)?.name ?? failed.chapterId
  const close = () => {
    setDismissed(stamp)
    setError(null)
  }
  const playWithDefault = async () => {
    try {
      await update({ profileName: '' })
    } catch (e) {
      setError(errMsg(e))
      return
    }
    close()
    await launch(failed.chapterId)
  }
  const openAccount = () => {
    close()
    onOpenAccount()
  }

  return (
    <Dialog title="Prism has no account by that name" facts={chapter} onCancel={close} closable>
      <p className="text-fg-soft m-0 leading-normal">
        None of the accounts in Prism is called <span className="text-fg">{profile.trim()}</span>,
        so the game did not start. Play with Prism&apos;s default account instead, or correct the
        name on the account page.
      </p>
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="play" onClick={() => void playWithDefault()}>
          <Icon icon={Play} size="sm" />
          <span>Play with Prism&apos;s default account</span>
        </Button>
        <Button onClick={openAccount}>
          <Icon icon={UserRound} size="sm" />
          <span>Open account settings</span>
        </Button>
      </div>
      {error && <p className="text-danger m-0 text-xs">{error}</p>}
    </Dialog>
  )
}
