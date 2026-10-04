import type { Chapter, ChapterSettings, ChapterSettingsInfo } from '../../types'
import {
  MEMORY_STEP_MB,
  MIN_MEMORY_MB,
  memoryLabel,
  presetLabel,
  sliderMaxMb,
} from '../../lib/chapterSettings'
import { Button } from '../ui/Button'
import { ChoiceCards } from '../ui/ChoiceCards'
import { ErrorLine, Hint, WarningLine } from '../ui/Notes'
import { RangeField } from '../ui/RangeField'
import { SubHeading } from '../ui/Section'
import { StableLabel } from '../ui/StableLabel'
import { RunningHint } from './RunningHint'

interface Props {
  chapter: Chapter
  /** What Go holds: the machine's memory, Prism's default, the presets. */
  info: ChapterSettingsInfo
  draft: ChapterSettings
  onDraft: (draft: ChapterSettings) => void
  dirty: boolean
  saving: boolean
  /** The game tracker's answer: the one thing that holds Save back. */
  playing: boolean
  error: string | null
  onSave: () => void
}

const SAVE_LABELS = ['Save', 'Saving'] as const

/** The memory line: what the slider is bounded by, and what Prism and the pack would pick. */
function memoryHint(info: ChapterSettingsInfo): string {
  const machine = info.machineMemoryMb > 0 ? memoryLabel(info.machineMemoryMb) : 'an unknown amount'
  const pack =
    info.packMemoryMb > 0 ? `; the pack recommends ${memoryLabel(info.packMemoryMb)}` : ''
  return `The most the game may take, of ${machine} on this machine. Prism would pick ${memoryLabel(info.prismDefaultMb)}${pack}.`
}

/**
 * The editable part of a chapter's settings: memory, the Java preset, and Save
 * with whatever it has to say. The blocks are ten steps apart, what is in a
 * block three; Save sits under the last block at five, a step of its own. The
 * status beside Save wraps rather than overflows, and the button keeps its box
 * when it says Saving.
 */
export function ChapterSettingsForm({
  chapter,
  info,
  draft,
  onDraft,
  dirty,
  saving,
  playing,
  error,
  onSave,
}: Props) {
  const max = sliderMaxMb(info.machineMemoryMb)
  return (
    <>
      <RangeField
        label="Memory"
        valueLabel={memoryLabel(draft.maxMemoryMb)}
        min={MIN_MEMORY_MB}
        max={max}
        step={MEMORY_STEP_MB}
        value={Math.min(max, draft.maxMemoryMb)}
        onChange={(maxMemoryMb) => onDraft({ ...draft, maxMemoryMb })}
        hint={memoryHint(info)}
      />

      <div className="flex flex-col gap-5">
        <div className="flex flex-col gap-3">
          <SubHeading>Java arguments</SubHeading>
          <ChoiceCards
            label="Java arguments"
            value={draft.jvm}
            choices={['', ...info.presets].map((name) => ({
              value: name,
              title: presetLabel(name),
            }))}
            onChange={(jvm) => onDraft({ ...draft, jvm })}
          />
          <Hint>
            A preset is a fixed set of arguments the launcher knows; nothing typed here reaches the
            game.
          </Hint>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={onSave} disabled={!dirty || saving || playing}>
            <StableLabel current={saving ? 'Saving' : 'Save'} labels={SAVE_LABELS} />
          </Button>
          {playing && (
            <WarningLine>
              {chapter.name} looks to be running. Close the game to change these.
            </WarningLine>
          )}
          {!playing && info.running && <RunningHint chapterName={chapter.name} />}
          {error && <ErrorLine>{error}</ErrorLine>}
        </div>
      </div>
    </>
  )
}
