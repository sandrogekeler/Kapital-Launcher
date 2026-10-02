/**
 * Where a chapter's pack comes from, for the state line (#41). A developer can
 * point a chapter at a local `packwiz serve` in settings.json; an instance
 * installed that way keeps syncing from it until the player switches it back in
 * the chapter's settings: the launcher touches an instance in one key only, the
 * pack address in its pre-launch command (ADR-2, seventh amendment). Go
 * validates the addresses; these helpers only describe them.
 */

const LOOPBACK = ['localhost', '127.0.0.1', '[::1]']

/** A pack URL parsed, or null when it is not one. */
const parse = (url: string): URL | null => (URL.canParse(url) ? new URL(url) : null)

/** host:port of a pack URL, or the URL itself when it cannot be parsed. */
export const packHost = (url: string): string => parse(url)?.host ?? url

/** Whether a pack URL is on this machine: a local packwiz serve. */
export const isLocalPack = (url: string): boolean => LOOPBACK.includes(parse(url)?.hostname ?? '')

/**
 * The state line for an instance that syncs from somewhere other than the
 * manifest's pack, or null when it syncs from the manifest's pack or the
 * launcher cannot tell (an instance it did not create).
 */
export function packSourceLine(
  instancePack: string | undefined,
  manifestPack: string | null | undefined,
): [string, string] | null {
  if (!instancePack || instancePack === manifestPack) return null
  const where = `Syncs from ${packHost(instancePack)}`
  return isLocalPack(instancePack) ? ['● Dev pack', where] : ['● Other pack', where]
}
