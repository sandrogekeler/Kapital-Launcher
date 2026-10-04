import { useEffect, useId, useState } from 'react'
import { GetAppVersion, OpenExternal } from '../../../wailsjs/go/main/App'
import { DISCLAIMER } from '../../lib/disclaimer'
import { errMsg, readOr } from '../../lib/ipc'
import { CREDITS, PRISM_SOURCE } from '../../lib/licences'
import type { Credit } from '../../lib/licences'
import { Info } from '../../lib/icons'
import { SettingsCard, SettingsSection } from '../ui/SettingsLayout'
import { TextLink } from '../ui/TextLink'

/**
 * What the app is made of and what it owes (#87): its version, the disclaimer
 * the Minecraft guidelines require, and a credit with the full licence text for
 * the fonts and icons it ships. Prism is credited too, but it is not part of
 * this app: the launcher only downloads Prism's own release when a player asks
 * (ADR-11), so what is shown for it is a source link, not a licence text.
 */
export function AboutSection() {
  const [version, setVersion] = useState<string | null>(null)
  useEffect(() => {
    void readOr(GetAppVersion, null).then(setVersion)
  }, [])

  return (
    <SettingsSection
      title="About"
      icon={Info}
      aside={
        <span className="flex gap-1.5">
          Version
          {version ? <span className="text-fg font-mono">{version}</span> : <span>Unknown</span>}
        </span>
      }
    >
      <SettingsCard label="Credits">
        {CREDITS.map((c) => (
          <CreditRow key={c.id} credit={c} />
        ))}
        <div className="flex flex-col gap-2">
          <span className="text-fg text-sm font-medium">Prism Launcher</span>
          <p className="text-fg-muted m-0 text-xs leading-normal select-text">
            Prism Launcher is free software under the GPL-3.0 and is not part of Kapital Launcher.
            When you ask for it, Kapital Launcher downloads an unmodified official release from
            Prism's GitHub releases. Its source is at github.com/PrismLauncher/PrismLauncher.
          </p>
          <LinkButton url={PRISM_SOURCE}>Prism Launcher source</LinkButton>
        </div>
      </SettingsCard>
      <p className="text-fg-muted m-0 text-xs leading-normal select-text">{DISCLAIMER}</p>
    </SettingsSection>
  )
}

/**
 * One credit. Its licence text opens in place and is fetched on the first
 * open, so closed rows cost nothing and the text sits in its own chunk.
 */
function CreditRow({ credit }: { credit: Credit }) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const panelId = useId()

  const toggle = () => {
    const next = !open
    setOpen(next)
    if (next && text === null) {
      credit.loadText().then(setText, (e: unknown) => setError(errMsg(e)))
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-4">
        <div className="flex flex-col gap-0.5">
          <span className="text-fg text-sm font-medium">{credit.name}</span>
          <span className="text-fg-muted text-xs">
            {credit.role} · {credit.licence}
          </span>
        </div>
        <TextLink
          size="xs"
          aria-expanded={open}
          aria-controls={panelId}
          aria-label={`${open ? 'Hide' : 'Show'} licence, ${credit.name}`}
          onClick={toggle}
        >
          {open ? 'Hide licence' : 'Show licence'}
        </TextLink>
      </div>
      <div id={panelId} hidden={!open}>
        {open && (
          <pre className="bg-sunken border-line text-fg-muted text-2xs m-0 max-h-80 overflow-auto rounded-md border p-4 font-mono leading-normal whitespace-pre-wrap select-text">
            {error ?? text ?? 'Reading the licence.'}
          </pre>
        )}
      </div>
    </div>
  )
}

/** A link that opens in the system browser, through Go's http(s)-only check. */
function LinkButton({ url, children }: { url: string; children: string }) {
  return (
    <TextLink
      size="xs"
      className="w-fit"
      onClick={() => OpenExternal(url).catch((e) => console.warn('open link', errMsg(e)))}
    >
      {children}
    </TextLink>
  )
}
