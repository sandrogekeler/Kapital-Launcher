import { Suspense, lazy, useState } from 'react'
import type { AppSettings, Chapter } from '../../types'
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
import { FolderOpen } from '../../lib/icons'
import { ChoosePrismExecutable, ChoosePrismRoot } from '../../../wailsjs/go/main/App'
import { AboutSection } from './AboutSection'
import { Button } from '../ui/Button'
import { Hint } from '../ui/Notes'
import { Icon } from '../ui/Icon'
import { Page } from '../ui/Page'
import { Section, SubHeading } from '../ui/Section'
import { Segmented } from '../ui/Segmented'
import { TextField } from '../ui/TextField'
import { Toggle } from '../ui/Toggle'
import { SupportSection } from './SupportSection'

// The previews are a developer's tool, so the section loads when the settings
// screen shows it and stays out of the launcher's bundle budget
// (scripts/check-bundle-size.mjs), as the pack source section does.
const PreviewSection = lazy(() =>
  import('./PreviewSection').then((m) => ({ default: m.PreviewSection })),
)

interface Props {
  onClose: () => void
  /** Brings a chapter up on its main view, closing every page over it. */
  onShowChapter: (chapterId: string) => void
}

/** A field name, for filing its error; the pack fields are keyed by chapter. */
type Field = keyof AppSettings | `pack:${string}`

/** What each section needs of the screen: the settings, a save and the errors it filed. */
interface SectionProps {
  settings: AppSettings
  errors: Partial<Record<Field, string>>
  save: (field: Field, patch: Partial<AppSettings>) => Promise<void>
}

/**
 * The settings screen (#5). It takes the chapter card's place while open, in
 * the same frame; the sidebar stays. Each field saves the moment it is
 * committed: a theme on click, a path or the profile on Enter, blur or a
 * Browse pick. A rejection from Go shows under that field, and the store has
 * already put the old value back.
 *
 * An empty path field shows what detection resolved, so "empty means default"
 * is something the player can see. Browse asks Go for a native picker and
 * commits the pick through the same save as typing, so every path is validated
 * once, in Go, whichever way it arrived.
 */
export function SettingsPanel({ onClose, onShowChapter }: Props) {
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)
  const loadEngine = useEngineStore((s) => s.load)
  const chapters = useChapterStore((s) => s.manifest.chapters)
  // The page reveals once Go has said what the settings are.
  const loaded = useSettingsStore((s) => s.loaded)
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({})

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
  const fail = (field: Field, e: unknown) => setErrors((prev) => ({ ...prev, [field]: errMsg(e) }))
  const common = { settings, errors, save }

  return (
    <Page label="Settings" title="Settings" onBack={onClose} ready={loaded}>
      <PrismSection {...common} onError={fail} />
      <AppearanceSection {...common} />
      <DeveloperSection {...common} chapters={chapters} onShowChapter={onShowChapter} />
      <SupportSection />
      <AboutSection />
    </Page>
  )
}

function PrismSection({
  settings,
  errors,
  save,
  onError,
}: SectionProps & { onError: (field: Field, e: unknown) => void }) {
  const engine = useEngineStore((s) => s.engine)
  const instances = useEngineStore((s) => s.instances)

  // A native picker needs a window; the browser-only preview has none to
  // show, so Browse does nothing there. A cancelled dialog returns "".
  const browse = async (field: 'prismExecutable' | 'prismRoot', pick: () => Promise<string>) => {
    if (!hasWailsBridge()) return
    let picked: string
    try {
      picked = await pick()
    } catch (e) {
      onError(field, e)
      return
    }
    if (picked) await save(field, { [field]: picked })
  }

  return (
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
  )
}

function AppearanceSection({ settings, errors, save }: SectionProps) {
  return (
    <Section title="Appearance">
      <Segmented
        label="Theme"
        value={settings.theme}
        options={THEME_OPTIONS}
        error={errors.theme}
        onChange={(theme) => void save('theme', { theme })}
      />
      <Toggle
        label="Picture slideshow"
        checked={!(settings.staticArt ?? false)}
        hint="Cycles each chapter's pictures from the wiki, with a post about what they show, on every switch and every minute. Off shows each chapter's own picture."
        error={errors.staticArt}
        onChange={(on) => void save('staticArt', { staticArt: !on })}
      />
      {/* Go says where the splash can run at all (#43); elsewhere there is
          nothing to choose. The effective value is derived, so the save
          carries it along for the switch to follow at once. */}
      {settings.loadingSplashAvailable && (
        <Toggle
          label="Loading splash"
          checked={settings.loadingSplashOn ?? false}
          hint="Shows a small loading card from Play until the game's own loading screen."
          error={errors.loadingSplash}
          onChange={(on) => void save('loadingSplash', { loadingSplash: on, loadingSplashOn: on })}
        />
      )}
    </Section>
  )
}

function DeveloperSection({
  settings,
  errors,
  save,
  chapters,
  onShowChapter,
}: SectionProps & { chapters: readonly Chapter[]; onShowChapter: (chapterId: string) => void }) {
  return (
    <Section title="Developer" collapsible>
      <Hint>
        A chapter can be installed from a packwiz serve running on this machine instead of its
        published pack. Loopback addresses only; the field is empty for a normal install. An
        installed chapter is switched between its packs in its own settings (the pen in the hero).
      </Hint>
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
      {/* A preview shows its screen, which is not this one: the chapter is
          opened and every page over it closes (issue 124). */}
      <Suspense fallback={<PreviewFallback />}>
        <PreviewSection onStarted={onShowChapter} />
      </Suspense>
    </Section>
  )
}

/**
 * What the preview section's place holds while it loads: its heading and about
 * the room it takes, so Support and About do not jump when it arrives. The
 * loaded section keeps the same minimum.
 */
function PreviewFallback() {
  return (
    <div className="flex min-h-128 flex-col gap-3">
      <SubHeading>Preview</SubHeading>
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
