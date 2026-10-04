import type { GameFailReason, GameState } from '../types'

/**
 * What a failed start says, in one sentence: the row sits beside Play and
 * must leave room for the server's status. The details are the run report's
 * (Details beside Play, the log on the card), so no sentence points at them.
 */
function failedDetail(reason: GameFailReason | undefined): string {
  switch (reason) {
    case 'packsync':
      return 'The pack could not be synced'
    case 'launch':
      return 'Prism stopped before the game'
    default:
      return 'Prism gave up or never started it'
  }
}

// A stop the player asked for is not an error, so it is not in the error tone.
const STOPPED_START: [string, string, string] = [
  '○ The game did not start',
  'Stopped from the launcher',
  'text-fg-muted',
]
const STOPPED_GAME: [string, string, string] = [
  '○ The game was stopped',
  'Stopped from the launcher',
  'text-fg-muted',
]

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
      return ['◐ Starting', `Getting ${chapterName} ready`, 'text-accent']
    case 'mods':
      return ['◐ Loading mods', 'The game has started', 'text-accent']
    case 'window':
      return ['◐ Loading mods', 'The game window is running', 'text-accent']
    case 'resources':
      return ['◐ Loading resources', 'Almost there', 'text-accent']
    case 'running':
      return ['● Playing', `${chapterName} is running`, 'text-accent']
    case 'stopping':
      return ['◐ Closing', 'Saving and shutting down', 'text-accent']
    case 'crashed':
      if (state.reason === 'stopped') return STOPPED_GAME
      return [
        '○ The game stopped',
        state.exitCode == null
          ? 'It closed before it finished'
          : `It closed before it finished, exit code ${state.exitCode}`,
        'text-danger',
      ]
    case 'failed':
      if (state.reason === 'stopped') return STOPPED_START
      return ['○ The game did not start', failedDetail(state.reason), 'text-danger']
    default:
      return null
  }
}
