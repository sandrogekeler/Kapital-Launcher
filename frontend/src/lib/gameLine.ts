import type { GameState } from '../types'

/**
 * The state line's rows for a game in progress or just ended: the state, the
 * detail that goes with it and its tone. Null when there is nothing to say,
 * which is a chapter never launched or one whose game closed normally, and
 * the bar's usual line returns.
 */
export function gameLine(
  state: GameState | undefined,
  chapterName: string,
): [string, string, string] | null {
  switch (state?.phase) {
    case 'starting':
      return ['◐ Starting', `Prism is getting ${chapterName} ready`, 'text-accent']
    case 'mods':
      return ['◐ Loading mods', 'The game has started', 'text-accent']
    case 'window':
      return ['◐ Loading mods', 'The game window is up', 'text-accent']
    case 'resources':
      return ['◐ Loading resources', 'Almost there', 'text-accent']
    case 'running':
      return ['● Playing', `${chapterName} is running`, 'text-accent']
    case 'stopping':
      return ['◐ Closing', 'Saving and shutting down', 'text-accent']
    case 'crashed':
      return [
        '○ The game stopped',
        state.exitCode == null
          ? 'It closed before it finished'
          : `It closed before it finished, exit code ${state.exitCode}`,
        'text-danger',
      ]
    case 'failed':
      return ['○ The game did not start', 'Prism gave up or never started it', 'text-danger']
    default:
      return null
  }
}
