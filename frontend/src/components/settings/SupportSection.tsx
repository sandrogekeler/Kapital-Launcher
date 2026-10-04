import { useState } from 'react'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { errMsg } from '../../lib/ipc'
import { CopyLogButton } from '../ui/CopyLogButton'
import { LifeBuoy } from '../../lib/icons'
import { SettingsCard, SettingsSection } from '../ui/SettingsLayout'
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
    <SettingsSection title="Support" icon={LifeBuoy}>
      <SettingsCard>
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
          <span className="text-fg text-sm font-medium">Log for a bug report</span>
          <CopyLogButton onClick={() => void copy()} result={result} disabled={busy} />
        </div>
      </SettingsCard>
    </SettingsSection>
  )
}
