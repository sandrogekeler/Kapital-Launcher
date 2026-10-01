import { useEffect, useId, useState } from 'react'
import { GetAppVersion, OpenExternal } from '../../../wailsjs/go/main/App'
import { DISCLAIMER } from '../../lib/disclaimer'
import { errMsg, readOr } from '../../lib/ipc'
import { CREDITS, PRISM_SOURCE } from '../../lib/licences'
import type { Credit } from '../../lib/licences'
import { Fact } from '../ui/Fact'

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
    <div className="flex flex-col gap-5">
      <h2 className="text-fg-faint m-0 text-xs font-medium">About</h2>
      <Fact label="Version" value={version || 'unknown'} />
      <p className="text-fg-faint m-0 text-xs leading-normal select-text">{DISCLAIMER}</p>

      <ul className="m-0 flex list-none flex-col gap-3 p-0">
        {CREDITS.map((c) => (
          <CreditRow key={c.id} credit={c} />
        ))}
      </ul>

      <div className="flex flex-col gap-1.5">
        <span className="text-fg text-sm">Prism Launcher</span>
        <p className="text-fg-muted m-0 text-xs leading-normal select-text">
          Prism Launcher is free software under the GPL-3.0 and is not part of Kapital Launcher.
          When you ask for it, Kapital Launcher downloads an unmodified official release from
          Prism's GitHub releases. Its source is at github.com/PrismLauncher/PrismLauncher.
        </p>
        <LinkButton url={PRISM_SOURCE}>Prism Launcher source</LinkButton>
      </div>
    </div>
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
    <li className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-4">
        <div className="flex flex-col">
          <span className="text-fg text-sm">{credit.name}</span>
          <span className="text-fg-muted text-xs">
            {credit.role} · {credit.licence}
          </span>
        </div>
        <button
          type="button"
          aria-expanded={open}
          aria-controls={panelId}
          aria-label={`${open ? 'Hide' : 'Show'} licence, ${credit.name}`}
          onClick={toggle}
          className="text-accent duration-fast ease-standard cursor-pointer text-xs transition-[filter] hover:brightness-(--effect-hover-brightness)"
        >
          {open ? 'Hide licence' : 'Show licence'}
        </button>
      </div>
      <div id={panelId} hidden={!open}>
        {open && (
          <pre className="bg-sunken border-line text-fg-muted text-2xs m-0 max-h-80 overflow-auto rounded-md border p-4 font-mono leading-normal whitespace-pre-wrap select-text">
            {error ?? text ?? 'Loading...'}
          </pre>
        )}
      </div>
    </li>
  )
}

/** A link that opens in the system browser, through Go's http(s)-only check. */
function LinkButton({ url, children }: { url: string; children: string }) {
  return (
    <button
      type="button"
      onClick={() => OpenExternal(url).catch((e) => console.warn('open link', errMsg(e)))}
      className="text-accent duration-fast ease-standard w-fit cursor-pointer text-xs transition-[filter] hover:brightness-(--effect-hover-brightness)"
    >
      {children}
    </button>
  )
}
