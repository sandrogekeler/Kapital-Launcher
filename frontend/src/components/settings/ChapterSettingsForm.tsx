import type { Chapter, ChapterSettings, ChapterSettingsInfo } from '../../types'
import {
  MEMORY_STEP_MB,
  MIN_MEMORY_MB,
  memoryLabel,
  presetLabel,
  presetNote,
  sliderMaxMb,
} from '../../lib/chapterSettings'
import { Button } from '../ui/Button'
import { ChoiceCards } from '../ui/ChoiceCards'
import { ErrorLine, WarningLine } from '../ui/Notes'
import { RangeField } from '../ui/RangeField'
import { SubHeading } from '../ui/Section'
import { SettingsCard } from '../ui/SettingsLayout'
import { StableLabel } from '../ui/StableLabel'
import { JavaArgsList } from './JavaArgsList'
import { RunningHint } from './RunningHint'

interface Props {
  chapter: Chapter
  /** What Go holds: the machine's memory, the pack's recommendation, the presets. */
  info: ChapterSettingsInfo
  draft: ChapterSettings
  onDraft: (draft: ChapterSettings) => void
  dirty: boolean
  saving: boolean
  /** The last save went through and nothing has changed since. */
  saved: boolean
  /** The game tracker's answer: the one thing that holds Save back. */
  playing: boolean
  error: string | null
  onSave: () => void
  /** Puts the draft back to what Go read. */
  onRevert: () => void
}

const SAVE_LABELS = ['Save', 'Saving'] as const

/**
 * The editable part of a chapter's settings, one row each of the Game card (issues 188, 189):
 * memory with the pack's recommendation marked on the track, the Java preset and the player's own
 * arguments after it (issue 191), and a footer that
 * says whether there is anything to save, with Revert and Save. The button keeps its box when it
 * says Saving.
 */
export function ChapterSettingsForm({
  chapter,
  info,
  draft,
  onDraft,
  dirty,
  saving,
  saved,
  playing,
  error,
  onSave,
  onRevert,
}: Props) {
  const max = sliderMaxMb(info.machineMemoryMb)
  const state = dirty ? 'Unsaved changes' : saved ? 'Saved' : 'No changes to save'
  return (
    <SettingsCard>
      <RangeField
        label="Memory"
        valueLabel={memoryLabel(draft.maxMemoryMb)}
        valueNote={info.machineMemoryMb > 0 ? `of ${memoryLabel(info.machineMemoryMb)}` : undefined}
        min={MIN_MEMORY_MB}
        max={max}
        step={MEMORY_STEP_MB}
        value={Math.min(max, draft.maxMemoryMb)}
        onChange={(maxMemoryMb) => onDraft({ ...draft, maxMemoryMb })}
        mark={
          info.packMemoryMb > 0
            ? {
                value: info.packMemoryMb,
                label: 'Recommended',
                valueLabel: memoryLabel(info.packMemoryMb),
              }
            : null
        }
      />

      <div className="flex flex-col gap-3">
        <SubHeading>Java arguments</SubHeading>
        <ChoiceCards
          label="Java arguments"
          columns={2}
          value={draft.jvm}
          choices={['', ...info.presets].map((name) => ({
            value: name,
            title: presetLabel(name),
            note: presetNote(name),
          }))}
          onChange={(jvm) => onDraft({ ...draft, jvm })}
        />
        <JavaArgsList
          args={draft.jvmArgs}
          disabled={playing || saving}
          onChange={(jvmArgs) => onDraft({ ...draft, jvmArgs })}
        />
      </div>

      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span role="status" className={`text-xs ${dirty ? 'text-warning' : 'text-fg-muted'}`}>
            {state}
          </span>
          <div className="flex gap-2">
            {dirty && !saving && <Button onClick={onRevert}>Revert</Button>}
            <Button onClick={onSave} disabled={!dirty || saving || playing}>
              <StableLabel current={saving ? 'Saving' : 'Save'} labels={SAVE_LABELS} />
            </Button>
          </div>
        </div>
        {playing && (
          <WarningLine>
            {chapter.name} looks to be running. Close the game to change these.
          </WarningLine>
        )}
        {!playing && info.running && <RunningHint chapterName={chapter.name} />}
        {error && <ErrorLine>{error}</ErrorLine>}
      </div>
    </SettingsCard>
  )
}
