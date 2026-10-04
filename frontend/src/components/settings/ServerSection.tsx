import type { AppSettings, Chapter } from '../../types'
import { chosenLabel } from '../../lib/manifest'
import { Hint } from '../ui/Notes'
import { Section } from '../ui/Section'
import { Segmented } from '../ui/Segmented'

interface Props {
  settings: AppSettings
  chapters: readonly Chapter[]
  /** What each save was refused with, filed under `server:<chapter id>`. */
  errors: Partial<Record<string, string>>
  save: (field: `server:${string}`, patch: Partial<AppSettings>) => Promise<void>
}

/**
 * The Server section of the settings screen (issue 151): for each chapter whose
 * server has more than one address, a choice between them by label. The choice
 * is saved as that label, and Go checks it against the chapter's own list; it
 * is the address the launcher pings and, for a chapter that joins on Play,
 * joins. A chapter with one address has nothing to choose, so with none that
 * has two the section is not there at all.
 */
export function ServerSection({ settings, chapters, errors, save }: Props) {
  const choosable = chapters.filter((c) => (c.server?.addresses.length ?? 0) > 1)
  if (choosable.length === 0) return null
  return (
    <Section title="Server">
      <Hint>The launcher checks and joins the chosen address.</Hint>
      {choosable.map((c) => {
        const choice = settings.serverChoices?.[c.id]
        return (
          <Segmented
            key={c.id}
            label={`Server for ${c.name}`}
            value={chosenLabel(c.server, choice) ?? ''}
            options={(c.server?.addresses ?? []).map((a) => ({ value: a.label, label: a.label }))}
            error={errors[`server:${c.id}`]}
            onChange={(label) =>
              void save(`server:${c.id}`, {
                serverChoices: { ...settings.serverChoices, [c.id]: label },
              })
            }
          />
        )
      })}
    </Section>
  )
}
