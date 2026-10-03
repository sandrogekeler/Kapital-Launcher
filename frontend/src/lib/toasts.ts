import type { GameState } from '../types'
import { gameLine } from './gameLine'

/** One notice in the launcher's bottom-right corner. */
export interface Toast {
  /** What the notice is about, stable across its updates: `engine`, `stop:<chapter>`, `game:<chapter>`. */
  id: string
  /**
   * Which occurrence this is. A dismissed notice stays away while its stamp is
   * the same and comes back when the source has something new to say.
   */
  stamp: string
  /** The state in one line, as the state line writes it. */
  title: string
  /** The sentence under it; the error's own words for a refusal. */
  detail: string
  /** `danger` for an error, `muted` for a stop the player asked for. */
  tone: 'danger' | 'muted'
  /** The chapter whose run report Details opens; absent when there is none to read. */
  reportFor?: string
}

export interface ToastSources {
  /** The engine store's error: a launch, an install or a refresh the backend refused. */
  engineError: string | null
  /** Why each chapter's last Stop was refused. */
  stopErrors: Record<string, string>
  /** Where each chapter's game is. */
  games: Record<string, GameState>
  /** A chapter's name by id, for the game's own line. */
  chapterName: (chapterId: string) => string
}

/** The phases in which a run has ended and has something to say about it. */
const ENDED: readonly GameState['phase'][] = ['crashed', 'failed']

/**
 * The notices the launcher shows, derived from the stores and kept nowhere:
 * what a store already records as an error is the source, and a notice goes
 * when its source does. Nothing in the chapter card moves for one.
 *
 * Three kinds: the backend refusing a launch or an install (the engine
 * store's error), a Stop it refused, and a run that crashed or never started,
 * which keeps its notice until the next Play, with Details to the run report.
 * A run the player stopped is confirmed in the muted tone, with nothing to
 * explain.
 */
export function errorToasts({
  engineError,
  stopErrors,
  games,
  chapterName,
}: ToastSources): Toast[] {
  const toasts: Toast[] = []
  if (engineError) {
    toasts.push({
      id: 'engine',
      stamp: engineError,
      title: '○ Something went wrong',
      detail: engineError,
      tone: 'danger',
    })
  }
  for (const [chapterId, error] of Object.entries(stopErrors)) {
    toasts.push({
      id: `stop:${chapterId}`,
      stamp: error,
      title: '○ The game could not be stopped',
      detail: error,
      tone: 'danger',
    })
  }
  for (const [chapterId, game] of Object.entries(games)) {
    if (!ENDED.includes(game.phase)) continue
    const rows = gameLine(game, chapterName(chapterId))
    if (!rows) continue
    const stopped = game.reason === 'stopped'
    toasts.push({
      id: `game:${chapterId}`,
      stamp: `${game.phase}:${game.since}`,
      title: rows[0],
      detail: rows[1],
      tone: stopped ? 'muted' : 'danger',
      ...(stopped ? {} : { reportFor: chapterId }),
    })
  }
  return toasts
}
