import { create } from 'zustand'
import type { PlayerProfile, PlayerStatus } from '../types'
import { hasWailsBridge, readOr } from '../lib/ipc'
import { CopyPlayerUUID, GetPlayerProfile } from '../../wailsjs/go/main/App'

/**
 * What Mojang's public profile lookup says about the account page's profile
 * name (ADR-13, issue 193): the face, the UUID, and whether the name is a
 * profile at all. Go does the asking, caches for a day and answers every
 * outcome as a status, so a read never fails here: with no bridge, or on any
 * rejection, the answer is `unknown` and the page shows initials and says
 * nothing. Only the lazy account page imports this store and it empties it on
 * the way out.
 */
interface PlayerStore {
  /** The last answer; null before one, while a check is on its way and for an empty name. */
  profile: PlayerProfile | null
  /** Asks Go about `name` (the saved profile name, never a draft). An empty name asks nothing. */
  check: (name: string) => Promise<void>
  /** Puts the found profile's UUID on the clipboard, through Go. Rejects when it cannot. */
  copyUuid: () => Promise<void>
  clear: () => void
}

const UNKNOWN: PlayerProfile = { name: '', uuid: '', faceSrc: '', status: 'unknown' }
const STATUSES: readonly string[] = [
  'found',
  'not_found',
  'unknown',
  'invalid',
] satisfies PlayerStatus[]

/** Go's status is a plain string in the generated class; anything it does not list is unknown. */
function fromGo(p: PlayerProfile | { status: string } | undefined | null): PlayerProfile {
  if (!p || !('name' in p) || !STATUSES.includes(p.status)) return UNKNOWN
  return { name: p.name, uuid: p.uuid, faceSrc: p.faceSrc, status: p.status as PlayerStatus }
}

// An answer that arrives after the name changed or the page left is dropped.
let epoch = 0

export const usePlayerStore = create<PlayerStore>((set) => ({
  profile: null,

  check: async (name) => {
    const mine = ++epoch
    set({ profile: null })
    if (name.trim() === '') return
    const profile = await readOr(() => GetPlayerProfile(name.trim()), UNKNOWN)
    if (mine === epoch) set({ profile: fromGo(profile) })
  },

  // The clipboard is Go's. With no bridge there is none, and saying so beats a
  // TypeError from the binding.
  copyUuid: async () => {
    if (!hasWailsBridge()) throw new Error('The UUID can only be copied from the app window.')
    await CopyPlayerUUID()
  },

  clear: () => {
    epoch++
    set({ profile: null })
  },
}))
