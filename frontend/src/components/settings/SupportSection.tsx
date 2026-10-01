import { useState } from 'react'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { errMsg } from '../../lib/ipc'
import { copyLogDone } from '../../lib/settingsView'
import { Copy } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'

type Outcome = { kind: 'done'; lines: number } | { kind: 'failed'; message: string }

/**
 * The Support section of the settings screen (#84): one button that puts the
 * end of the launcher's log on the clipboard for a bug report. Go reads the
 * log and masks the player's name, folders and server addresses before it
 * copies, so the line here says how much went and that it was masked; a
 * failure shows in place of that line.
 */
export function SupportSection() {
  const copyLog = useSettingsStore((s) => s.copyLog)
  const [busy, setBusy] = useState(false)
  const [outcome, setOutcome] = useState<Outcome | null>(null)

  const copy = async () => {
    setBusy(true)
    try {
      setOutcome({ kind: 'done', lines: await copyLog() })
    } catch (e) {
      setOutcome({ kind: 'failed', message: errMsg(e) })
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <h2 className="text-fg-faint m-0 text-xs font-medium">Support</h2>
      <p className="text-fg-faint m-0 text-xs leading-normal">
        Reporting a bug? Copy the end of the launcher's log to paste into the report. Your name,
        folders and server addresses are masked first, and nothing is saved to disk.
      </p>
      <div className="flex items-center gap-4">
        <Button onClick={() => void copy()} disabled={busy}>
          <Icon icon={Copy} size="sm" />
          <span>Copy log</span>
        </Button>
        <span role="status" className="text-xs select-text">
          {outcome?.kind === 'done' && (
            <span className="text-fg-muted">{copyLogDone(outcome.lines)}</span>
          )}
          {outcome?.kind === 'failed' && <span className="text-danger">{outcome.message}</span>}
        </span>
      </div>
    </div>
  )
}
