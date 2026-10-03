import { Hint } from '../ui/Notes'

interface Props {
  chapterName: string
}

/**
 * What the settings say when only the guess thinks the game is running: its
 * log changed in the last minute (ChapterSettingsInfo.running). A guess can be
 * wrong either way, so this is a hint and never a reason to disable a button;
 * Go checks again at the write and says why it refused.
 */
export function RunningHint({ chapterName }: Props) {
  return <Hint>{chapterName} may be running: its game log changed in the last minute.</Hint>
}
