import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import type { AppSettings } from '../../types'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import { useSettingsStore } from '../../stores/useSettingsStore'
import { errMsg, hasWailsBridge } from '../../lib/ipc'
import {
  THEME_OPTIONS,
  executableHint,
  executablePlaceholder,
  rootHint,
  rootPlaceholder,
  withPackOverride,
} from '../../lib/settingsView'
import { ArrowLeft, FolderOpen } from '../../lib/icons'
import { ChoosePrismExecutable, ChoosePrismRoot } from '../../../wailsjs/go/main/App'
import { AboutSection } from './AboutSection'
import { Button } from '../ui/Button'
import { CheckboxField } from '../ui/CheckboxField'
import { Icon } from '../ui/Icon'
import { IconButton } from '../ui/IconButton'
import { TextField } from '../ui/TextField'
import { SupportSection } from './SupportSection'

interface Props {
  onClose: () => void
}

/** A field name, for filing its error; the pack fields are keyed by chapter. */
type Field = keyof AppSettings | `pack:${string}`

/**
 * The settings screen (#5). It takes the main column's place while open; the
 * sidebar stays. Each field saves the moment it is committed: a theme on
 * click, a path or the profile on Enter, blur or a Browse pick. A rejection
 * from Go shows under that field, and the store has already put the old value
 * back.
 *
 * An empty path field shows what detection resolved, so "empty means default"
 * is something the player can see. Browse asks Go for a native picker and
 * commits the pick through the same save as typing, so every path is validated
 * once, in Go, whichever way it arrived.
 */
export function SettingsPanel({ onClose }: Props) {
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)
  const engine = useEngineStore((s) => s.engine)
  const instances = useEngineStore((s) => s.instances)
  const loadEngine = useEngineStore((s) => s.load)
  const chapters = useChapterStore((s) => s.manifest.chapters)
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({})

  // Escape closes the panel. A field with an unsaved draft takes the key
  // first and reverts instead (TextField), so one press never loses a save.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  // One save per committed field. Go re-detects Prism when the executable or
  // the root changed (App.SaveSettings), so those two re-read the engine and
  // the instances afterwards; the rest change nothing Go holds in memory.
  const save = async (field: Field, patch: Partial<AppSettings>) => {
    try {
      await update(patch)
      setErrors(({ [field]: _cleared, ...rest }) => rest)
      if (field === 'prismExecutable' || field === 'prismRoot') await loadEngine()
    } catch (e) {
      setErrors((prev) => ({ ...prev, [field]: errMsg(e) }))
    }
  }

  // A native picker needs a window; the browser-only preview has none to
  // show, so Browse does nothing there. A cancelled dialog returns "".
  const browse = async (field: 'prismExecutable' | 'prismRoot', pick: () => Promise<string>) => {
    if (!hasWailsBridge()) return
    let picked: string
    try {
      picked = await pick()
    } catch (e) {
      setErrors((prev) => ({ ...prev, [field]: errMsg(e) }))
      return
    }
    if (picked) await save(field, { [field]: picked })
  }

  return (
    <section aria-label="Settings" className="flex flex-col">
      <div className="border-line flex items-center gap-3 border-b px-14 py-5">
        <IconButton icon={ArrowLeft} title="Back" onClick={onClose} />
        <h1 className="font-display m-0 text-2xl font-semibold">Settings</h1>
      </div>

      {/* The bottom padding keeps the last field clear of the fixed disclaimer. */}
      <div className="flex max-w-200 flex-col gap-10 px-14 pt-8 pb-16">
        <Section title="Prism Launcher">
          <TextField
            label="Prism program"
            value={settings.prismExecutable}
            placeholder={executablePlaceholder(engine)}
            hint={executableHint(settings, engine)}
            error={errors.prismExecutable}
            mono
            onCommit={(v) => void save('prismExecutable', { prismExecutable: v })}
            trailing={
              <BrowseButton onClick={() => void browse('prismExecutable', ChoosePrismExecutable)} />
            }
          />
          <TextField
            label="Prism data folder"
            value={settings.prismRoot}
            placeholder={rootPlaceholder(instances)}
            hint={rootHint(settings, engine)}
            error={errors.prismRoot}
            mono
            onCommit={(v) => void save('prismRoot', { prismRoot: v })}
            trailing={<BrowseButton onClick={() => void browse('prismRoot', ChoosePrismRoot)} />}
          />
          <TextField
            label="Profile name"
            value={settings.profileName}
            placeholder="Prism's default account"
            hint="The profile name of a Microsoft account signed in to Prism. Kapital Launcher only passes the name on and never sees the account."
            error={errors.profileName}
            onCommit={(v) => void save('profileName', { profileName: v })}
          />
        </Section>

        <Section title="Appearance">
          <div className="flex flex-col gap-1.5">
            <span className="text-fg-muted text-sm">Theme</span>
            <div
              role="group"
              aria-label="Theme"
              className="bg-sunken border-line-strong inline-flex w-fit rounded-md border p-0.5"
            >
              {THEME_OPTIONS.map((o) => {
                const selected = settings.theme === o.value
                return (
                  <button
                    key={o.value}
                    type="button"
                    aria-pressed={selected}
                    onClick={() => void save('theme', { theme: o.value })}
                    className={`duration-fast ease-standard h-9 cursor-pointer rounded-sm px-4 text-sm transition-colors ${
                      selected ? 'bg-raised text-fg' : 'text-fg-muted hover:text-fg'
                    }`}
                  >
                    {o.label}
                  </button>
                )
              })}
            </div>
            {errors.theme && (
              <span className="text-danger text-xs select-text">{errors.theme}</span>
            )}
          </div>
        </Section>

        <Section title="Developer">
          <p className="text-fg-faint m-0 text-xs leading-normal">
            A chapter can be installed from a packwiz serve running on this machine instead of its
            published pack. Loopback addresses only; the field is empty for a normal install.
          </p>
          {chapters.map((c) => (
            <TextField
              key={c.id}
              label={`Local pack for ${c.name}`}
              value={settings.packOverrides?.[c.id] ?? ''}
              placeholder="http://localhost:8080/pack.toml"
              error={errors[`pack:${c.id}`]}
              mono
              onCommit={(v) =>
                void save(`pack:${c.id}`, {
                  packOverrides: withPackOverride(settings.packOverrides, c.id, v),
                })
              }
            />
          ))}
          <CheckboxField
            label="Keep the game window hidden until it is ready"
            checked={settings.holdGameWindow ?? false}
            hint="A test for the loading splash (#43). Without it you see nothing until the game is ready."
            error={errors.holdGameWindow}
            onChange={(v) => void save('holdGameWindow', { holdGameWindow: v })}
          />
        </Section>
        <SupportSection />

        <AboutSection />
      </div>
    </section>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-5">
      <h2 className="text-fg-faint m-0 text-xs font-medium">{title}</h2>
      {children}
    </div>
  )
}

function BrowseButton({ onClick }: { onClick: () => void }) {
  return (
    <Button onClick={onClick}>
      <Icon icon={FolderOpen} size="sm" />
      <span>Browse</span>
    </Button>
  )
}
