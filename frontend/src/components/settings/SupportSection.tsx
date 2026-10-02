import { useState } from 'react'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { errMsg } from '../../lib/ipc'
import { CopyLogButton } from '../ui/CopyLogButton'
import type { CopyResult } from '../ui/CopyLogButton'

/**
 * The Support section of the settings screen (#84): one button that puts the
 * end of the launcher's log on the clipboard for a bug report. Go reads the
 * log and masks the player's name, folders and server addresses before it
 * copies, which the paragraph says; the button says how it went, Copied or
 * Copy failed, and goes back to itself.
 */
export function SupportSection() {
  const copyLog = useSettingsStore((s) => s.copyLog)
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<CopyResult | null>(null)

  const copy = async () => {
    setBusy(true)
    try {
      await copyLog()
      setResult({})
    } catch (e) {
      setResult({ error: errMsg(e) })
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
        <CopyLogButton onClick={() => void copy()} result={result} disabled={busy} />
      </div>
    </div>
  )
}
